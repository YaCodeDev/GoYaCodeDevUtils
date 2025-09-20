package yalocales_test

import (
	"embed"
	"io/fs"
	"strings"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
)

//go:embed testdata/*
var cgLocalesFS embed.FS

func TestGenerateCode(t *testing.T) {
	sub, err := fs.Sub(cgLocalesFS, "testdata/valid")
	if err != nil {
		t.Fatalf("failed to access sub fs: %v", err)
	}

	loc := yalocales.NewYaLocalizer("en")
	if yaErr := loc.LoadLocales(sub); yaErr != nil {
		t.Fatalf("Failed to load locales: %v", yaErr)
	}

	code, err := loc.GenerateLocaleCode(yalocales.GoCodeExportOptions{PackageName: "locales"})
	if err != nil {
		t.Fatalf("GenerateGoCode failed: %v", err)
	}

	if !strings.Contains(code, "type Locale struct {") {
		t.Fatalf("expected type declaration in code, got:\n%s", code)
	}

	if !strings.Contains(code, "Subtest struct {") ||
		!strings.Contains(code, "Subsubtest struct {") {
		t.Fatalf("expected nested structs for subfolders, got:\n%s", code)
	}

	if !strings.Contains(code, "Test string") || !strings.Contains(code, "Test1 string") ||
		!strings.Contains(code, "Test2 string") {
		t.Fatalf("expected leaf string fields, got:\n%s", code)
	}

	if strings.Contains(code, "var En ") || strings.Contains(code, "var Ua ") {
		t.Fatalf("did not expect language variables in default output, got:\n%s", code)
	}

	if !strings.Contains(code, "var Keys Locale") {
		t.Fatalf("expected single Keys variable, got:\n%s", code)
	}

	if !strings.Contains(code, "\"test\"") ||
		!strings.Contains(code, "\"subtest.subsubtest.test\"") {
		t.Fatalf("expected composite key strings in Keys literal, got:\n%s", code)
	}
}
