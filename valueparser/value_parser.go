package valueparser

import (
	"net/http"
	"reflect"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// ParseValue is a generic function that converts a string value to the specified type T.
// It returns the converted value and an error if the conversion fails.
//
// Example usage:
//
//	var intValue int
//	intValue, err := ParseValue[int]("123")
//	if err != nil {
//		// Handle error
//	}
func ParseValue[T ParsableType](value string) (T, yaerrors.Error) {
	return ParseValueWithCustomType[T](value, reflect.TypeOf(new(T)).Elem())
}

// ParseValueWithCustomType is a generic function that converts a string value to the specified type T,
// using the provided valueType for conversion. It returns the converted value and an error if the conversion fails.
// This function is useful when you need to specify a custom type for parsing, such as when using a custom unmarshal.
//
// Example usage:
//
//	type YourCustomType uint64
//
//	func (s *YourCustomType) Unmarshal(data string) error {
//		if s == nil {
//			return fmt.Errorf("nil pointer to YourCustomType")
//		}
//
//		switch data {
//		case "FIRST":
//			*s = 1
//		case "SECOND":
//			*s = 2
//		default:
//			return fmt.Errorf("unknown value: %s", data)
//		}
//
//		return nil
//	}
//
//	customValue, err := ParseValueWithCustomType[uint64]("FIRST", reflect.TypeOf(YourCustomType(0)))
//
//	if err != nil {
//		// Handle error
//	}
func ParseValueWithCustomType[T ParsableType](
	value string,
	valueType reflect.Type,
) (T, yaerrors.Error) {
	var zero T

	zeroType := reflect.TypeOf(zero)

	switch valueType.Kind() {
	case reflect.String:
		if val, ok := any(value).(T); ok {
			unmarshaled, err := TryUnmarshal[T](value, valueType)
			if err == nil {
				return unmarshaled, nil
			}

			return val, nil
		}

		return zero, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrInvalidValue,
			"parse value: value is not a string",
		)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			if val, ok := reflect.ValueOf(intValue).Convert(zeroType).Interface().(T); ok {
				return val, nil
			}
		}

	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			if val, ok := reflect.ValueOf(uintValue).Convert(zeroType).Interface().(T); ok {
				return val, nil
			}
		}

	case reflect.Float32, reflect.Float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			if val, ok := reflect.ValueOf(floatValue).Convert(zeroType).Interface().(T); ok {
				return val, nil
			}
		}

	case reflect.Bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			if val, ok := reflect.ValueOf(boolValue).Convert(zeroType).Interface().(T); ok {
				return val, nil
			}
		}

	case reflect.Slice:
		if valueType.Elem().Kind() == reflect.Uint8 {
			if val, ok := any([]byte(value)).(T); ok {
				return val, nil
			}
		}

	case reflect.Invalid,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Ptr,
		reflect.Struct,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.UnsafePointer:
		return zero, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrInvalidValue,
			"parse value: unsupported type "+valueType.String(),
		)
	}

	val, err := TryUnmarshal[T](value, valueType)
	if err != nil {
		return zero, err.Wrap("parse value: failed to unmarshal")
	}

	return val, nil
}
