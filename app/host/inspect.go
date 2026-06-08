package host

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
	"gopkg.in/yaml.v3"
)

// PluginInspect 是单个插件面向用户的完整配置视图。
type PluginInspect struct {
	Name              string
	ID                string
	Source            string
	Version           string
	Description       string
	Module            string
	Configured        bool
	Enabled           bool
	Available         bool
	ConfigPath        string
	ConfigYAML        string
	DefaultAvailable  bool
	DefaultConfigYAML string
	CheckState        string
	CheckDetail       string
}

// InspectPlugin 合并注册表、外部插件锁和配置文件，返回单个插件的可读视图。
func InspectPlugin(configPath string, registry absdk.Registry, lock PluginLock, id string) (PluginInspect, error) {
	if configPath == "" {
		configPath = "anybot.yaml"
	}
	if id == "" {
		return PluginInspect{}, fmt.Errorf("plugin id is required")
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return PluginInspect{}, err
	}
	status, ok := pluginStatusByID(cfg, registry, lock, id)
	if !ok {
		return PluginInspect{}, fmt.Errorf("unknown plugin %q", id)
	}
	inspect := PluginInspect{
		Name:        status.Name,
		ID:          status.ID,
		Source:      status.Source,
		Version:     status.Version,
		Description: status.Description,
		Module:      status.Module,
		Configured:  status.Configured,
		Enabled:     status.Enabled,
		Available:   status.Available,
	}
	if entry, ok := cfg.Plugins[id]; ok {
		config, err := yamlBlock(&entry.Config)
		if err != nil {
			return PluginInspect{}, fmt.Errorf("plugin %s config: %w", id, err)
		}
		inspect.ConfigYAML = config
		if path, ok, err := pluginConfigLocationPath(configPath, id); err != nil {
			return PluginInspect{}, err
		} else if ok {
			inspect.ConfigPath = path
		}
	}
	if factory, ok := registry.Factory(id); ok {
		defaultConfig, err := defaultConfigNode(factory.Default)
		if err != nil {
			return PluginInspect{}, fmt.Errorf("plugin %s default config: %w", id, err)
		}
		defaultYAML, err := yamlBlock(defaultConfig)
		if err != nil {
			return PluginInspect{}, fmt.Errorf("plugin %s default config: %w", id, err)
		}
		inspect.DefaultAvailable = true
		inspect.DefaultConfigYAML = defaultYAML
	}
	for _, check := range PluginConfigChecks(cfg, registry, lock) {
		if check.ID == id {
			inspect.CheckState = check.State
			inspect.CheckDetail = check.Detail
			break
		}
	}
	return inspect, nil
}

// WritePluginInspect 输出单个插件的配置与默认值，便于最终用户直接编辑。
func WritePluginInspect(w io.Writer, inspect PluginInspect) error {
	lines := []string{
		"ID：" + displayValue(inspect.ID),
		"名称：" + displayValue(inspect.Name),
		"来源：" + sourceLabel(inspect.Source),
		"配置：" + boolLabel(inspect.Configured),
		"启用：" + boolLabel(inspect.Enabled),
		"可加载：" + boolLabel(inspect.Available),
	}
	if inspect.Version != "" {
		lines = append(lines, "版本："+inspect.Version)
	}
	if inspect.Description != "" {
		lines = append(lines, "说明："+inspect.Description)
	}
	if inspect.Module != "" {
		lines = append(lines, "模块："+inspect.Module)
	}
	if inspect.ConfigPath != "" {
		lines = append(lines, "配置文件："+inspect.ConfigPath)
	}
	if inspect.CheckState != "" {
		detail := pluginCheckStateLabel(inspect.CheckState)
		if inspect.CheckDetail != "" {
			detail += "：" + inspect.CheckDetail
		}
		lines = append(lines, "检查："+detail)
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if inspect.Configured {
		if err := writeYAMLSection(w, "当前配置", inspect.ConfigYAML); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintln(w, "当前配置：-"); err != nil {
		return err
	}
	if inspect.DefaultAvailable {
		return writeYAMLSection(w, "默认配置", inspect.DefaultConfigYAML)
	}
	_, err := fmt.Fprintln(w, "默认配置：-")
	return err
}

func pluginStatusByID(cfg Config, registry absdk.Registry, lock PluginLock, id string) (PluginStatus, bool) {
	for _, status := range PluginStatuses(cfg, registry, lock) {
		if status.ID == id {
			return status, true
		}
	}
	return PluginStatus{}, false
}

func pluginConfigLocationPath(configPath, id string) (string, bool, error) {
	entryPath, err := pluginConfigEntryPath(PluginConfigDir(configPath), id)
	if err != nil {
		return "", false, err
	}
	for _, candidate := range pluginConfigEntryCandidates(entryPath) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true, nil
		} else if !os.IsNotExist(err) {
			return "", false, err
		}
	}
	return "", false, nil
}

func yamlBlock(node *yaml.Node) (string, error) {
	if emptyYAMLNode(node) {
		node = emptyConfigNode()
	}
	doc := yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{cloneYAMLNode(node)}}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		_ = enc.Close()
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return strings.TrimRight(out.String(), "\n") + "\n", nil
}

func writeYAMLSection(w io.Writer, title, yamlText string) error {
	if yamlText == "" {
		yamlText = "{}\n"
	}
	if _, err := fmt.Fprintf(w, "%s：\n", title); err != nil {
		return err
	}
	_, err := io.WriteString(w, yamlText)
	return err
}
