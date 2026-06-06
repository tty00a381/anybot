package sdk

import (
	"fmt"
	"sort"

	"github.com/tty00a381/anybot/core"
	"gopkg.in/yaml.v3"
)

// Factory 根据宿主配置创建插件实例。
type Factory struct {
	Info    Manifest
	Default any
	Build   func(yaml.Node) (core.Plugin, error)
}

// WithName 返回使用指定注册名的工厂，适合宿主为外部插件提供配置别名。
func (f Factory) WithName(name string) Factory {
	if name == "" {
		return f
	}
	f.Info.Name = name
	if f.Build == nil {
		return f
	}
	build := f.Build
	f.Build = func(node yaml.Node) (core.Plugin, error) {
		plugin, err := build(node)
		if err != nil {
			return nil, err
		}
		return namedPlugin{plugin: plugin, name: name}, nil
	}
	return f
}

// Registry 保存宿主可加载的插件工厂。
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

type namedPlugin struct {
	plugin core.Plugin
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

func (p namedPlugin) Install(app *core.App) error {
	if p.plugin == nil {
		return nil
	}
	if plugin, ok := p.plugin.(interface {
		installWithManifest(*core.App, Manifest) error
	}); ok {
		return plugin.installWithManifest(app, p.Manifest())
	}
	return p.plugin.Install(app)
}
