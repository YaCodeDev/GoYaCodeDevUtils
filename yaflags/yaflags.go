package yaflags

import (
	"fmt"
	"math/bits"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

type uints interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint
}

func typeBitSize[T uints]() uint8 {
	var zero T

	return uint8(bits.Len64(uint64(^zero)))
}

// PackBitIndexes packs a list of bit positions into an unsigned integer of type T.
// The function returns an error if the number of bits exceeds the size of T or if a bit index
// is outside of the allowable range for T.
func PackBitIndexes[T uints](bits []uint8) (T, yaerrors.Error) {
	var flags T

	maxBits := typeBitSize[T]()

	if len(bits) > int(maxBits) {
		return 0, yaerrors.FromError(
			http.StatusBadRequest,
			ErrTooManyBits,
			fmt.Sprintf(
				"pack bits: received %d bits but %T supports at most %d",
				len(bits),
				flags,
				maxBits,
			),
		)
	}

	for _, bit := range bits {
		if bit >= maxBits {
			return 0, yaerrors.FromError(
				http.StatusBadRequest,
				ErrBitIndexOutOfRange,
				fmt.Sprintf(
					"pack bits: index %d out of range for %T (%d bits)",
					bit,
					flags,
					maxBits,
				),
			)
		}

		flags |= T(1) << bit
	}

	return flags, nil
}

// UnpackBitIndexes unpacks a list of bit positions from an unsigned integer of type T.
func UnpackBitIndexes[T uints](flags T) []uint8 {
	if flags == 0 {
		return nil
	}

	maxBits := typeBitSize[T]()

	result := make([]uint8, 0, bits.OnesCount64(uint64(flags)))

	for bit := range maxBits {
		if flags&(T(1)<<bit) != 0 {
			result = append(result, bit)
		}
	}

	return result
}
