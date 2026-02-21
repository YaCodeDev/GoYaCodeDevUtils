package yaautoflags

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// PackFlags packs boolean fields of a struct into a single flags field.
// The struct must have a field named "Flags" of type uint64, uint32, uint16, uint8, uint or uintptr.
// The boolean fields are packed into the flags field, where each bit represents a boolean field.
// The first boolean field corresponds to the least significant bit of the flags field.
// If the number of boolean fields exceeds the size of the flags field, an error is returned.
// The flags field must be of a type that can hold the number of boolean fields.
// If the flags field is not found or is of an incorrect type, an error is returned.
//
// Example usage:
//
//	type MyFlags struct {
//	    A, B, C bool
//	    Flags  uint8 // or uint64, uint32, uint16, uint, uintptr
//	}
//
//	err := PackFlags(&MyFlags{A: true, B: false, C: true})
//
//	if err != nil {
//	    // handle error
//	}
func PackFlags[T any](instance *T) yaerrors.Error {
	if instance == nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrInstanceNil,
			"pack flags",
		)
	}

	reflectValue := reflect.ValueOf(instance).Elem()
	reflectType := reflectValue.Type()

	if reflectValue.Kind() != reflect.Struct {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrInstanceNotStruct,
			"pack flags",
		)
	}

	var flagsField reflect.Value

	var nextFlagIndex uint8

	var flags uint64

	var flagsSize uint8

	for i := range reflectValue.NumField() {
		fieldValue := reflectValue.Field(i)
		fieldType := reflectType.Field(i)

		if fieldValue.Kind() == reflect.Bool {
			if fieldValue.Bool() {
				flags |= 1 << nextFlagIndex
			}

			nextFlagIndex++
		}

		if fieldType.Name == flagsFieldName {
			flagsField = fieldValue
			flagsSize = uint8( //nolint:gosec,lll // There is no overflow risk here, as the maximum size of any uint type is 8 bytes, which is well within the limits of uint8
				flagsField.Type().Size() * bitsInByte,
			)

			//nolint:exhaustive // The flags field must be of an unsigned integer type, so only those types are handled
			switch flagsField.Kind() {
			case reflect.Uint64,
				reflect.Uint32,
				reflect.Uint16,
				reflect.Uint8,
				reflect.Uint,
				reflect.Uintptr:
			default:
				return yaerrors.FromError(
					http.StatusInternalServerError,
					ErrFlagsFieldTypeMismatch,
					"pack flags: got "+flagsField.Kind().String(),
				)
			}
		}
	}

	if flagsSize == 0 {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrFlagsFieldNotFound,
			"pack flags",
		)
	}

	if nextFlagIndex >= flagsSize {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrFlagsFieldNotFound,
			fmt.Sprintf(
				"pack flags: maximum is %d for %s",
				flagsSize,
				flagsField.Kind().String(),
			),
		)
	}

	flagsField.SetUint(flags)

	return nil
}

// UnpackFlags unpacks a flags field into boolean fields of a struct.
// The struct must have a field named "Flags" of type uint64, uint32, uint16, uint8, uint or uintptr.
// The boolean fields are unpacked from the flags field, where each bit represents a boolean field.
// The first boolean field corresponds to the least significant bit of the flags field.
// If the number of boolean fields exceeds the size of the flags field, an error is returned.
// The flags field must be of a type that can hold the number of boolean fields.
// If the flags field is not found or is of an incorrect type, an error is returned.
// Example usage:
//
//	type MyFlags struct {
//		A, B, C bool
//		Flags  uint8 // or uint64, uint32, uint16, uint, uintptr
//	}
//
//	err := UnpackFlags(&MyFlags{Flags: 0b101})
//
//	err != nil {
//		// handle error
//	}
func UnpackFlags[T any](instance *T) yaerrors.Error {
	if instance == nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrInstanceNil,
			"unpack flags",
		)
	}

	reflectValue := reflect.ValueOf(instance).Elem()

	if reflectValue.Kind() != reflect.Struct {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrInstanceNotStruct,
			"unpack flags",
		)
	}

	flagsField := reflectValue.FieldByName(flagsFieldName)

	if !flagsField.IsValid() {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrFlagsFieldNotFound,
			"unpack flags",
		)
	}

	//nolint:exhaustive // The flags field must be of an unsigned integer type, so only those types are handled
	switch flagsField.Kind() {
	case reflect.Uint64,
		reflect.Uint32,
		reflect.Uint16,
		reflect.Uint8,
		reflect.Uint,
		reflect.Uintptr:
	default:
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrFlagsFieldTypeMismatch,
			"unpack flags: got "+flagsField.Kind().String(),
		)
	}

	flags := flagsField.Uint()

	flagsSize := uint8( //nolint:gosec,lll // There is no overflow risk here, as the maximum size of any uint type is 8 bytes, which is well within the limits of uint8
		flagsField.Type().Size() * bitsInByte,
	)

	var nextFlagIndex uint8

	for i := range reflectValue.NumField() {
		fieldValue := reflectValue.Field(i)

		if fieldValue.Kind() != reflect.Bool {
			continue
		}

		if nextFlagIndex >= flagsSize {
			return yaerrors.FromError(
				http.StatusInternalServerError,
				ErrTooManyFlags,
				fmt.Sprintf(
					"unpack flags: maximum is %d for %s",
					flagsSize,
					flagsField.Kind().String(),
				),
			)
		}

		val := (flags>>nextFlagIndex)&1 == 1
		fieldValue.SetBool(val)

		nextFlagIndex++
	}

	return nil
}
