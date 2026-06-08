package host

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPluginStatusesMergesRegistryLockAndConfig(t *testing.T) {
	yes := true
	cfg := Config{Plugins: map[string]PluginEntry{
		"help":    {Enabled: &yes},
		"weather": {Enabled: &yes},
		"ghost":   {Enabled: &yes},
	}}
	lock := PluginLock{Plugins: []PluginModule{
		{Name: "weather", Module: "github.com/acme/weather", Version: "v1.2.3", Replace: "../weather", Symbol: "Module"},
	}}
	statuses := PluginStatuses(cfg, DefaultRegistry(), lock)
	byName := map[string]PluginStatus{}
	for _, status := range statuses {
		byName[status.Name] = status
	}
	if status := byName["help"]; status.Source != "builtin" || !status.Configured || !status.Enabled || !status.Available {
		t.Fatalf("help status = %#v", status)
	}
	if status := byName["weather"]; status.Source != "external" || status.Module != "github.com/acme/weather@v1.2.3 => ../weather" || !status.Configured || !status.Enabled || status.Available {
		t.Fatalf("weather status = %#v", status)
	}
	if status := byName["ghost"]; status.Source != "config" || !status.Configured || !status.Enabled || status.Available {
		t.Fatalf("ghost status = %#v", status)
	}
}

func TestWritePluginStatusTable(t *testing.T) {
	var out bytes.Buffer
	err := WritePluginStatusTable(&out, []PluginStatus{
		{Name: "weather", Source: "external", Configured: true, Enabled: true, Available: true, Version: "1.0.0", Module: "example.com/weather"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "名称\t来源\t配置\t启用\t可加载\t版本\t模块") ||
		!strings.Contains(out.String(), "weather\t外部\t是\t是\t是\t1.0.0\texample.com/weather") {
		t.Fatalf("status table:\n%s", out.String())
	}
}

func TestPluginConfigChecks(t *testing.T) {
	yes := true
	no := false
	cfg := Config{Plugins: map[string]PluginEntry{
		"help":      {Enabled: &yes},
		"ratelimit": {Enabled: &yes, Config: yamlNode(t, "limit: 0\nwindow: 1m\n")},
		"weather":   {Enabled: &yes},
		"legacy":    {Enabled: &no},
		"ghost":     {Enabled: &yes},
	}}
	lock := PluginLock{Plugins: []PluginModule{
		{Name: "weather", Module: "github.com/acme/weather", Symbol: "Module"},
	}}
	checks := PluginConfigChecks(cfg, DefaultRegistry(), lock)
	byName := map[string]PluginConfigCheck{}
	for _, check := range checks {
		byName[check.Name] = check
	}
	if check := byName["help"]; check.State != pluginCheckOK || check.Source != "builtin" {
		t.Fatalf("help check = %#v", check)
	}
	if check := byName["ratelimit"]; check.State != pluginCheckInvalid || !strings.Contains(check.Detail, "ratelimit.limit") {
		t.Fatalf("ratelimit check = %#v", check)
	}
	if check := byName["weather"]; check.State != pluginCheckUnavailable || check.Source != "external" {
		t.Fatalf("weather check = %#v", check)
	}
	if check := byName["legacy"]; check.State != pluginCheckDisabled {
		t.Fatalf("legacy check = %#v", check)
	}
	if check := byName["ghost"]; check.State != pluginCheckUnknown || check.Source != "config" {
		t.Fatalf("ghost check = %#v", check)
	}
	if !PluginConfigCheckFailed(checks) {
		t.Fatal("checks should be failed")
	}
}

func TestWritePluginConfigCheckTable(t *testing.T) {
	var out bytes.Buffer
	err := WritePluginConfigCheckTable(&out, []PluginConfigCheck{
		{Name: "weather", Source: "external", State: pluginCheckUnavailable, Detail: "外部插件尚未构建到当前框架"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "名称\t来源\t状态\t说明") ||
		!strings.Contains(out.String(), "weather\t外部\t待构建\t外部插件尚未构建到当前框架") {
		t.Fatalf("check table:\n%s", out.String())
	}
}

func yamlNode(t *testing.T, src string) yaml.Node {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(src), &node); err != nil {
		t.Fatal(err)
	}
	return *documentRoot(&node)
}
