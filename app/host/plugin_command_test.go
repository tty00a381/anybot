package host

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPluginCommandStatus(t *testing.T) {
	dir := t.TempDir()
	configPath := writePluginCommandConfig(t, dir)
	writePluginCommandLock(t, dir, builtinInstall(testHelpID, "help"))
	if err := writePluginConfigFile(dir, testHelpID, "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:       []string{"status"},
		Output:     &out,
		ConfigPath: configPath,
		LockPath:   PluginLockPath(dir),
		Registry:   EmptyRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID\t名称\t来源\t配置\t启用\t可加载\t版本\t模块") ||
		!strings.Contains(out.String(), ShortPluginID(testHelpID)+"\thelp\t内置\t是\t是\t是\t1.0.1\t-") {
		t.Fatalf("status output:\n%s", out.String())
	}
}

func TestRunPluginCommandUsageDoesNotRequireLock(t *testing.T) {
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:   []string{"inspect"},
		Output: &out,
	})
	if err == nil || !strings.Contains(err.Error(), "用法：plugin inspect <id>") {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("output should be empty, got %q", out.String())
	}
}

func TestRunPluginCommandUnknownDoesNotRequireLock(t *testing.T) {
	err := RunPluginCommand(PluginCommandOptions{Args: []string{"missing"}})
	if err == nil || !strings.Contains(err.Error(), `未知插件命令 "missing"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPluginCommandEnableSyncsDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := writePluginCommandConfig(t, dir)
	writePluginCommandLock(t, dir, builtinInstall(testHelpID, "help"))
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:       []string{"enable", ShortPluginID(testHelpID)},
		Output:     &out,
		ConfigPath: configPath,
		LockPath:   PluginLockPath(dir),
		Registry:   EmptyRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已启用："+testHelpID) ||
		!strings.Contains(out.String(), "默认配置已同步：1 项更新") {
		t.Fatalf("enable output:\n%s", out.String())
	}
	config := readFile(t, pluginConfigPath(dir, testHelpID))
	if !strings.Contains(config, "enabled: true") ||
		!strings.Contains(config, "command: help") {
		t.Fatalf("plugin config:\n%s", config)
	}
}

func TestRunPluginCommandConfigUpdatesPluginConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := writePluginCommandConfig(t, dir)
	writePluginCommandLock(t, dir, builtinInstall(testHelpID, "help"))
	if err := writePluginConfigFile(dir, testHelpID, `enabled: true
config:
  command: help
`); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:       []string{"config", ShortPluginID(testHelpID), "command=assist"},
		Output:     &out,
		ConfigPath: configPath,
		LockPath:   PluginLockPath(dir),
		Registry:   EmptyRegistry(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件配置已更新："+testHelpID+"（1 项）") {
		t.Fatalf("config output:\n%s", out.String())
	}
	if config := readFile(t, pluginConfigPath(dir, testHelpID)); !strings.Contains(config, "command: assist") {
		t.Fatalf("plugin config:\n%s", config)
	}
}

func TestRunPluginCommandCheckReportsFailures(t *testing.T) {
	dir := t.TempDir()
	configPath := writePluginCommandConfig(t, dir)
	if err := writePluginConfigFile(dir, testGhostID, "enabled: true\nconfig: {}\n"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := RunPluginCommand(PluginCommandOptions{
		Args:       []string{"check"},
		Output:     &out,
		ConfigPath: configPath,
		LockPath:   PluginLockPath(dir),
		Registry:   EmptyRegistry(),
	})
	if err == nil || !strings.Contains(err.Error(), "插件配置检查失败") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), ShortPluginID(testGhostID)+"\t配置\t未知\t注册表中没有该插件") {
		t.Fatalf("check output:\n%s", out.String())
	}
}

func writePluginCommandLock(t *testing.T, dir string, installs ...PluginInstall) {
	t.Helper()
	if err := SavePluginLock(PluginLockPath(dir), PluginLock{Plugins: installs}); err != nil {
		t.Fatal(err)
	}
}

func writePluginCommandConfig(t *testing.T, dir string) string {
	t.Helper()
	path := ConfigPath(dir)
	writeTestConfig(t, path, "runtime:\n  log_level: info\n")
	return path
}
