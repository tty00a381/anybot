package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigMergesPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "plugins.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins.d", "weather.yaml"), []byte(`enabled: true
config:
  city: Hangzhou
`), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte(`plugin_config_dir: plugins.d
plugins:
  help:
    enabled: true
    config:
      command: help
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Plugins["help"]; !ok {
		t.Fatalf("help plugin missing: %#v", cfg.Plugins)
	}
	weather, ok := cfg.Plugins["weather"]
	if !ok || weather.Enabled == nil || !*weather.Enabled {
		t.Fatalf("weather plugin = %#v", weather)
	}
	if city := mappingValue(&weather.Config, "city"); city == nil || city.Value != "Hangzhou" {
		t.Fatalf("weather config = %#v", weather.Config)
	}
}

func TestLoadConfigRejectsDuplicatePluginConfigDirEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "plugins.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins.d", "help.yaml"), []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte(`plugin_config_dir: plugins.d
plugins:
  help:
    enabled: true
    config: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(config)
	if err == nil || !strings.Contains(err.Error(), "configured more than once") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadConfigIgnoresMissingPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte("plugin_config_dir: plugins.d\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(config); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigRejectsYMLExtension(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "anybot.yml")
	if err := os.WriteFile(config, []byte("plugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(config)
	if err == nil || !strings.Contains(err.Error(), "必须使用 .yaml 扩展名") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadConfigRejectsSplitYMLExtension(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "plugins.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins.d", "echo.yml"), []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte("plugin_config_dir: plugins.d\nplugins: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(config)
	if err == nil || !strings.Contains(err.Error(), "必须使用 .yaml 扩展名") {
		t.Fatalf("err = %v", err)
	}
}
