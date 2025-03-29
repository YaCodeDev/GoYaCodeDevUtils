package valueparser

import (
	"encoding"
	"reflect"
)

// TryUnmarshal is a generic function that converts a string value to the specified type T.
// It returns the converted value and an error if the conversion fails.
//
// Example usage:
//
//	var intValue int
//	intValue, err := ParseValue[int]("123")
//	if err != nil {
//		// Handle error
//	}
func TryUnmarshal[T ParsableType](value string) (T, error) {
	var zero T
	typ := reflect.TypeOf(zero)

	ptr := reflect.New(typ)
	if unmarshaler, ok := ptr.Interface().(encoding.TextUnmarshaler); ok {
		if err := unmarshaler.UnmarshalText([]byte(value)); err == nil {
			if val, ok := ptr.Elem().Interface().(T); ok {
				return val, nil
			}

			return zero, ErrInvalidValue
		}
	}

	if unmarshaler, ok := ptr.Interface().(Unmarshalable); ok {
		if err := unmarshaler.Unmarshal(value); err == nil {
			if val, ok := ptr.Elem().Interface().(T); ok {
				return val, nil
			}

			return zero, ErrInvalidValue
		}
	}

	return zero, ErrUnparsableValue
}
