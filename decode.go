package phpserialize

import (
	"encoding"
	"errors"
	"reflect"
	"strconv"
	"strings"
)

const phasePanicMsg = "php serialize decoder out of sync - data changing underfoot?"

func Unmarshal(data []byte, v interface{}) error {

	var d decodeState
	err := checkValid(data, &d.scan)
	if err != nil {
		return err
	}

	d.init(data)
	return d.unmarshal(v)

}

type Unmarshaler interface {
	UnmarshalPHP([]byte) error
}

type UnSerializer interface {
	UnSerializePHP([]byte) error
}

type UnmarshalTypeError struct {
	Value  string
	Type   reflect.Type
	Offset int64
	Struct string
	Field  string
}

func (e *UnmarshalTypeError) Error() string {

	if e.Struct != "" || e.Field != "" {
		return "php serialize: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
	}
	return "php serialize: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()

}

type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {

	if e.Type == nil {
		return "php serialize: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "php serialize: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "php serialize: Unmarshal(nil " + e.Type.String() + ")"

}

type decodeState struct {
	data         []byte
	off          int
	parserState  int
	scan         scanner
	errorContext struct {
		Struct     reflect.Type
		FieldStack []string
	}
	savedError error
}

func (d *decodeState) readIndex() int {
	return d.off - 1
}

func (d *decodeState) init(data []byte) *decodeState {

	d.data = data
	d.off = 0
	d.savedError = nil
	d.errorContext.Struct = nil

	d.errorContext.FieldStack = d.errorContext.FieldStack[:0]
	return d

}

func (d *decodeState) saveError(err error) {

	if d.savedError == nil {
		d.savedError = d.addErrorContext(err)
	}

}

func (d *decodeState) addErrorContext(err error) error {

	if d.errorContext.Struct != nil || len(d.errorContext.FieldStack) > 0 {
		switch err := err.(type) {
		case *UnmarshalTypeError:
			err.Struct = d.errorContext.Struct.Name()
			err.Field = strings.Join(d.errorContext.FieldStack, ".")
			return err
		}
	}
	return err

}

func (d *decodeState) unmarshal(v interface{}) error {

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	d.scan.reset()
	err := d.value(rv)
	if err != nil {
		return d.addErrorContext(err)
	}
	return d.savedError

}

func (d *decodeState) skip() {

	s, data, i := &d.scan, d.data, d.off
	depth := s.parserDepth()
	for {
		op := s.step(data[i])
		i++
		if s.parserDepth() <= depth {
			d.off = i
			d.parserState = op
			return
		}
	}

}

func (d *decodeState) scanNext() int {

	if d.off < len(d.data) {
		d.parserState = d.scan.step(d.data[d.off])
		d.off++
	} else {
		d.parserState = d.scan.eof()
		d.off = len(d.data) + 1
	}
	return d.parserState

}

func (d *decodeState) scanUntil(parserState int) {

	data, i := d.data, d.off
	for i < len(data) {
		newState := d.scanNext()
		if newState == parserState || newState == scanEnd || newState == scanError {
			return
		}
	}

	d.off = len(data) + 1
	d.parserState = d.scan.eof()

}

func (d *decodeState) dataBetween(s1, s2 int) []byte {

	d.scanUntil(s1)
	offset1 := d.readIndex()
	d.scanUntil(s2)
	offset2 := d.readIndex()
	return d.data[offset1:offset2]

}

func (d *decodeState) value(v reflect.Value) error {

	d.scanNext()

	u, ut, pv := indirect(v)
	if u != nil {
		start := d.readIndex()
		d.skip()
		return u.UnmarshalPHP(d.data[start:d.readIndex()])
	}

	if ut != nil {
		start := d.readIndex()
		d.skip()
		return ut.UnmarshalText(d.data[start:d.readIndex()])
	}

	v = pv

	switch d.parserState {
	case scanBeginScalarValue:

		tag := phpValueType(d.data[d.readIndex()])
		var data []byte
		if tag != phpTypeNull {
			data = d.dataBetween(scanInScalarValue, scanEndScalarValue)
		} else {
			d.scanUntil(scanEndScalarValue)
		}

		if v.IsValid() {
			if err := d.scalarValueStore(tag, data, v); err != nil {
				return err
			}
		}

	case scanBeginArray:

		if v.IsValid() {
			if err := d.array(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}

	case scanBeginObject:

		if v.IsValid() {
			if err := d.object(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}

	case scanBeginCustom:

		if v.IsValid() {
			if err := d.custom(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}

	default:
		panic(phasePanicMsg)
	}

	return nil

}

func (d *decodeState) scalarValueStore(tag phpValueType, data []byte, v reflect.Value) error {

	switch tag {
	case phpTypeNull:

		switch v.Kind() {
		case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice:
			v.Set(reflect.Zero(v.Type()))
		}

	case phpTypeBoolean:

		value := data[0] == '1'
		switch v.Kind() {
		case reflect.Bool:
			v.SetBool(value)
		case reflect.Interface:
			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(value))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		default:
			d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
		}

	case phpTypeInteger:

		s := string(data)
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:

			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil || v.OverflowInt(n) {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetInt(n)

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:

			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil || v.OverflowUint(n) {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetUint(n)

		case reflect.Float32, reflect.Float64:

			n, err := strconv.ParseFloat(s, v.Type().Bits())
			if err != nil || v.OverflowFloat(n) {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetFloat(n)

		case reflect.String:

			v.SetString(s)

		case reflect.Interface:

			if v.NumMethod() != 0 {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.Set(reflect.ValueOf(n))

		default:
			d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})

		}

	case phpTypeFloat:

		s := string(data)
		switch v.Kind() {
		case reflect.Float32, reflect.Float64:

			n, err := strconv.ParseFloat(s, v.Type().Bits())
			if err != nil || v.OverflowFloat(n) {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetFloat(n)

		case reflect.String:

			v.SetString(s)

		default:

			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}

			switch v.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				v.SetInt(int64(f))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				v.SetUint(uint64(f))

			case reflect.Interface:

				if v.NumMethod() != 0 {
					d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
					break
				}
				v.Set(reflect.ValueOf(f))

			default:
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})

			}

		}

	case phpTypeString:

		s := string(data)
		d.scanNext() //skip last "

		switch v.Kind() {
		case reflect.Slice:

			if v.Type().Elem().Kind() != reflect.Uint8 {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetBytes(data)

		case reflect.String:

			v.SetString(s)

		case reflect.Interface:

			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(s))
			} else {
				d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})
			}

		default:
			d.saveError(&UnmarshalTypeError{Value: s, Type: v.Type(), Offset: int64(d.readIndex())})

		}

	}

	return nil

}

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func (d *decodeState) array(v reflect.Value) error {

	switch v.Kind() {
	case reflect.Array, reflect.Slice,
		reflect.Map, reflect.Struct:
		break

	case reflect.Interface:

		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(d.arrayInterface()))
			return nil
		}
		fallthrough

	default:
		d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(d.off)})
		d.skip()
		return nil
	}

	d.scanUntil(scanEndKeyValueLength)
	arrayLength := d.scan.lastLength() / 2

	d.scanNext()       //skip {
	defer d.scanNext() //skip }

	if v.Kind() == reflect.Map {
		err := d.mapKv(arrayLength, "", v)
		if err != nil {
			return err
		}
		return nil
	}

	if v.Kind() == reflect.Struct {
		err := d.structKv(arrayLength, "", v)
		if err != nil {
			return err
		}
		return nil
	}

	// grow slice cap if necessary
	if v.Kind() == reflect.Slice {
		if arrayLength >= v.Cap() {
			newCap := arrayLength
			if newCap < 4 {
				newCap = 4
			}
			newV := reflect.MakeSlice(v.Type(), v.Len(), newCap)
			reflect.Copy(newV, v)
			v.Set(newV)
		}
		if arrayLength >= v.Len() {
			v.SetLen(arrayLength)
		}
	}

	for index := 0; index < arrayLength; index++ {

		if index < v.Len() {

			arrayIndex := d.valueInterface()
			if _, ok := arrayIndex.(int64); !ok {
				return errors.New("array index is not integer")
			}

			if err := d.value(v.Index(index)); err != nil {
				return err
			}

		} else {

			//skip array index and value
			if err := d.value(reflect.Value{}); err != nil {
				return err
			}
			if err := d.value(reflect.Value{}); err != nil {
				return err
			}

		}

	}

	if arrayLength < v.Len() {

		if v.Kind() == reflect.Array {
			z := reflect.Zero(v.Type().Elem())
			for ; arrayLength < v.Len(); arrayLength++ {
				v.Index(arrayLength).Set(z)
			}
		} else {
			v.SetLen(arrayLength)
		}

	}

	if arrayLength == 0 && v.Kind() == reflect.Slice {
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	}

	return nil

}

func (d *decodeState) object(v reflect.Value) error {

	switch v.Kind() {
	case reflect.Map, reflect.Struct:
		break

	case reflect.Interface:

		if v.NumMethod() == 0 {
			v.Set(reflect.ValueOf(d.arrayInterface()))
			return nil
		}
		fallthrough

	default:
		d.saveError(&UnmarshalTypeError{Value: "object", Type: v.Type(), Offset: int64(d.off)})
		d.skip()
		return nil
	}

	d.scanUntil(scanInClassName)
	classNameOffset := d.readIndex()
	d.scanUntil(scanEndClassName)

	className := string(d.data[classNameOffset:d.readIndex()])

	d.scanUntil(scanEndKeyValueLength)
	arrayLength := d.scan.lastLength() / 2

	d.scanNext()       //skip {
	defer d.scanNext() //skip }

	if v.Kind() == reflect.Map {
		err := d.mapKv(arrayLength, className, v)
		if err != nil {
			return err
		}
		return nil
	}

	err := d.structKv(arrayLength, className, v)
	if err != nil {
		return err
	}
	return nil

}

var unSerializerType = reflect.TypeOf((*UnSerializer)(nil)).Elem()

func (d *decodeState) custom(v reflect.Value) error {

	t := v.Type()
	if !reflect.PtrTo(t).Implements(unSerializerType) {
		d.saveError(&UnmarshalTypeError{Value: "custom", Type: t, Offset: int64(d.off)})
		d.skip()
		return nil
	}

	var className string
	if t.Implements(phpClassType) {
		phpClass := v.Interface().(PHPClass)
		className = phpClass.GetPHPClassName()
	} else if reflect.PtrTo(t).Implements(phpClassType) {
		phpClass := v.Addr().Interface().(PHPClass)
		className = phpClass.GetPHPClassName()
	}

	d.scanUntil(scanInClassName)
	classNameOffset := d.readIndex()
	d.scanUntil(scanEndClassName)

	decodedClassName := string(d.data[classNameOffset:d.readIndex()])
	if className != "" && className != decodedClassName {
		d.saveError(&UnmarshalTypeError{Value: "class " + decodedClassName, Type: t, Offset: int64(d.off)})
		d.skip()
		return nil
	}

	d.scanUntil(scanEndValueLength)
	dataLength := d.scan.lastLength()

	d.scanNext() //skip {

	offset := d.readIndex()
	d.scanUntil(scanEndCustom)

	data := d.data[offset+1 : d.readIndex()]
	if len(data) != dataLength {
		panic(phasePanicMsg)
	}

	o := v.Addr().Interface().(UnSerializer)
	err := o.UnSerializePHP(data)
	if err != nil {
		d.saveError(&UnmarshalTypeError{Value: string(data), Type: t, Offset: int64(d.off)})
	}
	return nil

}

func (d *decodeState) mapKv(kvLength int, className string, v reflect.Value) error {

	t := v.Type()
	kt := t.Key().Kind()

	switch kt {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
	default:
		if !reflect.PtrTo(t.Key()).Implements(textUnmarshalerType) {
			d.saveError(&UnmarshalTypeError{Value: "map", Type: t, Offset: int64(d.off)})
			d.skip()
			return nil
		}
	}

	if v.IsNil() {
		v.Set(reflect.MakeMap(t))
	}

	var err error
	var mapKey reflect.Value
	var mapElem reflect.Value

	for index := 0; index < kvLength; index++ {

		if !mapKey.IsValid() {
			mapKey = reflect.New(t.Key()).Elem()
		} else {
			mapKey.Set(reflect.Zero(t.Key()))
		}

		err = d.value(mapKey)
		if err != nil {
			return err
		}

		if className != "" &&
			mapKey.Kind() == reflect.String {
			key := mapKey.String()
			if key[0] == 0 {
				if key[1:len(className)+1] == className {
					key = key[len(className)+2:]
					mapKey.SetString(key)
				}
			}
		}

		elemType := t.Elem()
		if elemType.Kind() == reflect.Interface {
			mapElem = reflect.ValueOf(d.valueInterface())
		} else {

			if !mapElem.IsValid() {
				mapElem = reflect.New(elemType).Elem()
			} else {
				mapElem.Set(reflect.Zero(elemType))
			}

			err = d.value(mapElem)
			if err != nil {
				return err
			}

		}

		v.SetMapIndex(mapKey, mapElem)

	}

	return nil

}

func (d *decodeState) structKv(kvLength int, className string, v reflect.Value) error {

	fields := cachedTypeFields(v.Type())

	for index := 0; index < kvLength; index++ {

		var key string
		err := d.value(reflect.ValueOf(&key))
		if err != nil {
			return err
		}

		if key[0] == 0 && className != "" {
			if key[1:len(className)+1] == className {
				key = key[len(className)+2:]
			}
		}

		if i, ok := fields.nameIndex[key]; ok {

			err = d.value(v.Field(i))
			if err != nil {
				return err
			}

		} else {
			d.skip()
		}

	}

	return nil

}

func (d *decodeState) arrayInterface() (val interface{}) {

	d.scanUntil(scanEndKeyValueLength)
	arrayLength := d.scan.lastLength() / 2

	d.scanNext() //skip {
	defer d.scanUntil(scanEndArray)

	if arrayLength > 0 {

		arrayMap := make(map[interface{}]interface{})

		for index := 0; index < arrayLength; index++ {
			key := d.valueInterface()
			value := d.valueInterface()
			arrayMap[key] = value
		}

		//check all the keys in arrayMap
		for k := range arrayMap {
			//key is not int, just return arrayMap
			if _, ok := k.(int64); !ok {
				val = arrayMap
				return
			}
		}

		//else change arrayMap into slice
		array := make([]interface{}, arrayLength)
		for index := int64(0); index < int64(arrayLength); index++ {
			array[index] = arrayMap[index]
		}
		val = array

	} else {
		val = []interface{}{}
	}

	return

}

func (d *decodeState) valueInterface() (val interface{}) {

	d.scanNext()

	switch d.parserState {

	case scanBeginArray, scanBeginObject:

		val = d.arrayInterface()

	case scanBeginScalarValue:

		tag := phpValueType(d.data[d.readIndex()])
		var data []byte
		if tag != phpTypeNull {
			data = d.dataBetween(scanInScalarValue, scanEndScalarValue)
		} else {
			d.scanUntil(scanEndScalarValue)
		}

		val = d.scalarInterface(tag, data)

	default:
		panic(phasePanicMsg)

	}
	return

}

func (d *decodeState) scalarInterface(tag phpValueType, data []byte) interface{} {

	switch tag {
	case phpTypeNull:

		return nil

	case phpTypeBoolean:

		return data[0] == '1'

	case phpTypeInteger:

		s := string(data)
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			d.saveError(err)
		}
		return n

	case phpTypeFloat:

		s := string(data)
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			d.saveError(err)
		}
		return n

	case phpTypeString:

		d.scanNext() //skip the last "
		return string(data)

	}

	return nil

}

func indirect(v reflect.Value) (Unmarshaler, encoding.TextUnmarshaler, reflect.Value) {

	v0 := v
	haveAddr := false

	if v.Kind() != reflect.Ptr && v.Type().Name() != "" && v.CanAddr() {
		haveAddr = true
		v = v.Addr()
	}

	for {

		if v.Kind() == reflect.Interface && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Ptr && !e.IsNil() && e.Elem().Kind() == reflect.Ptr {
				haveAddr = false
				v = e
				continue
			}
		}

		if v.Kind() != reflect.Ptr {
			break
		}

		if v.Elem().Kind() != reflect.Ptr && v.CanSet() {
			break
		}

		if v.Elem().Kind() == reflect.Interface && v.Elem().Elem() == v {
			v = v.Elem()
			break
		}

		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}

		if v.Type().NumMethod() > 0 && v.CanInterface() {
			if u, ok := v.Interface().(Unmarshaler); ok {
				return u, nil, reflect.Value{}
			}
			if u, ok := v.Interface().(encoding.TextUnmarshaler); ok {
				return nil, u, reflect.Value{}
			}
		}

		if haveAddr {
			v = v0
			haveAddr = false
		} else {
			v = v.Elem()
		}

	}
	return nil, nil, v

}
