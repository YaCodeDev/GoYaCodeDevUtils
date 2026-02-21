package yalocales_test

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
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

	localesMap := yalocales.NewLocalizer("en", true)

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

	loc := yalocales.NewLocalizer("", false)
	if yaErr := loc.LoadLocales(sub); yaErr == nil {
		t.Fatalf("expected key mismatch error, got nil")
	}
}

func TestLoadLocalesDefaultMustBeSuperset(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/default_missing_keys")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	loc := yalocales.NewLocalizer("en", false)
	if yaErr := loc.LoadLocales(sub); yaErr == nil {
		t.Fatalf("expected default coverage error, got nil")
	}
}

func TestFormattedValueWithMap(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/format")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en", false)
	if yaErr := l.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	got, yaErr := l.GetFormattedValueByCompositeKeyAndLang("msg", "en", map[string]string{
		"formatable_json_locale": "Formatable Locale Replacement",
	})
	if yaErr != nil {
		t.Fatalf("format value: %v", yaErr)
	}

	//nolint:goconst // Who cares about consts in tests?
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

	l := yalocales.NewLocalizer("en", false)
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

	l := yalocales.NewLocalizer("", false)
	if yaErr := l.LoadLocales(sub); yaErr == nil {
		t.Fatalf("expected placeholder mismatch error, got nil")
	}
}

func TestPlaceholdersMismatchWithDefault(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/placeholders_mismatch_with_default")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en", false)
	if yaErr := l.LoadLocales(sub); yaErr == nil {
		t.Fatalf("expected placeholder mismatch error, got nil")
	}
}

func TestDeriveNewDefaultLang(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/valid")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	base := yalocales.NewLocalizer("en", true)
	if yaErr := base.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	derived, yaErr := base.DeriveNewDefaultLang("ua")
	if yaErr != nil {
		t.Fatalf("derive new default: %v", yaErr)
	}

	if derived == nil {
		t.Fatalf("derive new default returned nil localizer")
	}

	got, yaErr := derived.GetDefaultLangValueByCompositeKey("test")
	if yaErr != nil {
		t.Fatalf("get default value on derived localizer: %v", yaErr)
	}

	if want := "тест"; got != want {
		t.Fatalf("unexpected derived default value: got %q want %q", got, want)
	}
}

func TestDeriveNewDefaultLangRequiresConsistency(t *testing.T) {
	base := yalocales.NewLocalizer("en", false)

	derived, yaErr := base.DeriveNewDefaultLang("ua")
	if yaErr == nil {
		t.Fatalf("expected consistency error, got nil")
	}

	if derived != nil {
		t.Fatalf("expected nil localizer on error, got %v", derived)
	}

	if yaErr.Code() != http.StatusTeapot {
		t.Fatalf("unexpected error code: got %d want %d", yaErr.Code(), http.StatusTeapot)
	}

	if !errors.Is(yaErr.Unwrap(), yalocales.ErrConsistencyRequired) {
		t.Fatalf("unexpected error cause: %v", yaErr.Unwrap())
	}
}

func TestDeriveNewDefaultLangMissingLanguage(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/valid")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	base := yalocales.NewLocalizer("en", true)
	if yaErr := base.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	derived, yaErr := base.DeriveNewDefaultLang("fr")
	if yaErr == nil {
		t.Fatalf("expected missing language error, got nil")
	}

	if derived != nil {
		t.Fatalf("expected nil localizer on error, got %v", derived)
	}

	if yaErr.Code() != http.StatusTeapot {
		t.Fatalf("unexpected error code: got %d want %d", yaErr.Code(), http.StatusTeapot)
	}

	if !errors.Is(yaErr.Unwrap(), yalocales.ErrInvalidLanguage) {
		t.Fatalf("unexpected error cause: %v", yaErr.Unwrap())
	}
}

func TestGetFormattedDefaultLangValueByCompositeKey(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/format")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en", false)
	if yaErr := l.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	got, yaErr := l.GetFormattedDefaultLangValueByCompositeKey("msg", map[string]string{
		"formatable_json_locale": "Formatable Locale Replacement",
	})
	if yaErr != nil {
		t.Fatalf("get formatted default value: %v", yaErr)
	}

	if want := "This is a Formatable Locale Replacement"; got != want {
		t.Fatalf("unexpected formatted default value: got %q want %q", got, want)
	}
}

func TestGetFormattedDefaultLangValueByCompositeKeyNoDefaultLang(t *testing.T) {
	l := yalocales.NewLocalizer("", false)

	_, yaErr := l.GetFormattedDefaultLangValueByCompositeKey("msg", map[string]string{})
	if yaErr == nil {
		t.Fatalf("expected no default language error, got nil")
	}

	if yaErr.Code() != http.StatusTeapot {
		t.Fatalf("unexpected error code: got %d want %d", yaErr.Code(), http.StatusTeapot)
	}

	if !errors.Is(yaErr.Unwrap(), yalocales.ErrNoDefaultLanguage) {
		t.Fatalf("unexpected error cause: %v", yaErr.Unwrap())
	}
}

func TestGetFormattedDefaultLangValueByCompositeKeyMissingArgs(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/format")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en", false)
	if yaErr := l.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	_, yaErr := l.GetFormattedDefaultLangValueByCompositeKey("msg", map[string]string{})
	if yaErr == nil {
		t.Fatalf("expected missing args error, got nil")
	}

	if yaErr.Code() != http.StatusBadRequest {
		t.Fatalf("unexpected error code: got %d want %d", yaErr.Code(), http.StatusBadRequest)
	}

	if !errors.Is(yaErr.Unwrap(), yalocales.ErrMissingFormatArgs) {
		t.Fatalf("unexpected error cause: %v", yaErr.Unwrap())
	}

	msg := yaErr.Error()
	if !strings.Contains(msg, "failed to get formatted value from default language") {
		t.Fatalf("error message missing default language context: %s", msg)
	}

	if !strings.Contains(msg, "failed to format value for key 'msg'") {
		t.Fatalf("error message missing key context: %s", msg)
	}
}

func TestGetDefaultLangJSONByCompositeKey(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/valid")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en", false)
	if yaErr := l.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	got, yaErr := l.GetDefaultLangJSONByCompositeKey("test")
	if yaErr != nil {
		t.Fatalf("get default JSON: %v", yaErr)
	}

	expectedJSON, err := json.Marshal(map[string]string{"test": "test"})
	if err != nil {
		t.Fatalf("marshal expected JSON: %v", err)
	}

	if string(got) != string(expectedJSON) {
		t.Fatalf("unexpected default JSON: got %s want %s", string(got), string(expectedJSON))
	}
}

func TestGetDefaultLangJSONByCompositeKeyNoDefaultLang(t *testing.T) {
	l := yalocales.NewLocalizer("", false)

	_, yaErr := l.GetDefaultLangJSONByCompositeKey("test")
	if yaErr == nil {
		t.Fatalf("expected no default language error, got nil")
	}

	if yaErr.Code() != http.StatusTeapot {
		t.Fatalf("unexpected error code: got %d want %d", yaErr.Code(), http.StatusTeapot)
	}

	if !errors.Is(yaErr.Unwrap(), yalocales.ErrNoDefaultLanguage) {
		t.Fatalf("unexpected error cause: %v", yaErr.Unwrap())
	}
}

func TestGetDefaultLangValueByCompositeKey(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/valid")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	l := yalocales.NewLocalizer("en", false)
	if yaErr := l.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	got, yaErr := l.GetDefaultLangValueByCompositeKey("test1")
	if yaErr != nil {
		t.Fatalf("get default value: %v", yaErr)
	}

	if want := "test1"; got != want {
		t.Fatalf("unexpected default value: got %q want %q", got, want)
	}
}

func TestGetDefaultLangValueByCompositeKeyNoDefaultLang(t *testing.T) {
	l := yalocales.NewLocalizer("", false)

	_, yaErr := l.GetDefaultLangValueByCompositeKey("test")
	if yaErr == nil {
		t.Fatalf("expected no default language error, got nil")
	}

	if yaErr.Code() != http.StatusTeapot {
		t.Fatalf("unexpected error code: got %d want %d", yaErr.Code(), http.StatusTeapot)
	}

	if !errors.Is(yaErr.Unwrap(), yalocales.ErrNoDefaultLanguage) {
		t.Fatalf("unexpected error cause: %v", yaErr.Unwrap())
	}
}

func TestLoadLocalesNestedBlocksMergedWithFolders(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/nested_blocks_with_folders")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	loc := yalocales.NewLocalizer("en", false)
	if yaErr := loc.LoadLocales(sub); yaErr != nil {
		t.Fatalf("load locales: %v", yaErr)
	}

	cases := []struct {
		key  string
		lang string
		want string
	}{
		{key: "root", lang: "en", want: "Root"},
		{key: "home.title", lang: "en", want: "Home"},
		{key: "home.subtitle", lang: "ua", want: "Ласкаво просимо"},
		{key: "home.cta", lang: "ua", want: "До дому"},
	}

	for _, tc := range cases {
		got, yaErr := loc.GetValueByCompositeKeyAndLang(tc.key, tc.lang)
		if yaErr != nil {
			t.Fatalf("get value for key %q lang %q: %v", tc.key, tc.lang, yaErr)
		}

		if got != tc.want {
			t.Fatalf(
				"unexpected value for key %q lang %q: got %q want %q",
				tc.key,
				tc.lang,
				got,
				tc.want,
			)
		}
	}

	gotJSON, yaErr := loc.GetJSONByCompositeKeyAndLang("home", "en")
	if yaErr != nil {
		t.Fatalf("get JSON for key %q lang %q: %v", "home", "en", yaErr)
	}

	wantJSON, err := json.Marshal(map[string]string{
		"cta":      "Go Home",
		"subtitle": "Welcome",
		"title":    "Home",
	})
	if err != nil {
		t.Fatalf("marshal expected JSON: %v", err)
	}

	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("unexpected merged JSON: got %s want %s", string(gotJSON), string(wantJSON))
	}
}

func TestLoadLocalesPathConflict(t *testing.T) {
	sub, err := fs.Sub(localesFS, "testdata/path_conflict")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	loc := yalocales.NewLocalizer("en", false)

	yaErr := loc.LoadLocales(sub)
	if yaErr == nil {
		t.Fatalf("expected path conflict error, got nil")
	}

	if !errors.Is(yaErr.Unwrap(), yalocales.ErrPathConflict) {
		t.Fatalf("unexpected error cause: %v", yaErr.Unwrap())
	}
}
