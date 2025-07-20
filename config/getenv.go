package config

import (
	"fmt"
	"net/http"
	"os"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

// GetEnv retrieves the value of an environment variable, parses it to the specified type T,
// and returns it. If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs and returns an error.
//
// Example usage:
//
//	myInt, err := GetEnv("MY_ENV_VAR", 42, true, log)
//	if err != nil {
//	    // handle error
//	}
func GetEnv[T valueparser.ParsableType](
	key string,
	fallback T,
	required bool,
	log yalogger.Logger,
) (T, yaerrors.Error) {
	return GetEnvWithCustomType(
		key,
		fallback,
		required,
		reflect.TypeOf(new(T)).Elem(),
		log,
	)
}

// GetEnvWithCustomType retrieves the value of an environment variable, parses it to the specified type T,
// and returns it. If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs an error and returns an error.
// This function is useful when you need to specify a custom type for parsing.
//
// Example usage:
//
//	type YourCustomType uint64
//
//	func (s *YourCustomType) Unmarshal(data string) error {
//			if s == nil {
//				return fmt.Errorf("nil pointer to YourCustomType")
//			}
//
//			switch data {
//			case "FIRST":
//				*s = 1
//			case "SECOND":
//				*s = 2
//			default:
//				return fmt.Errorf("unknown value: %s", data)
//			}
//
//			return nil
//		}
//
//		myValue, err := GetEnvWithCustomType(
//			"MY_ENV_VAR",
//			uint64(0),
//			true,
//			reflect.TypeOf(YourCustomType(0)),
//			log,
//		)
func GetEnvWithCustomType[T valueparser.ParsableType](
	key string,
	fallback T,
	required bool,
	vType reflect.Type,
	log yalogger.Logger,
) (T, yaerrors.Error) {
	safetyCheck(&log)

	if value, exists := os.LookupEnv(key); exists {
		if parsed, err := valueparser.ParseValueWithCustomType[T](value, vType); err == nil {
			return parsed, nil
		}
	}

	if required {
		return fallback, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			ErrValueIsRequired,
			fmt.Sprintf("get env: environment variable %s is required", key),
			log,
		)
	}

	log.Warnf(
		"Environment variable %s is not set or failed to parse, using default value %v",
		key,
		fallback,
	)

	return fallback, nil
}

// GetEnvArray retrieves the value of an environment variable, splits it by a specified separator, (default is ","),
// parses each part into the specified type T, and returns a slice of T.
// If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs and returns an error.
//
// Example usage:
//
//	myArray, err := GetEnvArray("MY_ENV_VAR", []int{1, 2, 3}, nil, true, log)
//	if err != nil {
//	    // handle error
//	}
func GetEnvArray[T valueparser.ParsableType](
	key string,
	fallback []T,
	separator *string,
	required bool,
	log yalogger.Logger,
) ([]T, yaerrors.Error) {
	return GetEnvArrayWithCustomType(
		key,
		fallback,
		separator,
		required,
		reflect.TypeOf(new(T)).Elem(),
		log,
	)
}

// GetEnvArrayWithCustomType retrieves the value of an environment variable, splits it by a specified separator
// (default is ","), parses each part into the specified type T, and returns a slice of T.
// If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs and returns an error.
// This function is useful when you need to specify a custom type for parsing.
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
//	myArray, err := GetEnvArrayWithCustomType(
//		"MY_ENV_VAR",
//		[]uint64{1, 2, 3},
//		nil,
//		true,
//		reflect.TypeOf(YourCustomType(0)),
//		log,
//	)
func GetEnvArrayWithCustomType[T valueparser.ParsableType](
	key string,
	fallback []T,
	separator *string,
	required bool,
	vType reflect.Type,
	log yalogger.Logger,
) ([]T, yaerrors.Error) {
	safetyCheck(&log)

	if value, exists := os.LookupEnv(key); exists {
		parsed, err := valueparser.ParseArrayWithCustomType[T](value, separator, vType)
		if err == nil {
			return parsed, nil
		}

		log.Errorf("Failed to parse environment variable %s: %v", key, err)
	}

	if required {
		return nil, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			ErrValueIsRequired,
			fmt.Sprintf(
				"get env array: environment variable %s is required",
				key,
			),
			log,
		)
	}

	log.Warnf("Environment variable %s is not set, using default value %v", key, fallback)

	return fallback, nil
}

// GetEnvMap retrieves the value of an environment variable, splits it by a specified entry separator (default is ","),
// and each entry by a specified key-value separator (default is ":").
// It parses the key and value into the specified types K and V, and returns a map of K to V.
// If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs and returns an error.
//
// Example usage:
//
//	myMap, err := GetEnvMap("MY_ENV_VAR", map[string]int{"key": 1}, true, nil, nil, log)
//	if err != nil {
//	    // handle error
//	}
func GetEnvMap[K valueparser.ParsableComparableType, V valueparser.ParsableType](
	key string,
	fallback map[K]V,
	required bool,
	entrySeparator *string,
	kvSeparator *string,
	log yalogger.Logger,
) (map[K]V, yaerrors.Error) {
	return GetEnvMapWithCustomType(
		key,
		fallback,
		required,
		entrySeparator,
		kvSeparator,
		reflect.TypeOf(new(K)).Elem(),
		reflect.TypeOf(new(V)).Elem(),
		log,
	)
}

// GetEnvMapWithCustomType retrieves the value of an environment variable, splits it by a specified entry separator
// (default is ","), and each entry by a specified key-value separator (default is ":").
// It parses the key and value into the specified types K and V, and returns a map of K to V.
// If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs and returns an error.
// This function is useful when you need to specify custom types for parsing keys and/or values.
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
//	myMap, err := GetEnvMapWithCustomType(
//		"MY_ENV_VAR",
//		map[uint64]int{1: 10, 2: 20},
//		true,
//		nil,
//		nil,
//		reflect.TypeOf(YourCustomType(0)),
//		reflect.TypeOf(0),
//		log,
//	)
func GetEnvMapWithCustomType[K valueparser.ParsableComparableType, V valueparser.ParsableType](
	key string,
	fallback map[K]V,
	required bool,
	entrySeparator *string,
	kvSeparator *string,
	kType reflect.Type,
	vType reflect.Type,
	log yalogger.Logger,
) (map[K]V, yaerrors.Error) {
	safetyCheck(&log)

	if value, exists := os.LookupEnv(key); exists {
		parsed, err := valueparser.ParseMapWithCustomType[K, V](
			value,
			entrySeparator,
			kvSeparator,
			kType,
			vType,
		)
		if err == nil {
			return parsed, nil
		}

		log.Errorf("Failed to parse environment variable %s: %v", key, err)
	}

	if required {
		return nil, yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			ErrValueIsRequired,
			fmt.Sprintf(
				"get env map: environment variable %s is required",
				key,
			),
			log,
		)
	}

	log.Warnf("Environment variable %s is not set, using default value %v", key, fallback)

	return fallback, nil
}
