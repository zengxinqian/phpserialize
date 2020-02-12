package phpserialize

import (
	"bytes"
	"io"

	"github.com/pkg/errors"
)

type Decoder struct {
	r       io.Reader
	buf     []byte
	d       decodeState
	scanp   int   // start of unread data in buf
	scanned int64 // amount of data already scanned
	scan    scanner
	err     error
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

func (dec *Decoder) Decode(v interface{}) error {

	if dec.err != nil {
		return dec.err
	}

	// scan and read whole php serialized data into buffer
	n, err := dec.readValue()
	if err != nil {
		return err
	}
	dec.d.init(dec.buf[dec.scanp : dec.scanp+n])
	dec.scanp += n

	return dec.d.unmarshal(v)

}

func (dec *Decoder) Buffered() io.Reader {
	return bytes.NewReader(dec.buf[dec.scanp:])
}

func (dec *Decoder) readValue() (int, error) {

	dec.scan.reset()

	scanp := dec.scanp
	var err error
Input:

	for scanp >= 0 {

		// scan the buffer for a new value
		for ; scanp < len(dec.buf); scanp++ {
			c := dec.buf[scanp]
			dec.scan.bytes++
			switch dec.scan.step(c) {
			case scanEnd:
				break Input
			case scanError:
				dec.err = dec.scan.err
				return 0, dec.scan.err
			}
		}

		if err != nil {
			if err == io.EOF {
				if dec.d.parserState == scanEnd {
					break Input
				}
				err = io.ErrUnexpectedEOF
			}
			dec.err = err
			return 0, err
		}

		n := scanp - dec.scanp
		err = dec.refill()
		scanp = dec.scanp + n

	}
	return scanp - dec.scanp, nil

}

func (dec *Decoder) refill() error {

	if dec.scanp > 0 {
		dec.scanned += int64(dec.scanp)
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		dec.scanp = 0
	}

	const minRead = 512
	if cap(dec.buf)-len(dec.buf) < minRead {
		newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)
		copy(newBuf, dec.buf)
		dec.buf = newBuf
	}

	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	dec.buf = dec.buf[0 : len(dec.buf)+n]

	return err

}

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (enc *Encoder) Encode(v interface{}) error {

	e := newEncodeState()
	defer encodeStatePool.Put(e)

	err := e.marshal(v)
	if err != nil {
		return err
	}

	b := e.Bytes()
	if _, err = enc.w.Write(b); err != nil {
		return err
	}

	return nil

}

type RawMessage []byte

func (m RawMessage) MarshalPHP() ([]byte, error) {

	if m == nil {
		return []byte(phpNullValue), nil
	}
	return m, nil

}

func (m *RawMessage) UnmarshalPHP(data []byte) error {

	if m == nil {
		return errors.New("phpserialize.RawMessage: UnmarshalPHP on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil

}
