package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigLoadsFixedPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	if err := writePluginConfigFile(dir, testWeatherID, `enabled: true
config:
  city: Hangzhou
`); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte("runtime:\n  log_level: debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	weather, ok := cfg.Plugins[testWeatherID]
	if !ok || weather.Enabled == nil || !*weather.Enabled {
		t.Fatalf("weather plugin = %#v", weather)
	}
	if city := mappingValue(&weather.Config, "city"); city == nil || city.Value != "Hangzhou" {
		t.Fatalf("weather config = %#v", weather.Config)
	}
}

func TestLoadConfigIgnoresInlinePlugins(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte(`plugins:
  help:
    enabled: true
    config: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Plugins) != 0 {
		t.Fatalf("inline plugins should be ignored: %#v", cfg.Plugins)
	}
}

func TestLoadConfigIgnoresMissingPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(config); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigAcceptsYMLExtension(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "plugins.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins.d", testEchoID+".yml"), []byte("enabled: false\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Plugins[testEchoID]; !ok {
		t.Fatalf("echo plugin missing: %#v", cfg.Plugins)
	}
}

func TestLoadConfigRejectsInvalidPluginIDFilename(t *testing.T) {
	dir := t.TempDir()
	if err := writePluginConfigFile(dir, "天气", "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(config, []byte("runtime:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(config)
	if err == nil || !strings.Contains(err.Error(), "plugin id") {
		t.Fatalf("err = %v", err)
	}
}

func writePluginConfigFile(dir, id, content string) error {
	path := filepath.Join(dir, "plugins.d", id+".yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
