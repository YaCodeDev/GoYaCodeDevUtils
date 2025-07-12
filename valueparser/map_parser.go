package valueparser

import (
	"fmt"
	"strings"
)

// ParseMapFrom parses a string into a map[K]V using the provided separators.
// It splits the string by 'entrySeparator' and each entry by 'kvSeparator'.
// If 'entrySeparator' is nil, it defaults to DefaultEntrySeparator.
// If 'kvSeparator' is nil, it defaults to DefaultKVSeparator.
// If the string is empty, it returns an empty map.
//
// Example usage:
//
//	var myMap map[string]int
//	myMap, err := ParseMap[string, int]("key1:1,key2:1", nil, nil)
//	if err != nil {
//		// Handle error
//	}
func ParseMap[K ParsableComparableType, V ParsableType](
	str string,
	entrySeparator *string,
	kvSeparator *string,
) (map[K]V, error) {
	result := make(map[K]V)

	if str == "" {
		return result, nil
	}

	var (
		k   K
		v   V
		err error
	)

	if entrySeparator == nil {
		s := DefaultEntrySeparator
		entrySeparator = &s
	}

	if kvSeparator == nil {
		s := DefaultKVSeparator
		kvSeparator = &s
	}

	entries := strings.SplitSeq(str, *entrySeparator)
	for item := range entries {
		parts := strings.Split(item, *kvSeparator)
		if len(parts) == MapPartsCount {
			k, err = ParseValue[K](strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf(
					"failed to parse key '%s': %w",
					strings.TrimSpace(parts[0]),
					err,
				)
			}

			v, err = ParseValue[V](strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf(
					"failed to parse value '%s': %w",
					strings.TrimSpace(parts[1]),
					err,
				)
			}

			result[k] = v
		} else {
			return nil, fmt.Errorf("%w: expected %d parts, got %d", ErrInvalidEntry, MapPartsCount, len(parts))
		}
	}

	return result, nil
}
