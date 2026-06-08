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
		testHelpID:    {Enabled: &yes},
		testWeatherID: {Enabled: &yes},
		testGhostID:   {Enabled: &yes},
	}}
	lock := PluginLock{Plugins: []PluginInstall{
		{ID: testHelpID, Builtin: "help"},
		{ID: testWeatherID, Module: "github.com/acme/weather", Version: "v1.2.3", Replace: "../weather"},
	}}
	statuses := PluginStatuses(cfg, testBuiltinRegistry(t, builtinInstall(testHelpID, "help")), lock)
	byID := map[string]PluginStatus{}
	for _, status := range statuses {
		byID[status.ID] = status
	}
	if status := byID[testHelpID]; status.Name != "help" || status.Source != "builtin" || !status.Configured || !status.Enabled || !status.Available {
		t.Fatalf("help status = %#v", status)
	}
	if status := byID[testWeatherID]; status.Source != "external" || status.Module != "github.com/acme/weather@v1.2.3 => ../weather" || !status.Configured || !status.Enabled || status.Available {
		t.Fatalf("weather status = %#v", status)
	}
	if status := byID[testGhostID]; status.Source != "config" || !status.Configured || !status.Enabled || status.Available {
		t.Fatalf("ghost status = %#v", status)
	}
}

func TestWritePluginStatusTable(t *testing.T) {
	var out bytes.Buffer
	err := WritePluginStatusTable(&out, []PluginStatus{
		{ID: "plg_abcdefghijklmnopqrstuvwxyz", Name: "天气", Source: "external", Configured: true, Enabled: true, Available: true, Version: "1.0.0", Module: "example.com/weather"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID\t名称\t来源\t配置\t启用\t可加载\t版本\t模块") ||
		!strings.Contains(out.String(), "plg_abcdefgh\t天气\t外部\t是\t是\t是\t1.0.0\texample.com/weather") {
		t.Fatalf("status table:\n%s", out.String())
	}
}

func TestResolvePluginIDAcceptsUniquePrefix(t *testing.T) {
	cfg := Config{Plugins: map[string]PluginEntry{testWeatherID: {}}}
	id, err := ResolvePluginID(cfg, EmptyRegistry(), PluginLock{}, ShortPluginID(testWeatherID), true)
	if err != nil {
		t.Fatal(err)
	}
	if id != testWeatherID {
		t.Fatalf("id = %q", id)
	}
}

func TestResolvePluginIDRejectsAmbiguousPrefix(t *testing.T) {
	first := "plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"
	second := "plg_aaaaaaaaaaaaaaaaaaaaaaaaab"
	cfg := Config{Plugins: map[string]PluginEntry{first: {}, second: {}}}
	_, err := ResolvePluginID(cfg, EmptyRegistry(), PluginLock{}, "plg_aaaaaaaa", true)
	if err == nil || !strings.Contains(err.Error(), "不唯一") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginConfigChecks(t *testing.T) {
	yes := true
	no := false
	cfg := Config{Plugins: map[string]PluginEntry{
		testHelpID:      {Enabled: &yes},
		testRateLimitID: {Enabled: &yes, Config: yamlNode(t, "limit: 0\nwindow: 1m\n")},
		testWeatherID:   {Enabled: &yes},
		testMemoryID:    {Enabled: &no},
		testGhostID:     {Enabled: &yes},
	}}
	lock := PluginLock{Plugins: []PluginInstall{
		builtinInstall(testHelpID, "help"),
		builtinInstall(testRateLimitID, "ratelimit"),
		{ID: testWeatherID, Module: "github.com/acme/weather"},
	}}
	registry := testBuiltinRegistry(t, builtinInstall(testHelpID, "help"), builtinInstall(testRateLimitID, "ratelimit"))
	checks := PluginConfigChecks(cfg, registry, lock)
	byID := map[string]PluginConfigCheck{}
	for _, check := range checks {
		byID[check.ID] = check
	}
	if check := byID[testHelpID]; check.State != pluginCheckOK || check.Source != "builtin" {
		t.Fatalf("help check = %#v", check)
	}
	if check := byID[testRateLimitID]; check.State != pluginCheckInvalid || !strings.Contains(check.Detail, "ratelimit.limit") {
		t.Fatalf("ratelimit check = %#v", check)
	}
	if check := byID[testWeatherID]; check.State != pluginCheckUnavailable || check.Source != "external" {
		t.Fatalf("weather check = %#v", check)
	}
	if check := byID[testMemoryID]; check.State != pluginCheckDisabled {
		t.Fatalf("legacy check = %#v", check)
	}
	if check := byID[testGhostID]; check.State != pluginCheckUnknown || check.Source != "config" {
		t.Fatalf("ghost check = %#v", check)
	}
	if !PluginConfigCheckFailed(checks) {
		t.Fatal("checks should be failed")
	}
}

func TestWritePluginConfigCheckTable(t *testing.T) {
	var out bytes.Buffer
	err := WritePluginConfigCheckTable(&out, []PluginConfigCheck{
		{ID: testWeatherID, Source: "external", State: pluginCheckUnavailable, Detail: "外部插件尚未构建到当前框架"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID\t来源\t状态\t说明") ||
		!strings.Contains(out.String(), ShortPluginID(testWeatherID)+"\t外部\t待构建\t外部插件尚未构建到当前框架") {
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
