package valueparser

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
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
) (map[K]V, yaerrors.Error) {
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
				return nil, yaerrors.FromError(
					http.StatusInternalServerError,
					err,
					fmt.Sprintf(
						"parse map: failed to parse key '%s'",
						strings.TrimSpace(parts[0]),
					),
				)
			}

			v, err = ParseValue[V](strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, yaerrors.FromError(
					http.StatusInternalServerError,
					err,
					fmt.Sprintf(
						"parse map: failed to parse value '%s'",
						strings.TrimSpace(parts[1]),
					),
				)
			}

			result[k] = v
		} else {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				ErrInvalidEntry,
				fmt.Sprintf(
					"parse map: expected %d parts, got %d",
					MapPartsCount,
					len(parts),
				),
			)
		}
	}

	return result, nil
}
