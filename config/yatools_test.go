package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/config"
	"github.com/google/go-cmp/cmp"
)

type yaToolsNested struct {
	A string `json:"a"`
	B string `json:"b"`
}

type yaToolsSample struct {
	Name   string            `json:"name"`
	Count  int               `json:"count"`
	Nested yaToolsNested     `json:"nested"`
	Tags   []string          `json:"tags"`
	Labels map[string]string `json:"labels"`
}

const (
	yaToolsDirPerm  = 0o755
	yaToolsFilePerm = 0o644
)

func writeYaToolsFile(t *testing.T, dir, name, body string) {
	t.Helper()

	full := filepath.Join(dir, config.YaToolsDirName, name+config.YaToolsFileExtension)

	if err := os.MkdirAll(filepath.Dir(full), yaToolsDirPerm); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(full, []byte(body), yaToolsFilePerm); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func reserveEnv(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		t.Setenv(key, "")
	}
}

func TestLoadYaToolsConfigMergesHomeAndProject(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	t.Setenv("HOME", home)

	writeYaToolsFile(t, home, "sample", `{
		"name": "home",
		"count": 1,
		"nested": {"a": "ha", "b": "hb"},
		"tags": ["t1"],
		"labels": {"k1": "home"}
	}`)
	writeYaToolsFile(t, project, "sample", `{
		"name": "project",
		"nested": {"b": "pb"},
		"tags": ["t2", "t3"],
		"labels": {"k2": "project"}
	}`)

	got := yaToolsSample{}

	found, err := config.LoadYaToolsConfigFromDir(project, "sample", &got)
	if err != nil {
		t.Fatalf("LoadYaToolsConfigFromDir: %v", err)
	}

	if !found {
		t.Fatal("expected the config to be found")
	}

	want := yaToolsSample{
		Name:   "project",
		Count:  1,
		Nested: yaToolsNested{A: "ha", B: "pb"},
		Tags:   []string{"t2", "t3"},
		Labels: map[string]string{"k1": "home", "k2": "project"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("merged config mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadYaToolsConfigProjectOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := t.TempDir()
	writeYaToolsFile(t, project, "sample", `{"name": "project", "count": 7}`)

	got := yaToolsSample{}

	found, err := config.LoadYaToolsConfigFromDir(project, "sample", &got)
	if err != nil {
		t.Fatalf("LoadYaToolsConfigFromDir: %v", err)
	}

	if !found {
		t.Fatal("expected the config to be found")
	}

	if got.Name != "project" || got.Count != 7 {
		t.Fatalf("unexpected config: %+v", got)
	}
}

func TestLoadYaToolsConfigNoFilesKeepsDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	got := yaToolsSample{Name: "preset"}

	found, err := config.LoadYaToolsConfigFromDir(t.TempDir(), "sample", &got)
	if err != nil {
		t.Fatalf("LoadYaToolsConfigFromDir: %v", err)
	}

	if found {
		t.Fatal("expected no config to be found")
	}

	if got.Name != "preset" {
		t.Fatalf("expected preset value to be preserved, got %q", got.Name)
	}
}

func TestLoadYaToolsConfigInvalidJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := t.TempDir()
	writeYaToolsFile(t, project, "sample", `{"name":`)

	got := yaToolsSample{}

	if _, err := config.LoadYaToolsConfigFromDir(project, "sample", &got); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestLoadYaToolsConfigUsesWorkingDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := t.TempDir()
	writeYaToolsFile(t, project, "sample", `{"name": "cwd"}`)
	t.Chdir(project)

	got := yaToolsSample{}

	found, err := config.LoadYaToolsConfig("sample", &got)
	if err != nil {
		t.Fatalf("LoadYaToolsConfig: %v", err)
	}

	if !found || got.Name != "cwd" {
		t.Fatalf("expected working-directory config, got found=%v %+v", found, got)
	}
}

func TestSeedEnvFromYaToolsConfigFlattensValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reserveEnv(t, "THREADS", "RATIO", "ENABLED", "NAME", "TAGS", "NESTED_CHILD")

	project := t.TempDir()
	writeYaToolsFile(t, project, "tool", `{
		"THREADS": 16,
		"RATIO": 1.5,
		"ENABLED": true,
		"NAME": "value",
		"TAGS": ["a", "b"],
		"NESTED": {"CHILD": "c"}
	}`)
	t.Chdir(project)

	if err := config.SeedEnvFromYaToolsConfig("tool"); err != nil {
		t.Fatalf("SeedEnvFromYaToolsConfig: %v", err)
	}

	want := map[string]string{
		"THREADS":      "16",
		"RATIO":        "1.5",
		"ENABLED":      "true",
		"NAME":         "value",
		"TAGS":         "a,b",
		"NESTED_CHILD": "c",
	}

	for key, value := range want {
		if got := os.Getenv(key); got != value {
			t.Fatalf("env %s: got %q want %q", key, got, value)
		}
	}
}

func TestSeedEnvFromYaToolsConfigProjectOverridesHome(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	t.Setenv("HOME", home)
	reserveEnv(t, "NAME", "HOME_ONLY")

	writeYaToolsFile(t, home, "tool", `{"NAME": "home", "HOME_ONLY": "yes"}`)
	writeYaToolsFile(t, project, "tool", `{"NAME": "project"}`)
	t.Chdir(project)

	if err := config.SeedEnvFromYaToolsConfig("tool"); err != nil {
		t.Fatalf("SeedEnvFromYaToolsConfig: %v", err)
	}

	if got := os.Getenv("NAME"); got != "project" {
		t.Fatalf("NAME: got %q want %q", got, "project")
	}

	if got := os.Getenv("HOME_ONLY"); got != "yes" {
		t.Fatalf("HOME_ONLY: got %q want %q", got, "yes")
	}
}

func TestSeedEnvFromYaToolsConfigKeepsExistingEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("THREADS", "99")

	project := t.TempDir()
	writeYaToolsFile(t, project, "tool", `{"THREADS": 16}`)
	t.Chdir(project)

	if err := config.SeedEnvFromYaToolsConfig("tool"); err != nil {
		t.Fatalf("SeedEnvFromYaToolsConfig: %v", err)
	}

	if got := os.Getenv("THREADS"); got != "99" {
		t.Fatalf("THREADS: got %q want %q (existing env must win)", got, "99")
	}
}

func TestSeedEnvFromYaToolsConfigMissingFilesNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	if err := config.SeedEnvFromYaToolsConfig("tool"); err != nil {
		t.Fatalf("expected no error for missing files, got %v", err)
	}
}

func TestSeedEnvFromYaToolsConfigRejectsUnsupportedValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := t.TempDir()
	writeYaToolsFile(t, project, "tool", `{"BAD": [{"x": 1}]}`)
	t.Chdir(project)

	if err := config.SeedEnvFromYaToolsConfig("tool"); err == nil {
		t.Fatal("expected an error for an unsupported nested value")
	}
}

type yaToolsAppConfig struct {
	Threads int    `default:"8"`
	Region  string `default:"eu"`
	Token   string `default:"none"`
	Extra   string `default:"def"`
}

func TestYaToolsConfigPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wantProject := filepath.Join("proj", config.YaToolsDirName, "tool"+config.YaToolsFileExtension)
	if got := config.YaToolsConfigPath("proj", "tool"); got != wantProject {
		t.Fatalf("YaToolsConfigPath: got %q want %q", got, wantProject)
	}

	wantHome := filepath.Join(home, config.YaToolsDirName, "tool"+config.YaToolsFileExtension)
	if got := config.YaToolsHomeConfigPath("tool"); got != wantHome {
		t.Fatalf("YaToolsHomeConfigPath: got %q want %q", got, wantHome)
	}
}

func TestWriteYaToolsConfigToDirRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	want := yaToolsSample{
		Name:   "written",
		Count:  3,
		Nested: yaToolsNested{A: "x", B: "y"},
		Tags:   []string{"a", "b"},
		Labels: map[string]string{"k": "v"},
	}

	if err := config.WriteYaToolsConfigToDir(dir, "sample", &want); err != nil {
		t.Fatalf("WriteYaToolsConfigToDir: %v", err)
	}

	got := yaToolsSample{}

	found, err := config.LoadYaToolsConfigFromDir(dir, "sample", &got)
	if err != nil {
		t.Fatalf("LoadYaToolsConfigFromDir: %v", err)
	}

	if !found {
		t.Fatal("expected the written config to be found")
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestWriteYaToolsConfigCreatesDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	t.Chdir(dir)

	value := yaToolsSample{Name: "cwd"}
	if err := config.WriteYaToolsConfig("sample", &value); err != nil {
		t.Fatalf("WriteYaToolsConfig: %v", err)
	}

	path := filepath.Join(dir, config.YaToolsDirName, "sample"+config.YaToolsFileExtension)

	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("expected written file at %s: %v", path, statErr)
	}

	if perm := info.Mode().Perm(); perm != config.YaToolsFilePerm {
		t.Fatalf("file permissions: got %o want %o", perm, config.YaToolsFilePerm)
	}
}

func TestWriteYaToolsHomeConfigRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	want := yaToolsSample{Name: "home", Count: 9}
	if err := config.WriteYaToolsHomeConfig("sample", &want); err != nil {
		t.Fatalf("WriteYaToolsHomeConfig: %v", err)
	}

	expected := filepath.Join(home, config.YaToolsDirName, "sample"+config.YaToolsFileExtension)
	if _, statErr := os.Stat(expected); statErr != nil {
		t.Fatalf("expected written file at %s: %v", expected, statErr)
	}

	got := yaToolsSample{}

	found, err := config.LoadYaToolsConfigFromDir(t.TempDir(), "sample", &got)
	if err != nil {
		t.Fatalf("LoadYaToolsConfigFromDir: %v", err)
	}

	if !found || got.Name != "home" || got.Count != 9 {
		t.Fatalf("expected per-user config to round-trip, got found=%v %+v", found, got)
	}
}

func mkYaToolsDirAt(t *testing.T, dir, name string) {
	t.Helper()

	full := filepath.Join(dir, config.YaToolsDirName, name+config.YaToolsFileExtension)

	if err := os.MkdirAll(full, yaToolsDirPerm); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func TestLoadYaToolsConfigProjectReadError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := t.TempDir()
	mkYaToolsDirAt(t, project, "sample")

	got := yaToolsSample{}
	if _, err := config.LoadYaToolsConfigFromDir(project, "sample", &got); err == nil {
		t.Fatal("expected a read error when the project config path is a directory")
	}
}

func TestLoadYaToolsConfigHomeReadError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkYaToolsDirAt(t, home, "sample")

	got := yaToolsSample{}
	if _, err := config.LoadYaToolsConfigFromDir(t.TempDir(), "sample", &got); err == nil {
		t.Fatal("expected a read error when the per-user config path is a directory")
	}
}

func TestSeedEnvFromYaToolsConfigReadError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := t.TempDir()
	mkYaToolsDirAt(t, project, "tool")
	t.Chdir(project)

	if err := config.SeedEnvFromYaToolsConfig("tool"); err == nil {
		t.Fatal("expected a read error when the config path is a directory")
	}
}

func TestWriteYaToolsConfigMkdirError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	blocker := filepath.Join(dir, config.YaToolsDirName)
	if err := os.WriteFile(blocker, []byte("x"), yaToolsFilePerm); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	value := yaToolsSample{Name: "x"}
	if err := config.WriteYaToolsConfigToDir(dir, "sample", &value); err == nil {
		t.Fatal("expected a directory-creation error when .yatools is a file")
	}
}

type yaToolsUnmarshalable struct {
	Channel chan int `json:"channel"`
}

func TestWriteYaToolsConfigMarshalError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	value := yaToolsUnmarshalable{Channel: make(chan int)}
	if err := config.WriteYaToolsConfigToDir(t.TempDir(), "sample", &value); err == nil {
		t.Fatal("expected a marshal error for a value that cannot be encoded as JSON")
	}
}

func TestWriteYaToolsHomeConfigWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")

	value := yaToolsSample{Name: "x"}
	if err := config.WriteYaToolsHomeConfig("sample", &value); err == nil {
		t.Fatal("expected an error when the home directory is unavailable")
	}
}

func TestSeedEnvFromYaToolsConfigHomeReadError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkYaToolsDirAt(t, home, "tool")
	t.Chdir(t.TempDir())

	if err := config.SeedEnvFromYaToolsConfig("tool"); err == nil {
		t.Fatal("expected a read error for the per-user config path")
	}
}

func TestSeedEnvFromYaToolsConfigRejectsNestedUnsupported(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	project := t.TempDir()
	writeYaToolsFile(t, project, "tool", `{"OUTER": {"INNER": [{"x": 1}]}}`)
	t.Chdir(project)

	if err := config.SeedEnvFromYaToolsConfig("tool"); err == nil {
		t.Fatal("expected an error for a nested unsupported value")
	}
}

func TestLoadConfigStructFromEnvWithYaToolsDotEnvBeatsYaTools(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reserveEnv(t, "REGION")

	project := t.TempDir()
	writeYaToolsFile(t, project, "app", `{"REGION": "from-yatools"}`)

	dotenv := filepath.Join(project, ".env")
	if err := os.WriteFile(dotenv, []byte("REGION=from-dotenv\n"), yaToolsFilePerm); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Chdir(project)

	got := yaToolsAppConfig{}
	if err := config.LoadConfigStructFromEnvWithYaTools("app", &got, nil); err != nil {
		t.Fatalf("LoadConfigStructFromEnvWithYaTools: %v", err)
	}

	if got.Region != "from-dotenv" {
		t.Fatalf("Region: got %q want %q (.env must outrank .yatools)", got.Region, "from-dotenv")
	}
}

func TestLoadYaToolsConfigNilDestination(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := config.LoadYaToolsConfigFromDir(t.TempDir(), "sample", (*yaToolsSample)(nil)); err == nil {
		t.Fatal("expected an error for a nil destination")
	}
}

func TestWriteYaToolsConfigNilValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := config.WriteYaToolsConfigToDir(t.TempDir(), "sample", (*yaToolsSample)(nil)); err == nil {
		t.Fatal("expected an error for a nil value")
	}
}

func TestLoadConfigStructFromEnvWithYaToolsNilInstance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	if err := config.LoadConfigStructFromEnvWithYaTools[yaToolsAppConfig]("app", nil, nil); err == nil {
		t.Fatal("expected an error for a nil instance")
	}
}

func TestLoadConfigStructFromEnvWithYaToolsPrecedence(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	t.Setenv("HOME", home)
	reserveEnv(t, "REGION", "TOKEN")
	t.Setenv("THREADS", "20")

	writeYaToolsFile(t, home, "app", `{"THREADS": 10, "REGION": "home", "TOKEN": "htoken"}`)
	writeYaToolsFile(t, project, "app", `{"REGION": "project"}`)
	t.Chdir(project)

	got := yaToolsAppConfig{}

	if err := config.LoadConfigStructFromEnvWithYaTools("app", &got, nil); err != nil {
		t.Fatalf("LoadConfigStructFromEnvWithYaTools: %v", err)
	}

	want := yaToolsAppConfig{
		Threads: 20,
		Region:  "project",
		Token:   "htoken",
		Extra:   "def",
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("config precedence mismatch (-want +got):\n%s", diff)
	}
}
