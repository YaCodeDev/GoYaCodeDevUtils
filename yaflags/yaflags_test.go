package yaflags

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

type customFlags uint16

func TestPackBitIndexesSuccess(t *testing.T) {
	t.Parallel()

	flags, err := PackBitIndexes[uint16]([]uint8{0, 3, 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const want uint16 = (1 << 0) | (1 << 3) | (1 << 7)
	if flags != want {
		t.Fatalf("wrong flags: want %016b, got %016b", want, flags)
	}
}

func TestPackBitIndexesTooManyBits(t *testing.T) {
	t.Parallel()

	_, err := PackBitIndexes[uint8]([]uint8{0, 1, 2, 3, 4, 5, 6, 7, 0})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrTooManyBits) {
		t.Fatalf("expected ErrTooManyBits, got %v", err)
	}

	if err.Code() != http.StatusBadRequest {
		t.Fatalf("expected HTTP status %d in error code, got %d", http.StatusBadRequest, err.Code())
	}
}

func TestPackBitIndexesBitIndexOutOfRange(t *testing.T) {
	t.Parallel()

	_, err := PackBitIndexes[uint8]([]uint8{0, 8})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrBitIndexOutOfRange) {
		t.Fatalf("expected ErrBitIndexOutOfRange, got %v", err)
	}
}

func TestUnpackBitIndexesReturnsSortedIndexes(t *testing.T) {
	t.Parallel()

	const value customFlags = (1 << 0) | (1 << 4) | (1 << 9)

	got := UnpackBitIndexes(value)
	want := []uint8{0, 4, 9}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestUnpackBitIndexesZeroValue(t *testing.T) {
	t.Parallel()

	got := UnpackBitIndexes[uint32](0)
	if got != nil {
		t.Fatalf("expected nil slice, got %v", got)
	}
}
