package phpserialize

import (
	"bytes"
	"encoding"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/pkg/errors"
)

var SerializePrecision = -1

func Marshal(v interface{}) ([]byte, error) {

	e := newEncodeState()

	err := e.marshal(v)
	if err != nil {
		return nil, err
	}
	buf := append([]byte(nil), e.Bytes()...)

	e.Reset()
	encodeStatePool.Put(e)

	return buf, nil

}

type Marshaler interface {
	MarshalPHP() ([]byte, error)
}

type PHPClass interface {
	GetPHPClassName() string
}

type Serializer interface {
	SerializePHP() ([]byte, error)
}

type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "php serialize: unsupported type: " + e.Type.String()
}

type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "php serialize: unsupported value: " + e.Str
}

type MarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *MarshalerError) Error() string {
	return "php serialize: error calling MarshalJSON for type " + e.Type.String() + ": " + e.Err.Error()
}

func (e *MarshalerError) Unwrap() error { return e.Err }

type encodeState struct {
	bytes.Buffer
	scratch [64]byte
}

var encodeStatePool sync.Pool

func newEncodeState() *encodeState {

	if v := encodeStatePool.Get(); v != nil {
		e := v.(*encodeState)
		e.Reset()
		return e
	}
	return new(encodeState)

}

type phpSerializeError struct{ error }

func (e *encodeState) marshal(v interface{}) (err error) {

	defer func() {
		if r := recover(); r != nil {
			if je, ok := r.(phpSerializeError); ok {
				err = je.error
			} else {
				panic(r)
			}
		}
	}()
	e.reflectValue(reflect.ValueOf(v))
	return nil

}

func (e *encodeState) writeTag(tag phpValueType) error {

	err := e.WriteByte(byte(tag))
	if err != nil {
		return errors.Wrap(err, "can not write php type tag")
	}

	if tag == phpTypeNull {
		err = e.WriteByte(phpTerminator)
		if err != nil {
			return errors.Wrap(err, "can not write php type tag terminator")
		}
		return nil
	}

	err = e.WriteByte(phpSeparator)
	if err != nil {
		return errors.Wrap(err, "can not write php type tag separator")
	}
	return nil

}

func (e *encodeState) writeTagAndLength(tag phpValueType, length int) error {

	err := e.writeTag(tag)
	if err != nil {
		return err
	}

	if tag == phpTypeString || tag == phpTypeArray {
		_, err = e.WriteString(strconv.Itoa(length))
		if err != nil {
			return errors.Wrap(err, "can not write php value length")
		}
		err = e.WriteByte(phpSeparator)
		if err != nil {
			return errors.Wrap(err, "can not write php type tag separator")
		}
	}
	return nil

}

func (e *encodeState) error(err error) {
	panic(phpSerializeError{err})
}

func isEmptyValue(v reflect.Value) bool {

	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false

}

func (e *encodeState) reflectValue(v reflect.Value) {
	valueEncoder(v)(e, v)
}

type encoderFunc func(e *encodeState, v reflect.Value)

var encoderCache sync.Map // map[reflect.Type]encoderFunc

func valueEncoder(v reflect.Value) encoderFunc {

	if !v.IsValid() {
		return invalidValueEncoder
	}
	return typeEncoder(v.Type())

}

func typeEncoder(t reflect.Type) encoderFunc {

	if fi, ok := encoderCache.Load(t); ok {
		return fi.(encoderFunc)
	}

	var (
		wg sync.WaitGroup
		f  encoderFunc
	)
	wg.Add(1)
	fi, loaded := encoderCache.LoadOrStore(t, encoderFunc(func(e *encodeState, v reflect.Value) {
		wg.Wait()
		f(e, v)
	}))
	if loaded {
		return fi.(encoderFunc)
	}

	f = newTypeEncoder(t, true)
	wg.Done()
	encoderCache.Store(t, f)
	return f

}

var (
	marshalerType     = reflect.TypeOf((*Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	phpClassType      = reflect.TypeOf((*PHPClass)(nil)).Elem()
	serializerType    = reflect.TypeOf((*Serializer)(nil)).Elem()
)

func newTypeEncoder(t reflect.Type, allowAddr bool) encoderFunc {

	//check Marshaler
	if t.Implements(marshalerType) {
		return marshalerEncoder
	}
	if t.Kind() != reflect.Ptr && allowAddr && reflect.PtrTo(t).Implements(marshalerType) {
		return newCondAddrEncoder(addrMarshalerEncoder, newTypeEncoder(t, false))
	}

	//check encoding.TextMarshaler
	if t.Implements(textMarshalerType) {
		return textMarshalerEncoder
	}
	if t.Kind() != reflect.Ptr && allowAddr && reflect.PtrTo(t).Implements(textMarshalerType) {
		return newCondAddrEncoder(addrTextMarshalerEncoder, newTypeEncoder(t, false))
	}

	//check Serializer
	if t.Implements(serializerType) && t.Implements(phpClassType) {
		return serializerEncoder
	}
	if t.Kind() != reflect.Ptr && allowAddr &&
		reflect.PtrTo(t).Implements(serializerType) && reflect.PtrTo(t).Implements(phpClassType) {
		return newCondAddrEncoder(addrSerializerEncoder, newTypeEncoder(t, false))
	}

	switch t.Kind() {
	case reflect.Bool:
		return boolEncoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintEncoder
	case reflect.Float32:
		return float32Encoder
	case reflect.Float64:
		return float64Encoder
	case reflect.String:
		return stringEncoder
	case reflect.Interface:
		return interfaceEncoder
	case reflect.Struct:
		return newStructEncoder(t)
	case reflect.Map:
		return newMapEncoder(t)
	case reflect.Slice:
		return newSliceEncoder(t)
	case reflect.Array:
		return newArrayEncoder(t)
	case reflect.Ptr:
		return newPtrEncoder(t)
	default:
		return unsupportedTypeEncoder
	}

}

func invalidValueEncoder(e *encodeState, _ reflect.Value) {
	e.WriteString(phpNullValue)
}

func marshalerEncoder(e *encodeState, v reflect.Value) {

	if v.Kind() == reflect.Ptr && v.IsNil() {
		e.WriteString(phpNullValue)
		return
	}
	m, ok := v.Interface().(Marshaler)
	if !ok {
		e.WriteString(phpNullValue)
		return
	}
	b, err := m.MarshalPHP()
	if err == nil {
		_, err = e.Write(b)
	}
	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}

}

func addrMarshalerEncoder(e *encodeState, v reflect.Value) {

	va := v.Addr()
	if va.IsNil() {
		e.WriteString(phpNullValue)
		return
	}
	m := va.Interface().(Marshaler)
	b, err := m.MarshalPHP()
	if err == nil {
		_, err = e.Write(b)
	}
	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}

}

func textMarshalerEncoder(e *encodeState, v reflect.Value) {

	if v.Kind() == reflect.Ptr && v.IsNil() {
		e.WriteString(phpNullValue)
		return
	}
	m := v.Interface().(encoding.TextMarshaler)
	b, err := m.MarshalText()
	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}
	e.Write(b)

}

func addrTextMarshalerEncoder(e *encodeState, v reflect.Value) {

	va := v.Addr()
	if va.IsNil() {
		e.WriteString(phpNullValue)
		return
	}
	m := va.Interface().(encoding.TextMarshaler)
	b, err := m.MarshalText()
	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}
	e.Write(b)

}

func serializerEncoder(e *encodeState, v reflect.Value) {

	if v.Kind() == reflect.Ptr && v.IsNil() {
		e.WriteString(phpNullValue)
		return
	}

	c := v.Interface().(PHPClass)
	className := c.GetPHPClassName()

	m := v.Interface().(Serializer)
	b, err := m.SerializePHP()
	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}

	e.writeTag(phpTypeCustom)
	e.WriteString(strconv.Itoa(len(className)))
	e.WriteByte(phpSeparator)
	e.WriteByte(phpDoubleQuote)
	e.WriteString(className)
	e.WriteByte(phpDoubleQuote)
	e.WriteByte(phpSeparator)
	e.WriteString(strconv.Itoa(len(b)))
	e.WriteByte(phpSeparator)
	e.WriteByte(phpLeftBraces)
	e.Write(b)
	e.WriteByte(phpRightBraces)

}

func addrSerializerEncoder(e *encodeState, v reflect.Value) {

	va := v.Addr()
	if va.IsNil() {
		e.WriteString(phpNullValue)
		return
	}

	c := va.Interface().(PHPClass)
	className := c.GetPHPClassName()

	m := va.Interface().(Serializer)
	b, err := m.SerializePHP()
	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}

	e.writeTag(phpTypeCustom)
	e.WriteString(strconv.Itoa(len(className)))
	e.WriteByte(phpSeparator)
	e.WriteByte(phpDoubleQuote)
	e.WriteString(className)
	e.WriteByte(phpDoubleQuote)
	e.WriteByte(phpSeparator)
	e.WriteString(strconv.Itoa(len(b)))
	e.WriteByte(phpSeparator)
	e.WriteByte(phpLeftBraces)
	e.Write(b)
	e.WriteByte(phpRightBraces)

}

func boolEncoder(e *encodeState, v reflect.Value) {

	e.writeTag(phpTypeBoolean)
	if v.Bool() {
		e.WriteByte('1')
	} else {
		e.WriteByte('0')
	}
	e.WriteByte(phpTerminator)

}

func intEncoderRaw(e *encodeState, v int) {

	e.writeTag(phpTypeInteger)
	e.Write(strconv.AppendInt(e.scratch[:0], int64(v), 10))
	e.WriteByte(phpTerminator)

}

func intEncoder(e *encodeState, v reflect.Value) {

	e.writeTag(phpTypeInteger)
	e.Write(strconv.AppendInt(e.scratch[:0], v.Int(), 10))
	e.WriteByte(phpTerminator)

}

func uintEncoder(e *encodeState, v reflect.Value) {

	e.writeTag(phpTypeInteger)
	b := strconv.AppendUint(e.scratch[:0], v.Uint(), 10)
	e.Write(b)
	e.WriteByte(phpTerminator)

}

type floatEncoder int // number of bits

func (bits floatEncoder) encode(e *encodeState, v reflect.Value) {

	f := v.Float()
	if math.IsInf(f, 0) || math.IsNaN(f) {
		e.error(&UnsupportedValueError{v, strconv.FormatFloat(f, 'G', SerializePrecision, int(bits))})
	}

	b := e.scratch[:0]
	b = strconv.AppendFloat(b, f, 'G', SerializePrecision, int(bits))
	n := len(b)
	if n >= 4 && b[n-4] == 'E' && b[n-3] == '-' && b[n-2] == '0' {
		b[n-2] = b[n-1]
		b = b[:n-1]
	}

	e.writeTag(phpTypeFloat)
	e.Write(b)
	e.WriteByte(phpTerminator)

}

var (
	float32Encoder = (floatEncoder(32)).encode
	float64Encoder = (floatEncoder(64)).encode
)

func stringEncoderRaw(e *encodeState, v string) {

	e.writeTagAndLength(phpTypeString, len(v))
	e.WriteByte('"')
	e.WriteString(v)
	e.WriteByte('"')
	e.WriteByte(phpTerminator)

}

func stringEncoder(e *encodeState, v reflect.Value) {

	e.writeTagAndLength(phpTypeString, v.Len())
	e.WriteByte('"')
	e.WriteString(v.String())
	e.WriteByte('"')
	e.WriteByte(phpTerminator)

}

func interfaceEncoder(e *encodeState, v reflect.Value) {

	if v.IsNil() {
		e.WriteString(phpNullValue)
		return
	}
	e.reflectValue(v.Elem())

}

func unsupportedTypeEncoder(e *encodeState, v reflect.Value) {
	e.error(&UnsupportedTypeError{v.Type()})
}

type structEncoder struct {
	fields structFields
}

type structFields struct {
	list      []field
	nameIndex map[string]int
}

func (se structEncoder) encode(e *encodeState, v reflect.Value) {

	fieldsCount := 0
	structEncodeState := newEncodeState()

FieldLoop:
	for i := range se.fields.list {
		f := &se.fields.list[i]

		fv := v
		for _, i := range f.index {
			if fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					continue FieldLoop
				}
				fv = fv.Elem()
			}
			fv = fv.Field(i)
		}

		if f.omitEmpty && isEmptyValue(fv) {
			continue
		}

		fieldsCount++

		//write filed name
		stringEncoderRaw(structEncodeState, f.name)
		//write field value
		f.encoder(structEncodeState, fv)

	}

	if v.Type().Implements(phpClassType) {

		phpClass := v.Interface().(PHPClass)
		phpClassName := phpClass.GetPHPClassName()
		e.writeTag(phpTypeObject)
		e.WriteString(strconv.Itoa(len(phpClassName)))
		e.WriteByte(phpSeparator)
		e.WriteByte(phpDoubleQuote)
		e.WriteString(phpClassName)
		e.WriteByte(phpDoubleQuote)
		e.WriteByte(phpSeparator)
		e.WriteString(strconv.Itoa(fieldsCount))
		e.WriteByte(phpSeparator)

	} else {
		e.writeTagAndLength(phpTypeArray, fieldsCount)
	}

	e.WriteByte(phpLeftBraces)
	e.Write(structEncodeState.Bytes())
	e.WriteByte(phpRightBraces)

	structEncodeState.Reset()
	encodeStatePool.Put(structEncodeState)

}

func newStructEncoder(t reflect.Type) encoderFunc {
	se := structEncoder{fields: cachedTypeFields(t)}
	return se.encode
}

type mapEncoder struct {
	elemEnc encoderFunc
}

func (m mapEncoder) encode(e *encodeState, v reflect.Value) {

	if v.IsNil() {
		e.WriteString(phpNullValue)
		return
	}

	e.writeTagAndLength(phpTypeArray, v.Len())
	e.WriteByte(phpLeftBraces)

	// Extract and sort the keys.
	keys := v.MapKeys()
	sv := make([]reflectWithString, len(keys))
	for i, v := range keys {
		sv[i].v = v
		if err := sv[i].resolve(); err != nil {
			e.error(&MarshalerError{v.Type(), err})
		}
	}
	sort.Slice(sv, func(i, j int) bool { return sv[i].s < sv[j].s })

	for _, kv := range sv {
		stringEncoderRaw(e, kv.s)
		m.elemEnc(e, v.MapIndex(kv.v))
	}
	e.WriteByte(phpRightBraces)

}

func newMapEncoder(t reflect.Type) encoderFunc {

	switch t.Key().Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
	default:
		if !t.Key().Implements(textMarshalerType) {
			return unsupportedTypeEncoder
		}
	}
	me := mapEncoder{typeEncoder(t.Elem())}
	return me.encode

}

func encodeByteSlice(e *encodeState, v reflect.Value) {

	if v.IsNil() {
		e.WriteString(phpNullValue)
		return
	}
	e.writeTagAndLength(phpTypeString, v.Len())
	e.WriteByte('"')
	e.Write(v.Bytes())
	e.WriteByte('"')
	e.WriteByte(phpTerminator)

}

type sliceEncoder struct {
	arrayEnc encoderFunc
}

func (se sliceEncoder) encode(e *encodeState, v reflect.Value) {

	if v.IsNil() {
		e.WriteString(phpNullValue)
		return
	}
	se.arrayEnc(e, v)

}

func newSliceEncoder(t reflect.Type) encoderFunc {

	if t.Elem().Kind() == reflect.Uint8 {
		p := reflect.PtrTo(t.Elem())
		if !p.Implements(marshalerType) && !p.Implements(textMarshalerType) {
			return encodeByteSlice
		}
	}
	enc := sliceEncoder{newArrayEncoder(t)}
	return enc.encode

}

type arrayEncoder struct {
	elemEnc encoderFunc
}

func (ae arrayEncoder) encode(e *encodeState, v reflect.Value) {

	n := v.Len()
	e.writeTagAndLength(phpTypeArray, n)
	e.WriteByte(phpLeftBraces)

	for i := 0; i < n; i++ {
		intEncoderRaw(e, i)
		ae.elemEnc(e, v.Index(i))
	}

	e.WriteByte(phpRightBraces)

}

func newArrayEncoder(t reflect.Type) encoderFunc {
	enc := arrayEncoder{typeEncoder(t.Elem())}
	return enc.encode
}

type ptrEncoder struct {
	elemEnc encoderFunc
}

func (pe ptrEncoder) encode(e *encodeState, v reflect.Value) {

	if v.IsNil() {
		e.WriteString(phpNullValue)
		return
	}
	pe.elemEnc(e, v.Elem())

}

func newPtrEncoder(t reflect.Type) encoderFunc {
	enc := ptrEncoder{typeEncoder(t.Elem())}
	return enc.encode
}

type condAddrEncoder struct {
	canAddrEnc, elseEnc encoderFunc
}

func (ce condAddrEncoder) encode(e *encodeState, v reflect.Value) {
	if v.CanAddr() {
		ce.canAddrEnc(e, v)
	} else {
		ce.elseEnc(e, v)
	}
}

func newCondAddrEncoder(canAddrEnc, elseEnc encoderFunc) encoderFunc {
	enc := condAddrEncoder{canAddrEnc: canAddrEnc, elseEnc: elseEnc}
	return enc.encode
}

func isValidTag(s string) bool {

	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("_", c):
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true

}

func typeByIndex(t reflect.Type, index []int) reflect.Type {

	for _, i := range index {
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		t = t.Field(i).Type
	}
	return t

}

type reflectWithString struct {
	v reflect.Value
	s string
}

func (w *reflectWithString) resolve() error {

	if w.v.Kind() == reflect.String {
		w.s = w.v.String()
		return nil
	}
	if tm, ok := w.v.Interface().(encoding.TextMarshaler); ok {
		buf, err := tm.MarshalText()
		w.s = string(buf)
		return err
	}
	switch w.v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.s = strconv.FormatInt(w.v.Int(), 10)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.s = strconv.FormatUint(w.v.Uint(), 10)
		return nil
	}
	panic("unexpected map key type")

}
