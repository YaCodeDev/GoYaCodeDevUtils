package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// LoadYaToolsConfig merges the per-user ~/.yatools/<name>.json file with a project-level
// .yatools/<name>.json file located in the current working directory and decodes the
// result into instance. Values from the project file override the per-user file on a
// per-key basis, following the standard encoding/json semantics of decoding into an
// already-populated value. Missing files are skipped. It reports whether at least one file
// was found.
func LoadYaToolsConfig[T any](name string, instance *T) (bool, yaerrors.Error) {
	return LoadYaToolsConfigFromDir(".", name, instance)
}

// LoadYaToolsConfigFromDir behaves like LoadYaToolsConfig but reads the project-level file
// from projectDir instead of the current working directory. This suits tools that operate
// on a target directory that differs from where they are invoked.
func LoadYaToolsConfigFromDir[T any](
	projectDir string,
	name string,
	instance *T,
) (bool, yaerrors.Error) {
	if instance == nil {
		return false, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNilYaToolsDestination,
			"load yatools config",
		)
	}

	homeApplied, homeErr := applyYaToolsFile(yaToolsHomePath(name), instance)
	if homeErr != nil {
		return false, homeErr.Wrap("load yatools config: per-user file")
	}

	projectApplied, projectErr := applyYaToolsFile(yaToolsPath(projectDir, name), instance)
	if projectErr != nil {
		return false, projectErr.Wrap("load yatools config: project file")
	}

	return homeApplied || projectApplied, nil
}

// SeedEnvFromYaToolsConfig imports values from the .yatools/<name>.json files into the
// process environment for tools that are configured through environment variables. The
// project-level file in the current working directory is applied before the per-user
// ~/.yatools/<name>.json file, and neither overwrites a variable that already holds a
// value, so it must run after LoadDotEnv for the real environment and .env to keep
// priority. Nested objects are flattened with YaToolsKeySeparator and keys are upper-cased
// so they line up with the SCREAMING_SNAKE_CASE keys produced by LoadConfigStructFromEnv.
func SeedEnvFromYaToolsConfig(name string) yaerrors.Error {
	if err := seedEnvFromYaToolsFile(yaToolsPath(".", name)); err != nil {
		return err.Wrap("seed env from yatools config: project file")
	}

	if err := seedEnvFromYaToolsFile(yaToolsHomePath(name)); err != nil {
		return err.Wrap("seed env from yatools config: per-user file")
	}

	return nil
}

// WriteYaToolsConfig encodes value as indented JSON and writes it to the project-level
// .yatools/<name>.json file in the current working directory, creating the directory when
// necessary. It is the counterpart of LoadYaToolsConfig.
func WriteYaToolsConfig[T any](name string, value *T) yaerrors.Error {
	return WriteYaToolsConfigToDir(".", name, value)
}

// WriteYaToolsConfigToDir behaves like WriteYaToolsConfig but writes the file inside dir
// instead of the current working directory.
func WriteYaToolsConfigToDir[T any](dir string, name string, value *T) yaerrors.Error {
	return writeYaToolsFile(yaToolsPath(dir, name), value)
}

// WriteYaToolsHomeConfig encodes value as indented JSON and writes it to the per-user
// ~/.yatools/<name>.json file, creating the directory when necessary.
func WriteYaToolsHomeConfig[T any](name string, value *T) yaerrors.Error {
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			homeErr,
			"resolve home directory for yatools config",
		)
	}

	return writeYaToolsFile(yaToolsPath(home, name), value)
}

// YaToolsConfigPath returns the .yatools/<name>.json path inside dir.
func YaToolsConfigPath(dir string, name string) string {
	return yaToolsPath(dir, name)
}

// YaToolsHomeConfigPath returns the per-user ~/.yatools/<name>.json path, or an empty
// string when the home directory cannot be determined.
func YaToolsHomeConfigPath(name string) string {
	return yaToolsHomePath(name)
}

func writeYaToolsFile[T any](path string, value *T) yaerrors.Error {
	if value == nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNilYaToolsValue,
			"write yatools config "+path,
		)
	}

	data, marshalErr := json.MarshalIndent(value, "", "  ")
	if marshalErr != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			marshalErr,
			"encode yatools config "+path,
		)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(path), YaToolsDirPerm); mkdirErr != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			mkdirErr,
			"create yatools directory for "+path,
		)
	}

	if writeErr := os.WriteFile(path, append(data, '\n'), YaToolsFilePerm); writeErr != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			writeErr,
			"write yatools config "+path,
		)
	}

	return nil
}

func applyYaToolsFile[T any](path string, instance *T) (bool, yaerrors.Error) {
	data, found, readErr := readYaToolsFile(path)
	if readErr != nil {
		return false, readErr
	}

	if !found {
		return false, nil
	}

	if unmarshalErr := json.Unmarshal(data, instance); unmarshalErr != nil {
		return false, yaerrors.FromError(
			http.StatusInternalServerError,
			unmarshalErr,
			"decode yatools config "+path,
		)
	}

	return true, nil
}

func seedEnvFromYaToolsFile(path string) yaerrors.Error {
	data, found, readErr := readYaToolsFile(path)
	if readErr != nil {
		return readErr
	}

	if !found {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	raw := map[string]any{}
	if decodeErr := decoder.Decode(&raw); decodeErr != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			decodeErr,
			"decode yatools config "+path,
		)
	}

	flattened := map[string]string{}
	if flattenErr := flattenYaToolsEnv("", raw, flattened); flattenErr != nil {
		return flattenErr.Wrap("flatten yatools config " + path)
	}

	return setEnvIfAbsent(flattened)
}

func readYaToolsFile(path string) ([]byte, bool, yaerrors.Error) {
	if path == "" {
		return nil, false, nil
	}

	content, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil, false, nil
		}

		return nil, false, yaerrors.FromError(
			http.StatusInternalServerError,
			readErr,
			"read yatools config "+path,
		)
	}

	return content, true, nil
}

func flattenYaToolsEnv(
	prefix string,
	raw map[string]any,
	out map[string]string,
) yaerrors.Error {
	for key, value := range raw {
		envKey := strings.ToUpper(key)
		if prefix != "" {
			envKey = prefix + YaToolsKeySeparator + envKey
		}

		switch typed := value.(type) {
		case nil:
			continue
		case map[string]any:
			if err := flattenYaToolsEnv(envKey, typed, out); err != nil {
				return err
			}
		case []any:
			joined, joinErr := joinYaToolsArray(typed)
			if joinErr != nil {
				return joinErr.Wrap("flatten array " + envKey)
			}

			out[envKey] = joined
		default:
			scalar, scalarErr := yaToolsScalar(typed)
			if scalarErr != nil {
				return scalarErr.Wrap("flatten value " + envKey)
			}

			out[envKey] = scalar
		}
	}

	return nil
}

func joinYaToolsArray(values []any) (string, yaerrors.Error) {
	parts := make([]string, 0, len(values))

	for _, value := range values {
		scalar, scalarErr := yaToolsScalar(value)
		if scalarErr != nil {
			return "", scalarErr
		}

		parts = append(parts, scalar)
	}

	return strings.Join(parts, YaToolsArraySeparator), nil
}

func yaToolsScalar(value any) (string, yaerrors.Error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case json.Number:
		return typed.String(), nil
	default:
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			ErrUnsupportedYaToolsValue,
			fmt.Sprintf("yatools config value of type %T", value),
		)
	}
}

func setEnvIfAbsent(values map[string]string) yaerrors.Error {
	for key, value := range values {
		if os.Getenv(key) != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				"set environment variable "+key,
			)
		}
	}

	return nil
}

func yaToolsPath(dir string, name string) string {
	return filepath.Join(dir, YaToolsDirName, name+YaToolsFileExtension)
}

func yaToolsHomePath(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return yaToolsPath(home, name)
}
