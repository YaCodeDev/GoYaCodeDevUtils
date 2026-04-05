package yalocales

import (
	"errors"
	"io/fs"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// SupportedLanguage represents a locale exposed by the localizer.
// Code stores the normalized locale identifier loaded into the localizer.
// Emoji stores the inferred flag or fallback symbol for that locale.
// Label stores the human-readable language name.
type SupportedLanguage struct {
	Code  string
	Emoji string
	Label string
}

const (
	regionalIndicatorOffset = 0x1F1E6
	latinUppercaseOffset    = 'A'
	fallbackLanguageEmoji   = "🌐"
	flagRegionCodeLength    = 2
	flagEmojiBuilderSize    = 8
)

var errNoSupportedLanguages = errors.New("no locale files found")

// LoadSupportedLanguages loads locale files through YaLocalizer and returns metadata
// for every loaded language.
// It accepts the same filesystem layouts supported by YaLocalizer.LoadLocales.
//
// Example usage:
//
//	langs, err := yalocales.LoadSupportedLanguages(localesFS)
//	if err != nil {
//	    // handle error
//	}
func LoadSupportedLanguages(localesFS fs.FS) ([]SupportedLanguage, error) {
	localizer := NewYaLocalizer("", false)
	if yaErr := localizer.LoadLocales(localesFS); yaErr != nil {
		return nil, yaErr
	}

	langs := localizer.GetSupportedLanguages()
	if len(langs) == 0 {
		return nil, errNoSupportedLanguages
	}

	return langs, nil
}

// NormalizeLanguageTag normalizes a raw language tag to its base language code.
// It trims whitespace, lowercases the value, replaces underscores with hyphens,
// and drops any region suffix.
//
// Example usage:
//
//	code := yalocales.NormalizeLanguageTag("uk-UA")
//	// code == "uk"
func NormalizeLanguageTag(raw string) string {
	raw = normalizeStoredLanguageTag(raw)
	if raw == "" {
		return ""
	}

	if idx := strings.IndexByte(raw, '-'); idx >= 0 {
		raw = raw[:idx]
	}

	return raw
}

// LookupSupportedLanguage finds the best supported language match for the provided raw tag.
// It matches exact tags, base tags, and canonical aliases such as "ua" and "uk".
//
// Example usage:
//
//	lang, ok := yalocales.LookupSupportedLanguage(langs, "uk-UA")
//	if !ok {
//	    // handle missing language
//	}
func LookupSupportedLanguage(langs []SupportedLanguage, raw string) (SupportedLanguage, bool) {
	normalized := normalizeStoredLanguageTag(raw)
	if normalized == "" {
		return SupportedLanguage{}, false
	}

	for _, lang := range langs {
		if languageTagsMatch(lang.Code, normalized) {
			return lang, true
		}
	}

	return SupportedLanguage{}, false
}

// IsSupportedLanguage reports whether the provided raw tag matches any supported language.
//
// Example usage:
//
//	if yalocales.IsSupportedLanguage(langs, "de-DE") {
//	    // language is supported
//	}
func IsSupportedLanguage(langs []SupportedLanguage, raw string) bool {
	_, ok := LookupSupportedLanguage(langs, raw)

	return ok
}

// SupportedLanguageCodes returns the language codes from the input slice in order.
//
// Example usage:
//
//	codes := yalocales.SupportedLanguageCodes(langs)
func SupportedLanguageCodes(langs []SupportedLanguage) []string {
	codes := make([]string, 0, len(langs))
	for _, lang := range langs {
		codes = append(codes, lang.Code)
	}

	return codes
}

// GetSupportedLanguages returns supported-language metadata derived from the locales
// loaded into the localizer.
// The returned slice is sorted by language code and is a copy safe for modification
// by the caller.
//
// Example usage:
//
//	loc := yalocales.NewYaLocalizer("en", false)
//	_ = loc.LoadLocales(localesFS)
//	langs := loc.GetSupportedLanguages()
func (l *YaLocalizer) GetSupportedLanguages() []SupportedLanguage {
	if l == nil {
		return nil
	}

	if len(l.supportedLanguages) == 0 && len(l.data) > 0 {
		l.refreshSupportedLanguages()
	}

	langs := make([]SupportedLanguage, len(l.supportedLanguages))
	copy(langs, l.supportedLanguages)

	return langs
}

func (l *YaLocalizer) refreshSupportedLanguages() {
	if l == nil {
		return
	}

	if len(l.data) == 0 {
		l.supportedLanguages = nil

		return
	}

	codes := make([]string, 0, len(l.data))
	for code := range l.data {
		normalized := normalizeStoredLanguageTag(code)
		if normalized == "" {
			continue
		}

		codes = append(codes, normalized)
	}

	sort.Strings(codes)

	languages := make([]SupportedLanguage, 0, len(codes))
	for _, code := range codes {
		languages = append(languages, SupportedLanguage{
			Code:  code,
			Emoji: inferLanguageSymbol(code),
			Label: inferLanguageLabel(code),
		})
	}

	l.supportedLanguages = languages
}

func normalizeStoredLanguageTag(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}

	replacer := strings.NewReplacer("_", "-", " ", "")

	return replacer.Replace(raw)
}

func languageTagsMatch(lhs, rhs string) bool {
	leftExact := normalizeStoredLanguageTag(lhs)
	rightExact := normalizeStoredLanguageTag(rhs)

	if leftExact == "" || rightExact == "" {
		return false
	}

	if leftExact == rightExact {
		return true
	}

	leftBase := NormalizeLanguageTag(leftExact)
	rightBase := NormalizeLanguageTag(rightExact)

	if leftBase == rightBase {
		return true
	}

	return canonicalLanguageCode(leftBase) == canonicalLanguageCode(rightBase)
}

func inferLanguageLabel(code string) string {
	tag := displayLanguageTag(code)

	label := strings.TrimSpace(display.Self.Name(tag))
	if label == "" {
		englishNamer := display.English.Languages()
		if englishNamer != nil {
			label = strings.TrimSpace(englishNamer.Name(tag))
		}
	}

	if label == "" {
		return strings.ToUpper(normalizeStoredLanguageTag(code))
	}

	return strings.TrimSpace(cases.Title(language.Und).String(label))
}

func displayLanguageTag(code string) language.Tag {
	exactCode := normalizeStoredLanguageTag(code)
	if exactCode == "" {
		return language.Und
	}

	canonicalBase := canonicalLanguageCode(exactCode)
	if canonicalBase == "" {
		return language.Und
	}

	tagCode := canonicalBase
	if idx := strings.IndexByte(exactCode, '-'); idx >= 0 {
		tagCode += exactCode[idx:]
	}

	return language.Make(tagCode)
}

func canonicalLanguageCode(raw string) string {
	switch NormalizeLanguageTag(raw) {
	case "ua":
		return "uk"
	default:
		return NormalizeLanguageTag(raw)
	}
}

func inferLanguageSymbol(code string) string {
	tag := displayLanguageTag(code)

	if base, _ := tag.Base(); strings.EqualFold(base.String(), "ru") {
		return "🏴‍☠️"
	}

	region, _ := tag.Region()

	regionCode := strings.ToUpper(strings.TrimSpace(region.String()))
	if len(regionCode) != flagRegionCodeLength {
		return fallbackLanguageEmoji
	}

	for _, r := range regionCode {
		if r < 'A' || r > 'Z' {
			return fallbackLanguageEmoji
		}
	}

	return countryCodeToFlagEmoji(regionCode)
}

func countryCodeToFlagEmoji(regionCode string) string {
	var b strings.Builder
	b.Grow(flagEmojiBuilderSize)

	for _, r := range regionCode {
		b.WriteRune(regionalIndicatorOffset + (r - latinUppercaseOffset))
	}

	return b.String()
}
