package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	absdk "github.com/tty00a381/anybot/sdk"
	"gopkg.in/yaml.v3"
)

func TestEnsurePluginConfigEntryAddsDisabledPlaceholder(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	changed, err := EnsurePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("entry should be added")
	}
	out := readFile(t, pluginConfigPath(dir, "weather"))
	if !strings.Contains(out, "enabled: false") || !strings.Contains(out, "config: {}") {
		t.Fatalf("plugin config:\n%s", out)
	}
	main := readFile(t, path)
	if strings.Contains(main, "weather:") {
		t.Fatalf("main config should not contain plugin entry:\n%s", main)
	}
	changed, err = EnsurePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("existing entry should be left unchanged")
	}
}

func TestSetPluginEnabled(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "weather", "enabled: false\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	changed, err := SetPluginEnabled(path, "weather", true)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("enable should change config")
	}
	out := readFile(t, pluginConfigPath(dir, "weather"))
	if !strings.Contains(out, "enabled: true") {
		t.Fatalf("plugin config:\n%s", out)
	}
	changed, err = SetPluginEnabled(path, "weather", true)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("same enabled state should be unchanged")
	}
}

func TestSetPluginEnabledCreatesEntry(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if _, err := SetPluginEnabled(path, "weather", true); err != nil {
		t.Fatal(err)
	}
	out := readFile(t, pluginConfigPath(dir, "weather"))
	if !strings.Contains(out, "enabled: true") || !strings.Contains(out, "config: {}") {
		t.Fatalf("plugin config:\n%s", out)
	}
}

func TestSetPluginEnabledUpdatesExistingYMLConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	pluginPath := filepath.Join(dir, "plugins.d", "weather.yml")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SetPluginEnabled(path, "weather", true)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("existing .yml config should be changed")
	}
	out := readFile(t, pluginPath)
	if !strings.Contains(out, "enabled: true") {
		t.Fatalf("plugin config:\n%s", out)
	}
	if _, err := os.Stat(pluginConfigPath(dir, "weather")); !os.IsNotExist(err) {
		t.Fatalf("new .yaml config should not be created: %v", err)
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

func TestSetPluginConfigValues(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	changed, err := SetPluginConfigValues(path, "help", []PluginConfigAssignment{
		{Path: []string{"command"}, Value: yamlScalar("docs")},
		{Path: []string{"lines"}, Value: yamlSequence("a", "b")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("config should change")
	}
	out := readFile(t, pluginConfigPath(dir, "help"))
	if !strings.Contains(out, "command: docs") || !strings.Contains(out, "- a") {
		t.Fatalf("plugin config:\n%s", out)
	}
}

func TestRemovePluginConfigValues(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "ai", `enabled: true
config:
  provider:
    model: custom
    timeout: 30s
  history:
    enabled: true
`); err != nil {
		t.Fatal(err)
	}
	changed, err := RemovePluginConfigValues(path, "ai", [][]string{{"provider", "model"}, {"history", "enabled"}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("reset should remove configured values")
	}
	out := readFile(t, pluginConfigPath(dir, "ai"))
	if strings.Contains(out, "model: custom") ||
		strings.Contains(out, "history:") ||
		!strings.Contains(out, "timeout: 30s") {
		t.Fatalf("plugin config:\n%s", out)
	}
	changed, err = RemovePluginConfigValues(path, "ai", [][]string{{"provider", "model"}})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("missing config path should be unchanged")
	}
}

func TestRemovePluginConfigValuesRejectsScalarParent(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "help", `enabled: true
config:
  command: docs
`); err != nil {
		t.Fatal(err)
	}
	_, err := RemovePluginConfigValues(path, "help", [][]string{{"command", "name"}})
	if err == nil || !strings.Contains(err.Error(), "command must be a YAML mapping") {
		t.Fatalf("err = %v", err)
	}
}

func TestRemovePluginConfigEntry(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "weather", "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	changed, err := RemovePluginConfigEntry(path, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("entry should be removed")
	}
	if _, err := os.Stat(pluginConfigPath(dir, "weather")); !os.IsNotExist(err) {
		t.Fatalf("plugin config should be removed: %v", err)
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
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "echo", "enabled: false\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	if err := writePluginConfigFile(dir, "help", `enabled: true
config:
  command: custom
`); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 2 {
		t.Fatalf("changed = %d", changed)
	}
	echo := readFile(t, pluginConfigPath(dir, "echo"))
	if !strings.Contains(echo, "command: echo") {
		t.Fatalf("echo default missing:\n%s", echo)
	}
	help := readFile(t, pluginConfigPath(dir, "help"))
	if !strings.Contains(help, "command: custom") || !strings.Contains(help, "lines:") {
		t.Fatalf("help config:\n%s", help)
	}
	changed, err = SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 {
		t.Fatalf("second sync changed %d entries", changed)
	}
}

func TestSyncPluginConfigEntriesFillsMissingEnabled(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "echo", "config: {}\n"); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed = %d", changed)
	}
	out := readFile(t, pluginConfigPath(dir, "echo"))
	if !strings.Contains(out, "enabled: true") || !strings.Contains(out, "command: echo") {
		t.Fatalf("plugin config:\n%s", out)
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
		Info:     absdk.Manifest{Name: "AI"},
		PluginID: "ai",
		Default:  aiConfig{Provider: providerConfig{Model: "gpt", Timeout: "30s"}},
		Build: func(yaml.Node) (absdk.Plugin, error) {
			return nil, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "ai", `enabled: true
config:
  provider:
    model: custom
`); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncPluginConfigEntries(path, registry)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed = %d", changed)
	}
	out := readFile(t, pluginConfigPath(dir, "ai"))
	if !strings.Contains(out, "model: custom") || !strings.Contains(out, "timeout: 30s") {
		t.Fatalf("plugin config:\n%s", out)
	}
}

func TestSyncPluginConfigEntryIgnoresOtherUnknownPlugins(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "ratelimit", "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	if err := writePluginConfigFile(dir, "missing", "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	result, err := SyncPluginConfigEntry(path, DefaultRegistry(), "ratelimit")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || !result.Changed {
		t.Fatalf("result = %#v", result)
	}
	out := readFile(t, pluginConfigPath(dir, "ratelimit"))
	if !strings.Contains(out, "limit: 5") || !strings.Contains(out, "window: 1m") {
		t.Fatalf("plugin config:\n%s", out)
	}
	if _, err := os.Stat(pluginConfigPath(dir, "missing")); err != nil {
		t.Fatal(err)
	}
}

func TestSyncPluginConfigEntryUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "weather", "enabled: true\nconfig: {}\n"); err != nil {
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
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "missing", "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncPluginConfigEntries(path, DefaultRegistry()); err == nil {
		t.Fatal("unknown plugin should be rejected")
	}
}

func TestSyncPluginConfigEntriesSkipsDisabledUnknownPlugin(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "echo", "enabled: false\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	if err := writePluginConfigFile(dir, "legacy", "enabled: false\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	result, err := syncPluginConfigEntries(path, DefaultRegistry(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed != 1 || len(result.UnknownDisabled) != 1 || result.UnknownDisabled[0] != "legacy" {
		t.Fatalf("result = %#v", result)
	}
	out := readFile(t, pluginConfigPath(dir, "echo"))
	if !strings.Contains(out, "command: echo") {
		t.Fatalf("plugin config:\n%s", out)
	}
}

func TestSyncPluginConfigEntriesForLockSkipsExternalPlugins(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "echo", "enabled: false\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	if err := writePluginConfigFile(dir, "weather", "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	result, err := SyncPluginConfigEntriesForLock(path, DefaultRegistry(), PluginLock{
		Plugins: []PluginModule{{ID: "weather", Module: "github.com/acme/weather"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed != 1 || len(result.Skipped) != 1 || result.Skipped[0] != "weather" {
		t.Fatalf("result = %#v", result)
	}
	out := readFile(t, pluginConfigPath(dir, "echo"))
	if !strings.Contains(out, "command: echo") {
		t.Fatalf("plugin config:\n%s", out)
	}
}

func TestSyncPluginConfigEntriesForLockRejectsUnknownConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeHostConfig(t, dir)
	if err := writePluginConfigFile(dir, "missing", "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncPluginConfigEntriesForLock(path, DefaultRegistry(), PluginLock{}); err == nil {
		t.Fatal("unknown plugin should be rejected")
	}
}

func writeHostConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func pluginConfigPath(dir, id string) string {
	return filepath.Join(dir, "plugins.d", id+".yaml")
}

func yamlScalar(value string) yaml.Node {
	return yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func yamlSequence(values ...string) yaml.Node {
	node := yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		child := yamlScalar(value)
		node.Content = append(node.Content, &child)
	}
	return node
}
