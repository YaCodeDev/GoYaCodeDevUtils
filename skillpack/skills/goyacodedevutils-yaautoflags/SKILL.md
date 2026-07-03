---
name: goyacodedevutils-yaautoflags
description: Reflection-based bit-packing of a struct's bool fields into/out of a single unsigned integer Flags field. Use when a struct's bools should serialize/compare as one integer.
---

# yaautoflags Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaautoflags`.

Reflection-based bit-packing of a struct's bool fields into/out of a single unsigned integer `Flags` field.

## Key API

- `PackFlags[T](instance *T) yaerrors.Error` — packs bool fields, in declaration order, into bit positions of the `Flags` field.
- `UnpackFlags[T](instance *T) yaerrors.Error` — reverse operation.
- `ErrInstanceNil`, `ErrInstanceNotStruct`, `ErrFlagsFieldNotFound`, `ErrFlagsFieldTypeMismatch`, `ErrTooManyFlags`.

## Usage Notes

- The struct must have an exported field literally named `Flags` of type `uint`/`uint8`/`16`/`32`/`64`/`uintptr`. The number of bool fields must not exceed that field's bit width, or `PackFlags`/`UnpackFlags` returns an error.
- Bit order follows struct field declaration order (first bool = least significant bit). The struct is mutated in place via reflection — always pass a pointer.
- Depends only on `yaerrors`. For flags defined as named bit constants (not struct bool fields), use `yaflags` instead.
