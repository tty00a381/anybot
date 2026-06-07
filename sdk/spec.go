package sdk

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Plugin 是运行框架已经完成配置解析后可安装的插件实例。
type Plugin interface {
	Manifest() Manifest
	Setup(*Context) error
}

// Definition 是插件定义，可转成运行框架工厂，也可用默认配置构建插件实例。
type Definition interface {
	Manifest() Manifest
	Build() (Plugin, error)
	Factory() Factory
}

// SetupFunc 是 typed config 插件的安装函数。
type SetupFunc[T any] func(*Context, T) error

type typedDefinition[T any] struct {
	Info    Manifest
	Default T
	SetupFn SetupFunc[T]
}

// Define 创建 typed config 插件定义。
func Define[T any](info Manifest, defaults T, setup SetupFunc[T]) Definition {
	return typedDefinition[T]{Info: info, Default: defaults, SetupFn: setup}
}

// Manifest 返回插件清单。
func (d typedDefinition[T]) Manifest() Manifest {
	return d.Info
}

// Factory 返回可供运行框架注册的插件工厂。
func (d typedDefinition[T]) Factory() Factory {
	return Factory{
		Info:    d.Info,
		Default: d.Default,
		Build: func(node yaml.Node) (Plugin, error) {
			cfg, err := cloneTypedConfig(d.Default)
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
			return configuredPlugin[T]{definition: d, config: cfg}, nil
		},
	}
}

// Build 使用默认配置创建插件实例，主要供嵌入式程序和插件测试使用。
func (d typedDefinition[T]) Build() (Plugin, error) {
	return d.Factory().Build(yaml.Node{})
}

type configuredPlugin[T any] struct {
	definition typedDefinition[T]
	config     T
}

func (p configuredPlugin[T]) Manifest() Manifest {
	return p.definition.Info
}

func (p configuredPlugin[T]) Setup(ctx *Context) error {
	if p.definition.SetupFn == nil {
		return fmt.Errorf("plugin %s setup function is required", p.definition.Info.Name)
	}
	return p.definition.SetupFn(ctx, p.config)
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
