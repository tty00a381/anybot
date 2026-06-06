package sdk

import (
	"fmt"

	"github.com/tty00a381/anybot/core"
	"gopkg.in/yaml.v3"
)

// Module 是面向插件化宿主的成熟插件接口。
type Module interface {
	Manifest() Manifest
	Setup(*Context) error
}

// SetupFunc 是 typed config 插件的安装函数。
type SetupFunc[T any] func(*Context, T) error

// Spec 用 typed config 声明一个插件模块。
type Spec[T any] struct {
	Info    Manifest
	Default T
	SetupFn SetupFunc[T]
}

// Define 创建 typed config 插件规格。
func Define[T any](info Manifest, defaults T, setup SetupFunc[T]) Spec[T] {
	return Spec[T]{Info: info, Default: defaults, SetupFn: setup}
}

// Manifest 返回插件清单。
func (s Spec[T]) Manifest() Manifest {
	return s.Info
}

// Setup 使用默认配置安装插件。
func (s Spec[T]) Setup(ctx *Context) error {
	if s.SetupFn == nil {
		return fmt.Errorf("plugin %s setup function is required", s.Info.Name)
	}
	cfg, err := cloneTypedConfig(s.Default)
	if err != nil {
		return fmt.Errorf("plugin %s default config: %w", s.Info.Name, err)
	}
	if err := validateTypedConfig(cfg); err != nil {
		return err
	}
	return s.SetupFn(ctx, cfg)
}

// Factory 返回可供宿主注册的插件工厂。
func (s Spec[T]) Factory() Factory {
	return Factory{
		Info:    s.Info,
		Default: s.Default,
		Build: func(node yaml.Node) (core.Plugin, error) {
			cfg, err := cloneTypedConfig(s.Default)
			if err != nil {
				return nil, fmt.Errorf("default config: %w", err)
			}
			if node.Kind != 0 && node.Tag != "!!null" {
				resolved, err := resolveConfigEnv(node)
				if err != nil {
					return nil, err
				}
				if err := resolved.Decode(&cfg); err != nil {
					return nil, err
				}
			}
			if err := validateTypedConfig(cfg); err != nil {
				return nil, err
			}
			return AsPlugin(configuredSpec[T]{spec: s, config: cfg}), nil
		},
	}
}

type configuredSpec[T any] struct {
	spec   Spec[T]
	config T
}

func (s configuredSpec[T]) Manifest() Manifest {
	return s.spec.Info
}

func (s configuredSpec[T]) Setup(ctx *Context) error {
	if s.spec.SetupFn == nil {
		return fmt.Errorf("plugin %s setup function is required", s.spec.Info.Name)
	}
	return s.spec.SetupFn(ctx, s.config)
}

type pluginAdapter struct {
	module Module
}

// AsPlugin 将插件 SDK Module 适配为核心 core.Plugin。
func AsPlugin(module Module) core.Plugin {
	return pluginAdapter{module: module}
}

func (p pluginAdapter) Manifest() Manifest {
	if p.module == nil {
		return Manifest{}
	}
	return p.module.Manifest()
}

func (p pluginAdapter) Install(app *core.App) error {
	if p.module == nil {
		return nil
	}
	return p.installWithManifest(app, p.module.Manifest())
}

func (p pluginAdapter) installWithManifest(app *core.App, manifest Manifest) error {
	if p.module == nil {
		return nil
	}
	return p.module.Setup(NewContext(app, manifest))
}

func validateTypedConfig[T any](config T) error {
	if validator, ok := any(config).(interface{ Validate() error }); ok {
		return validator.Validate()
	}
	if validator, ok := any(&config).(interface{ Validate() error }); ok {
		return validator.Validate()
	}
	return nil
}

func cloneTypedConfig[T any](config T) (T, error) {
	data, err := yaml.Marshal(config)
	if err != nil {
		var zero T
		return zero, err
	}
	var out T
	if err := yaml.Unmarshal(data, &out); err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}
