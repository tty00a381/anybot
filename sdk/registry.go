package sdk

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Factory 根据框架配置创建插件实例。
type Factory struct {
	Info       Manifest
	InstanceID string
	Default    any
	Build      func(yaml.Node) (Plugin, error)
}

// WithName 返回使用指定配置名的工厂，适合运行框架为外部插件提供安装别名。
func (f Factory) WithName(name string) Factory {
	if name == "" {
		return f
	}
	if f.InstanceID == "" {
		f.InstanceID = f.Info.Name
	}
	if err := ValidatePluginName(name); err != nil {
		f.Info.Name = name
		f.Build = func(yaml.Node) (Plugin, error) {
			return nil, err
		}
		return f
	}
	f.Info.Name = name
	if f.Build == nil {
		return f
	}
	build := f.Build
	f.Build = func(node yaml.Node) (Plugin, error) {
		plugin, err := build(node)
		if err != nil {
			return nil, err
		}
		return namedPlugin{plugin: plugin, name: name}, nil
	}
	return f
}

// WithInstanceID 返回使用指定稳定实例 ID 的工厂，适合运行框架为外部插件固定持久化命名空间。
func (f Factory) WithInstanceID(id string) Factory {
	if id == "" {
		return f
	}
	f.InstanceID = id
	if err := ValidatePluginName(id); err != nil {
		f.Build = func(yaml.Node) (Plugin, error) {
			return nil, err
		}
	}
	return f
}

// StorageName 返回当前工厂用于 Store/DataDir 的持久化命名空间。
func (f Factory) StorageName() string {
	if f.InstanceID != "" {
		return f.InstanceID
	}
	return f.Info.Name
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
	if err := ValidatePluginName(factory.Info.Name); err != nil {
		return err
	}
	if factory.InstanceID != "" {
		if err := ValidatePluginName(factory.InstanceID); err != nil {
			return err
		}
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

type namedPlugin struct {
	plugin Plugin
	name   string
}

func (p namedPlugin) Manifest() Manifest {
	if p.plugin == nil {
		return Manifest{Name: p.name}
	}
	manifest := p.plugin.Manifest()
	manifest.Name = p.name
	return manifest
}

func (p namedPlugin) Setup(ctx *Context) error {
	if p.plugin == nil {
		return nil
	}
	return p.plugin.Setup(ctx)
}
