package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"gopkg.in/yaml.v3"
)

// Config 是 anybot 的框架配置。
type Config struct {
	Runtime  RuntimeConfig          `yaml:"runtime"`
	Adapter  AdapterConfig          `yaml:"adapter"`
	Security SecurityConfig         `yaml:"security"`
	Plugins  map[string]PluginEntry `yaml:"-"`

	configPath string
}

// RuntimeConfig 描述框架运行时参数。
type RuntimeConfig struct {
	LogLevel string      `yaml:"log_level"`
	Workers  string      `yaml:"workers"`
	Buffer   int         `yaml:"buffer"`
	Serial   string      `yaml:"serial"`
	DataDir  string      `yaml:"data_dir"`
	Store    StoreConfig `yaml:"store"`
}

// StoreConfig 描述框架会话状态存储。
type StoreConfig struct {
	Type string `yaml:"type"`
	Path string `yaml:"path"`
}

// AdapterConfig 描述框架使用的协议适配器。
type AdapterConfig struct {
	Protocol  string                   `yaml:"protocol"`
	Transport onebot11.TransportConfig `yaml:"transport"`
}

// SecurityConfig 描述框架级安全策略。
type SecurityConfig struct {
	SuperUsers []string `yaml:"superusers"`
}

// PluginEntry 描述一个插件的启用状态和原始配置。
type PluginEntry struct {
	Enabled *bool     `yaml:"enabled"`
	Config  yaml.Node `yaml:"config"`
}

// LoadConfig 从 YAML 文件读取 anybot 配置。
func LoadConfig(path string) (Config, error) {
	if path == "" {
		path = "anybot.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.configPath = path
	cfg.applyDefaults()
	if err := cfg.loadPluginConfigs(path); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg *Config) applyDefaults() {
	if cfg.Runtime.LogLevel == "" {
		cfg.Runtime.LogLevel = "info"
	}
	if cfg.Runtime.Workers == "" {
		cfg.Runtime.Workers = "auto"
	}
	if cfg.Runtime.Buffer == 0 {
		cfg.Runtime.Buffer = 256
	}
	if cfg.Runtime.Serial == "" {
		cfg.Runtime.Serial = "conversation"
	}
	if cfg.Runtime.DataDir == "" {
		cfg.Runtime.DataDir = ".anybot"
	}
	if cfg.Runtime.Store.Type == "" {
		cfg.Runtime.Store.Type = "file"
	}
	if cfg.Runtime.Store.Path == "" && strings.ToLower(strings.TrimSpace(cfg.Runtime.Store.Type)) == "file" {
		cfg.Runtime.Store.Path = "store.json"
	}
	if cfg.Adapter.Protocol == "" {
		cfg.Adapter.Protocol = "onebot11"
	}
	if cfg.Plugins == nil {
		cfg.Plugins = map[string]PluginEntry{}
	}
}

func (cfg Config) onebotConfig() onebot11.Config {
	return onebot11.Config{
		Protocol:  cfg.Adapter.Protocol,
		Transport: cfg.Adapter.Transport,
	}
}

func parseWorkers(value string) (int, bool, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "auto" {
		return 0, false, nil
	}
	workers, err := strconv.Atoi(value)
	if err != nil || workers < 0 {
		return 0, false, fmt.Errorf("runtime.workers 必须是 auto 或非负整数")
	}
	return workers, true, nil
}

func pluginEnabled(entry PluginEntry) bool {
	return entry.Enabled == nil || *entry.Enabled
}

func (cfg *Config) loadPluginConfigs(configPath string) error {
	dir := PluginConfigDir(configPath)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		id, ok, err := pluginConfigID(entry.Name())
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := ValidatePluginID(id); err != nil {
			return fmt.Errorf("%s: %w", filepath.Join(dir, entry.Name()), err)
		}
		if _, exists := cfg.Plugins[id]; exists {
			return fmt.Errorf("plugin %s is configured more than once", id)
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		var plugin PluginEntry
		if err := yaml.Unmarshal(data, &plugin); err != nil {
			return fmt.Errorf("%s: %w", filepath.Join(dir, entry.Name()), err)
		}
		cfg.Plugins[id] = plugin
	}
	return nil
}

// PluginConfigDir 返回当前机器人目录下固定的插件配置目录。
func PluginConfigDir(configPath string) string {
	base := filepath.Dir(configPath)
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "plugins.d")
}

func pluginConfigID(file string) (string, bool, error) {
	ext := filepath.Ext(file)
	if ext != ".yaml" && ext != ".yml" {
		return "", false, nil
	}
	name := strings.TrimSuffix(file, ext)
	if name == "" || strings.HasPrefix(name, ".") {
		return "", false, nil
	}
	return name, true, nil
}

func resolveConfigRelativePath(configPath, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	base := filepath.Dir(configPath)
	if base == "" {
		base = "."
	}
	return filepath.Join(base, value)
}
