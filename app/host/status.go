package host

import (
	"fmt"
	"io"
	"sort"

	absdk "github.com/tty00a381/anybot/sdk"
)

// PluginStatus 是 anybot 对一个插件的运行视图。
type PluginStatus struct {
	Name        string
	Source      string
	Version     string
	Description string
	Module      string
	Symbol      string
	Configured  bool
	Enabled     bool
	Available   bool
}

// PluginStatuses 合并注册表、外部插件工作区和配置文件，生成稳定排序的插件状态。
func PluginStatuses(cfg Config, registry absdk.Registry, workspace PluginWorkspace) []PluginStatus {
	workspace.applyDefaults()
	statuses := map[string]PluginStatus{}
	external := map[string]PluginModule{}
	for _, item := range workspace.Plugins {
		if item.Name == "" {
			continue
		}
		external[item.Name] = item
		statuses[item.Name] = PluginStatus{
			Name:   item.Name,
			Source: "external",
			Module: pluginModuleRef(item),
			Symbol: item.Symbol,
		}
	}
	for _, manifest := range registry.Plugins() {
		status := statuses[manifest.Name]
		status.Name = manifest.Name
		if _, ok := external[manifest.Name]; ok {
			status.Source = "external"
		} else {
			status.Source = "builtin"
		}
		status.Version = manifest.Version
		status.Description = manifest.Description
		status.Available = true
		statuses[manifest.Name] = status
	}
	for name, entry := range cfg.Plugins {
		status := statuses[name]
		if status.Name == "" {
			status.Name = name
			status.Source = "config"
		}
		status.Configured = true
		status.Enabled = pluginEnabled(entry)
		statuses[name] = status
	}
	names := make([]string, 0, len(statuses))
	for name := range statuses {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]PluginStatus, 0, len(names))
	for _, name := range names {
		out = append(out, statuses[name])
	}
	return out
}

// EnsureKnownPluginTarget 确认插件名指向内置插件、外部工作区插件，或一个允许编辑的既有配置项。
func EnsureKnownPluginTarget(cfg Config, registry absdk.Registry, workspace PluginWorkspace, name string, allowConfigured bool) error {
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if allowConfigured {
		if _, ok := cfg.Plugins[name]; ok {
			return nil
		}
	}
	if _, ok := registry.Factory(name); ok {
		return nil
	}
	workspace.applyDefaults()
	for _, item := range workspace.Plugins {
		if item.Name == name {
			return nil
		}
	}
	return fmt.Errorf("未知插件 %q；请先用 anybot plugins 查看内置插件，或用 anybot plugin add <module> 添加外部插件", name)
}

func pluginModuleRef(item PluginModule) string {
	out := item.Module
	if item.Version != "" {
		out += "@" + item.Version
	}
	if item.Replace != "" {
		out += " => " + item.Replace
	}
	return out
}

// WritePluginStatusTable 以稳定的表格格式输出插件状态。
func WritePluginStatusTable(w io.Writer, statuses []PluginStatus) error {
	if _, err := fmt.Fprintln(w, "名称\t来源\t配置\t启用\t可加载\t版本\t模块"); err != nil {
		return err
	}
	for _, status := range statuses {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			status.Name,
			sourceLabel(status.Source),
			boolLabel(status.Configured),
			boolLabel(status.Enabled),
			boolLabel(status.Available),
			displayValue(status.Version),
			displayValue(status.Module),
		); err != nil {
			return err
		}
	}
	return nil
}

func sourceLabel(source string) string {
	switch source {
	case "builtin":
		return "内置"
	case "external":
		return "外部"
	default:
		return "配置"
	}
}

func boolLabel(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func displayValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
