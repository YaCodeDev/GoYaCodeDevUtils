package yalocales_test

import (
	"embed"
	"encoding/json"
	"io/fs"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
)

//go:embed testdata/*
var localesFS embed.FS

func TestLoadLocales(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/valid")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	localesMap := yalocales.NewLocalizer("en")

	yaErr := localesMap.LoadLocales(sub)
	if yaErr != nil {
		t.Fatalf("Failed to load locales: %v", yaErr)
	}

	testJSON, yaErr := localesMap.GetJSONByCompositeKeyAndLang("test", "ua")
	if yaErr != nil {
		t.Fatalf("Failed to get JSON for lang 'ua' and key 'test': %v", yaErr)
	}

	equivalentMap := map[string]string{"test": "тест"}

	expectedJSON, err := json.Marshal(equivalentMap)
	if err != nil {
		t.Fatalf("Failed to marshal expected JSON: %v", err)
	}

	if string(testJSON) != string(expectedJSON) {
		t.Errorf(
			"Unexpected JSON for lang 'ua' and key 'test': got %v, want %v",
			string(testJSON),
			string(expectedJSON),
		)
	}

	testValue, yaErr := localesMap.GetValueByCompositeKeyAndLang("test1", "en")
	if yaErr != nil {
		t.Fatalf("Failed to get value for lang 'en' and key 'test1': %v", yaErr)
	}

	expectedValue := "test1"

	if testValue != expectedValue {
		t.Errorf(
			"Unexpected value for lang 'en' and key 'test1': got %v, want %v",
			testValue,
			expectedValue,
		)
	}

	subsubValue, yaErr := localesMap.GetValueByCompositeKeyAndLang("subtest.subsubtest.test2", "ua")
	if yaErr != nil {
		t.Fatalf("Failed to get value for lang 'ua' and key 'test2': %v", yaErr)
	}

	expectedSubsubValue := "тест2"

	if subsubValue != expectedSubsubValue {
		t.Errorf(
			"Unexpected value for lang 'ua' and key 'test2': got %v, want %v",
			subsubValue,
			expectedSubsubValue,
		)
	}
}

func TestLoadLocalesNoDefaultKeysMustMatch(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/mismatch_no_default")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	loc := yalocales.NewLocalizer("")
	if yaErr := loc.LoadLocales(sub); yaErr == nil {
		t.Fatalf("expected key mismatch error, got nil")
	}
}

func TestLoadLocalesDefaultMustBeSuperset(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/default_missing_keys")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	loc := yalocales.NewLocalizer("en")
	if yaErr := loc.LoadLocales(sub); yaErr == nil {
		t.Fatalf("expected default coverage error, got nil")
	}
}

func TestFormattedValueWithMap(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/format")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en")
	if yaErr := l.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	got, yaErr := l.GetFormattedValueByCompositeKeyAndLang("msg", "en", map[string]string{
		"formatable_json_locale": "Formatable Locale Replacement",
	})
	if yaErr != nil {
		t.Fatalf("format value: %v", yaErr)
	}

	want := "This is a Formatable Locale Replacement"
	if got != want {
		t.Fatalf("unexpected formatted value: got %q want %q", got, want)
	}
}

func TestFormattedValueWithStruct(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/format")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en")
	if yaErr := l.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	args := struct {
		FormatableJSONLocale string
		OtherData            string
	}{
		FormatableJSONLocale: "Formatable Locale Replacement",
		OtherData:            "unused",
	}

	got, yaErr := l.GetFormattedValueByCompositeKeyAndLang("msg", "en", args)
	if yaErr != nil {
		t.Fatalf("format value: %v", yaErr)
	}

	want := "This is a Formatable Locale Replacement"
	if got != want {
		t.Fatalf("unexpected formatted value: got %q want %q", got, want)
	}
}

func TestPlaceholdersMismatchNoDefault(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/placeholders_mismatch")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("")
	if yaErr := l.LoadLocales(sub); yaErr == nil {
		t.Fatalf("expected placeholder mismatch error, got nil")
	}
}

func TestPlaceholdersMismatchWithDefault(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/placeholders_mismatch_with_default")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en")
	if yaErr := l.LoadLocales(sub); yaErr == nil {
		t.Fatalf("expected placeholder mismatch error, got nil")
	}
}
