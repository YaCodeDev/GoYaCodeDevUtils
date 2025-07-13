package valueparser

import (
	"net/http"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// ConvertValue converts a reflect.Value to the specified target type.
// It checks if the value is valid and convertible to the target type.
// If the value is valid and convertible, it returns the converted value.
// If the value is invalid, it returns a zero value of the target type.
// If the value is valid but not convertible, it panics with an error message.
func ConvertValue(val reflect.Value, targetType reflect.Type) (reflect.Value, yaerrors.Error) {
	if !val.IsValid() {
		return reflect.Zero(targetType), nil
	}

	if val.Type().ConvertibleTo(targetType) {
		return val.Convert(targetType), nil
	}

	return reflect.Value{}, yaerrors.FromError(
		http.StatusInternalServerError,
		ErrInvalidValue,
		"convert value: value is not convertible to target type",
	)
}
