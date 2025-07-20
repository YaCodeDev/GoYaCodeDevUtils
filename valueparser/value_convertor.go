package valueparser

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// ConvertValue converts a reflect.Value to the specified target type.
// It checks if the value is valid and convertible to the target type.
// If the value is valid and convertible, it returns the converted value.
// If the value is invalid, it returns a zero value of the target type.
// If the value is valid but not convertible, an error is returned.
//
// Example usage:
//
// converted, err := ConvertValue(reflect.ValueOf(42), reflect.TypeOf(float64(0)))
//
//	if err != nil {
//	    // handle error
//	}
func ConvertValue(val reflect.Value, targetType reflect.Type) (reflect.Value, yaerrors.Error) {
	if !val.IsValid() {
		return reflect.Zero(targetType), yaerrors.FromError(
			http.StatusInternalServerError,
			ErrInvalidValue,
			"convert value: value is invalid",
		)
	}

	if val.Type().ConvertibleTo(targetType) {
		return val.Convert(targetType), nil
	}

	return reflect.Zero(targetType), yaerrors.FromError(
		http.StatusInternalServerError,
		ErrUnconvertibleType,
		fmt.Sprintf(
			"convert value: %s is not convertible to %s",
			val.Type().String(),
			targetType.String(),
		),
	)
}
