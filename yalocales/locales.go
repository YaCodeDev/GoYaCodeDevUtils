package yalocales

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

type (
	Localizer interface {
		// DeriveNewDefaultLang creates a new Localizer instance where the data is derived from the current instance,
		// but with a different default language.
		// Note: This method ONLY works if enforceLocaleConsistency was enabled on the source Localizer.
		//
		// Example usage:
		//
		//	newLoc, err := loc.DeriveNewDefaultLang("en")
		//	if err != nil {
		//	    // handle error
		//	}
		DeriveNewDefaultLang(newDefaultLang string) (Localizer, yaerrors.Error)
		// GetFormattedDefaultLangValueByCompositeKey retrieves a localized string value and formats
		// placeholders in the form {name} using provided args. Args may be either:
		// - map[string]string with keys matching placeholder names
		// - struct (or pointer to struct) where field names match exportIdentifier(placeholder)
		// If any required placeholder is missing in args, an error is returned.
		//
		// Example usage:
		//
		//	msg, err := loc.GetFormattedDefaultLangValueByCompositeKey("greeting", map[string]string{"name": "John"})
		//	// or using struct
		//	type Args struct {
		//		Name string
		//	}
		//	msg, err := loc.GetFormattedDefaultLangValueByCompositeKey("greeting", Args{Name: "John"})
		//	if err != nil {
		//	    // handle error
		//	}
		GetFormattedDefaultLangValueByCompositeKey(
			key string,
			args any,
		) (string, yaerrors.Error)
		// GetDefaultLangJSONByCompositeKey retrieves the JSON representation of the value associated with the composite key
		// for the default language. If the key is not found, it returns an error.
		//
		// Example usage:
		//
		//	jsonData, err := loc.GetDefaultLangJSONByCompositeKey("greeting")
		//	if err != nil {
		//	    // handle error
		//	}
		GetDefaultLangJSONByCompositeKey(key string) ([]byte, yaerrors.Error)
		// GetDefaultLangValueByCompositeKey retrieves the string value associated with the composite key for the default
		// language. If the key is not found, it returns an error.
		//
		// Example usage:
		//
		//	msg, err := loc.GetDefaultLangValueByCompositeKey("greeting")
		//	if err != nil {
		//	    // handle error
		//	}
		//
		GetDefaultLangValueByCompositeKey(key string) (string, yaerrors.Error)
		// GetFormattedValueByCompositeKeyAndLang retrieves a localized string value and formats
		// placeholders in the form {name} using provided args. Args may be either:
		// - map[string]string with keys matching placeholder names
		// - struct (or pointer to struct) where field names match exportIdentifier(placeholder)
		// If any required placeholder is missing in args, an error is returned.
		//
		// Example usage:
		//
		//	// using map
		//	msg, err := loc.GetFormattedValueByCompositeKeyAndLang("greeting", "en", map[string]string{"name": "John"})
		//
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
		//	jsonData, err := loc.GetJSONByCompositeKeyAndLang("greeting", "en")
		//	if err != nil {
		//	    // handle error
		//	}
		GetJSONByCompositeKeyAndLang(key string, lang string) ([]byte, yaerrors.Error)
		// GetValueByCompositeKeyAndLang retrieves the string value associated with the composite key for the specified
		// language. If the key or language is not found, it falls back to the default language if set, otherwise returns
		// an error.
		//
		// Example usage:
		//
		//	msg, err := loc.GetValueByCompositeKeyAndLang("greeting", "en")
		//	if err != nil {
		//	    // handle error
		//	}
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
		fallbackLang       string
		data               map[string]*compiledLocale
		enforceConsistency bool
	}
)

// NewLocalizer creates a new Localizer instance with an optional fallback language.
// Note: enforceLocaleConsistency enables locales to be derived from each other, replacing the default in the copy.
//
// Example usage:
//
//	loc := locales.NewLocalizer("en", true)
func NewLocalizer(fallbackLang string, enforceLocaleConsistency bool) Localizer {
	return NewYaLocalizer(fallbackLang, enforceLocaleConsistency)
}

// NewYaLocalizer creates a new YaLocalizer instance with an optional fallback language.
//
// Example usage:
//
//	loc := locales.NewYaLocalizer("en", true)
func NewYaLocalizer(fallbackLang string, enforceLocaleConsistency bool) *YaLocalizer {
	return &YaLocalizer{
		fallbackLang:       fallbackLang,
		data:               make(map[string]*compiledLocale),
		enforceConsistency: enforceLocaleConsistency,
	}
}

// DeriveNewDefaultLang creates a new Localizer instance where the data is derived from the current instance,
// but with a different default language.
// Note: This method ONLY works if enforceLocaleConsistency was enabled on the source Localizer.
//
// Example usage:
//
//	newLoc, err := loc.DeriveNewDefaultLang("en")
//	if err != nil {
//	    // handle error
//	}
func (l *YaLocalizer) DeriveNewDefaultLang(newDefaultLang string) (Localizer, yaerrors.Error) {
	if !l.enforceConsistency {
		return nil, yaerrors.FromError(
			http.StatusTeapot,
			ErrConsistencyRequired,
			"Cannot derive new default language when locale consistency is not enforced",
		)
	}

	for lang := range l.data {
		if lang == newDefaultLang {
			return &YaLocalizer{
				fallbackLang:       newDefaultLang,
				data:               l.data,
				enforceConsistency: true,
			}, nil
		}
	}

	return nil, yaerrors.FromError(
		http.StatusTeapot,
		ErrInvalidLanguage,
		fmt.Sprintf("Language '%s' not found among locales", newDefaultLang),
	)
}

// GetFormattedDefaultLangValueByCompositeKey retrieves a localized string value and formats
// placeholders in the form {name} using provided args. Args may be either:
// - map[string]string with keys matching placeholder names
// - struct (or pointer to struct) where field names match exportIdentifier(placeholder)
// If any required placeholder is missing in args, an error is returned.
//
// Example usage:
//
//	msg, err := loc.GetFormattedDefaultLangValueByCompositeKey("greeting", map[string]string{"name": "John"})
//	// or using struct
//	type Args struct {
//		Name string
//	}
//	msg, err := loc.GetFormattedDefaultLangValueByCompositeKey("greeting", Args{Name: "John"})
//	if err != nil {
//	    // handle error
//	}
func (l *YaLocalizer) GetFormattedDefaultLangValueByCompositeKey(
	key string,
	args any,
) (string, yaerrors.Error) {
	if l.fallbackLang == "" {
		return "", yaerrors.FromError(
			http.StatusTeapot,
			ErrNoDefaultLanguage,
			"Cannot get default language value when no default language is set",
		)
	}

	value, err := l.GetFormattedValueByCompositeKeyAndLang(key, l.fallbackLang, args)
	if err != nil {
		return "", err.Wrap("failed to get formatted value from default language")
	}

	return value, nil
}

// GetDefaultLangJSONByCompositeKey retrieves the JSON representation of the value associated with the composite key for
// the default language. If the key is not found, it returns an error.
//
// Example usage:
//
//	jsonData, err := loc.GetDefaultLangJSONByCompositeKey("greeting")
//	if err != nil {
//	    // handle error
//	}

func (l *YaLocalizer) GetDefaultLangJSONByCompositeKey(key string) ([]byte, yaerrors.Error) {
	if l.fallbackLang == "" {
		return nil, yaerrors.FromError(
			http.StatusTeapot,
			ErrNoDefaultLanguage,
			"Cannot get default language JSON when no default language is set",
		)
	}

	value, err := l.GetJSONByCompositeKeyAndLang(key, l.fallbackLang)
	if err != nil {
		return nil, err.Wrap("failed to get JSON from default language")
	}

	return value, nil
}

// GetDefaultLangValueByCompositeKey retrieves the string value associated with the composite key for the default
// language. If the key is not found, it returns an error.
//
// Example usage:
//
//	msg, err := loc.GetDefaultLangValueByCompositeKey("greeting")
//	if err != nil {
//	    // handle error
//	}
func (l *YaLocalizer) GetDefaultLangValueByCompositeKey(key string) (string, yaerrors.Error) {
	if l.fallbackLang == "" {
		return "", yaerrors.FromError(
			http.StatusTeapot,
			ErrNoDefaultLanguage,
			"Cannot get default language value when no default language is set",
		)
	}

	value, err := l.GetValueByCompositeKeyAndLang(key, l.fallbackLang)
	if err != nil {
		return "", err.Wrap("failed to get value from default language")
	}

	return value, nil
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
	if l.data[lang] == nil {
		if l.fallbackLang != "" && lang != l.fallbackLang {
			lang = l.fallbackLang
		} else {
			return nil, yaerrors.FromError(
				http.StatusNotFound,
				ErrInvalidLanguage,
				fmt.Sprintf("Language '%s' not found", lang),
			)
		}
	}

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
	if l.data[lang] == nil {
		if l.fallbackLang != "" && lang != l.fallbackLang {
			lang = l.fallbackLang
		} else {
			return "", yaerrors.FromError(
				http.StatusNotFound,
				ErrInvalidLanguage,
				fmt.Sprintf("Language '%s' not found", lang),
			)
		}
	}

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
//	// using map
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
	var locales map[string]json.RawMessage

	err := json.Unmarshal(data, &locales)
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("Failed to parse JSON at key '%s'", compositeKey),
		)
	}

	for key, rawValue := range locales {
		if key == "" {
			return yaerrors.FromError(
				http.StatusTeapot,
				ErrInvalidTranslation,
				fmt.Sprintf("Empty translation key at path '%s'", compositeKey),
			)
		}

		fullKey := l.joinCompositeKeys(compositeKey, key)

		yaErr := l.processJSONNode(rawValue, lang, fullKey)
		if yaErr != nil {
			return yaErr.Wrap(fmt.Sprintf("Failed to process JSON node at key '%s'", fullKey))
		}
	}

	return nil
}

func (l *YaLocalizer) processJSONNode(
	rawValue json.RawMessage,
	lang, fullKey string,
) yaerrors.Error {
	var value string
	if err := json.Unmarshal(rawValue, &value); err == nil {
		yaErr := l.insertByCompositeKeyAndLang(fullKey, lang, value)
		if yaErr != nil {
			return yaErr.Wrap(fmt.Sprintf("Failed to insert locale at key '%s'", fullKey))
		}

		return nil
	}

	var block map[string]json.RawMessage
	if err := json.Unmarshal(rawValue, &block); err == nil {
		for key, subRawValue := range block {
			if key == "" {
				return yaerrors.FromError(
					http.StatusTeapot,
					ErrInvalidTranslation,
					fmt.Sprintf("Empty translation key at path '%s'", fullKey),
				)
			}

			subKey := l.joinCompositeKeys(fullKey, key)

			yaErr := l.processJSONNode(subRawValue, lang, subKey)
			if yaErr != nil {
				return yaErr.Wrap(fmt.Sprintf("Failed to process JSON block at key '%s'", subKey))
			}
		}

		return nil
	}

	return yaerrors.FromError(
		http.StatusTeapot,
		ErrInvalidTranslation,
		fmt.Sprintf(
			"Invalid translation value at key '%s'; expected string or JSON object",
			fullKey,
		),
	)
}

func (l *YaLocalizer) joinCompositeKeys(prefix, key string) string {
	if prefix == "" {
		return key
	}

	return prefix + Separator + key
}

func (l *YaLocalizer) insertByCompositeKeyAndLang(key, lang, value string) yaerrors.Error {
	if key == "" {
		return yaerrors.FromError(
			http.StatusTeapot,
			ErrInvalidTranslation,
			"Empty translation key is not allowed",
		)
	}

	if _, ok := l.data[lang]; !ok {
		if lang == "" {
			return yaerrors.FromError(
				http.StatusTeapot,
				ErrInvalidLanguage,
				"Language tag cannot be empty",
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

	if l.fallbackLang == "" || l.enforceConsistency {
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
