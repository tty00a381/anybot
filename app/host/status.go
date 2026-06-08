package host

import (
	"fmt"
	"io"
	"sort"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

const pluginIDShortLength = 12

// PluginStatus 是 anybot 对一个插件的运行视图。
type PluginStatus struct {
	ID          string
	Name        string
	Source      string
	Version     string
	Description string
	Module      string
	Configured  bool
	Enabled     bool
	Available   bool
}

// PluginStatuses 合并注册表、插件安装锁和配置文件，生成稳定排序的插件状态。
func PluginStatuses(cfg Config, registry absdk.Registry, lock PluginLock) []PluginStatus {
	lock.applyDefaults()
	statuses := map[string]PluginStatus{}
	installs := map[string]PluginInstall{}
	for _, item := range lock.Plugins {
		if item.ID == "" {
			continue
		}
		installs[item.ID] = item
		source := "builtin"
		module := ""
		if item.Module != "" {
			source = "external"
			module = pluginModuleRef(item)
		}
		statuses[item.ID] = PluginStatus{
			ID:     item.ID,
			Source: source,
			Module: module,
		}
	}
	for _, id := range registry.PluginIDs() {
		factory, _ := registry.Factory(id)
		status := statuses[id]
		status.ID = id
		status.Name = factory.Info.Name
		if status.Source == "" {
			if item, ok := installs[id]; ok && item.Module != "" {
				status.Source = "external"
				status.Module = pluginModuleRef(item)
			} else {
				status.Source = "registry"
			}
		}
		status.Version = factory.Info.Version
		status.Description = factory.Info.Description
		status.Available = true
		statuses[id] = status
	}
	for id, entry := range cfg.Plugins {
		status := statuses[id]
		if status.ID == "" {
			status.ID = id
			status.Source = "config"
		}
		status.Configured = true
		status.Enabled = pluginEnabled(entry)
		statuses[id] = status
	}
	ids := make([]string, 0, len(statuses))
	for id := range statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]PluginStatus, 0, len(ids))
	for _, id := range ids {
		out = append(out, statuses[id])
	}
	return out
}

// ResolvePluginID 把用户输入解析为插件 ID；支持完整 ID 或唯一前缀。
func ResolvePluginID(cfg Config, registry absdk.Registry, lock PluginLock, target string, allowConfigured bool) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("plugin id is required")
	}
	ids := pluginTargetIDs(cfg, registry, lock, allowConfigured)
	for _, id := range ids {
		if id == target {
			return id, nil
		}
	}
	matches := make([]string, 0, 4)
	for _, id := range ids {
		if strings.HasPrefix(id, target) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("未知插件 ID %q；请先用 anybot plugin status 查看插件 ID", target)
	default:
		short := make([]string, 0, len(matches))
		for _, id := range matches {
			short = append(short, ShortPluginID(id))
		}
		return "", fmt.Errorf("插件 ID 前缀 %q 不唯一：%s", target, strings.Join(short, ", "))
	}
}

func pluginTargetIDs(cfg Config, registry absdk.Registry, lock PluginLock, allowConfigured bool) []string {
	seen := map[string]struct{}{}
	lock.applyDefaults()
	for _, item := range lock.Plugins {
		if item.ID != "" {
			seen[item.ID] = struct{}{}
		}
	}
	for _, id := range registry.PluginIDs() {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
	}
	if allowConfigured {
		for id := range cfg.Plugins {
			seen[id] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ShortPluginID 返回适合命令行首列展示的插件 ID。
func ShortPluginID(id string) string {
	if len(id) <= pluginIDShortLength {
		return id
	}
	return id[:pluginIDShortLength]
}

func pluginModuleRef(item PluginInstall) string {
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
	if _, err := fmt.Fprintln(w, "ID\t名称\t来源\t配置\t启用\t可加载\t版本\t模块"); err != nil {
		return err
	}
	for _, status := range statuses {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			ShortPluginID(status.ID),
			displayValue(status.Name),
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
	case "registry":
		return "注册表"
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
