package yalocales

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// compiledLocale represents a compiled localization file node or leaf
type compiledLocale struct {
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

	subLocale, ok := c.SubMap[keyPart[0]]

	if !ok {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			ErrKeyNotFound,
			fmt.Sprintf("Key '%s' not found", keyPart[0]),
		)
	}

	if len(keyPart) == 1 {
		return subLocale.JSON, nil
	}

	value, err := subLocale.retriveJSONByCompositeKey(keyPart[1])
	if err != nil {
		return nil, err.Wrap(fmt.Sprintf("Failed to retrieve JSON for key part '%s'", keyPart[0]))
	}

	return value, nil
}

func (c *compiledLocale) retriveValueByCompositeKey(key string) (string, yaerrors.Error) {
	if c == nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNilLocale,
			"Locale is nil",
		)
	}
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

	subLocale, ok := c.SubMap[keyPart[0]]

	if !ok || subLocale == nil {
		return "", yaerrors.FromError(
			http.StatusNotFound,
			ErrKeyNotFound,
			fmt.Sprintf("Key '%s' not found", keyPart[0]),
		)
	}

	if len(keyPart) == 1 {
		return subLocale.Value, nil
	}

	value, err := subLocale.retriveValueByCompositeKey(keyPart[1])
	if err != nil {
		return "", err.Wrap(fmt.Sprintf("Failed to retrieve value for key part '%s'", keyPart[0]))
	}

	return value, nil
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

	if c.Value != "" {
		return yaerrors.FromError(
			http.StatusTeapot,
			ErrPathConflict,
			fmt.Sprintf("Key '%s' is a leaf and cannot contain subkeys", c.Key),
		)
	}

	if len(keyPart) == keySplitMaxParts {
		if c.SubMap == nil {
			c.SubMap = make(map[string]*compiledLocale)
		}

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

	if c.SubMap == nil {
		c.SubMap = make(map[string]*compiledLocale)
	}

	if existing, ok := c.SubMap[key]; ok {
		if existing != nil && existing.SubMap != nil {
			return yaerrors.FromError(
				http.StatusTeapot,
				ErrPathConflict,
				fmt.Sprintf("Key '%s' already exists as a namespace", key),
			)
		}

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
