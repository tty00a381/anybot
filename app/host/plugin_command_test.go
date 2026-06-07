package host

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPluginCommandStatus(t *testing.T) {
	dir := t.TempDir()
	configPath := writePluginCommandConfig(t, dir, "plugins:\n  help:\n    enabled: true\n    config: {}\n")
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:          []string{"status"},
		Output:        &out,
		ConfigPath:    configPath,
		WorkspacePath: filepath.Join(dir, PluginWorkspaceFile),
		Registry:      DefaultRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "名称\t来源\t配置\t启用\t可加载\t版本\t模块") ||
		!strings.Contains(out.String(), "help\t内置\t是\t是\t是\t1.0.0\t-") {
		t.Fatalf("status output:\n%s", out.String())
	}
}

func TestRunPluginCommandEnableSyncsDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := writePluginCommandConfig(t, dir, "plugins: {}\n")
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:          []string{"enable", "help"},
		Output:        &out,
		ConfigPath:    configPath,
		WorkspacePath: filepath.Join(dir, PluginWorkspaceFile),
		Registry:      DefaultRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已启用：help") ||
		!strings.Contains(out.String(), "默认配置已同步：1 项更新") {
		t.Fatalf("enable output:\n%s", out.String())
	}
	config := readFile(t, configPath)
	if !strings.Contains(config, "enabled: true") ||
		!strings.Contains(config, "command: help") {
		t.Fatalf("config:\n%s", config)
	}
}

func TestRunPluginCommandConfigUpdatesPluginConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := writePluginCommandConfig(t, dir, `plugins:
  help:
    enabled: true
    config:
      command: help
`)
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:          []string{"config", "help", "command=assist"},
		Output:        &out,
		ConfigPath:    configPath,
		WorkspacePath: filepath.Join(dir, PluginWorkspaceFile),
		Registry:      DefaultRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件配置已更新：help（1 项）") {
		t.Fatalf("config output:\n%s", out.String())
	}
	if config := readFile(t, configPath); !strings.Contains(config, "command: assist") {
		t.Fatalf("config:\n%s", config)
	}
}

func TestRunPluginCommandCheckReportsFailures(t *testing.T) {
	dir := t.TempDir()
	configPath := writePluginCommandConfig(t, dir, "plugins:\n  ghost:\n    enabled: true\n    config: {}\n")
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:          []string{"check"},
		Output:        &out,
		ConfigPath:    configPath,
		WorkspacePath: filepath.Join(dir, PluginWorkspaceFile),
		Registry:      DefaultRegistry(),
	})
	if err == nil || !strings.Contains(err.Error(), "插件配置检查失败") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "ghost\t配置\t未知\t注册表中没有该插件") {
		t.Fatalf("check output:\n%s", out.String())
	}
}

func writePluginCommandConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "anybot.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
