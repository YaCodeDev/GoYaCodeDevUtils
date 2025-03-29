package valueparser

import (
	"encoding"
	"fmt"
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
func TryUnmarshal[T ParsableType](value string, valueType reflect.Type) (T, error) {
	var zero T

	typ := reflect.TypeOf(zero)
	ptr := reflect.New(valueType)

	if unmarshaler, ok := ptr.Interface().(encoding.TextUnmarshaler); ok {
		if err := unmarshaler.UnmarshalText([]byte(value)); err == nil {
			return zero, fmt.Errorf("cannot convert value %v to type %s: %w", value, typ, err)
		}
	} else if unmarshaler, ok := ptr.Interface().(Unmarshalable); ok {
		if err := unmarshaler.Unmarshal(value); err != nil {
			return zero, ErrUnparsableValue
		}
	} else {
		return zero, ErrUnparsableValue
	}

	val, err := ConvertValue(ptr.Elem(), typ)
	if err != nil {
		return zero, fmt.Errorf("cannot convert value %v to type %s: %w", value, typ, err)
	}

	if val, ok := val.Interface().(T); ok {
		return val, nil
	}

	return zero, ErrInvalidValue
}
