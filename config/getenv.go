package config

import (
	"os"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

// GetEnv retrieves the value of an environment variable, parses it to the specified type T,
// and returns it. If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs an error and exits the program.
//
// Example usage:
//
//	myInt := GetEnv("MY_ENV_VAR", 42, true, log)
//
// PANICS if the environment variable is required and not set.
func GetEnv[T valueparser.ParsableType](
	key string,
	fallback T,
	required bool,
	log yalogger.Logger,
) T {
	safetyCheck(&log)

	if value, exists := os.LookupEnv(key); exists {
		if parsed, err := valueparser.ParseValue[T](value); err == nil {
			return parsed
		}
	}

	if required {
		log.Fatalf("Environment variable %s is required", key)
	}

	log.Warnf(
		"Environment variable %s is not set or failed to parse, using default value %v",
		key,
		fallback,
	)

	return fallback
}

// GetEnvArray retrieves the value of an environment variable, splits it by a specified separator, (default is ","),
// parses each part into the specified type T, and returns a slice of T.
// If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs an error and exits the program.
//
// Example usage:
//
//	myArray := GetEnvArray("MY_ENV_VAR", []int{1, 2, 3}, nil, true, log)
//
// PANICS if the environment variable is required and not set.
func GetEnvArray[T valueparser.ParsableType](
	key string,
	fallback []T,
	separator *string,
	required bool,
	log yalogger.Logger,
) []T {
	safetyCheck(&log)

	if value, exists := os.LookupEnv(key); exists {
		parsed, err := valueparser.ParseArray[T](value, separator)
		if err == nil {
			return parsed
		}

		log.Errorf("Failed to parse environment variable %s: %v", key, err)
	}

	if required {
		log.Fatalf("Environment variable %s is required", key)
	}

	log.Warnf("Environment variable %s is not set, using default value %v", key, fallback)

	return fallback
}

// GetEnvMap retrieves the value of an environment variable, splits it by a specified entry separator (default is ","),
// and each entry by a specified key-value separator (default is ":").
// It parses the key and value into the specified types K and V, and returns a map of K to V.
// If the variable is not set, it returns a fallback value.
// If the variable is required and not set, it logs an error and exits the program.
//
// Example usage:
//
//	myMap := GetEnvMap("MY_ENV_VAR", map[string]int{"key": 1}, true, nil, nil, log)
//
// PANICS if the environment variable is required and not set.
func GetEnvMap[K valueparser.ParsableComparableType, V valueparser.ParsableType](
	key string,
	fallback map[K]V,
	required bool,
	entrySeparator *string,
	kvSeparator *string,
	log yalogger.Logger,
) map[K]V {
	safetyCheck(&log)

	if value, exists := os.LookupEnv(key); exists {
		parsed, err := valueparser.ParseMap[K, V](value, entrySeparator, kvSeparator)
		if err == nil {
			return parsed
		}

		log.Errorf("Failed to parse environment variable %s: %v", key, err)
	}

	if required {
		log.Fatalf("Environment variable %s is required", key)
	}

	log.Warnf("Environment variable %s is not set, using default value %v", key, fallback)

	return fallback
}
