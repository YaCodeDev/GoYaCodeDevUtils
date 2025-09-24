package localizer

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

type Localizer struct {
	strings     map[string]map[string]string
	defaultLang string
}

func NewLocalizer(fsys fs.FS, defaultLang string) (*Localizer, yaerrors.Error) {
	loc := &Localizer{
		strings:     make(map[string]map[string]string),
		defaultLang: defaultLang,
	}

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		lang := strings.TrimSuffix(filepath.Base(path), ".json")

		bytes, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		var data map[string]string
		if err := json.Unmarshal(bytes, &data); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}

		loc.strings[lang] = data

		return nil
	})
	if err != nil {
		return nil, yaerrors.FromError(500, err, "failed to walk directory")
	}

	return loc, nil
}

func (l *Localizer) Lang(lang string) func(key string) string {
	return func(key string) string {
		if val, ok := l.strings[lang][key]; ok {
			return val
		}

		if val, ok := l.strings[l.defaultLang][key]; ok {
			return val
		}

		return key
	}
}

func (l *Localizer) T(lang, key string) string {
	if val, ok := l.strings[lang][key]; ok {
		return val
	}

	if val, ok := l.strings[l.defaultLang][key]; ok {
		return val
	}

	return key
}
