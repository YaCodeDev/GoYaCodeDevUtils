package valueparser

import (
	"fmt"
	"strings"
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
) ([]T, error) {
	if str == "" {
		return []T{}, nil
	}

	if separator == nil {
		s := DefaultEntrySeparator
		separator = &s
	}

	var (
		parsed T
		err    error
	)

	parts := strings.Split(str, *separator)
	result := make([]T, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if parsed, err = ParseValue[T](trimmed); err == nil {
			result = append(result, parsed)
		} else {
			return nil, fmt.Errorf("failed to parse part '%s': %w", trimmed, err)
		}
	}

	return result, nil
}
