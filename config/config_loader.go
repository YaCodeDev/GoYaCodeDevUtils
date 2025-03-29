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
// It supports various field types including maps, slices, and basic types (int, uint, float, bool, string).
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

		if !fieldVal.CanSet() {
			log.Warnf("Field %s cannot be set", field.Name)

			continue
		}

		envKey := toScreamingSnakeCase(field.Name)
		required := fieldVal.IsZero()

		switch field.Type.Kind() {
		case reflect.Map:
			mapType := getMapType(fieldVal)
			switch mapType {
			case stringStringMap:
				mapCopy := make(map[string]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringIntMap:
				mapCopy := make(map[string]int64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringUintMap:
				mapCopy := make(map[string]uint64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringFloatMap:
				mapCopy := make(map[string]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringBoolMap:
				mapCopy := make(map[string]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case stringByteSliceMap:
				mapCopy := make(map[string][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intStringMap:
				mapCopy := make(map[int]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intIntMap:
				mapCopy := make(map[int]int)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intUintMap:
				mapCopy := make(map[int]uint)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intFloatMap:
				mapCopy := make(map[int]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intBoolMap:
				mapCopy := make(map[int]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case intByteSliceMap:
				mapCopy := make(map[int][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintStringMap:
				mapCopy := make(map[uint]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintIntMap:
				mapCopy := make(map[uint]int)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintUintMap:
				mapCopy := make(map[uint]uint)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintFloatMap:
				mapCopy := make(map[uint]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintBoolMap:
				mapCopy := make(map[uint]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case uintByteSliceMap:
				mapCopy := make(map[uint][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatStringMap:
				mapCopy := make(map[float64]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatIntMap:
				mapCopy := make(map[float64]int)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatUintMap:
				mapCopy := make(map[float64]uint)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatFloatMap:
				mapCopy := make(map[float64]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatBoolMap:
				mapCopy := make(map[float64]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case floatByteSliceMap:
				mapCopy := make(map[float64][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolStringMap:
				mapCopy := make(map[bool]string)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolIntMap:
				mapCopy := make(map[bool]int)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolUintMap:
				mapCopy := make(map[bool]uint)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolFloatMap:
				mapCopy := make(map[bool]float64)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolBoolMap:
				mapCopy := make(map[bool]bool)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case boolByteSliceMap:
				mapCopy := make(map[bool][]byte)
				copyMap(fieldVal, reflect.ValueOf(mapCopy))
				mapCopy = GetEnvMap(envKey, mapCopy, required, nil, nil, log)
				copyMap(reflect.ValueOf(mapCopy), fieldVal)

			case invalidMap:
				log.Warnf("Unsupported map type for field %s", field.Name)
			}

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[int64](value)
			if err != nil {
				fieldVal.SetInt(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to int64: %v", value, err)
			}

			fieldVal.SetInt(GetEnv(envKey, fieldVal.Int(), required, log))

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[uint64](value)
			if err != nil {
				fieldVal.SetUint(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to uint64: %v", value, err)
			}

			fieldVal.SetUint(GetEnv(envKey, fieldVal.Uint(), required, log))

		case reflect.Float32, reflect.Float64:
			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[float64](value)
			if err != nil {
				fieldVal.SetFloat(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to float64: %v", value, err)
			}

			fieldVal.SetFloat(GetEnv(envKey, fieldVal.Float(), required, log))

		case reflect.Bool:
			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[bool](value)
			if err != nil {
				fieldVal.SetBool(val)

				continue
			}

			if !errors.Is(err, valueparser.ErrUnparsableValue) {
				log.Warnf("Failed to unmarshal value %s to bool: %v", value, err)
			}

			fieldVal.SetBool(GetEnv(envKey, fieldVal.Bool(), required, log))

		case reflect.String:
			value := GetEnv(envKey, "", false, log)

			val, err := valueparser.TryUnmarshal[string](value)
			if err != nil {
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
