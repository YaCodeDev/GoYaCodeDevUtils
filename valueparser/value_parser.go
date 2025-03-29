package valueparser

import (
	"reflect"
	"strconv"
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
func ParseValue[T ParsableType](value string) (T, error) {
	var zero T
	typ := reflect.TypeOf(zero)

	switch typ.Kind() {
	case reflect.String:
		if val, ok := any(value).(T); ok {
			return val, nil
		}

		return zero, ErrInvalidValue

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			if val, ok := reflect.ValueOf(intValue).Convert(typ).Interface().(T); ok {
				return val, nil
			}
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			if val, ok := reflect.ValueOf(uintValue).Convert(typ).Interface().(T); ok {
				return val, nil
			}
		}

	case reflect.Float32, reflect.Float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			if val, ok := reflect.ValueOf(floatValue).Convert(typ).Interface().(T); ok {
				return val, nil
			}
		}

	case reflect.Bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			if val, ok := reflect.ValueOf(boolValue).Convert(typ).Interface().(T); ok {
				return val, nil
			}
		}

	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
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
		reflect.Uintptr,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.UnsafePointer:
		return zero, ErrInvalidType
	}

	val, err := TryUnmarshal[T](value)

	if err != nil {
		return zero, err
	}

	if val, ok := reflect.ValueOf(val).Convert(typ).Interface().(T); ok {
		return val, nil
	}

	return zero, ErrInvalidValue
}
