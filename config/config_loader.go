package config

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

// LoadConfigStructFromEnv loads environment variables into a struct.
// It uses the field names of the struct as keys to look up values in the environment.
// The keys are converted to SCREAMING_SNAKE_CASE.
// If a field is not set in the environment, it uses the default value of the field type.
// If a field is required and not set, it logs an error and exits the program.
// It supports various field types including maps, slices, and basic types (int, uint, float, bool, string)
// and their derivatives. Same value parsing capabilities apply to default tag values.
//
// This is a wrapper around LoadConfigStructFromEnvHandlingError that panics on error.
//
// Example usage:
//
//	type SubConfig struct {
//		Test          string
//		TestInt       int
//	}
//
//	type Config struct {
//		SubConfigFld  SubConfig
//		Test          string
//		TestInt       int
//		TestUint      uint
//		TestFloat     float64
//		TestBool      bool
//		TestMap       map[string]string
//		TestMapInt    map[string]uint8
//		TestListInt   []int
//		TestListStr   []string
//		TestListBool  []bool
//		TestListFloat []float64
//		TestURL       string
//		ExistingVar   string
//		DefaultMap    map[bool]float64 `default:"true:1.0,false:2.1"`
//		Level         logrus.Level     `default:"info"`
//		AlsoLevel     logrus.Level     `default:"4"`
//	}
//
//	config := Config{
//		ExistingVar: "existing_value",
//	}
//
//	config.LoadConfigStructFromEnv(&config, nil)
//
//	fmt.Printf("%+v\n", config)
func LoadConfigStructFromEnv[T any](instance *T, log yalogger.Logger) {
	err := LoadConfigStructFromEnvHandlingError(instance, log)
	if err != nil {
		log.Fatalf("Failed to load config struct from env: %v", err)
	}
}

// LoadConfigStructFromEnvHandlingError loads environment variables into a struct.
// It uses the field names of the struct as keys to look up values in the environment.
// The keys are converted to SCREAMING_SNAKE_CASE.
// If a field is not set in the environment, it uses the default value of the field type.
// If a field is required and not set, it logs an error and exits the program.
// It supports various field types including maps, slices, and basic types (int, uint, float, bool, string)
// and their derivatives. Same value parsing capabilities apply to default tag values.
//
// Example usage:
//
//	type SubConfig struct {
//		Test          string
//		TestInt       int
//	}
//
//	type Config struct {
//		SubConfigFld  SubConfig
//		Test          string
//		TestInt       int
//		TestUint      uint
//		TestFloat     float64
//		TestBool      bool
//		TestMap       map[string]string
//		TestMapInt    map[string]uint8
//		TestListInt   []int
//		TestListStr   []string
//		TestListBool  []bool
//		TestListFloat []float64
//		TestURL       string
//		ExistingVar   string
//		DefaultMap    map[bool]float64 `default:"true:1.0,false:2.1"`
//		Level         logrus.Level     `default:"info"`
//		AlsoLevel     logrus.Level     `default:"4"`
//	}
//
//	config := Config{
//		ExistingVar: "existing_value",
//	}
//
//	err := config.LoadConfigStructFromEnvHandlingError(&config, nil)
//
//	if err != nil {
//			// handle error
//	}
//
//	fmt.Printf("%+v\n", config)
func LoadConfigStructFromEnvHandlingError[T any](instance *T, log yalogger.Logger) yaerrors.Error {
	safetyCheck(&log)

	err := loadDotEnv()
	if err != nil {
		log.Warnf("Error loading .env file: %v", err)
	}

	value := reflect.ValueOf(instance).Elem()
	if value.Kind() != reflect.Struct {
		return yaerrors.FromErrorWithLog(
			http.StatusInternalServerError,
			ErrConfigStructMustBeStruct,
			fmt.Sprintf(
				"config loader, got %T",
				instance,
			),
			log,
		)
	}

	return loadConfigStructFromEnv(value, "", log)
}

// Internal function to load config struct from environment variables.
// It recursively processes each field of the struct, checking for the presence of environment variables.
// Does the actual work of LoadConfigStructFromEnv.
func loadConfigStructFromEnv(
	structValue reflect.Value,
	keyPath string,
	log yalogger.Logger,
) yaerrors.Error {
	structType := structValue.Type()

	var err yaerrors.Error

	for i := range structValue.NumField() {
		field := structType.Field(i)
		fieldVal := structValue.Field(i)
		defaultValStr := field.Tag.Get(DefaultTagName)

		if !fieldVal.CanSet() {
			log.Warnf("Field %s cannot be set", field.Name)

			continue
		}

		envKey := toScreamingSnakeCase(field.Name)

		if keyPath != "" {
			envKey = fmt.Sprintf(
				"%s_%s",
				keyPath,
				envKey,
			)
		}

		required := fieldVal.IsZero() && defaultValStr == ""

		useDefaultFromTag := fieldVal.IsZero() && defaultValStr != ""

		switch field.Type.Kind() {
		case reflect.Struct:
			if err = loadConfigStructFromEnv(fieldVal, envKey, log); err != nil {
				return err.WrapWithLog(
					"failed to load struct field "+field.Name,
					log,
				)
			}
		case reflect.Map:
			mapType := getMapType(fieldVal)
			switch mapType {
			case stringStringMap:
				if useDefaultFromTag {
					var val map[string]string

					val, err = valueparser.ParseMap[string, string](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringIntMap:
				if useDefaultFromTag {
					var val map[string]int64

					val, err = valueparser.ParseMap[string, int64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringUintMap:
				if useDefaultFromTag {
					var val map[string]uint64

					val, err = valueparser.ParseMap[string, uint64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringFloatMap:
				if useDefaultFromTag {
					var val map[string]float64

					val, err = valueparser.ParseMap[string, float64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringBoolMap:
				if useDefaultFromTag {
					var val map[string]bool

					val, err = valueparser.ParseMap[string, bool](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intStringMap:
				if useDefaultFromTag {
					var val map[int64]string

					val, err = valueparser.ParseMap[int64, string](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intIntMap:
				if useDefaultFromTag {
					var val map[int64]int64

					val, err = valueparser.ParseMap[int64, int64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intUintMap:
				if useDefaultFromTag {
					var val map[int64]uint64

					val, err = valueparser.ParseMap[int64, uint64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intFloatMap:
				if useDefaultFromTag {
					var val map[int64]float64

					val, err = valueparser.ParseMap[int64, float64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intBoolMap:
				if useDefaultFromTag {
					var val map[int64]bool

					val, err = valueparser.ParseMap[int64, bool](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintStringMap:
				if useDefaultFromTag {
					var val map[uint64]string

					val, err = valueparser.ParseMap[uint64, string](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintIntMap:
				if useDefaultFromTag {
					var val map[uint64]int64

					val, err = valueparser.ParseMap[uint64, int64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintUintMap:
				if useDefaultFromTag {
					var val map[uint64]uint64

					val, err = valueparser.ParseMap[uint64, uint64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintFloatMap:
				if useDefaultFromTag {
					var val map[uint64]float64

					val, err = valueparser.ParseMap[uint64, float64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintBoolMap:
				if useDefaultFromTag {
					var val map[uint64]bool

					val, err = valueparser.ParseMap[uint64, bool](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatStringMap:
				if useDefaultFromTag {
					var val map[float64]string

					val, err = valueparser.ParseMap[float64, string](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatIntMap:
				if useDefaultFromTag {
					var val map[float64]int64

					val, err = valueparser.ParseMap[float64, int64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatUintMap:
				if useDefaultFromTag {
					var val map[float64]uint64

					val, err = valueparser.ParseMap[float64, uint64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatFloatMap:
				if useDefaultFromTag {
					var val map[float64]float64

					val, err = valueparser.ParseMap[float64, float64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatBoolMap:
				if useDefaultFromTag {
					var val map[float64]bool

					val, err = valueparser.ParseMap[float64, bool](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolStringMap:
				if useDefaultFromTag {
					var val map[bool]string

					val, err = valueparser.ParseMap[bool, string](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[bool]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolIntMap:
				if useDefaultFromTag {
					var val map[bool]int64

					val, err = valueparser.ParseMap[bool, int64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[bool]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolUintMap:
				mapCopy := make(map[bool]uint64)

				if useDefaultFromTag {
					var val map[bool]uint64

					val, err = valueparser.ParseMap[bool, uint64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolFloatMap:
				mapCopy := make(map[bool]float64)

				if useDefaultFromTag {
					var val map[bool]float64

					val, err = valueparser.ParseMap[bool, float64](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolBoolMap:
				mapCopy := make(map[bool]bool)

				if useDefaultFromTag {
					var val map[bool]bool

					val, err = valueparser.ParseMap[bool, bool](defaultValStr, nil, nil)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				copyMap(fieldVal, reflect.ValueOf(mapCopy))

				mapCopy, err = GetEnvMapWithCustomType(
					envKey,
					mapCopy,
					required,
					nil,
					nil,
					fieldVal.Type().Key(),
					fieldVal.Type().Elem(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case invalidMap:
				log.Warnf("Unsupported map type for field %s", field.Name)
			}

		// nolint: dupl
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if useDefaultFromTag {
				var val int64

				val, err = valueparser.ParseValueWithCustomType[int64](defaultValStr, field.Type)
				if err != nil {
					return err.WrapWithLog(
						fmt.Sprintf(
							"config loader: field %s default: %v",
							field.Name,
							err,
						),
						log,
					)
				}

				fieldVal.SetInt(val)
			}

			var val int64

			val, err = GetEnv(envKey, fieldVal.Int(), required, log)
			if err != nil {
				return err.WrapWithLog(
					"load config struct from env",
					log,
				)
			}

			fieldVal.SetInt(val)

		// nolint: dupl
		case reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64,
			reflect.Uintptr:
			if useDefaultFromTag {
				var val uint64

				val, err = valueparser.ParseValueWithCustomType[uint64](defaultValStr, field.Type)
				if err != nil {
					return err.WrapWithLog(
						fmt.Sprintf(
							"config loader: field %s default: %v",
							field.Name,
							err,
						),
						log,
					)
				}

				fieldVal.SetUint(val)
			}

			var val uint64

			val, err = GetEnv(envKey, fieldVal.Uint(), required, log)
			if err != nil {
				return err.WrapWithLog(
					"load config struct from env",
					log,
				)
			}

			fieldVal.SetUint(val)

		case reflect.Float32, reflect.Float64:
			if useDefaultFromTag {
				var val float64

				val, err = valueparser.ParseValueWithCustomType[float64](defaultValStr, field.Type)
				if err != nil {
					return err.WrapWithLog(
						fmt.Sprintf(
							"config loader: field %s default: %v",
							field.Name,
							err,
						),
						log,
					)
				}

				fieldVal.SetFloat(val)
			}

			var val float64

			val, err = GetEnv(envKey, fieldVal.Float(), required, log)
			if err != nil {
				return err.WrapWithLog(
					"load config struct from env",
					log,
				)
			}

			fieldVal.SetFloat(val)

		case reflect.Bool:
			if useDefaultFromTag {
				var val bool

				val, err = valueparser.ParseValueWithCustomType[bool](defaultValStr, field.Type)
				if err != nil {
					return err.WrapWithLog(
						fmt.Sprintf(
							"config loader: field %s default: %v",
							field.Name,
							err,
						),
						log,
					)
				}

				fieldVal.SetBool(val)
			}

			var val bool

			val, err = GetEnv(envKey, fieldVal.Bool(), required, log)
			if err != nil {
				return err.WrapWithLog(
					"load config struct from env",
					log,
				)
			}

			fieldVal.SetBool(val)

		case reflect.String:
			if useDefaultFromTag {
				var val string

				val, err = valueparser.TryUnmarshal[string](defaultValStr, field.Type)
				if err != nil {
					fieldVal.SetString(defaultValStr)
				} else {
					fieldVal.SetString(val)
				}
			}

			var val string

			val, err = GetEnv(envKey, fieldVal.String(), required, log)
			if err != nil {
				return err.WrapWithLog(
					"load config struct from env",
					log,
				)
			}

			fieldVal.SetString(val)

		case reflect.Slice:
			switch field.Type.Elem().Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				array := make([]int64, fieldVal.Len())

				if useDefaultFromTag {
					array, err = valueparser.ParseArrayWithCustomType[int64](
						defaultValStr,
						nil,
						field.Type.Elem(),
					)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyArray(reflect.ValueOf(array), fieldVal)
				}

				array, err = GetEnvArrayWithCustomType(
					envKey,
					array,
					nil,
					required,
					fieldVal.Type(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.Uint,
				reflect.Uint8,
				reflect.Uint16,
				reflect.Uint32,
				reflect.Uint64,
				reflect.Uintptr:
				array := make([]uint64, fieldVal.Len())

				if useDefaultFromTag {
					array, err = valueparser.ParseArrayWithCustomType[uint64](
						defaultValStr,
						nil,
						field.Type.Elem(),
					)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyArray(reflect.ValueOf(array), fieldVal)
				}

				array, err = GetEnvArrayWithCustomType(
					envKey,
					array,
					nil,
					required,
					fieldVal.Type(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.Float32, reflect.Float64:
				array := make([]float64, fieldVal.Len())

				if useDefaultFromTag {
					array, err = valueparser.ParseArrayWithCustomType[float64](
						defaultValStr,
						nil,
						field.Type.Elem(),
					)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyArray(reflect.ValueOf(array), fieldVal)
				}

				array, err = GetEnvArrayWithCustomType(
					envKey,
					array,
					nil,
					required,
					fieldVal.Type(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.Bool:
				array := make([]bool, fieldVal.Len())

				if useDefaultFromTag {
					array, err = valueparser.ParseArrayWithCustomType[bool](
						defaultValStr,
						nil,
						field.Type.Elem(),
					)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyArray(reflect.ValueOf(array), fieldVal)
				}

				array, err = GetEnvArrayWithCustomType(
					envKey,
					array,
					nil,
					required,
					fieldVal.Type(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.String:
				array := make([]string, fieldVal.Len())

				if useDefaultFromTag {
					array, err = valueparser.ParseArrayWithCustomType[string](
						defaultValStr,
						nil,
						field.Type.Elem(),
					)
					if err != nil {
						return err.WrapWithLog(
							fmt.Sprintf(
								"config loader: field %s default: %v",
								field.Name,
								err,
							),
							log,
						)
					}

					copyArray(reflect.ValueOf(array), fieldVal)
				}

				array, err = GetEnvArrayWithCustomType(
					envKey,
					array,
					nil,
					required,
					fieldVal.Type(),
					log,
				)
				if err != nil {
					return err.WrapWithLog(
						"load config struct from env",
						log,
					)
				}

				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.Invalid,
				reflect.Chan,
				reflect.Func,
				reflect.Interface,
				reflect.Ptr,
				reflect.Struct,
				reflect.Slice,
				reflect.Complex64,
				reflect.Complex128,
				reflect.Array,
				reflect.Map,
				reflect.UnsafePointer:
				log.Warnf("Unsupported field type for field %s", field.Name)

			default:
				log.Warnf("Unsupported slice type for field %s", field.Name)
			}
		case reflect.Invalid,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Ptr,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Array,
			reflect.UnsafePointer:
			log.Warnf("Unsupported field type for field %s", field.Name)

		default:
			log.Warnf("Unsupported field type for field %s", field.Name)
		}
	}

	return nil
}
