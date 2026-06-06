package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tty00a381/anybot/core"
	absdk "github.com/tty00a381/anybot/sdk"
	"gopkg.in/yaml.v3"
)

func TestEnsurePluginConfigEntryAddsDisabledPlaceholder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	data := []byte(`runtime:
  log_level: info
plugins:
  help:
    enabled: true
    config: {}
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := EnsurePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("entry should be added")
	}
	out := readFile(t, path)
	if !strings.Contains(out, "weather:") ||
		!strings.Contains(out, "enabled: false") ||
		!strings.Contains(out, "config: {}") {
		t.Fatalf("config:\n%s", out)
	}
	changed, err = EnsurePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("existing entry should be left unchanged")
	}
}

func TestEnsurePluginConfigEntryCreatesPluginsMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsurePluginConfigEntry(path, "weather"); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "plugins:") || !strings.Contains(out, "weather:") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestEnsurePluginConfigEntryUsesPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("plugin_config_dir: plugins.d\nplugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := EnsurePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("entry should be added")
	}
	out := readFile(t, filepath.Join(dir, "plugins.d", "weather.yaml"))
	if !strings.Contains(out, "enabled: false") || !strings.Contains(out, "config: {}") {
		t.Fatalf("plugin config:\n%s", out)
	}
	main := readFile(t, path)
	if strings.Contains(main, "weather:") {
		t.Fatalf("main config should not contain split plugin entry:\n%s", main)
	}
}

func TestSetPluginEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("plugins:\n  weather:\n    enabled: false\n    config: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SetPluginEnabled(path, "weather", true)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("enable should change config")
	}
	out := readFile(t, path)
	if !strings.Contains(out, "enabled: true") {
		t.Fatalf("config:\n%s", out)
	}
	changed, err = SetPluginEnabled(path, "weather", true)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("same enabled state should be unchanged")
	}
}

func TestSetPluginEnabledUsesPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("plugin_config_dir: plugins.d\nplugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPluginEnabled(path, "weather", true); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, filepath.Join(dir, "plugins.d", "weather.yaml"))
	if !strings.Contains(out, "enabled: true") {
		t.Fatalf("plugin config:\n%s", out)
	}
}

func TestSetPluginEnabledCreatesEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPluginEnabled(path, "weather", true); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "weather:") || !strings.Contains(out, "enabled: true") {
		t.Fatalf("config:\n%s", out)
	}
	if strings.Contains(out, "外部插件") {
		t.Fatalf("generic entry should not contain external-plugin comments:\n%s", out)
	}
}

func TestSetPluginEnabledNormalizesNullEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("plugins:\n  weather:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SetPluginEnabled(path, "weather", true)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("null entry should be normalized")
	}
	out := readFile(t, path)
	if !strings.Contains(out, "weather:") || !strings.Contains(out, "enabled: true") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestParsePluginConfigChanges(t *testing.T) {
	change, err := ParsePluginConfigChanges([]string{"-reset", "provider.model", "history.enabled"})
	if err != nil {
		t.Fatal(err)
	}
	if len(change.Assignments) != 0 ||
		len(change.ResetPaths) != 2 ||
		strings.Join(change.ResetPaths[0], ".") != "provider.model" ||
		strings.Join(change.ResetPaths[1], ".") != "history.enabled" {
		t.Fatalf("change = %#v", change)
	}

	change, err = ParsePluginConfigChanges([]string{"-reset=command"})
	if err != nil {
		t.Fatal(err)
	}
	if len(change.ResetPaths) != 1 || strings.Join(change.ResetPaths[0], ".") != "command" {
		t.Fatalf("change = %#v", change)
	}

	change, err = ParsePluginConfigChanges([]string{"command=docs", "limit=3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(change.Assignments) != 2 || len(change.ResetPaths) != 0 {
		t.Fatalf("change = %#v", change)
	}
}

func TestParsePluginConfigChangesRejectsMixedSetAndReset(t *testing.T) {
	_, err := ParsePluginConfigChanges([]string{"-reset", "command", "limit=3"})
	if err == nil || !strings.Contains(err.Error(), "不能同时设置和重置插件配置") {
		t.Fatalf("err = %v", err)
	}
}

func TestRemovePluginConfigValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(`plugins:
  ai:
    enabled: true
    config:
      provider:
        model: custom
        timeout: 30s
      history:
        enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := RemovePluginConfigValues(path, "ai", [][]string{{"provider", "model"}, {"history", "enabled"}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("reset should remove configured values")
	}
	out := readFile(t, path)
	if strings.Contains(out, "model: custom") ||
		strings.Contains(out, "history:") ||
		!strings.Contains(out, "timeout: 30s") {
		t.Fatalf("config:\n%s", out)
	}
	changed, err = RemovePluginConfigValues(path, "ai", [][]string{{"provider", "model"}})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("missing config path should be unchanged")
	}
}

func TestRemovePluginConfigValuesUsesPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	pluginPath := filepath.Join(dir, "plugins.d", "help.yaml")
	if err := os.Mkdir(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("plugin_config_dir: plugins.d\nplugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, []byte(`enabled: true
config:
  command: docs
  lines:
    - custom
`), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := RemovePluginConfigValues(path, "help", [][]string{{"command"}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("split plugin config should be changed")
	}
	out := readFile(t, pluginPath)
	if strings.Contains(out, "command: docs") || !strings.Contains(out, "custom") {
		t.Fatalf("plugin config:\n%s", out)
	}
	main := readFile(t, path)
	if strings.Contains(main, "command:") {
		t.Fatalf("main config should not be changed:\n%s", main)
	}
}

func TestRemovePluginConfigValuesRejectsScalarParent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(`plugins:
  help:
    enabled: true
    config:
      command: docs
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RemovePluginConfigValues(path, "help", [][]string{{"command", "name"}})
	if err == nil || !strings.Contains(err.Error(), "command must be a YAML mapping") {
		t.Fatalf("err = %v", err)
	}
}

func TestRemovePluginConfigEntryUsesPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	pluginPath := filepath.Join(dir, "plugins.d", "weather.yaml")
	if err := os.Mkdir(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("plugin_config_dir: plugins.d\nplugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := RemovePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("entry should be removed")
	}
	if _, err := os.Stat(pluginPath); !os.IsNotExist(err) {
		t.Fatalf("plugin config should be removed: %v", err)
	}
}

func TestRemovePluginConfigEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(`plugins:
  weather:
    enabled: true
    config: {}
  help:
    enabled: true
    config: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := RemovePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("entry should be removed")
	}
	out := readFile(t, path)
	if strings.Contains(out, "weather:") || !strings.Contains(out, "help:") {
		t.Fatalf("config:\n%s", out)
	}
	changed, err = RemovePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("missing entry should be unchanged")
	}
}

func TestSyncPluginConfigEntriesUsesRegistryDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	data := []byte(`plugins:
  echo:
    enabled: false
    config: {}
  help:
    enabled: true
    config:
      command: custom
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed == 0 {
		t.Fatal("sync should add defaults")
	}
	out := readFile(t, path)
	if !strings.Contains(out, "echo:") || !strings.Contains(out, "command: echo") {
		t.Fatalf("echo default missing:\n%s", out)
	}
	if !strings.Contains(out, "command: custom") {
		t.Fatalf("existing config was overwritten:\n%s", out)
	}
	changed, err = SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 {
		t.Fatalf("second sync changed %d entries", changed)
	}
}

func TestSyncPluginConfigEntriesUsesPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	pluginPath := filepath.Join(dir, "plugins.d", "echo.yaml")
	if err := os.Mkdir(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("plugin_config_dir: plugins.d\nplugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed = %d", changed)
	}
	out := readFile(t, pluginPath)
	if !strings.Contains(out, "command: echo") {
		t.Fatalf("plugin config:\n%s", out)
	}
	main := readFile(t, path)
	if strings.Contains(main, "command: echo") {
		t.Fatalf("main config should not receive split plugin defaults:\n%s", main)
	}
}

func TestSyncPluginConfigEntriesFillsMissingEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	data := []byte(`plugins:
  echo:
    config: {}
  help:
    enabled: false
    config: {}
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 2 {
		t.Fatalf("changed = %d", changed)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "echo:\n    enabled: true") ||
		!strings.Contains(out, "help:\n    enabled: false") ||
		!strings.Contains(out, "command: echo") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestSyncPluginConfigEntriesNormalizesNullEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("plugins:\n  echo:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed = %d", changed)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "echo:\n    enabled: true") || !strings.Contains(out, "command: echo") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestSyncPluginConfigEntriesMergesPartialDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(`plugins:
  ratelimit:
    enabled: true
    config:
      limit: 9
`), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed = %d", changed)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "limit: 9") || !strings.Contains(out, "window: 1m") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestSyncPluginConfigEntriesMergesNestedDefaults(t *testing.T) {
	type providerConfig struct {
		Model   string `yaml:"model"`
		Timeout string `yaml:"timeout"`
	}
	type aiConfig struct {
		Provider providerConfig `yaml:"provider"`
	}
	registry := absdk.NewRegistry()
	if err := registry.Register(absdk.Factory{
		Info:    core.Manifest{Name: "ai"},
		Default: aiConfig{Provider: providerConfig{Model: "gpt", Timeout: "30s"}},
		Build: func(yaml.Node) (core.Plugin, error) {
			return nil, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(`plugins:
  ai:
    enabled: true
    config:
      provider:
        model: custom
`), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, registry)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed = %d", changed)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "model: custom") || !strings.Contains(out, "timeout: 30s") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestSyncPluginConfigEntryIgnoresOtherUnknownPlugins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(`plugins:
  ratelimit:
    enabled: true
    config: {}
  missing:
    enabled: true
    config: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := SyncPluginConfigEntry(path, DefaultRegistry(), "ratelimit")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || !result.Changed {
		t.Fatalf("result = %#v", result)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "limit: 5") || !strings.Contains(out, "window: 1m") || !strings.Contains(out, "missing:") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestSyncPluginConfigEntryUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("plugins:\n  weather:\n    enabled: true\n    config: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := SyncPluginConfigEntry(path, DefaultRegistry(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	if result.Available || result.Changed {
		t.Fatalf("result = %#v", result)
	}
}

func TestSyncPluginConfigEntriesRejectsUnknownPlugin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("plugins:\n  missing:\n    enabled: true\n    config: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncPluginConfigEntries(path, DefaultRegistry()); err == nil {
		t.Fatal("unknown plugin should be rejected")
	}
}

func TestSyncPluginConfigEntriesSkipsDisabledUnknownPlugin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(`plugins:
  echo:
    enabled: false
    config: {}
  legacy:
    enabled: false
    config: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := syncPluginConfigEntries(path, DefaultRegistry(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed != 1 || len(result.UnknownDisabled) != 1 || result.UnknownDisabled[0] != "legacy" {
		t.Fatalf("result = %#v", result)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "command: echo") || !strings.Contains(out, "legacy:") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestSyncPluginConfigEntriesForWorkspaceSkipsExternalPlugins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	data := []byte(`plugins:
  echo:
    enabled: false
    config: {}
  weather:
    enabled: true
    config: {}
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := SyncPluginConfigEntriesForWorkspace(path, DefaultRegistry(), PluginWorkspace{
		Plugins: []PluginModule{{Name: "weather", Module: "github.com/acme/weather", Symbol: "Module"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed != 1 || len(result.Skipped) != 1 || result.Skipped[0] != "weather" {
		t.Fatalf("result = %#v", result)
	}
	out := readFile(t, path)
	if !strings.Contains(out, "command: echo") || !strings.Contains(out, "weather:") {
		t.Fatalf("config:\n%s", out)
	}
}

func TestSyncPluginConfigEntriesForWorkspaceRejectsUnknownConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("plugins:\n  missing:\n    enabled: true\n    config: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncPluginConfigEntriesForWorkspace(path, DefaultRegistry(), PluginWorkspace{}); err == nil {
		t.Fatal("unknown plugin should be rejected")
	}
}
