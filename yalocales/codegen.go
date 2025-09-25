package yalocales

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var nonIdent = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// GoCodeExportOptions controls how Go code is generated for loaded locales.
type GoCodeExportOptions struct {
	// PackageName for the generated file (default: locale).
	PackageName string
	// TypeName of the top-level struct (default: "Locale").
	TypeName string
	// MainLang selects the language whose key tree defines the struct shape.
	// If empty, fallbackLang is used; if no fallback, an arbitrary language is used.
	MainLang string
}

// GenerateLocaleCode renders Go code representing the loaded locales as nested structs.
// It returns a single Go source file content that defines a top-level struct type and
// a single variable `Keys` where each leaf holds its composite key string for easy retrieval.
func (l *YaLocalizer) GenerateLocaleCode(opts GoCodeExportOptions) (string, yaerrors.Error) {
	if l == nil || len(l.data) == 0 {
		return "", yaerrors.FromError(
			http.StatusTeapot,
			ErrNilLocale,
			"cannot generate code",
		)
	}

	pkg := strings.TrimSpace(opts.PackageName)
	if pkg == "" {
		pkg = "locale"
	}

	typeName := strings.TrimSpace(opts.TypeName)
	if typeName == "" {
		typeName = "Locale"
	}

	refLang := strings.TrimSpace(opts.MainLang)
	if refLang == "" {
		if l.fallbackLang != "" {
			refLang = l.fallbackLang
		} else {
			langs := make([]string, 0, len(l.data))
			for k := range l.data {
				langs = append(langs, k)
			}

			sort.Strings(langs)
			refLang = langs[0]
		}
	}

	refNode, ok := l.data[refLang]
	if !ok || refNode == nil {
		return "", yaerrors.FromError(
			http.StatusBadRequest,
			ErrInvalidLanguage,
			fmt.Sprintf("reference language '%s' not found", refLang),
		)
	}

	typeBody := buildAnonStructType(refNode)

	var b strings.Builder
	b.WriteString("package ")
	b.WriteString(pkg)
	b.WriteString("\n\n")

	b.WriteString("type ")
	b.WriteString(typeName)
	b.WriteString(" ")
	b.WriteString(typeBody)
	b.WriteString("\n\n")

	b.WriteString("var Keys ")
	b.WriteString(typeName)
	b.WriteString(" = ")
	b.WriteString(typeName)
	b.WriteString("{")
	b.WriteString("\n")
	b.WriteString(buildKeyLiteralFields(refNode, 1, ""))
	b.WriteString("}\n\n")

	return b.String(), nil
}

// HelperRepresentLocaleAsStruct generates Go code and writes it to the given file path within the provided fs.FS root.
// If writeFS is nil, it writes to the real filesystem.
func (l *YaLocalizer) HelperRepresentLocaleAsStruct(
	opts GoCodeExportOptions,
	filePath string,
) yaerrors.Error {
	code, yaErr := l.GenerateLocaleCode(opts)
	if yaErr != nil {
		return yaErr.Wrap("GenerateLocaleCode failed")
	}

	err := os.WriteFile(filePath, []byte(code), defaultFilePerm)
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("Failed to write file '%s'", filePath),
		)
	}

	return nil
}

func buildAnonStructType(node *compiledLocale) string {
	return buildStructTypeFromSubmap(node.SubMap, 0)
}

func buildStructTypeFromSubmap(sub map[string]*compiledLocale, indent int) string {
	ind := strings.Repeat("\t", indent)
	ind2 := strings.Repeat("\t", indent+1)

	keys := make([]string, 0, len(sub))
	for k := range sub {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("struct {")

	if len(keys) > 0 {
		b.WriteString("\n")
	}

	used := map[string]int{}

	for _, k := range keys {
		child := sub[k]

		fieldName := exportIdentifier(k)
		if c := used[fieldName]; c > 0 {
			fieldName = fmt.Sprintf("%s_%d", fieldName, c+1)
		}

		used[fieldName]++

		b.WriteString(ind2)
		b.WriteString(fieldName)
		b.WriteString(" ")

		if child.SubMap != nil {
			b.WriteString(buildStructTypeFromSubmap(child.SubMap, indent+1))
		} else {
			b.WriteString("string")
		}

		b.WriteString("\n")
	}

	b.WriteString(ind)
	b.WriteString("}")

	return b.String()
}

func buildKeyLiteralFields(refNode *compiledLocale, indent int, prefix string) string {
	ind := strings.Repeat("\t", indent)

	keys := make([]string, 0, len(refNode.SubMap))
	for k := range refNode.SubMap {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	used := map[string]int{}

	var b strings.Builder

	for _, k := range keys {
		refChild := refNode.SubMap[k]

		fieldName := exportIdentifier(k)
		if c := used[fieldName]; c > 0 {
			fieldName = fmt.Sprintf("%s_%d", fieldName, c+1)
		}

		used[fieldName]++

		b.WriteString(ind)
		b.WriteString(fieldName)
		b.WriteString(": ")

		if refChild.SubMap != nil {
			b.WriteString(buildStructTypeFromSubmap(refChild.SubMap, indent))
			b.WriteString("{")
			b.WriteString("\n")

			nextPrefix := k
			if prefix != "" {
				nextPrefix = prefix + Separator + k
			}

			b.WriteString(buildKeyLiteralFields(refChild, indent+1, nextPrefix))
			b.WriteString(strings.Repeat("\t", indent))
			b.WriteString("}")
		} else {
			full := k
			if prefix != "" {
				full = prefix + Separator + k
			}

			b.WriteString(strconv.Quote(full))
		}

		b.WriteString(",")
		b.WriteString("\n")
	}

	return b.String()
}

func exportIdentifier(s string) string {
	if s == "" {
		return "X"
	}

	s = nonIdent.ReplaceAllString(s, " ")

	parts := strings.Fields(s)
	for i, p := range parts {
		parts[i] = cases.Title(language.English).String(p)
	}

	res := strings.Join(parts, "")
	if res == "" {
		res = "X"
	}

	r, size := utf8DecodeRuneInString(res)
	if !unicode.IsLetter(r) || !unicode.IsUpper(r) {
		res = "X" + res
	}

	switch strings.ToLower(res) {
	case "break",
		"default",
		"func",
		"interface",
		"select",
		"case",
		"defer",
		"go",
		"map",
		"struct",
		"chan",
		"else",
		"goto",
		"package",
		"switch",
		"const",
		"fallthrough",
		"if",
		"range",
		"type",
		"continue",
		"for",
		"import",
		"return",
		"var":
		res += "_"
	default:
		_ = size
	}

	return res
}
