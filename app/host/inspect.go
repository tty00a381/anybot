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
	InstanceID        string
	Source            string
	Version           string
	Description       string
	Module            string
	Symbol            string
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
func InspectPlugin(configPath string, registry absdk.Registry, lock PluginLock, name string) (PluginInspect, error) {
	if configPath == "" {
		configPath = "anybot.yaml"
	}
	if name == "" {
		return PluginInspect{}, fmt.Errorf("plugin name is required")
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return PluginInspect{}, err
	}
	status, ok := pluginStatusByName(cfg, registry, lock, name)
	if !ok {
		return PluginInspect{}, fmt.Errorf("unknown plugin %q", name)
	}
	inspect := PluginInspect{
		Name:        status.Name,
		InstanceID:  status.InstanceID,
		Source:      status.Source,
		Version:     status.Version,
		Description: status.Description,
		Module:      status.Module,
		Symbol:      status.Symbol,
		Configured:  status.Configured,
		Enabled:     status.Enabled,
		Available:   status.Available,
	}
	if entry, ok := cfg.Plugins[name]; ok {
		config, err := yamlBlock(&entry.Config)
		if err != nil {
			return PluginInspect{}, fmt.Errorf("plugin %s config: %w", name, err)
		}
		inspect.ConfigYAML = config
		if path, ok, err := pluginConfigLocationPath(configPath, name); err != nil {
			return PluginInspect{}, err
		} else if ok {
			inspect.ConfigPath = path
		}
	}
	if factory, ok := registry.Factory(name); ok {
		defaultConfig, err := defaultConfigNode(factory.Default)
		if err != nil {
			return PluginInspect{}, fmt.Errorf("plugin %s default config: %w", name, err)
		}
		defaultYAML, err := yamlBlock(defaultConfig)
		if err != nil {
			return PluginInspect{}, fmt.Errorf("plugin %s default config: %w", name, err)
		}
		inspect.DefaultAvailable = true
		inspect.DefaultConfigYAML = defaultYAML
	}
	for _, check := range PluginConfigChecks(cfg, registry, lock) {
		if check.Name == name {
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
		"名称：" + displayValue(inspect.Name),
		"实例ID：" + displayValue(inspect.InstanceID),
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
	if inspect.Symbol != "" {
		lines = append(lines, "导出："+inspect.Symbol)
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

func pluginStatusByName(cfg Config, registry absdk.Registry, lock PluginLock, name string) (PluginStatus, bool) {
	for _, status := range PluginStatuses(cfg, registry, lock) {
		if status.Name == name {
			return status, true
		}
	}
	return PluginStatus{}, false
}

func pluginConfigLocationPath(configPath, name string) (string, bool, error) {
	doc, err := loadYAMLDocument(configPath)
	if err != nil {
		return "", false, err
	}
	root := documentRoot(&doc)
	if root.Kind == 0 {
		return "", false, nil
	}
	if root.Kind != yaml.MappingNode {
		return "", false, fmt.Errorf("%s root must be a YAML mapping", configPath)
	}
	plugins := mappingValue(root, "plugins")
	if plugins != nil && plugins.Kind != yaml.MappingNode {
		return "", false, fmt.Errorf("plugins must be a YAML mapping")
	}
	if mappingValue(plugins, name) != nil {
		return configPath, true, nil
	}
	dir, split, err := pluginConfigDir(root, configPath)
	if err != nil || !split {
		return "", false, err
	}
	entryPath, err := pluginConfigEntryPath(dir, name)
	if err != nil {
		return "", false, err
	}
	for _, candidate := range pluginConfigEntryCandidates(entryPath) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true, nil
		} else if err != nil && !os.IsNotExist(err) {
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
