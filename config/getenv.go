package config

import (
	"os"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
	"github.com/sirupsen/logrus"
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
	log *logrus.Entry,
) T {
	safetyCheck(&log)

	if log == nil {
		log = logrus.NewEntry(logrus.StandardLogger())

		log.Warn("Logger is nil, using default logger")
	}

	if value, exists := os.LookupEnv(key); exists {
		if parsed, err := valueparser.ParseValue[T](value); err == nil {
			return parsed
		}
	}

	if required {
		log.Fatalf("Environment variable %s is required", key)
	}

	log.Warnf("Environment variable %s is not set, using default value %v", key, fallback)

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
	log *logrus.Entry,
) []T {
	safetyCheck(&log)

	if separator == nil {
		separator = new(string)
		*separator = ","
	}

	if value, exists := os.LookupEnv(key); exists {
		parts := strings.Split(value, *separator)
		result := make([]T, 0, len(parts))

		for _, part := range parts {
			if converted, err := valueparser.ParseValue[T](strings.TrimSpace(part)); err == nil {
				result = append(result, converted)

				continue
			}

			log.Warnf("Failed to parse part %s of environment variable %s, skipping", part, key)
		}

		return result
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
//	myMap := GetEnvMap("MY_ENV_VAR", map[string]int{"key": 1}, nil, nil, true, log)
//
// PANICS if the environment variable is required and not set.
func GetEnvMap[K valueparser.ParsableComparableType, V valueparser.ParsableType](
	key string,
	fallback map[K]V,
	required bool,
	entrySeparator *string,
	kvSeparator *string,
	log *logrus.Entry,
) map[K]V {
	safetyCheck(&log)

	if value, exists := os.LookupEnv(key); exists {
		result := make(map[K]V)

		if entrySeparator == nil {
			entrySeparator = new(string)
			*entrySeparator = ","
		}

		if kvSeparator == nil {
			kvSeparator = new(string)
			*kvSeparator = ":"
		}

		for item := range strings.SplitSeq(value, *entrySeparator) {
			parts := strings.Split(item, *kvSeparator)
			if len(parts) == MapPartsCount {
				if k, err := valueparser.ParseValue[K](strings.TrimSpace(parts[0])); err == nil {
					if v, err := valueparser.ParseValue[V](strings.TrimSpace(parts[1])); err == nil {
						result[k] = v
					}
				}
			}
		}

		return result
	}

	if required {
		log.Fatalf("Environment variable %s is required", key)
	}

	log.Warnf("Environment variable %s is not set, using default value %v", key, fallback)

	return fallback
}
