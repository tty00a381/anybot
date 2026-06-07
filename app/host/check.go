package host

import (
	"fmt"
	"io"

	absdk "github.com/tty00a381/anybot/sdk"
)

const (
	pluginCheckOK          = "ok"
	pluginCheckDisabled    = "disabled"
	pluginCheckUnavailable = "unavailable"
	pluginCheckUnknown     = "unknown"
	pluginCheckInvalid     = "invalid"
)

// PluginConfigCheck 是一个插件配置项的静态检查结果。
type PluginConfigCheck struct {
	Name   string
	Source string
	State  string
	Detail string
}

// PluginConfigChecks 校验当前配置中的插件项。禁用插件只报告状态，不要求可加载。
func PluginConfigChecks(cfg Config, registry absdk.Registry, workspace PluginWorkspace) []PluginConfigCheck {
	workspace.applyDefaults()
	external := map[string]PluginModule{}
	for _, item := range workspace.Plugins {
		if item.Name != "" {
			external[item.Name] = item
		}
	}

	checks := make([]PluginConfigCheck, 0, len(cfg.Plugins))
	for _, name := range configuredPluginNames(cfg) {
		entry := cfg.Plugins[name]
		check := PluginConfigCheck{Name: name, Source: pluginCheckSource(name, registry, external)}
		if !pluginEnabled(entry) {
			check.State = pluginCheckDisabled
			checks = append(checks, check)
			continue
		}
		factory, ok := registry.Factory(name)
		if !ok {
			if _, ok := external[name]; ok {
				check.State = pluginCheckUnavailable
				check.Detail = "外部插件尚未构建到当前框架"
			} else {
				check.State = pluginCheckUnknown
				check.Detail = "注册表中没有该插件"
			}
			checks = append(checks, check)
			continue
		}
		if _, err := factory.Build(entry.Config); err != nil {
			check.State = pluginCheckInvalid
			check.Detail = err.Error()
		} else {
			check.State = pluginCheckOK
		}
		checks = append(checks, check)
	}
	return checks
}

func pluginCheckSource(name string, registry absdk.Registry, external map[string]PluginModule) string {
	if _, ok := external[name]; ok {
		return "external"
	}
	if _, ok := registry.Factory(name); ok {
		return "builtin"
	}
	return "config"
}

// PluginConfigCheckFailed 判断检查结果里是否存在会阻止运行的插件配置问题。
func PluginConfigCheckFailed(checks []PluginConfigCheck) bool {
	for _, check := range checks {
		switch check.State {
		case pluginCheckUnknown, pluginCheckUnavailable, pluginCheckInvalid:
			return true
		}
	}
	return false
}

// WritePluginConfigCheckTable 输出稳定的插件配置检查表。
func WritePluginConfigCheckTable(w io.Writer, checks []PluginConfigCheck) error {
	if _, err := fmt.Fprintln(w, "名称\t来源\t状态\t说明"); err != nil {
		return err
	}
	for _, check := range checks {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			check.Name,
			sourceLabel(check.Source),
			pluginCheckStateLabel(check.State),
			displayValue(check.Detail),
		); err != nil {
			return err
		}
	}
	return nil
}

func pluginCheckStateLabel(state string) string {
	switch state {
	case pluginCheckOK:
		return "可用"
	case pluginCheckDisabled:
		return "禁用"
	case pluginCheckUnavailable:
		return "待构建"
	case pluginCheckUnknown:
		return "未知"
	case pluginCheckInvalid:
		return "无效"
	default:
		return displayValue(state)
	}
}
