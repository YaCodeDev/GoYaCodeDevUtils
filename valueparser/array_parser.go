package valueparser

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// ParseArray splits a string by 'separator' and parses each part into T.
// If the string is empty, it returns an empty slice.
// If 'separator' is nil, it defaults to DefaultEntrySeparator.
//
// Example usage:
//
//	var myArray []int
//	myArray, err := ParseArray[int]("1,2,3", nil)
//	if err != nil {
//		// Handle error
//	}
func ParseArray[T ParsableType](
	str string,
	separator *string,
) ([]T, yaerrors.Error) {
	return ParseArrayWithCustomType[T](str, separator, reflect.TypeFor[T]())
}

// ParseArrayWithCustomType is a generic function that splits a string by 'separator'
// and parses each part into T using the provided type for conversion.
// If the string is empty, it returns an empty slice.
// If 'separator' is nil, it defaults to DefaultEntrySeparator.
// It is useful when you need to specify a custom type for parsing.
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
//	customValue, err := ParseArrayWithCustomType[uint64]("FIRST,SECOND", nil, reflect.TypeOf(YourCustomType(0)))
//	if err != nil {
//		// Handle error
//	}
func ParseArrayWithCustomType[T ParsableType](
	str string,
	separator *string,
	vType reflect.Type,
) ([]T, yaerrors.Error) {
	if str == "" {
		return []T{}, nil
	}

	if separator == nil {
		s := DefaultEntrySeparator
		separator = &s
	}

	var (
		parsed T
		err    yaerrors.Error
	)

	parts := strings.Split(str, *separator)
	result := make([]T, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if parsed, err = ParseValueWithCustomType[T](trimmed, vType); err == nil {
			result = append(result, parsed)
		} else {
			return nil, err.Wrap(
				fmt.Sprintf(
					"parse array: failed to parse part '%s'",
					trimmed,
				),
			)
		}
	}

	return result, nil
}
