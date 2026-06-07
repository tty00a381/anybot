package sdk

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Factory 根据框架配置创建插件实例。
type Factory struct {
	Info    Manifest
	Default any
	Build   func(yaml.Node) (Module, error)
}

// WithName 返回使用指定注册名的工厂，适合运行框架为外部插件提供配置别名。
func (f Factory) WithName(name string) Factory {
	if name == "" {
		return f
	}
	f.Info.Name = name
	if f.Build == nil {
		return f
	}
	build := f.Build
	f.Build = func(node yaml.Node) (Module, error) {
		module, err := build(node)
		if err != nil {
			return nil, err
		}
		return namedModule{module: module, name: name}, nil
	}
	return f
}

// Registry 保存运行框架可加载的插件工厂。
type Registry struct {
	factories map[string]Factory
}

// NewRegistry 创建空插件注册表。
func NewRegistry() Registry {
	return Registry{factories: map[string]Factory{}}
}

// Register 注册插件工厂。
func (r Registry) Register(factory Factory) error {
	if r.factories == nil {
		return fmt.Errorf("plugin registry is not initialized")
	}
	if factory.Info.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if factory.Build == nil {
		return fmt.Errorf("plugin %s build function is required", factory.Info.Name)
	}
	if _, exists := r.factories[factory.Info.Name]; exists {
		return fmt.Errorf("plugin %s already registered", factory.Info.Name)
	}
	r.factories[factory.Info.Name] = factory
	return nil
}

// Factory 返回指定插件工厂。
func (r Registry) Factory(name string) (Factory, bool) {
	factory, ok := r.factories[name]
	return factory, ok
}

// Plugins 返回按名称排序的插件清单。
func (r Registry) Plugins() []Manifest {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Manifest, 0, len(names))
	for _, name := range names {
		out = append(out, r.factories[name].Info)
	}
	return out
}

type namedModule struct {
	module Module
	name   string
}

func (m namedModule) Manifest() Manifest {
	if m.module == nil {
		return Manifest{Name: m.name}
	}
	manifest := m.module.Manifest()
	manifest.Name = m.name
	return manifest
}

func (m namedModule) Setup(ctx *Context) error {
	if m.module == nil {
		return nil
	}
	return m.module.Setup(ctx)
}
