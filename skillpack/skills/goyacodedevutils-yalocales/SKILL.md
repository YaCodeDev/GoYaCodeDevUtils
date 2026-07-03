---
name: goyacodedevutils-yalocales
description: Loads JSON locale files into a nested key-tree with lookup, {placeholder} formatting, JSON serving, language-tag normalization, and Go-struct codegen. Use for any i18n/localization instead of hand-rolled JSON locale loading.
---

# yalocales Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yalocales`.

Loads JSON locale files from an `fs.FS` into a nested key-tree, exposing lookup, `{placeholder}` formatting,
JSON serving, language-tag normalization, and Go-struct codegen for compile-time-safe key access.

## Key API

- `Localizer` interface — `LoadLocales(fs.FS)`, `GetDefaultLangValueByCompositeKey`, `GetValueByCompositeKeyAndLang`, `GetFormattedValueByCompositeKeyAndLang`, `GetJSONByCompositeKeyAndLang`/`GetDefaultLangJSONByCompositeKey`, `DeriveNewDefaultLang(newLang) (Localizer, error)`.
- `NewLocalizer(fallbackLang string, enforceLocaleConsistency bool) Localizer`.
- `NewYaLocalizer(...) *YaLocalizer` — concrete type; also has `GenerateLocaleCode`/`HelperRepresentLocaleAsStruct` for Go codegen and `GetSupportedLanguages`.
- `SupportedLanguage` struct — `{ Code, Emoji, Label }`; `LoadSupportedLanguages(fs.FS) ([]SupportedLanguage, error)`.
- `IsSupportedLanguage`/`LookupSupportedLanguage`/`NormalizeLanguageTag`/`SupportedLanguageCodes` helpers.
- `const Separator = "."` (composite key delimiter).
- `ErrInvalidLanguage`, `ErrMismatchedKeys`, `ErrMismatchedPlaceholders`, and other `Err*` vars.

## Usage Notes

- `LoadLocales` enforces strict cross-language consistency: every language must have identical key sets **and** identical `{placeholder}` sets per key, or loading fails — this catches missing translations at startup.
- Composite keys use `"."` to address nested JSON (e.g. `"greeting.hello"`); locale files are one JSON file per language, optionally nested in subfolders (the folder path becomes the key prefix).
- `enforceLocaleConsistency = true` is required to use `DeriveNewDefaultLang` (used by `yatgbot` for per-user language switching). Depends only on `yaerrors` + `golang.org/x/text`.
