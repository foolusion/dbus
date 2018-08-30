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
	for _, v := range vs {
		e.encode(reflect.ValueOf(v))
		if e.err != nil {
			return nil, e.err
		}
	}
	buf := append([]byte(nil), e.Bytes()...)
	encoderPool.Put(e)
	return buf, nil
}

var encoderPool sync.Pool

// An encoder encodes values to the D-Bus wire format.
type encoder struct {
	bytes.Buffer
	offset int
	err    error
}

func (enc *encoder) totalLen() int {
	return enc.Len() + enc.offset
}

func (enc *encoder) Write(b []byte) {
	if enc.err != nil {
		return
	}
	_, enc.err = enc.Buffer.Write(b)
}

func (enc *encoder) WriteString(s string) {
	if enc.err != nil {
		return
	}
	_, enc.err = enc.Buffer.WriteString(s)
}

func (enc *encoder) WriteByte(b byte) {
	if enc.err != nil {
		return
	}
	enc.err = enc.Buffer.WriteByte(b)
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
		e.err = nil
	} else {
		e = new(encoder)
	}
	e.offset = offset
	return e
}

// align writes padding to the encode buffer up to the next n byte
// alignment. If it is already aligned then nothing happens. panic on
// write error.
func (enc *encoder) align(n int) {
	if enc.err != nil {
		return
	}
	curOffset := enc.totalLen() % n
	var padding int
	if curOffset != 0 {
		padding = n - curOffset
	}
	if padding == 0 {
		return
	}
	enc.Write(make([]byte, padding))
}

// Encode encodes the given values to the underlying reader. All
// written values are aligned properly as required by the D-Bus spec.
func (enc *encoder) encode(v reflect.Value) {
	if enc.err != nil {
		return
	}
	enc.align(alignment(v.Type()))
	f := getEncoder(v.Type(), 0)
	err := f(enc, v)
	if enc.err != nil {
		return
	} else if err != nil {
		enc.err = err
		return
	}
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
	enc.WriteByte(b)
	return nil
}

func encodeBool(enc *encoder, v reflect.Value) error {
	if v.Bool() {
		return binary.Write(&enc.Buffer, binary.BigEndian, uint32(1))
	}
	return binary.Write(&enc.Buffer, binary.BigEndian, uint32(0))
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
	enc.Write(buf[8-sizeBytes:])
	return nil
}

func encodeFloat(enc *encoder, v reflect.Value) error {
	bits := math.Float64bits(v.Float())
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, bits)
	enc.Write(buf)
	return nil
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
	enc.encode(reflect.ValueOf(uint32(v.Len())))
	return encodeStringData(enc, v)
}

func encodeStringData(enc *encoder, v reflect.Value) error {
	enc.WriteString(v.String())
	enc.WriteByte(0)
	return nil
}

func encodeSlice(enc *encoder, v reflect.Value) error {
	temp := newEncoderAtOffset(enc.totalLen() + 4)
	for i := 0; i < v.Len(); i++ {
		temp.encode(v.Index(i))
	}
	enc.encode(reflect.ValueOf(uint32(temp.Len())))
	enc.Write(temp.Bytes())
	encoderPool.Put(temp)
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
		enc.encode(v.Field(i))
	}
	return nil
}

func encodeVariant(enc *encoder, v reflect.Value) error {
	variant := v.Interface().(Variant)
	enc.encode(reflect.ValueOf(variant.sig))
	enc.encode(reflect.ValueOf(variant.value))
	return nil
}

func encodeMap(enc *encoder, v reflect.Value) error {
	tempEnc := newEncoder()
	for _, k := range v.MapKeys() {
		kv := v.MapIndex(k)
		tempEnc.align(8)
		tempEnc.encode(k)
		tempEnc.encode(kv)
	}
	enc.encode(reflect.ValueOf(uint32(tempEnc.Len())))
	enc.align(8)
	enc.Write(tempEnc.Bytes())
	encoderPool.Put(tempEnc)
	return nil
}
