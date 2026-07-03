---
name: goyacodedevutils-valueparser
description: Generic string-to-typed-value parsing for scalars, arrays, and maps, including custom types via Unmarshalable/encoding.TextUnmarshaler. Foundational package powering config; use instead of hand-rolled strconv/strings.Split parsing.
---

# valueparser Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/valueparser`.

Generic string-to-typed-value parsing (scalars, arrays, maps) with support for custom types via an
`Unmarshalable` interface or `encoding.TextUnmarshaler`.

## Key API

- `ParsableType` — constraint covering scalar kinds plus `~[]byte`; `ParsableComparableType` — scalars only.
- `Unmarshalable` interface — `Unmarshal(string) error`.
- `ParseValue[T]` / `ParseValueWithCustomType[T](value string, reflect.Type)` — parse a single value.
- `ParseArray[T]` / `ParseArrayWithCustomType[T](str string, *separator, type)` — default separator `","`.
- `ParseMap[K, V]` / `ParseMapWithCustomType[K, V](str, *entrySep, *kvSep, kType, vType)` — default entry separator `","`, key/value separator `":"`.
- `TryUnmarshal[T](value string, type)` — tries `encoding.TextUnmarshaler` first, then `Unmarshalable`.
- `ConvertValue(reflect.Value, targetType)`.
- `const MapPartsCount = 2`, `DefaultKVSeparator = ":"`, `DefaultEntrySeparator = ","`.
- `ErrUnknownType`, `ErrInvalidValue`, `ErrUnconvertibleType`, `ErrInvalidType`, `ErrUnparsableValue`, `ErrInvalidEntry`.

## Usage Notes

- For a custom (named) type, implement either `UnmarshalText([]byte) error` or `Unmarshal(string) error` to plug into `ParseValueWithCustomType`; for a `string`-kinded type, both `Unmarshalable` and a direct string cast are tried.
- Foundational package: depends only on `yaerrors`. Widely depended upon — `config` and (indirectly, via `config`) `yahash` and the `yatg*` packages all use it.
