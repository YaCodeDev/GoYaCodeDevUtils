package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"
)

// safetyCheck ensures that the logger is not nil before performing any operations.
// If the logger is nil, it initializes a new logger and logs a warning message.
func safetyCheck(log **logrus.Entry) {
	if log == nil {
		return
	}

	if *log == nil {
		*log = logrus.NewEntry(logrus.StandardLogger())

		(*log).Warn("Logger is nil, using default logger")
	}
}

// toScreamingSnakeCase converts a string to SCREAMING_SNAKE_CASE.
// It replaces camelCase and PascalCase with underscores and converts to uppercase.
// For example, "myVariableName" becomes "MY_VARIABLE_NAME" and "MyVariableName" becomes "MY_VARIABLE_NAME".
// It also handles acronyms and abbreviations, ensuring they are treated as separate words.
// For example, "HTTPResponse" becomes "HTTP_RESPONSE" and "XMLParser" becomes "XML_PARSER".
func toScreamingSnakeCase(s string) string {
	s = matchFirstCap.ReplaceAllString(s, "${1}_${2}")
	s = matchAllCap.ReplaceAllString(s, "${1}_${2}")

	return strings.ToUpper(s)
}

func copyMap(src reflect.Value, dst reflect.Value) {
	if src.Kind() != reflect.Map || dst.Kind() != reflect.Map {
		panic("Both src and dst must be maps")
	}

	if dst.IsNil() {
		dst.Set(reflect.MakeMap(dst.Type()))
	}

	if src.IsNil() {
		return
	}

	for _, key := range src.MapKeys() {
		val := src.MapIndex(key)

		convertedKey := convertValue(key, dst.Type().Key())
		convertedVal := convertValue(val, dst.Type().Elem())

		dst.SetMapIndex(convertedKey, convertedVal)
	}
}

// convertValue converts a reflect.Value to the specified target type.
// It checks if the value is valid and convertible to the target type.
// If the value is valid and convertible, it returns the converted value.
// If the value is invalid, it returns a zero value of the target type.
// If the value is valid but not convertible, it panics with an error message.
// This function is used to ensure that the value being set in a map or slice is of the correct type.
// It is a helper function for copyMap and copyArray functions.
func convertValue(val reflect.Value, targetType reflect.Type) reflect.Value {
	if !val.IsValid() {
		return reflect.Zero(targetType)
	}

	if val.Type().ConvertibleTo(targetType) {
		return val.Convert(targetType)
	}

	panic(fmt.Sprintf("Cannot convert from %s to %s", val.Type(), targetType))
}

// copyArray copies elements from the source slice to the destination slice.
// It ensures that the destination slice is initialized and has the same length as the source slice.
// If the source slice is nil, the destination slice remains unchanged.
// If the source slice is not nil, it copies each element from the source to the destination,
// converting the type if necessary.
// It panics if the source or destination is not a slice.
func copyArray(src, dst reflect.Value) {
	if src.Kind() != reflect.Slice || dst.Kind() != reflect.Slice {
		panic("Both src and dst must be slices")
	}

	if dst.IsNil() {
		dst.Set(reflect.MakeSlice(dst.Type(), src.Len(), src.Cap()))
	}

	if src.IsNil() {
		return
	}

	for i := range src.Len() {
		val := src.Index(i)

		if val.IsValid() {
			dst.Index(i).Set(convertValue(val, dst.Type().Elem()))
		}
	}
}

// nolint: dupl
// getMapType determines the type of a map based on its key and value types.
// It returns a mapType constant that represents the specific type of the map.
// The function checks the key and value types of the map using reflection.
// It supports various combinations of key and value types, including string, int, uint, float, and bool.
// If the key or value type is not supported, it returns invalidMap.
// The function also handles special cases, such as byte slices and invalid types
// to ensure that the map type is correctly identified.
func getMapType(v reflect.Value) mapType {
	switch v.Type().Key().Kind() {
	case reflect.String:
		switch v.Type().Elem().Kind() {
		case reflect.String:
			return stringStringMap

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return stringIntMap

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return stringUintMap

		case reflect.Float32, reflect.Float64:
			return stringFloatMap

		case reflect.Bool:
			return stringBoolMap

		case reflect.Slice:
			if v.Type().Elem().Elem().Kind() == reflect.Uint8 {
				return stringByteSliceMap
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
			return invalidMap

		default:
			return invalidMap
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v.Type().Elem().Kind() {
		case reflect.String:
			return intStringMap

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return intIntMap

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return intUintMap

		case reflect.Float32, reflect.Float64:
			return intFloatMap

		case reflect.Bool:
			return intBoolMap

		case reflect.Slice:
			if v.Type().Elem().Elem().Kind() == reflect.Uint8 {
				return intByteSliceMap
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
			return invalidMap

		default:
			return invalidMap
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch v.Type().Elem().Kind() {
		case reflect.String:
			return uintStringMap

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return uintIntMap

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return uintUintMap

		case reflect.Float32, reflect.Float64:
			return uintFloatMap

		case reflect.Bool:
			return uintBoolMap

		case reflect.Slice:
			if v.Type().Elem().Elem().Kind() == reflect.Uint8 {
				return uintByteSliceMap
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
			return invalidMap

		default:
			return invalidMap
		}
	case reflect.Float32, reflect.Float64:
		switch v.Type().Elem().Kind() {
		case reflect.String:
			return floatStringMap

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return floatIntMap

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return floatUintMap

		case reflect.Float32, reflect.Float64:
			return floatFloatMap

		case reflect.Bool:
			return floatBoolMap

		case reflect.Slice:
			if v.Type().Elem().Elem().Kind() == reflect.Uint8 {
				return floatByteSliceMap
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
			return invalidMap

		default:
			return invalidMap
		}
	case reflect.Bool:
		switch v.Type().Elem().Kind() {
		case reflect.String:
			return boolStringMap

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return boolIntMap

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return boolUintMap

		case reflect.Float32, reflect.Float64:
			return boolFloatMap

		case reflect.Bool:
			return boolBoolMap

		case reflect.Slice:
			if v.Type().Elem().Elem().Kind() == reflect.Uint8 {
				return boolByteSliceMap
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
			return invalidMap

		default:
			return invalidMap
		}

	case reflect.Invalid,
		reflect.Slice,
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
		return invalidMap

	default:
		return invalidMap
	}

	return invalidMap
}
