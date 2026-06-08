package host

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectPluginBuiltinIncludesConfigAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePluginConfigFile(dir, "help", `enabled: true
config:
  command: docs
`); err != nil {
		t.Fatal(err)
	}
	inspect, err := InspectPlugin(path, DefaultRegistry(), PluginLock{}, "help")
	if err != nil {
		t.Fatal(err)
	}
	if inspect.Source != "builtin" || inspect.ID != "help" || inspect.Name != "help" || !inspect.Configured || !inspect.Enabled || !inspect.Available {
		t.Fatalf("inspect = %#v", inspect)
	}
	if inspect.ConfigPath != filepath.Join(dir, "plugins.d", "help.yaml") || inspect.CheckState != pluginCheckOK {
		t.Fatalf("inspect = %#v", inspect)
	}
	if !strings.Contains(inspect.ConfigYAML, "command: docs") ||
		!strings.Contains(inspect.DefaultConfigYAML, "command: help") ||
		!strings.Contains(inspect.DefaultConfigYAML, "lines:") {
		t.Fatalf("inspect config:\ncurrent:\n%s\ndefault:\n%s", inspect.ConfigYAML, inspect.DefaultConfigYAML)
	}
	var out bytes.Buffer
	if err := WritePluginInspect(&out, inspect); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID：help") ||
		!strings.Contains(out.String(), "名称：help") ||
		!strings.Contains(out.String(), "检查：可用") ||
		!strings.Contains(out.String(), "当前配置：") ||
		!strings.Contains(out.String(), "默认配置：") {
		t.Fatalf("inspect output:\n%s", out.String())
	}
}

func TestInspectPluginAcceptsYMLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "plugins.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(dir, "plugins.d", "echo.yml")
	if err := os.WriteFile(pluginPath, []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inspect, err := InspectPlugin(path, DefaultRegistry(), PluginLock{}, "echo")
	if err != nil {
		t.Fatal(err)
	}
	if inspect.ConfigPath != pluginPath || inspect.CheckState != pluginCheckDisabled {
		t.Fatalf("inspect = %#v", inspect)
	}
}

func TestInspectPluginExternalUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePluginConfigFile(dir, "weather", `enabled: true
config:
  city: Hangzhou
`); err != nil {
		t.Fatal(err)
	}
	lock := PluginLock{Plugins: []PluginModule{{
		ID:      "weather",
		Module:  "github.com/acme/weather",
		Version: "v1.2.3",
	}}}
	inspect, err := InspectPlugin(path, DefaultRegistry(), lock, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if inspect.Source != "external" || inspect.ID != "weather" || inspect.Available || inspect.DefaultAvailable {
		t.Fatalf("inspect = %#v", inspect)
	}
	if inspect.CheckState != pluginCheckUnavailable || !strings.Contains(inspect.Module, "github.com/acme/weather@v1.2.3") {
		t.Fatalf("inspect = %#v", inspect)
	}
	var out bytes.Buffer
	if err := WritePluginInspect(&out, inspect); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID：weather") ||
		!strings.Contains(out.String(), "检查：待构建") ||
		!strings.Contains(out.String(), "默认配置：-") {
		t.Fatalf("inspect output:\n%s", out.String())
	}
}
