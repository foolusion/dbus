package dbus

import (
	"bytes"
	"flag"
	"io/ioutil"
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestEncodeByte(t *testing.T) {
	tests := []struct {
		name     string
		in       byte
		expected []byte
	}{
		{"0 byte", 0, []byte{0}},
		{"2 byte", 2, []byte{2}},
	}

	for _, test := range tests {
		enc := newEncoder()
		if err := encodeByte(enc, reflect.ValueOf(test.in)); err != nil {
			t.Errorf("%s: %s", test.name, err)
		}
		if enc.err != nil {
			t.Errorf("%s: encoder err: %s", test.name, enc.err)
		}
		if !bytes.Equal(test.expected, enc.Bytes()) {
			t.Errorf("%s: got % x want % x", test.name, enc.Bytes(), test.expected)
		}
	}
}

func TestEncodeBool(t *testing.T) {
	tests := []struct {
		name     string
		in       bool
		expected []byte
	}{
		{"true", true, []byte{0, 0, 0, 1}},
		{"false", false, []byte{0, 0, 0, 0}},
	}
	for _, test := range tests {
		enc := newEncoder()
		if err := encodeBool(enc, reflect.ValueOf(test.in)); err != nil {
			t.Errorf("%s: %s", test.name, err)
		}
		if enc.err != nil {
			t.Errorf("%s: encoder err: %s", test.name, enc.err)
		}
		if !bytes.Equal(test.expected, enc.Bytes()) {
			t.Errorf("%s: got % x want % x", test.name, enc.Bytes(), test.expected)
		}
	}
}

func TestEncodeInt(t *testing.T) {
	tests := []struct {
		name     string
		in       reflect.Value
		expected []byte
	}{
		{"uint8 123", reflect.ValueOf(uint8(123)), []byte{123}},
		{"int16 -23", reflect.ValueOf(int16(-23)), []byte{0xff, 0xe9}},
		{"uint16", reflect.ValueOf(uint16(0xffe9)), []byte{0xff, 0xe9}},
		{"uint32", reflect.ValueOf(uint32(0xdeadbeef)), []byte{0xde, 0xad, 0xbe, 0xef}},
		{"int32", reflect.ValueOf(int32(-1091581186)), []byte{0xbe, 0xef, 0xca, 0xfe}},
		{
			"int64",
			reflect.ValueOf(int64(-2401053089206453570)),
			[]byte{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0xba, 0xbe},
		},
		{
			"uint64",
			reflect.ValueOf(uint64(0x01badcabfaceb00c)),
			[]byte{0x01, 0xba, 0xdc, 0xab, 0xfa, 0xce, 0xb0, 0x0c},
		},
	}
	for _, test := range tests {
		enc := newEncoder()
		if err := encodeInt(enc, test.in); err != nil {
			t.Errorf("%s: %s", test.name, err)
		}
		if enc.err != nil {
			t.Errorf("%s: encoder err: %s", test.name, enc.err)
		}
		if !bytes.Equal(test.expected, enc.Bytes()) {
			t.Errorf("%s: got % x want % x", test.name, enc.Bytes(), test.expected)
		}
	}
}

func TestEncodeFloat(t *testing.T) {
	tests := []struct {
		name string
		in   reflect.Value
	}{
		{"zero", reflect.ValueOf(float64(0))},
		{"point-5", reflect.ValueOf(float64(0.5))},
		{"max-float", reflect.ValueOf(float64(math.MaxFloat64))},
		{"smallest-nonzero", reflect.ValueOf(float64(math.SmallestNonzeroFloat64))},
	}
	for _, test := range tests {
		enc := newEncoder()
		if err := encodeFloat(enc, test.in); err != nil {
			t.Errorf("%s: %s", test.name, err)
		}
		if enc.err != nil {
			t.Errorf("%s: encoder err: %s", test.name, enc.err)
		}
		golden := filepath.Join("testdata", test.name+".golden")
		if *update {
			ioutil.WriteFile(golden, enc.Bytes(), 0644)
		}
		expected, err := ioutil.ReadFile(golden)
		if err != nil {
			t.Errorf("could not read test file %s: %v", golden, err)
		}
		if !bytes.Equal(expected, enc.Bytes()) {
			t.Errorf("%s: got % x, wanted % x", test.name, enc.Bytes(), expected)
		}
	}
}

func TestEncodeSignature(t *testing.T) {
	tests := []struct {
		name     string
		in       reflect.Value
		expected []byte
	}{
		{"empty", reflect.ValueOf(""), []byte{0, 0}},
		{"yyy", reflect.ValueOf("yyy"), []byte{3, 'y', 'y', 'y', 0}},
	}
	for _, test := range tests {
		enc := newEncoder()
		if err := encodeSignature(enc, test.in); err != nil {
			t.Errorf("%s: %s", test.name, err)
		}
		if enc.err != nil {
			t.Errorf("%s: encoder err: %s", test.name, enc.err)
		}
		if !bytes.Equal(test.expected, enc.Bytes()) {
			t.Errorf("%s: got % x want % x", test.name, enc.Bytes(), test.expected)
		}
	}
}

func TestEncodeString(t *testing.T) {
	tests := []struct {
		name     string
		in       reflect.Value
		expected []byte
	}{
		{"empty", reflect.ValueOf(""), []byte{0, 0, 0, 0, 0}},
		{"hello, world!", reflect.ValueOf("hello, world!"), []byte{0, 0, 0, 13, 'h', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd', '!', 0}},
	}
	for _, test := range tests {
		enc := newEncoder()
		if err := encodeString(enc, test.in); err != nil {
			t.Errorf("%s: %s", test.name, err)
		}
		if enc.err != nil {
			t.Errorf("%s: encoder err: %s", test.name, enc.err)
		}
		if !bytes.Equal(test.expected, enc.Bytes()) {
			t.Errorf("%s: got % x want % x", test.name, enc.Bytes(), test.expected)
		}
	}
}

func TestEncodeSlice(t *testing.T) {
	tests := []struct {
		name string
		in   reflect.Value
	}{
		{"empty-slice-of-int", reflect.ValueOf([]int{})},
		{"slice-of-string", reflect.ValueOf([]string{"hello,", "world!"})},
		{
			"slice-of-slice-of-struct-of-float64-and float64",
			reflect.ValueOf([][]struct {
				x float64
				y float64
			}{
				{
					{1.0, 2.0},
					{3.1, 4.1},
				},
				{
					{5.2, 6.2},
					{7.3, 8.3},
					{9.4, 10.4},
				},
			}),
		},
	}
	for _, test := range tests {
		enc := newEncoder()
		if err := encodeSlice(enc, test.in); err != nil {
			t.Errorf("%s: %s", test.name, err)
		}
		if enc.err != nil {
			t.Errorf("%s: encoder err: %s", test.name, enc.err)
		}
		golden := filepath.Join("testdata", test.name+".golden")
		if *update {
			ioutil.WriteFile(golden, enc.Bytes(), 0644)
		}
		expected, err := ioutil.ReadFile(golden)
		if err != nil {
			t.Errorf("could not read test file %s: %v", golden, err)
		}
		if !bytes.Equal(expected, enc.Bytes()) {
			t.Errorf("%s: got % x, wanted % x", test.name, enc.Bytes(), expected)
		}
	}
}

func TestEncodeStruct(t *testing.T) {
	tests := []struct {
		name string
		in   reflect.Value
	}{
		{"empty-struct", reflect.ValueOf(struct{}{})},
		{"simple-struct", reflect.ValueOf(struct {
			A int
			B string
			C float64
		}{1, "hello", 1.0})},
		{
			"nested-struct",
			reflect.ValueOf(struct {
				A struct{ B, C int }
				D struct{ E, F float64 }
			}{
				struct{ B, C int }{1, 2},
				struct{ E, F float64 }{1.0, 2.0},
			}),
		},
		{
			"struct-with-array",
			reflect.ValueOf(struct {
				A struct{ B, C int }
				D []string
			}{
				struct{ B, C int }{1, 2},
				[]string{"hello", "world", "nice", "to", "meet", "you"},
			}),
		},
	}
	for _, test := range tests {
		enc := newEncoder()
		if err := encodeStruct(enc, test.in); err != nil {
			t.Errorf("%s: %s", test.name, err)
		}
		if enc.err != nil {
			t.Errorf("%s: encoder err: %s", test.name, enc.err)
		}
		golden := filepath.Join("testdata", test.name+".golden")
		if *update {
			ioutil.WriteFile(golden, enc.Bytes(), 0644)
		}
		expected, err := ioutil.ReadFile(golden)
		if err != nil {
			t.Errorf("%s: could not read test file %s: %v", test.name, golden, err)
		}
		if !bytes.Equal(expected, enc.Bytes()) {
			t.Errorf("%s: got % x, want % x", test.name, enc.Bytes(), expected)
		}
	}
}
