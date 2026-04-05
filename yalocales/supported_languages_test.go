package yalocales_test

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
)

func TestLoadSupportedLanguagesUsesPrimaryLocalizer(t *testing.T) {
	locales := fstest.MapFS{
		"locales/bot_ui/EN.json": &fstest.MapFile{Data: []byte(`{"ok":"ok"}`)},
		"locales/bot_ui/ua.json": &fstest.MapFile{Data: []byte(`{"ok":"ok"}`)},
		"locales/bot_ui/ru.json": &fstest.MapFile{Data: []byte(`{"ok":"ok"}`)},
	}

	langs, err := yalocales.LoadSupportedLanguages(locales)
	if err != nil {
		t.Fatalf("LoadSupportedLanguages() error = %v", err)
	}

	gotCodes := yalocales.SupportedLanguageCodes(langs)
	wantCodes := []string{"en", "ru", "ua"}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("SupportedLanguageCodes() = %v, want %v", gotCodes, wantCodes)
	}

	ukrainian, ok := yalocales.LookupSupportedLanguage(langs, "uk-UA")
	if !ok {
		t.Fatal("LookupSupportedLanguage(uk-UA) = not found, want found")
	}

	if ukrainian.Code != "ua" {
		t.Fatalf("LookupSupportedLanguage(uk-UA).Code = %q, want %q", ukrainian.Code, "ua")
	}

	if ukrainian.Emoji != "🇺🇦" {
		t.Fatalf("LookupSupportedLanguage(uk-UA).Emoji = %q, want %q", ukrainian.Emoji, "🇺🇦")
	}

	if strings.TrimSpace(ukrainian.Label) == "" {
		t.Fatal("LookupSupportedLanguage(uk-UA).Label is empty")
	}

	russian, ok := yalocales.LookupSupportedLanguage(langs, "ru")
	if !ok {
		t.Fatal("LookupSupportedLanguage(ru) = not found, want found")
	}

	if russian.Emoji != "🏴‍☠️" {
		t.Fatalf("LookupSupportedLanguage(ru).Emoji = %q, want %q", russian.Emoji, "🏴‍☠️")
	}
}

func TestYaLocalizerNormalizesLanguageSelection(t *testing.T) {
	localizer := yalocales.NewYaLocalizer("EN-US", true)
	locales := fstest.MapFS{
		"EN.json": &fstest.MapFile{Data: []byte(`{"msg":"hello"}`)},
		"UA.json": &fstest.MapFile{Data: []byte(`{"msg":"привіт"}`)},
	}

	if yaErr := localizer.LoadLocales(locales); yaErr != nil {
		t.Fatalf("LoadLocales() error = %v", yaErr)
	}

	gotDefault, yaErr := localizer.GetDefaultLangValueByCompositeKey("msg")
	if yaErr != nil {
		t.Fatalf("GetDefaultLangValueByCompositeKey() error = %v", yaErr)
	}

	if gotDefault != "hello" {
		t.Fatalf("GetDefaultLangValueByCompositeKey() = %q, want %q", gotDefault, "hello")
	}

	gotUkrainian, yaErr := localizer.GetValueByCompositeKeyAndLang("msg", "uk-UA")
	if yaErr != nil {
		t.Fatalf("GetValueByCompositeKeyAndLang() error = %v", yaErr)
	}

	if gotUkrainian != "привіт" {
		t.Fatalf("GetValueByCompositeKeyAndLang() = %q, want %q", gotUkrainian, "привіт")
	}

	derived, yaErr := localizer.DeriveNewDefaultLang("UA-UA")
	if yaErr != nil {
		t.Fatalf("DeriveNewDefaultLang() error = %v", yaErr)
	}

	gotDerived, yaErr := derived.GetDefaultLangValueByCompositeKey("msg")
	if yaErr != nil {
		t.Fatalf("derived.GetDefaultLangValueByCompositeKey() error = %v", yaErr)
	}

	if gotDerived != "привіт" {
		t.Fatalf("derived.GetDefaultLangValueByCompositeKey() = %q, want %q", gotDerived, "привіт")
	}

	gotCodes := yalocales.SupportedLanguageCodes(localizer.GetSupportedLanguages())
	wantCodes := []string{"en", "ua"}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("SupportedLanguageCodes() = %v, want %v", gotCodes, wantCodes)
	}
}
