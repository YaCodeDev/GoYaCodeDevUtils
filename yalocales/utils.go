package yalocales

import (
	"regexp"
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

	return
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
