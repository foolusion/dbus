package dbus // import "github.com/godbus/dbus"

import (
	"fmt"
	"reflect"
	"strings"
)

var sigToType = map[byte]reflect.Type{
	'y': byteType,
	'b': boolType,
	'n': int16Type,
	'q': uint16Type,
	'i': int32Type,
	'u': uint32Type,
	'x': int64Type,
	't': uint64Type,
	'd': float64Type,
	's': stringType,
	'g': signatureType,
	'o': objectPathType,
	'v': variantType,
	'h': unixFDIndexType,
}

// Signature represents a correct type signature as specified by the D-Bus
// specification. The zero value represents the empty signature, "".
type Signature string

// SignatureOf returns the concatenation of all the signatures of the given
// values. It panics if one of them is not representable in D-Bus.
func SignatureOf(vs ...interface{}) Signature {
	var s Signature
	for _, v := range vs {
		s += getSignature(reflect.TypeOf(v))
	}
	return s
}

// SignatureOfType returns the signature of the given type. It panics if the
// type is not representable in D-Bus.
func SignatureOfType(t reflect.Type) Signature {
	return getSignature(t)
}

// getSignature returns the signature of the given type and panics on unknown types.
func getSignature(t reflect.Type) Signature {
	// handle simple types first
	switch t.Kind() {
	case reflect.Uint8:
		return "y"
	case reflect.Bool:
		return "b"
	case reflect.Int16:
		return "n"
	case reflect.Uint16:
		return "q"
	case reflect.Int, reflect.Int32:
		if t == unixFDType {
			return "h"
		}
		return "i"
	case reflect.Uint, reflect.Uint32:
		if t == unixFDIndexType {
			return "h"
		}
		return "u"
	case reflect.Int64:
		return "x"
	case reflect.Uint64:
		return "t"
	case reflect.Float64:
		return "d"
	case reflect.Ptr:
		return getSignature(t.Elem())
	case reflect.String:
		if t == objectPathType {
			return "o"
		}
		return "s"
	case reflect.Struct:
		if t == variantType {
			return "v"
		} else if t == signatureType {
			return "g"
		}
		var s Signature
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath == "" && field.Tag.Get("dbus") != "-" {
				s += getSignature(t.Field(i).Type)
			}
		}
		return "(" + s + ")"
	case reflect.Array, reflect.Slice:
		return "a" + getSignature(t.Elem())
	case reflect.Map:
		if !isKeyType(t.Key()) {
			panic(InvalidTypeError{t})
		}
		return "a{" + getSignature(t.Key()) + getSignature(t.Elem()) + "}"
	case reflect.Interface:
		return "v"
	}
	panic(InvalidTypeError{t})
}

// ParseSignature returns the signature represented by this string, or a
// SignatureError if the string is not a valid signature.
func ParseSignature(s Signature) (sig Signature, err error) {
	if len(s) == 0 {
		return
	}
	if len(s) > 255 {
		return "", SignatureError{string(s), "too long"}
	}
	sig = s
	for err == nil && len(s) != 0 {
		s, err = validSingle(s, 0)
	}
	if err != nil {
		sig = ""
	}

	return
}

// ParseSignatureMust behaves like ParseSignature, except that it panics if s
// is not valid.
func ParseSignatureMust(s Signature) Signature {
	sig, err := ParseSignature(s)
	if err != nil {
		panic(err)
	}
	return sig
}

// Empty retruns whether the signature is the empty signature.
func (s Signature) Empty() bool {
	return s == ""
}

// Single returns whether the signature represents a single, complete type.
func (s Signature) Single() bool {
	r, err := validSingle(s, 0)
	return err != nil && r == ""
}

// String returns the signature's string representation.
func (s Signature) String() string {
	return string(s)
}

// A SignatureError indicates that a signature passed to a function or received
// on a connection is not a valid signature.
type SignatureError struct {
	Sig    string
	Reason string
}

func (e SignatureError) Error() string {
	return fmt.Sprintf("dbus: invalid signature: %q (%s)", e.Sig, e.Reason)
}

// Try to read a single type from this string. If it was successful, err is nil
// and rem is the remaining unparsed part. Otherwise, err is a non-nil
// SignatureError and rem is "". depth is the current recursion depth which may
// not be greater than 64 and should be given as 0 on the first call.
func validSingle(s Signature, depth int) (rem Signature, err error) {
	if s == "" {
		return "", SignatureError{Sig: string(s), Reason: "empty signature"}
	}
	if depth > 64 {
		return "", SignatureError{Sig: string(s), Reason: "container nesting too deep"}
	}
	switch s[0] {
	case 'y', 'b', 'n', 'q', 'i', 'u', 'x', 't', 'd', 's', 'g', 'o', 'v', 'h':
		return s[1:], nil
	case 'a':
		if len(s) > 1 && s[1] == '{' {
			i := findMatching(s[1:], '{', '}')
			if i == -1 {
				return "", SignatureError{Sig: string(s), Reason: "unmatched '{'"}
			}
			i++
			rem = s[i+1:]
			s = s[2:i]
			if _, err = validSingle(s[:1], depth+1); err != nil {
				return "", err
			}
			nr, err := validSingle(s[1:], depth+1)
			if err != nil {
				return "", err
			}
			if nr != "" {
				return "", SignatureError{Sig: string(s), Reason: "too many types in dict"}
			}
			return rem, nil
		}
		return validSingle(s[1:], depth+1)
	case '(':
		i := findMatching(s, '(', ')')
		if i == -1 {
			return "", SignatureError{Sig: string(s), Reason: "unmatched ')'"}
		}
		rem = s[i+1:]
		s = s[1:i]
		for err == nil && s != "" {
			s, err = validSingle(s, depth+1)
		}
		if err != nil {
			rem = ""
		}
		return
	}
	return "", SignatureError{Sig: string(s), Reason: "invalid type character"}
}

func findMatching(s Signature, left, right rune) int {
	n := 0
	for i, v := range s {
		if v == left {
			n++
		} else if v == right {
			n--
		}
		if n == 0 {
			return i
		}
	}
	return -1
}

// typeFor returns the type of the given signature. It ignores any left over
// characters and panics if s doesn't start with a valid type signature.
func typeFor(s Signature) (t reflect.Type) {
	_, err := validSingle(s, 0)
	if err != nil {
		panic(err)
	}

	if t, ok := sigToType[s[0]]; ok {
		return t
	}
	switch s[0] {
	case 'a':
		if s[1] == '{' {
			i := strings.LastIndex(string(s), "}")
			t = reflect.MapOf(sigToType[s[2]], typeFor(s[3:i]))
		} else {
			t = reflect.SliceOf(typeFor(s[1:]))
		}
	case '(':
		t = interfacesType
	}
	return
}
