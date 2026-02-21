package config

import (
	"bufio"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

// safetyCheck ensures that the logger is not nil before performing any operations.
// If the logger is nil, it initializes a new logger and logs a warning message.
func safetyCheck(log *yalogger.Logger) {
	if *log != nil {
		return
	}

	*log = yalogger.NewBaseLogger(nil).NewLogger()
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

	var (
		convertedKey reflect.Value
		convertedVal reflect.Value
		err          error
	)

	for _, key := range src.MapKeys() {
		val := src.MapIndex(key)

		convertedKey, err = valueparser.ConvertValue(key, dst.Type().Key())
		if err != nil {
			panic("Cannot convert key: " + err.Error())
		}

		convertedVal, err = valueparser.ConvertValue(val, dst.Type().Elem())
		if err != nil {
			panic("Cannot convert value: " + err.Error())
		}

		dst.SetMapIndex(convertedKey, convertedVal)
	}
}

// copyArray copies elements from the source slice to the destination slice.
// It ensures that the destination slice is initialized and has the same length as the source slice.
// If the source slice is nil, the destination slice remains unchanged.
// If the source slice is not nil, it copies each element from the source to the destination,
// converting the type if necessary.
// It panics if the source or destination is not a slice.
func copyArray(src, dst reflect.Value) {
	if !dst.IsValid() {
		panic("Destination slice is not valid")
	}

	if src.Kind() != reflect.Slice || dst.Kind() != reflect.Slice {
		panic("Both src and dst must be slices")
	}

	if !dst.CanSet() {
		panic("Destination slice cannot be set")
	}

	if dst.IsNil() {
		dst.Set(reflect.MakeSlice(dst.Type(), src.Len(), src.Cap()))
	}

	if src.IsNil() {
		return
	}

	if src.Len() != dst.Len() {
		dst.Set(reflect.MakeSlice(dst.Type(), src.Len(), src.Cap()))
	}

	for i := range src.Len() {
		val := src.Index(i)

		if val.IsValid() {
			converted, err := valueparser.ConvertValue(val, dst.Type().Elem())
			if err != nil {
				panic("Cannot convert value: " + err.Error())
			}

			dst.Index(i).Set(converted)
		}
	}
}

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

		case reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64,
			reflect.Uintptr:
			return stringUintMap

		case reflect.Float32, reflect.Float64:
			return stringFloatMap

		case reflect.Bool:
			return stringBoolMap

		case reflect.Invalid,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Ptr,
			reflect.Slice,
			reflect.Struct,
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

		case reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64,
			reflect.Uintptr:
			return intUintMap

		case reflect.Float32, reflect.Float64:
			return intFloatMap

		case reflect.Bool:
			return intBoolMap

		case reflect.Invalid,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Ptr,
			reflect.Slice,
			reflect.Struct,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Array,
			reflect.UnsafePointer:
			return invalidMap

		default:
			return invalidMap
		}
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr:
		switch v.Type().Elem().Kind() {
		case reflect.String:
			return uintStringMap

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return uintIntMap

		case reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64,
			reflect.Uintptr:
			return uintUintMap

		case reflect.Float32, reflect.Float64:
			return uintFloatMap

		case reflect.Bool:
			return uintBoolMap

		case reflect.Invalid,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Ptr,
			reflect.Slice,
			reflect.Struct,
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

		case reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64,
			reflect.Uintptr:
			return floatUintMap

		case reflect.Float32, reflect.Float64:
			return floatFloatMap

		case reflect.Bool:
			return floatBoolMap

		case reflect.Invalid,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Ptr,
			reflect.Slice,
			reflect.Struct,
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

		case reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64,
			reflect.Uintptr:
			return boolUintMap

		case reflect.Float32, reflect.Float64:
			return boolFloatMap

		case reflect.Bool:
			return boolBoolMap

		case reflect.Invalid,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Ptr,
			reflect.Slice,
			reflect.Struct,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Array,
			reflect.UnsafePointer:
			return invalidMap

		default:
			return invalidMap
		}

	case reflect.Invalid,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Ptr,
		reflect.Slice,
		reflect.Struct,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.UnsafePointer:
		return invalidMap

	default:
		return invalidMap
	}
}

func loadDotEnv() yaerrors.Error {
	file, err := os.Open(DotEnvFile)
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"load dot env: cannot open .env file",
		)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				"load dot env: error reading .env file",
			)
		}

		scannedText := strings.TrimSpace(scanner.Text())
		if scannedText == "" || strings.HasPrefix(scannedText, "#") {
			continue
		}

		keyValue := strings.SplitN(scannedText, "=", DotEnvKVParts)
		if len(keyValue) != DotEnvKVParts {
			return yaerrors.FromError(
				http.StatusInternalServerError,
				ErrInvalidDotEnvFileFormat,
				"load dot env: invalid .env file format for line: "+scannedText,
			)
		}

		key := strings.TrimSpace(keyValue[0])

		value := strings.TrimSpace(keyValue[1])
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = strings.Trim(value, "\"")
		} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			value = strings.Trim(value, "'")
		}

		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return yaerrors.FromError(
					http.StatusInternalServerError,
					err,
					"load dot env: cannot set environment variable "+key,
				)
			}
		}
	}

	return nil
}
