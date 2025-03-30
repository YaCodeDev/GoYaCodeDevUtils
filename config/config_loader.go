package config

import (
	"errors"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/valueparser"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// LoadConfigStructFromEnv loads environment variables into a struct.
// It uses the field names of the struct as keys to look up values in the environment.
// The keys are converted to SCREAMING_SNAKE_CASE.
// If a field is not set in the environment, it uses the default value of the field type.
// If a field is required and not set, it logs an error and exits the program.
// It supports various field types including maps, slices, and basic types (int, uint, float, bool, string)
// and their derivatives. Same value parsing capabilities apply to default tag values.
//
// Example usage:
//
//	type Config struct {
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
func LoadConfigStructFromEnv[T any](instance *T, log *logrus.Entry) {
	safetyCheck(&log)

	err := godotenv.Load()
	if err != nil {
		log.Warnf("Error loading .env file: %v", err)
	}

	v := reflect.ValueOf(instance)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		log.Fatalf("Target must be a pointer to a struct, got %T", instance)
	}

	v = v.Elem()
	t := v.Type()

	for i := range v.NumField() {
		field := t.Field(i)
		fieldVal := v.Field(i)
		defaultValStr := field.Tag.Get(DefaultTagName)

		if !fieldVal.CanSet() {
			log.Warnf("Field %s cannot be set", field.Name)

			continue
		}

		envKey := toScreamingSnakeCase(field.Name)
		required := fieldVal.IsZero() && defaultValStr == ""

		useDefaultFromTag := fieldVal.IsZero() && defaultValStr != ""

		switch field.Type.Kind() {
		case reflect.Map:
			mapType := getMapType(fieldVal)
			switch mapType {
			case stringStringMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[string, string](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringIntMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[string, int64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringUintMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[string, uint64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringFloatMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[string, float64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringBoolMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[string, bool](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringByteSliceMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[string, []byte](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[string][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intStringMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[int64, string](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intIntMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[int64, int64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intUintMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[int64, uint64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intFloatMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[int64, float64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intBoolMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[int64, bool](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intByteSliceMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[int64, []byte](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[int64][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintStringMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[uint64, string](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintIntMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[uint64, int64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintUintMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[uint64, uint64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintFloatMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[uint64, float64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintBoolMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[uint64, bool](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintByteSliceMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[uint64, []byte](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[uint64][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatStringMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[float64, string](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatIntMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[float64, int64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatUintMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[float64, uint64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatFloatMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[float64, float64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatBoolMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[float64, bool](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatByteSliceMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[float64, []byte](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[float64][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolStringMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[bool, string](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[bool]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolIntMap:
				if useDefaultFromTag {
					val, err := valueparser.ParseMap[bool, int64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				mapCopy := make(map[bool]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolUintMap:
				mapCopy := make(map[bool]uint64)

				if useDefaultFromTag {
					val, err := valueparser.ParseMap[bool, uint64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolFloatMap:
				mapCopy := make(map[bool]float64)

				if useDefaultFromTag {
					val, err := valueparser.ParseMap[bool, float64](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolBoolMap:
				mapCopy := make(map[bool]bool)

				if useDefaultFromTag {
					val, err := valueparser.ParseMap[bool, bool](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolByteSliceMap:
				mapCopy := make(map[bool][]byte)

				if useDefaultFromTag {
					val, err := valueparser.ParseMap[bool, []byte](defaultValStr, nil, nil)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					copyMap(reflect.ValueOf(val), fieldVal)
				}

				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case invalidMap:
				log.Warnf("Unsupported map type for field %s", field.Name)
			}

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if useDefaultFromTag {
				val, err := valueparser.TryUnmarshal[int64](defaultValStr, field.Type)
				if err != nil {
					val, err = valueparser.ParseValue[int64](defaultValStr)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					fieldVal.SetInt(val)
				} else {
					fieldVal.SetInt(val)
				}
			}

			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[int64](value, field.Type)
			if err == nil {
				fieldVal.SetInt(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to int64: %v", value, err)
			}

			fieldVal.SetInt(GetEnv(envKey, fieldVal.Int(), required, log))

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if useDefaultFromTag {
				val, err := valueparser.TryUnmarshal[uint64](defaultValStr, field.Type)
				if err != nil {
					val, err = valueparser.ParseValue[uint64](defaultValStr)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					fieldVal.SetUint(val)
				} else {
					fieldVal.SetUint(val)
				}
			}

			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[uint64](value, field.Type)
			if err == nil {
				fieldVal.SetUint(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to uint64: %v", value, err)
			}

			fieldVal.SetUint(GetEnv(envKey, fieldVal.Uint(), required, log))

		case reflect.Float32, reflect.Float64:
			if useDefaultFromTag {
				val, err := valueparser.TryUnmarshal[float64](defaultValStr, field.Type)
				if err != nil {
					val, err = valueparser.ParseValue[float64](defaultValStr)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					fieldVal.SetFloat(val)
				} else {
					fieldVal.SetFloat(val)
				}
			}

			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[float64](value, field.Type)
			if err == nil {
				fieldVal.SetFloat(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to float64: %v", value, err)
			}

			fieldVal.SetFloat(GetEnv(envKey, fieldVal.Float(), required, log))

		case reflect.Bool:
			if useDefaultFromTag {
				val, err := valueparser.TryUnmarshal[bool](defaultValStr, field.Type)
				if err != nil {
					val, err = valueparser.ParseValue[bool](defaultValStr)
					if err != nil {
						log.Fatalf("Failed to parse default value tag for field %s: %v", field.Name, err)
					}

					fieldVal.SetBool(val)
				} else {
					fieldVal.SetBool(val)
				}
			}

			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[bool](value, field.Type)
			if err == nil {
				fieldVal.SetBool(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to bool: %v", value, err)
			}

			fieldVal.SetBool(GetEnv(envKey, fieldVal.Bool(), required, log))

		case reflect.String:
			if useDefaultFromTag {
				val, err := valueparser.TryUnmarshal[string](defaultValStr, field.Type)
				if err != nil {
					fieldVal.SetString(defaultValStr)
				} else {
					fieldVal.SetString(val)
				}
			}

			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[string](value, field.Type)
			if err == nil {
				fieldVal.SetString(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to string: %v", value, err)
			}

			fieldVal.SetString(GetEnv(envKey, fieldVal.String(), required, log))

		case reflect.Slice:
			switch field.Type.Elem().Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				array := make([]int64, fieldVal.Len())
				copyArray(fieldVal, reflect.ValueOf(array))
				array = GetEnvArray(envKey, array, nil, required, log)
				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				array := make([]uint64, fieldVal.Len())
				copyArray(fieldVal, reflect.ValueOf(array))
				array = GetEnvArray(envKey, array, nil, required, log)
				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.Float32, reflect.Float64:
				array := make([]float64, fieldVal.Len())
				copyArray(fieldVal, reflect.ValueOf(array))
				array = GetEnvArray(envKey, array, nil, required, log)
				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.Bool:
				array := make([]bool, fieldVal.Len())
				copyArray(fieldVal, reflect.ValueOf(array))
				array = GetEnvArray(envKey, array, nil, required, log)
				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.String:
				array := make([]string, fieldVal.Len())
				copyArray(fieldVal, reflect.ValueOf(array))
				array = GetEnvArray(envKey, array, nil, required, log)
				copyArray(reflect.ValueOf(array), fieldVal)

			case reflect.Slice:
				if field.Type.Elem().Elem().Kind() == reflect.Uint8 {
					array := make([][]byte, fieldVal.Len())
					copyArray(fieldVal, reflect.ValueOf(array))
					array = GetEnvArray(envKey, array, nil, required, log)
					copyArray(reflect.ValueOf(array), fieldVal)
				} else {
					log.Warnf("Unsupported slice type for field %s", field.Name)
				}

			case reflect.Invalid,
				reflect.Chan,
				reflect.Func,
				reflect.Interface,
				reflect.Ptr,
				reflect.Struct,
				reflect.Uintptr,
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
			reflect.Struct,
			reflect.Uintptr,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Array,
			reflect.UnsafePointer:
			log.Warnf("Unsupported field type for field %s", field.Name)

		default:
			log.Warnf("Unsupported field type for field %s", field.Name)
		}
	}
}
