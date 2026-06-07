package host

import (
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/core"
	absdk "github.com/tty00a381/anybot/sdk"
)

// UnknownPluginError 表示配置启用了当前注册表不可加载的插件。
type UnknownPluginError struct {
	Name string
}

func (err UnknownPluginError) Error() string {
	return fmt.Sprintf("未知插件 %q", err.Name)
}

// NewLogger 创建 anybot 使用的结构化日志器。
func NewLogger(level string, out io.Writer) (*slog.Logger, error) {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		slogLevel = slog.LevelInfo
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("runtime.log_level 不支持 %q", level)
	}
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: slogLevel})), nil
}

// AppOptions 描述框架运行时的附加参数。
type AppOptions struct {
	ConfigPath   string
	RuntimeState bool
}

// AppOption 调整框架运行时装配。
type AppOption func(*AppOptions)

// WithConfigPath 注入当前框架配置文件路径，供插件运行期写回自身配置。
func WithConfigPath(path string) AppOption {
	return func(opts *AppOptions) {
		opts.ConfigPath = path
	}
}

// WithRuntimeState 启用框架托管的运行时状态资源，包括 Store 和插件数据目录。
func WithRuntimeState() AppOption {
	return func(opts *AppOptions) {
		opts.RuntimeState = true
	}
}

// NewApp 根据框架配置和插件注册表创建 AnyBot 运行时。
func NewApp(cfg Config, registry absdk.Registry, logger *slog.Logger, appOptions ...AppOption) (*core.App, error) {
	var hostOpts AppOptions
	for _, opt := range appOptions {
		if opt != nil {
			opt(&hostOpts)
		}
	}
	rawRuntime := cfg.Runtime
	configPath := hostOpts.ConfigPath
	if configPath == "" {
		configPath = cfg.configPath
	}
	cfg.applyDefaults()
	if logger == nil {
		logger = slog.Default()
	}
	adapter, err := onebot11.AdapterFromConfig(cfg.onebotConfig(), onebot11.WithLogger(logger))
	if err != nil {
		return nil, err
	}
	opts := []core.Option{
		core.WithAdapter(adapter),
		core.WithLogger(logger),
		core.WithBuffer(cfg.Runtime.Buffer),
		core.WithSuperUsers(cfg.Security.SuperUsers...),
	}
	if hostOpts.RuntimeState {
		if store, err := newRuntimeStore(cfg.Runtime, rawRuntime, configPath); err != nil {
			return nil, err
		} else if store != nil {
			opts = append(opts, core.WithStore(store))
		}
		if dataDir, ok, err := runtimeDataDir(cfg.Runtime, rawRuntime, configPath); err != nil {
			return nil, err
		} else if ok {
			opts = append(opts, absdk.WithDataDir(dataDir))
		}
	}
	if hostOpts.ConfigPath != "" {
		opts = append(opts, absdk.WithConfigStore(newPluginConfigStore(hostOpts.ConfigPath)))
	}
	if workers, ok, err := parseWorkers(cfg.Runtime.Workers); err != nil {
		return nil, err
	} else if ok {
		opts = append(opts, core.WithWorkers(workers))
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Runtime.Serial)) {
	case "", "none", "off", "false":
	case "conversation":
		opts = append(opts, core.WithSerialByConversation())
	default:
		return nil, fmt.Errorf("runtime.serial 不支持 %q", cfg.Runtime.Serial)
	}

	app := core.New(opts...)
	app.Use(core.Recover(logger), core.Trace(logger))
	if err := InstallPlugins(app, cfg, registry); err != nil {
		return nil, err
	}
	return app, nil
}

// ValidateConfig 静态校验框架配置，不执行插件安装逻辑。
func ValidateConfig(cfg Config, registry absdk.Registry) error {
	rawRuntime := cfg.Runtime
	configPath := cfg.configPath
	cfg.applyDefaults()
	if _, err := onebot11.AdapterFromConfig(cfg.onebotConfig()); err != nil {
		return err
	}
	if _, _, err := parseWorkers(cfg.Runtime.Workers); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Runtime.Serial)) {
	case "", "none", "off", "false", "conversation":
	default:
		return fmt.Errorf("runtime.serial 不支持 %q", cfg.Runtime.Serial)
	}
	if _, err := newRuntimeStore(cfg.Runtime, rawRuntime, configPath); err != nil {
		return err
	}
	if _, _, err := runtimeDataDir(cfg.Runtime, rawRuntime, configPath); err != nil {
		return err
	}
	for _, name := range configuredPluginNames(cfg) {
		entry := cfg.Plugins[name]
		if !pluginEnabled(entry) {
			continue
		}
		factory, ok := registry.Factory(name)
		if !ok {
			return UnknownPluginError{Name: name}
		}
		if _, err := factory.Build(entry.Config); err != nil {
			return fmt.Errorf("插件 %s 配置无效: %w", name, err)
		}
	}
	return nil
}

// InstallPlugins 按配置启用插件。
func InstallPlugins(app *core.App, cfg Config, registry absdk.Registry) error {
	names := configuredPluginNames(cfg)
	for _, name := range names {
		entry := cfg.Plugins[name]
		if !pluginEnabled(entry) {
			continue
		}
		factory, ok := registry.Factory(name)
		if !ok {
			return UnknownPluginError{Name: name}
		}
		module, err := factory.Build(entry.Config)
		if err != nil {
			return fmt.Errorf("插件 %s 配置无效: %w", name, err)
		}
		if err := absdk.Install(app, module); err != nil {
			return err
		}
	}
	return nil
}

// EnabledPlugins 返回按名称排序的已启用插件名。
func EnabledPlugins(cfg Config, registry absdk.Registry) ([]string, error) {
	var enabled []string
	for _, name := range configuredPluginNames(cfg) {
		entry := cfg.Plugins[name]
		if !pluginEnabled(entry) {
			continue
		}
		if _, ok := registry.Factory(name); !ok {
			return nil, UnknownPluginError{Name: name}
		}
		enabled = append(enabled, name)
	}
	return enabled, nil
}

func configuredPluginNames(cfg Config) []string {
	names := make([]string, 0, len(cfg.Plugins))
	for name := range cfg.Plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
