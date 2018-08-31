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

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		in   reflect.Value
	}{
		{"byte-0", reflect.ValueOf(byte(0))},
		{"byte-2", reflect.ValueOf(byte(2))},
		{"bool-true", reflect.ValueOf(true)},
		{"bool-false", reflect.ValueOf(false)},
		{"uint8 123", reflect.ValueOf(uint8(123))},
		{"int16 -23", reflect.ValueOf(int16(-23))},
		{"uint16", reflect.ValueOf(uint16(0xffe9))},
		{"uint32", reflect.ValueOf(uint32(0xdeadbeef))},
		{"int32", reflect.ValueOf(int32(-1091581186))},
		{"int64", reflect.ValueOf(int64(-2401053089206453570))},
		{"uint64", reflect.ValueOf(uint64(0x01badcabfaceb00c))},
		{"float64-zero", reflect.ValueOf(float64(0))},
		{"float64-point-5", reflect.ValueOf(float64(0.5))},
		{"float64-max", reflect.ValueOf(float64(math.MaxFloat64))},
		{"float64-smallest-nonzero", reflect.ValueOf(float64(math.SmallestNonzeroFloat64))},
		{"signature-empty", reflect.ValueOf("")},
		{"signature-yyy", reflect.ValueOf("yyy")},
		{"string-empty", reflect.ValueOf("")},
		{"string-hello-world", reflect.ValueOf("hello, world!")},
		{"slice-empty-slice-of-int", reflect.ValueOf([]int{})},
		{"slice-of-string", reflect.ValueOf([]string{"hello,", "world!"})},
		{
			"slice-of-slice-of-struct-of-float64-and-float64",
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
		{"struct-empty", reflect.ValueOf(struct{}{})},
		{"struct-simple", reflect.ValueOf(struct {
			A int
			B string
			C float64
		}{1, "hello", 1.0})},
		{
			"struct-nested",
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
		enc.encode(test.in)
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
