package valueparser

// ParsableType is a type constraint that allows for any type that can be parsed from a string.
// It includes basic types like string, int, float, and bool,
// as well as slices of bytes (for byte arrays).
type ParsableType interface {
	ParsableComparableType | ~[]byte
}

// ParsableComparableType is a type constraint that allows for any comparable type.
// It includes basic types like string, int, float, and bool,
// but excludes slices and maps, which are not comparable in Go.
type ParsableComparableType interface {
	~string | ~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~bool
}
