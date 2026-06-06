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
	if err := os.WriteFile(path, []byte(`plugins:
  help:
    enabled: true
    config:
      command: docs
`), 0o644); err != nil {
		t.Fatal(err)
	}
	inspect, err := InspectPlugin(path, DefaultRegistry(), PluginWorkspace{}, "help")
	if err != nil {
		t.Fatal(err)
	}
	if inspect.Source != "builtin" || !inspect.Configured || !inspect.Enabled || !inspect.Available {
		t.Fatalf("inspect = %#v", inspect)
	}
	if inspect.ConfigPath != path || inspect.CheckState != pluginCheckOK {
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
	if !strings.Contains(out.String(), "名称：help") ||
		!strings.Contains(out.String(), "检查：可用") ||
		!strings.Contains(out.String(), "当前配置：") ||
		!strings.Contains(out.String(), "默认配置：") {
		t.Fatalf("inspect output:\n%s", out.String())
	}
}

func TestInspectPluginUsesSplitYAMLConfigPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.Mkdir(filepath.Join(dir, "plugins.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("plugin_config_dir: plugins.d\nplugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(dir, "plugins.d", "echo.yaml")
	if err := os.WriteFile(pluginPath, []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inspect, err := InspectPlugin(path, DefaultRegistry(), PluginWorkspace{}, "echo")
	if err != nil {
		t.Fatal(err)
	}
	if inspect.ConfigPath != pluginPath || inspect.CheckState != pluginCheckDisabled {
		t.Fatalf("inspect = %#v", inspect)
	}
}

func TestInspectPluginRejectsSplitYMLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.Mkdir(filepath.Join(dir, "plugins.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("plugin_config_dir: plugins.d\nplugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins.d", "echo.yml"), []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := InspectPlugin(path, DefaultRegistry(), PluginWorkspace{}, "echo")
	if err == nil || !strings.Contains(err.Error(), "必须使用 .yaml 扩展名") {
		t.Fatalf("err = %v", err)
	}
}

func TestInspectPluginExternalUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(`plugins:
  weather:
    enabled: true
    config:
      city: Hangzhou
`), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := PluginWorkspace{Plugins: []PluginModule{{
		Name:    "weather",
		Module:  "github.com/acme/weather",
		Version: "v1.2.3",
		Symbol:  "Module",
	}}}
	inspect, err := InspectPlugin(path, DefaultRegistry(), workspace, "weather")
	if err != nil {
		t.Fatal(err)
	}
	if inspect.Source != "external" || inspect.Available || inspect.DefaultAvailable {
		t.Fatalf("inspect = %#v", inspect)
	}
	if inspect.CheckState != pluginCheckUnavailable || !strings.Contains(inspect.Module, "github.com/acme/weather@v1.2.3") {
		t.Fatalf("inspect = %#v", inspect)
	}
	var out bytes.Buffer
	if err := WritePluginInspect(&out, inspect); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "检查：待构建") ||
		!strings.Contains(out.String(), "默认配置：-") {
		t.Fatalf("inspect output:\n%s", out.String())
	}
}
