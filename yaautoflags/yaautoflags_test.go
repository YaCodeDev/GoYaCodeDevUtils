package yaautoflags

import (
	"errors"
	"reflect"
	"testing"
)

func assertErrIs(t *testing.T, got error, want error) {
	t.Helper()

	if !errors.Is(got, want) {
		t.Fatalf("expected error %v, got %v", want, got)
	}
}

func assertNoErr(t *testing.T, got error) {
	t.Helper()

	if got != nil {
		t.Fatalf("unexpected error: %v", got)
	}
}

type noFlags struct {
	B0 bool
}

type wrongFlagsType struct {
	B0    bool
	Flags int64
}

type justEnough struct {
	B0, B1, B2, B3 bool
	Flags          uint8
}

type overflowUint8 struct {
	B0, B1, B2, B3, B4, B5, B6, B7, B8 bool
	Flags                              uint8
}

func TestPackFlags_NilInstance(t *testing.T) {
	var s *justEnough // typed nil ptr
	assertErrIs(t, PackFlags(s), ErrInstanceNil)
}

func TestPackFlags_NotStruct(t *testing.T) {
	v := 7
	assertErrIs(t, PackFlags(&v), ErrInstanceNotStruct)
}

func TestPackFlags_FlagsFieldNotFound(t *testing.T) {
	assertErrIs(t, PackFlags(&noFlags{B0: true}), ErrFlagsFieldNotFound)
}

func TestPackFlags_FlagsFieldWrongType(t *testing.T) {
	assertErrIs(t, PackFlags(&wrongFlagsType{B0: true}), ErrFlagsFieldTypeMismatch)
}

func TestPackFlags_TooManyBoolsForUint8(t *testing.T) {
	inst := &overflowUint8{B0: true, B8: true}
	assertErrIs(t, PackFlags(inst), ErrFlagsFieldNotFound)
}

func TestPackFlags_HappyPath(t *testing.T) {
	inst := &justEnough{B0: true, B1: false, B2: true, B3: true}
	assertNoErr(t, PackFlags(inst))

	const want uint8 = 0b1101
	if inst.Flags != want {
		t.Fatalf("expected Flags=%08b, got %08b", want, inst.Flags)
	}
}

func TestUnpackFlags_NilInstance(t *testing.T) {
	var s *justEnough
	assertErrIs(t, UnpackFlags(s), ErrInstanceNil)
}

func TestUnpackFlags_NotStruct(t *testing.T) {
	v := 42
	assertErrIs(t, UnpackFlags(&v), ErrInstanceNotStruct)
}

func TestUnpackFlags_FlagsFieldNotFound(t *testing.T) {
	assertErrIs(t, UnpackFlags(&noFlags{}), ErrFlagsFieldNotFound)
}

func TestUnpackFlags_FlagsFieldWrongType(t *testing.T) {
	assertErrIs(t, UnpackFlags(&wrongFlagsType{Flags: 1}), ErrFlagsFieldTypeMismatch)
}

func TestUnpackFlags_TooManyFlagsToUnpack(t *testing.T) {
	assertErrIs(t, UnpackFlags(&overflowUint8{Flags: 0xFF}), ErrTooManyFlags)
}

func TestUnpackFlags_HappyPath(t *testing.T) {
	src := &justEnough{Flags: 0b1010}
	assertNoErr(t, UnpackFlags(src))

	want := &justEnough{B1: true, B3: true, Flags: 0b1010}
	if !reflect.DeepEqual(src, want) {
		t.Fatalf("unpack failed: want %+v, got %+v", want, src)
	}
}

func TestRoundTrip_PackThenUnpack(t *testing.T) {
	orig := &justEnough{B0: true, B1: true, B2: false, B3: true}
	assertNoErr(t, PackFlags(orig))

	cloned := &justEnough{Flags: orig.Flags}
	assertNoErr(t, UnpackFlags(cloned))

	if !reflect.DeepEqual(orig, cloned) {
		t.Fatalf("round-trip mismatch: want %+v, got %+v", orig, cloned)
	}
}
