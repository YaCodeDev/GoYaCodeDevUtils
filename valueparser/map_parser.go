package valueparser

import (
	"fmt"
	"net/http"
	"reflect"
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
	return ParseMapWithCustomType[K, V](
		str,
		entrySeparator,
		kvSeparator,
		reflect.TypeOf(new(K)).Elem(),
		reflect.TypeOf(new(V)).Elem(),
	)
}

// ParseMapWithCustomType is a generic function that parses a string into a map[K]V
// using the provided separators and types for keys and values.
// It splits the string by 'entrySeparator' and each entry by 'kvSeparator'.
// If 'entrySeparator' is nil, it defaults to DefaultEntrySeparator.
// If 'kvSeparator' is nil, it defaults to DefaultKVSeparator.
// If the string is empty, it returns an empty map.
// It is useful when you need to specify custom types for parsing keys and/or values.
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
//	customMap, err := ParseMapWithCustomType[uint64, int](
//		"FIRST:1,SECOND:2",
//		nil,
//		nil,
//		reflect.TypeOf(YourCustomType(0)),
//		reflect.TypeOf(0),
//	)
//	if err != nil {
//		// Handle error
//	}
func ParseMapWithCustomType[K ParsableComparableType, V ParsableType](
	str string,
	entrySeparator *string,
	kvSeparator *string,
	kType reflect.Type,
	vType reflect.Type,
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
			k, err = ParseValueWithCustomType[K](strings.TrimSpace(parts[0]), kType)
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

			v, err = ParseValueWithCustomType[V](strings.TrimSpace(parts[1]), vType)
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
