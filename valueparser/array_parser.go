package valueparser

import (
	"fmt"
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
		if parsed, err = ParseValue[T](trimmed); err == nil {
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
