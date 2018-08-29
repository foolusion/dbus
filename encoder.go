package dbus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"sync"
)

// Marshall encodes the values into dbus wire format.
func Marshall(vs ...interface{}) ([]byte, error) {
	e := newEncoder()
	if err := e.encode(vs...); err != nil {
		return nil, err
	}
	buf := append([]byte(nil), e.Bytes()...)
	encoderPool.Put(e)
	return buf, nil
}

var encoderPool sync.Pool

// An encoder encodes values to the D-Bus wire format.
type encoder struct {
	bytes.Buffer
}

// NewEncoder returns a new encoder that writes to out in the given
// byte order.
func newEncoder() *encoder {
	return newEncoderAtOffset(0)
}

// newEncoderAtOffset returns a new encoder that writes to out in the given
// byte order. Specify the offset to initialize pos for proper alignment
// computation.
func newEncoderAtOffset(offset int) *encoder {
	var e *encoder
	if v := encoderPool.Get(); v != nil {
		e = v.(*encoder)
		e.Buffer.Reset()
	} else {
		e = new(encoder)
	}
	e.Write(make([]byte, offset))
	return e
}

// align writes padding to the encode buffer up to the next n byte
// alignment. If it is already aligned then nothing happens. panic on
// write error.
func (enc *encoder) align(n int) {
	curOffset := enc.Len() % n
	var padding int
	if curOffset != 0 {
		padding = n - curOffset
	}
	if padding == 0 {
		return
	}
	_, err := enc.Write(make([]byte, padding))
	if err != nil {
		panic(err)
	}
}

// pad returns the number of bytes of padding, based on current
// position and additional offset.  and alignment.
func (enc *encoder) padding(offset, algn int) int {
	abs := enc.Len() + offset
	if abs%algn != 0 {
		newabs := (abs + algn - 1) & ^(algn - 1)
		return newabs - abs
	}
	return 0
}

// Calls binary.Write(enc.out, enc.order, v) and panics on write errors.
func (enc *encoder) binwrite(v interface{}) {
	if err := binary.Write(enc, binary.BigEndian, v); err != nil {
		panic(err)
	}
}

// Encode encodes the given values to the underyling reader. All
// written values are aligned properly as required by the D-Bus spec.
func (enc *encoder) encode(vs ...interface{}) (err error) {
	defer func() {
		err, _ = recover().(error)
	}()
	for _, v := range vs {
		val := reflect.ValueOf(v)
		enc.align(alignment(val.Type()))
		f := getEncoder(val.Type(), 0)
		f(enc, val)
	}
	return nil
}

type encodeFn func(*encoder, reflect.Value) error

// encode encodes the given value to the writer and panics on
// error. depth holds the depth of the container nesting.
func getEncoder(t reflect.Type, depth int) encodeFn {
	switch t.Kind() {
	case reflect.Uint8:
		return encodeByte
	case reflect.Bool:
		return encodeBool
	case reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32,
		reflect.Int, reflect.Uint, reflect.Int64, reflect.Uint64:
		return encodeInt
	case reflect.Float64:
		return encodeFloat
	case reflect.String:
		return getStringEncoder(t)
	case reflect.Ptr:
		return getEncoder(t.Elem(), depth)
	case reflect.Slice, reflect.Array:
		return encodeSlice
	case reflect.Struct:
		return getStructEncoder(t)
	case reflect.Map:
		return encodeMap
	}
	return func(*encoder, reflect.Value) error { return errors.New("not implemented") }
}

func encodeByte(enc *encoder, v reflect.Value) error {
	b := byte(v.Uint())
	if err := enc.WriteByte(b); err != nil {
		return err
	}
	return nil
}

func encodeBool(enc *encoder, v reflect.Value) error {
	if v.Bool() {
		return binary.Write(enc, binary.BigEndian, uint32(1))
	}
	return binary.Write(enc, binary.BigEndian, uint32(0))
}

func encodeInt(enc *encoder, v reflect.Value) error {
	buf := make([]byte, 8)
	var u uint64
	var b int
	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u = v.Uint()
		b = v.Type().Bits()
	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
		u = uint64(v.Int())
		b = v.Type().Bits()
	}
	binary.BigEndian.PutUint64(buf, u)
	sizeBytes := b >> 3
	_, err := enc.Write(buf[8-sizeBytes:])
	return err
}

func encodeFloat(enc *encoder, v reflect.Value) error {
	bits := math.Float64bits(v.Float())
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, bits)
	_, err := enc.Write(buf)
	return err
}

func getStringEncoder(t reflect.Type) encodeFn {
	switch t {
	case signatureType:
		return encodeSignature
	}
	return encodeString
}

func encodeSignature(enc *encoder, v reflect.Value) error {
	enc.WriteByte(byte(v.Len()))
	return encodeStringData(enc, v)
}

func encodeString(enc *encoder, v reflect.Value) error {
	if err := enc.encode(uint32(v.Len())); err != nil {
		return err
	}
	return encodeStringData(enc, v)
}

func encodeStringData(enc *encoder, v reflect.Value) error {
	if _, err := enc.WriteString(v.String()); err != nil {
		return err
	}
	if err := enc.WriteByte(0); err != nil {
		return err
	}
	return nil
}

func encodeSlice(enc *encoder, v reflect.Value) error {
	var temp encoder
	f := getEncoder(v.Type().Elem(), 0)
	for i := 0; i < v.Len(); i++ {
		f(enc, v.Index(i))
	}
	enc.encode(uint32(temp.Len()))
	enc.align(alignment(v.Type().Elem()))
	_, err := enc.Write(temp.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func getStructEncoder(t reflect.Type) encodeFn {
	switch t {
	case signatureType:
		return encodeSignature
	case variantType:
		return encodeVariant
	}
	return encodeStruct
}

func encodeStruct(enc *encoder, v reflect.Value) error {
	for i := 0; i < v.NumField(); i++ {
		if err := enc.encode(v.Field(i)); err != nil {
			return err
		}
	}
	return nil
}

// FIXME: implement this
func encodeVariant(enc *encoder, v reflect.Value) error {
	variant := v.Interface().(Variant)
	enc.encode(variant.sig)
	enc.encode(variant.value)
	return nil
}

// FIXME: implement this.
func encodeMap(enc *encoder, v reflect.Value) error {
	return errors.New("not implemented")
	/*
		if !isKeyType(v.Type().Key()) {
			panic(InvalidTypeError{v.Type()})
		}
		keys := v.MapKeys()
		// Lookahead offset: 4 bytes for uint32 length (with alignment),
		// plus 8-byte alignment
		n := enc.padding(0, 4) + 4
		offset := enc.Len() + n + enc.padding(n, 8)

		bufenc := newEncoderAtOffset(offset)
		for _, k := range keys {
			bufenc.align(8)
			bufenc.encode(k, depth+2)
			bufenc.encode(v.MapIndex(k), depth+2)
		}
		enc.encode(reflect.ValueOf(uint32(bufenc.Len())), depth)
		enc.align(8)
		if _, err := bufenc.WriteTo(enc); err != nil {
			panic(err)
		}
	*/
}
