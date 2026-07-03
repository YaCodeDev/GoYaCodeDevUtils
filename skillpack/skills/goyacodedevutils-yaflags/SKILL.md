---
name: goyacodedevutils-yaflags
description: Pack/unpack a list of individual bit-index positions into/from an unsigned integer. Use for named bit-constant flag sets instead of hand-rolled bit-shift arithmetic.
---

# yaflags Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaflags`.

Packs/unpacks a list of individual bit-index positions (not struct fields) into/from an unsigned integer.

## Key API

- `PackBitIndexes[T uints](bits []uint8) (T, yaerrors.Error)` — sets each given bit index in a `T`.
- `UnpackBitIndexes[T uints](flags T) []uint8` — returns the set bit indexes.
- `T` is constrained to `~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint`.
- `ErrTooManyBits`, `ErrBitIndexOutOfRange`.

## Usage Notes

- Different from `yaautoflags`: this works with raw bit indexes (e.g. `[]uint8{0, 3, 5}`), not struct bool fields — use it when flags are defined as named bit constants rather than as struct fields.
- Errors if any index is `>=` the bit-width of `T`, or if `len(bits)` exceeds the bit-width. Depends only on `yaerrors`.
