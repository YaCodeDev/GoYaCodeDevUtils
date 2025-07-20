package valueparser

import (
	"encoding"
	"fmt"
	"net/http"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
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
func TryUnmarshal[T ParsableType](value string, valueType reflect.Type) (T, yaerrors.Error) {
	var zero T

	typ := reflect.TypeOf(zero)
	ptr := reflect.New(valueType)

	if unmarshaler, ok := ptr.Interface().(encoding.TextUnmarshaler); ok {
		if err := unmarshaler.UnmarshalText([]byte(value)); err != nil {
			return zero, yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				fmt.Sprintf(
					"try unmarshal: cannot convert value %v to type %s",
					value,
					valueType,
				),
			)
		}
	} else if unmarshaler, ok := ptr.Interface().(Unmarshalable); ok {
		if err := unmarshaler.Unmarshal(value); err != nil {
			return zero, yaerrors.FromError(
				http.StatusInternalServerError,
				ErrUnparsableValue,
				fmt.Sprintf(
					"try unmarshal: %v cannot be unmarshaled to type %s: %v",
					value,
					valueType,
					err,
				),
			)
		}
	} else {
		return zero, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrUnparsableValue,
			"try unmarshal: Unmarshalable interface not implemented",
		)
	}

	val, err := ConvertValue(ptr.Elem(), typ)
	if err != nil {
		return zero, err.Wrap(
			fmt.Sprintf(
				"try unmarshal: cannot convert value %v to type %s",
				value,
				typ,
			),
		)
	}

	if val, ok := val.Interface().(T); ok {
		return val, nil
	}

	return zero, yaerrors.FromError(
		http.StatusInternalServerError,
		ErrInvalidValue,
		"try unmarshal",
	)
}
