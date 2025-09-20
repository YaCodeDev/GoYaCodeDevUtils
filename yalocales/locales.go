package yalocales

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"reflect"
	"strings"
	"unicode"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

type (
	Localizer interface {
		// GetFormattedValueByCompositeKeyAndLang retrieves a localized string value and formats
		// placeholders in the form {name} using provided args. Args may be either:
		// - map[string]string with keys matching placeholder names
		// - struct (or pointer to struct) where field names match exportIdentifier(placeholder)
		// If any required placeholder is missing in args, an error is returned.
		//
		// Example usage:
		//
		//	msg, err := loc.GetFormattedValueByCompositeKeyAndLang("greeting", "en", map[string]string{"name": "John"})
		//	// or using struct
		//	type Args struct {
		//		Name string
		//	}
		//	msg, err := loc.GetFormattedValueByCompositeKeyAndLang("greeting", "en", Args{Name: "John"})
		//	if err != nil {
		//	    // handle error
		//	}
		GetFormattedValueByCompositeKeyAndLang(
			key string,
			lang string,
			args any,
		) (string, yaerrors.Error)
		// GetJSONByCompositeKeyAndLang retrieves the JSON representation of the value associated with the composite key for
		// the specified language. If the key or language is not found, it falls back to the default language if set,
		// otherwise returns an error.
		//
		// Example usage:
		//
		//	loc := locales.NewLocalizer("en")
		GetJSONByCompositeKeyAndLang(key string, lang string) ([]byte, yaerrors.Error)
		// GetValueByCompositeKeyAndLang retrieves the string value associated with the composite key for the specified
		// language. If the key or language is not found, it falls back to the default language if set, otherwise returns
		// an error.
		//
		// Example usage:
		//
		//	loc := locales.NewLocalizer("en")
		GetValueByCompositeKeyAndLang(key string, lang string) (string, yaerrors.Error)
		// LoadLocales loads localization files from the provided file system.
		// It expects the files to be organized in a directory structure where each language has its own JSON file.
		// The JSON files should contain key-value pairs for localized strings.
		// If a fallback language is set, it will be used when a requested language or key is not found.
		// Otherwise, an error will be returned on locale loading.
		//
		// Example usage:
		//
		//	loc := locales.NewLocalizer("en")
		//	err := loc.LoadLocales(yourEmbeddedFileSystem)
		//	if err != nil {
		//	    // handle error
		//	}
		LoadLocales(files fs.FS) yaerrors.Error
	}

	// YaLocalizer is a structure that holds localization data for multiple languages and implements methods
	// to load, retrieve, and manage localized strings and JSON objects.
	// If fallbackLang is set, it will be used when a requested language or key is not found.
	// Otherwise, an error will be returned on locale loading.
	YaLocalizer struct {
		fallbackLang string
		data         map[string]*compiledLocale
	}

	// compiledLocale represents a compiled localization file node or leaf
	compiledLocale struct {
		// The key for this node
		Key string
		// The sub-nodes for this node, if any. This is mutually exclusive with Value
		SubMap map[string]*compiledLocale
		// This is the actual value if this is a leaf node, might be empty if not a leaf, only useful for native lookups.
		// This is mutually exclusive with SubMap
		Value string
		// This is the JSON representation of this node and all its children, ready to be served as-is
		JSON []byte
	}
)

// NewLocalizer creates a new Localizer instance with an optional fallback language.
//
// Example usage:
//
//	loc := locales.NewLocalizer("en")
func NewLocalizer(fallbackLang string) Localizer {
	return NewYaLocalizer(fallbackLang)
}

// NewYaLocalizer creates a new YaLocalizer instance with an optional fallback language.
//
// Example usage:
//
//	loc := locales.NewLocalizer("en")
func NewYaLocalizer(fallbackLang string) *YaLocalizer {
	return &YaLocalizer{
		fallbackLang: fallbackLang,
		data:         make(map[string]*compiledLocale),
	}
}

// GetJSONByCompositeKeyAndLang retrieves the JSON representation of the value associated with the composite key for
// the specified language. If the key or language is not found, it falls back to the default language if set,
// otherwise returns an error.
//
// Example usage:
//
//	loc := locales.NewLocalizer("en")
func (l *YaLocalizer) GetJSONByCompositeKeyAndLang(
	key string,
	lang string,
) ([]byte, yaerrors.Error) {
	value, err := l.data[lang].retriveJSONByCompositeKey(key)
	if err != nil {
		if l.fallbackLang != "" && lang != l.fallbackLang {
			value, err = l.data[l.fallbackLang].retriveJSONByCompositeKey(key)
		}

		if err != nil {
			return nil, err.Wrap(
				fmt.Sprintf("Failed to get JSON for key '%s' and language '%s'", key, lang),
			)
		}
	}

	return value, nil
}

// GetValueByCompositeKeyAndLang retrieves the string value associated with the composite key for the specified
// language. If the key or language is not found, it falls back to the default language if set, otherwise returns
// an error.
//
// Example usage:
//
//	loc := locales.NewLocalizer("en")
func (l *YaLocalizer) GetValueByCompositeKeyAndLang(
	key string,
	lang string,
) (string, yaerrors.Error) {
	value, err := l.data[lang].retriveValueByCompositeKey(key)
	if err != nil {
		if l.fallbackLang != "" && lang != l.fallbackLang {
			value, err = l.data[l.fallbackLang].retriveValueByCompositeKey(key)
		}

		if err != nil {
			return "", err.Wrap(
				fmt.Sprintf("Failed to get value for key '%s' and language '%s'", key, lang),
			)
		}
	}

	return value, nil
}

// GetFormattedValueByCompositeKeyAndLang retrieves a localized string value and formats
// placeholders in the form {name} using provided args. Args may be either:
// - map[string]string with keys matching placeholder names
// - struct (or pointer to struct) where field names match exportIdentifier(placeholder)
// If any required placeholder is missing in args, an error is returned.
//
// Example usage:
//
//	msg, err := loc.GetFormattedValueByCompositeKeyAndLang("greeting", "en", map[string]string{"name": "John"})
//	if err != nil {
//	    // handle error
//	}
//
//	// or using struct
//	type Args struct {
//		Name string
//	}
//
//	msg, err := loc.GetFormattedValueByCompositeKeyAndLang("greeting", "en", Args{Name: "John"})
//	if err != nil {
//	    // handle error
//	}
func (l *YaLocalizer) GetFormattedValueByCompositeKeyAndLang(
	key string,
	lang string,
	args any,
) (string, yaerrors.Error) {
	raw, err := l.GetValueByCompositeKeyAndLang(key, lang)
	if err != nil {
		return "", err.Wrap("failed to get raw value")
	}

	formatted, yaErr := formatValueWithArgs(raw, args)
	if yaErr != nil {
		return "", yaErr.Wrap(fmt.Sprintf("failed to format value for key '%s'", key))
	}

	return formatted, nil
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

func (c *compiledLocale) retriveJSONByCompositeKey(key string) ([]byte, yaerrors.Error) {
	if key == "" {
		return c.JSON, nil
	}

	keyPart := strings.SplitN(key, Separator, keySplitMaxParts)

	if c.SubMap == nil {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			ErrSubMapNotFound,
			fmt.Sprintf("No submap for key part '%s'", keyPart[0]),
		)
	}

	if len(keyPart) == 1 {
		return c.SubMap[keyPart[0]].JSON, nil
	}

	subLocale, ok := c.SubMap[keyPart[0]]

	if !ok {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			ErrKeyNotFound,
			fmt.Sprintf("Key '%s' not found", keyPart[0]),
		)
	}

	value, err := subLocale.retriveJSONByCompositeKey(keyPart[1])
	if err != nil {
		return nil, err.Wrap(fmt.Sprintf("Failed to retrieve JSON for key part '%s'", keyPart[0]))
	}

	return value, nil
}

func (c *compiledLocale) retriveValueByCompositeKey(key string) (string, yaerrors.Error) {
	if key == "" {
		return c.Value, nil
	}

	keyPart := strings.SplitN(key, Separator, keySplitMaxParts)

	if c.SubMap == nil {
		return "", yaerrors.FromError(
			http.StatusNotFound,
			ErrSubMapNotFound,
			fmt.Sprintf("No submap for key part '%s'", keyPart[0]),
		)
	}

	if len(keyPart) == 1 {
		return c.SubMap[keyPart[0]].Value, nil
	}

	subLocale, ok := c.SubMap[keyPart[0]]

	if !ok {
		return "", yaerrors.FromError(
			http.StatusNotFound,
			ErrKeyNotFound,
			fmt.Sprintf("Key '%s' not found", keyPart[0]),
		)
	}

	value, err := subLocale.retriveValueByCompositeKey(keyPart[1])
	if err != nil {
		return "", err.Wrap(fmt.Sprintf("Failed to retrieve value for key part '%s'", keyPart[0]))
	}

	return value, nil
}

// LoadLocales loads localization files from the provided file system.
// It expects the files to be organized in a directory structure where each language has its own JSON file.
// The JSON files should contain key-value pairs for localized strings.
// If a fallback language is set, it will be used when a requested language or key is not found.
// Otherwise, an error will be returned on locale loading.
//
// Example usage:
//
//	loc := locales.NewLocalizer("en")
//	err := loc.LoadLocales(yourEmbeddedFileSystem)
//	if err != nil {
//	    // handle error
//	}
func (l *YaLocalizer) LoadLocales(files fs.FS) yaerrors.Error {
	contents, err := fs.ReadDir(files, ".")
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"Failed to access root folder",
		)
	}

	// This is a workaround for embed.FS which always has a single root folder
	if len(contents) == 1 && contents[0].IsDir() {
		var subFS fs.FS

		subFS, err = fs.Sub(files, contents[0].Name())
		if err != nil {
			return yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				fmt.Sprintf("Failed to access subfolder '%s'", contents[0].Name()),
			)
		}

		return l.LoadLocales(subFS)
	}

	yaErr := l.loadFolder(files, "")
	if yaErr != nil {
		return yaErr.Wrap("Failed to load locales")
	}

	if yaErr := l.validateKeyCoverage(); yaErr != nil {
		return yaErr.Wrap("Failed key coverage validation")
	}

	for lang, locale := range l.data {
		_, err := locale.representSubTreeReconcilingJSON()
		if err != nil {
			return err.Wrap(fmt.Sprintf("Failed to represent sub-tree for language '%s'", lang))
		}
	}

	return nil
}

func (l *YaLocalizer) loadFolder(fileSystem fs.FS, compositeKey string) yaerrors.Error {
	contents, err := fs.ReadDir(fileSystem, ".")
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("Failed to access path at key '%s'", compositeKey),
		)
	}

	files := make([]fs.DirEntry, 0, len(l.data))
	folders := make([]fs.DirEntry, 0, len(contents)-len(l.data))

	for _, d := range contents {
		if d.IsDir() {
			folders = append(folders, d)

			continue
		}

		files = append(files, d)
	}

	for _, d := range files {
		if !strings.HasSuffix(d.Name(), JSONExt) {
			continue
		}

		languageTag := strings.TrimSuffix(d.Name(), JSONExt)

		if languageTag == "" {
			return yaerrors.FromError(
				http.StatusTeapot,
				ErrInvalidLanguage,
				fmt.Sprintf("Invalid language tag at key '%s'", compositeKey),
			)
		}

		data, err := fs.ReadFile(fileSystem, d.Name())
		if err != nil {
			return yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				fmt.Sprintf("Failed to read file '%s' at key '%s'", d.Name(), compositeKey),
			)
		}

		yaErr := l.processJSONFile(data, languageTag, compositeKey)
		if yaErr != nil {
			return yaErr.Wrap(
				fmt.Sprintf("Failed to process JSON file '%s' at key '%s'", d.Name(), compositeKey),
			)
		}
	}

	for _, d := range folders {
		if d == nil {
			continue
		}

		subFS, err := fs.Sub(fileSystem, d.Name())
		if err != nil {
			return yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				fmt.Sprintf("Failed to access subfolder '%s' at key '%s'", d.Name(), compositeKey),
			)
		}

		newCompositeKey := d.Name()
		if compositeKey != "" {
			newCompositeKey = compositeKey + Separator + d.Name()
		}

		yaErr := l.loadFolder(subFS, newCompositeKey)
		if yaErr != nil {
			return yaErr.Wrap(
				fmt.Sprintf("Failed to load subfolder '%s' at key '%s'", d.Name(), compositeKey),
			)
		}
	}

	return nil
}

func (l *YaLocalizer) processJSONFile(data []byte, lang, compositeKey string) yaerrors.Error {
	var locales map[string]string

	err := json.Unmarshal(data, &locales)
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("Failed to parse JSON at key '%s'", compositeKey),
		)
	}

	for key, value := range locales {
		fullKey := key

		if compositeKey != "" {
			fullKey = compositeKey + Separator + key
		}

		err := l.insertByCompositeKeyAndLang(fullKey, lang, value)
		if err != nil {
			return err.Wrap(fmt.Sprintf("Failed to insert locale at key '%s'", fullKey))
		}
	}

	return nil
}

func (l *YaLocalizer) insertByCompositeKeyAndLang(key, lang, value string) yaerrors.Error {
	if _, ok := l.data[lang]; !ok {
		keyPart := strings.SplitN(key, Separator, keySplitMaxParts)

		if len(keyPart) == keySplitMaxParts {
			return yaerrors.FromError(
				http.StatusTeapot,
				ErrInvalidLanguage,
				"New languages must be populated top-to-bottom",
			)
		}

		l.data[lang] = &compiledLocale{
			Key:    lang,
			SubMap: make(map[string]*compiledLocale),
		}
	}

	err := (l.data)[lang].insertByCompositeKey(key, value)
	if err != nil {
		return err.Wrap(fmt.Sprintf("Failed to insert locale at key '%s'", key))
	}

	return nil
}

func (c *compiledLocale) insertByCompositeKey(key, value string) yaerrors.Error {
	if value == "" {
		return yaerrors.FromError(
			http.StatusTeapot,
			ErrInvalidTranslation,
			"Empty values are not allowed",
		)
	}

	keyPart := strings.SplitN(key, Separator, keySplitMaxParts)

	if c.SubMap == nil {
		c.SubMap = make(map[string]*compiledLocale)
	}

	if len(keyPart) == keySplitMaxParts {
		_, ok := c.SubMap[keyPart[0]]
		if !ok {
			c.SubMap[keyPart[0]] = &compiledLocale{
				Key: keyPart[0],
			}
		}

		err := c.SubMap[keyPart[0]].insertByCompositeKey(keyPart[1], value)
		if err != nil {
			return err.Wrap(fmt.Sprintf("Failed to insert key part '%s'", keyPart[0]))
		}

		return nil
	}

	if _, ok := c.SubMap[key]; ok {
		return yaerrors.FromError(
			http.StatusTeapot,
			ErrDuplicateKey,
			fmt.Sprintf("Key '%s' already exists", key),
		)
	}

	kvMap := map[string]string{
		key: value,
	}

	jsonData, err := json.Marshal(kvMap)
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("Failed to marshal JSON for key '%s'", key),
		)
	}

	c.SubMap[key] = &compiledLocale{
		Key:   key,
		Value: value,
		JSON:  jsonData,
	}

	return nil
}

func (c *compiledLocale) representSubTreeReconcilingJSON() (map[string]any, yaerrors.Error) {
	if c.SubMap != nil {
		result := make(map[string]any)

		for k, v := range c.SubMap {
			if v.SubMap != nil {
				subResult, err := v.representSubTreeReconcilingJSON()
				if err != nil {
					return nil, err.Wrap(
						fmt.Sprintf("Failed to represent sub-tree for key '%s'", k),
					)
				}

				result[k] = subResult
			} else {
				result[k] = v.Value
			}
		}

		jsonData, err := json.Marshal(result)
		if err != nil {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				"Failed to marshal JSON for sub-tree",
			)
		}

		c.JSON = jsonData

		return result, nil
	}

	return nil, yaerrors.FromError(
		http.StatusInternalServerError,
		ErrNilLocale,
		"Failed to represent sub-tree",
	)
}

func (l *YaLocalizer) validateKeyCoverage() yaerrors.Error {
	if len(l.data) == 0 {
		return nil
	}

	keySets := make(map[string]map[string]struct{}, len(l.data))
	for lang, root := range l.data {
		if root == nil {
			return yaerrors.FromError(
				http.StatusInternalServerError,
				ErrNilLocale,
				fmt.Sprintf("Nil locale for language '%s'", lang),
			)
		}

		set := make(map[string]struct{})
		for key, child := range root.SubMap {
			collectCompositeKeys(child, key, set)
		}

		keySets[lang] = set
	}

	if l.fallbackLang == "" {
		var refLang string
		for k := range keySets {
			refLang = k

			break
		}

		ref := keySets[refLang]
		for lang, set := range keySets {
			if lang == refLang {
				continue
			}

			missingInLang, extraInLang := setDiff(ref, set)
			if len(missingInLang) > 0 || len(extraInLang) > 0 {
				return yaerrors.FromError(
					http.StatusTeapot,
					ErrMismatchedKeys,
					fmt.Sprintf(
						"Language '%s' keys mismatch. Missing: %v; Extra: %v; Reference: '%s'",
						lang, missingInLang, extraInLang, refLang,
					),
				)
			}

			for key := range ref {
				refStr, yaErr := l.data[refLang].retriveValueByCompositeKey(key)
				if yaErr != nil {
					return yaErr.Wrap(
						fmt.Sprintf("failed retrieving reference value for key '%s'", key),
					)
				}

				langStr, yaErr := l.data[lang].retriveValueByCompositeKey(key)
				if yaErr != nil {
					return yaErr.Wrap(
						fmt.Sprintf(
							"failed retrieving value for key '%s' and language '%s'",
							key,
							lang,
						),
					)
				}

				refPH := extractPlaceholdersSet(refStr)
				langPH := extractPlaceholdersSet(langStr)

				miss, extra := setDiff(refPH, langPH)
				if len(miss) > 0 || len(extra) > 0 {
					return yaerrors.FromError(
						http.StatusTeapot,
						ErrMismatchedPlaceholders,
						fmt.Sprintf(
							"Language '%s' placeholders mismatch for key '%s'. Missing: %v; Extra: %v; Reference: '%s'",
							lang,
							key,
							miss,
							extra,
							refLang,
						),
					)
				}
			}
		}

		return nil
	}

	defSet, ok := keySets[l.fallbackLang]
	if !ok {
		return yaerrors.FromError(
			http.StatusTeapot,
			ErrInvalidLanguage,
			fmt.Sprintf("Fallback language '%s' not found among locales", l.fallbackLang),
		)
	}

	for lang, set := range keySets {
		if lang == l.fallbackLang {
			continue
		}

		missingInDefault := subtractSets(set, defSet)
		if len(missingInDefault) > 0 {
			return yaerrors.FromError(
				http.StatusTeapot,
				ErrDefaultCoverage,
				fmt.Sprintf(
					"Default language '%s' missing keys present in '%s': %v",
					l.fallbackLang, lang, missingInDefault,
				),
			)
		}

		for key := range set {
			defStr, yaErr := l.data[l.fallbackLang].retriveValueByCompositeKey(key)
			if yaErr != nil {
				return yaErr.Wrap(fmt.Sprintf("failed retrieving default value for key '%s'", key))
			}

			langStr, yaErr := l.data[lang].retriveValueByCompositeKey(key)
			if yaErr != nil {
				return yaErr.Wrap(
					fmt.Sprintf(
						"failed retrieving value for key '%s' and language '%s'",
						key,
						lang,
					),
				)
			}

			defPH := extractPlaceholdersSet(defStr)
			langPH := extractPlaceholdersSet(langStr)

			miss, extra := setDiff(defPH, langPH)
			if len(miss) > 0 || len(extra) > 0 {
				return yaerrors.FromError(
					http.StatusTeapot,
					ErrMismatchedPlaceholders,
					fmt.Sprintf(
						"Language '%s' placeholders mismatch for key '%s'. Missing: %v; Extra: %v; Default: '%s'",
						lang,
						key,
						miss,
						extra,
						l.fallbackLang,
					),
				)
			}
		}
	}

	return nil
}
