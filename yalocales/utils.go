package yalocales

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

var placeholderRegexp = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func extractPlaceholdersSet(s string) map[string]struct{} {
	res := make(map[string]struct{})

	if s == "" {
		return res
	}

	matches := placeholderRegexp.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) >= 2 && m[1] != "" {
			res[m[1]] = struct{}{}
		}
	}

	return res
}

func collectCompositeKeys(node *compiledLocale, prefix string, out map[string]struct{}) {
	if node == nil {
		return
	}

	if node.SubMap == nil {
		out[prefix] = struct{}{}

		return
	}

	for k, v := range node.SubMap {
		next := k
		if prefix != "" {
			next = prefix + Separator + k
		}

		collectCompositeKeys(v, next, out)
	}
}

func setDiff(a, b map[string]struct{}) (missingInB []string, extraInB []string) {
	for k := range a {
		if _, ok := b[k]; !ok {
			missingInB = append(missingInB, k)
		}
	}

	for k := range b {
		if _, ok := a[k]; !ok {
			extraInB = append(extraInB, k)
		}
	}

	return missingInB, extraInB
}

func subtractSets(a, b map[string]struct{}) []string {
	var res []string

	for k := range a {
		if _, ok := b[k]; !ok {
			res = append(res, k)
		}
	}

	return res
}

func utf8DecodeRuneInString(s string) (r rune, size int) {
	for i, rr := range s {
		return rr, i
	}

	return '\uFFFD', 0
}

func formatValueWithArgs(s string, args any) (string, yaerrors.Error) {
	if s == "" || args == nil {
		return s, nil
	}

	phSet := extractPlaceholdersSet(s)
	if len(phSet) == 0 {
		return s, nil
	}

	resolver := func(name string) (string, bool) {
		switch v := args.(type) {
		case map[string]string:
			val, ok := v[name]

			return val, ok
		case map[string]any:
			val, ok := v[name]
			if !ok {
				return "", false
			}

			return fmt.Sprint(val), true
		default:
			rv := reflect.ValueOf(args)
			if rv.Kind() == reflect.Ptr {
				if rv.IsNil() {
					return "", false
				}

				rv = rv.Elem()
			}

			if rv.Kind() == reflect.Struct {
				fieldName := exportIdentifier(name)

				fv := rv.FieldByName(fieldName)
				if !fv.IsValid() {
					if f2, ok := findFieldByPlaceholder(rv, name); ok {
						fv = f2
					}
				}

				if fv.IsValid() {
					if fv.CanAddr() {
						if s, ok := fv.Addr().Interface().(fmt.Stringer); ok {
							return s.String(), true
						}
					}

					if fv.CanInterface() {
						if s, ok := fv.Interface().(fmt.Stringer); ok {
							return s.String(), true
						}

						return fmt.Sprint(fv.Interface()), true
					}
				}
			}

			return "", false
		}
	}

	missing := make([]string, 0)
	resolved := make(map[string]string, len(phSet))

	for name := range phSet {
		val, ok := resolver(name)
		if !ok {
			missing = append(missing, name)

			continue
		}

		resolved[name] = val
	}

	if len(missing) > 0 {
		return "", yaerrors.FromError(
			http.StatusBadRequest,
			ErrMissingFormatArgs,
			fmt.Sprintf("missing placeholders: %v", missing),
		)
	}

	out := s
	for name, val := range resolved {
		out = strings.ReplaceAll(out, "{"+name+"}", val)
	}

	return out, nil
}

func findFieldByPlaceholder(reflectValue reflect.Value, placeholder string) (reflect.Value, bool) {
	if reflectValue.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	normalized := strings.ToLower(strings.ReplaceAll(placeholder, "_", ""))

	typ := reflectValue.Type()
	for i := range typ.NumField() {
		sf := typ.Field(i)
		if sf.Name == "" || !unicode.IsUpper(rune(sf.Name[0])) {
			continue
		}

		fnameNorm := strings.ToLower(sf.Name)
		if fnameNorm == normalized {
			return reflectValue.Field(i), true
		}
	}

	return reflect.Value{}, false
}
