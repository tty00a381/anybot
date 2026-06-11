package sdk

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Factory 根据框架配置创建插件对象。
type Factory struct {
	Info     Manifest
	PluginID string
	Default  any
	Build    func(yaml.Node) (Plugin, error)
}

// WithPluginID 返回使用指定本地插件 ID 的工厂。
func (f Factory) WithPluginID(id string) Factory {
	if id == "" {
		return f
	}
	f.PluginID = strings.TrimSpace(id)
	if err := ValidatePluginID(f.PluginID); err != nil {
		f.Build = func(yaml.Node) (Plugin, error) {
			return nil, err
		}
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
	if factory.PluginID == "" {
		return fmt.Errorf("plugin id is required")
	}
	if err := ValidatePluginID(factory.PluginID); err != nil {
		return err
	}
	if factory.Build == nil {
		return fmt.Errorf("plugin %s build function is required", factory.PluginID)
	}
	if _, exists := r.factories[factory.PluginID]; exists {
		return fmt.Errorf("plugin %s already registered", factory.PluginID)
	}
	r.factories[factory.PluginID] = factory
	return nil
}

// Factory 返回指定插件 ID 的工厂。
func (r Registry) Factory(id string) (Factory, bool) {
	factory, ok := r.factories[id]
	return factory, ok
}

// PluginIDs 返回按字典序排序的已注册插件 ID。
func (r Registry) PluginIDs() []string {
	ids := make([]string, 0, len(r.factories))
	for id := range r.factories {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Plugins 返回按插件 ID 排序的插件清单。
func (r Registry) Plugins() []Manifest {
	ids := r.PluginIDs()
	out := make([]Manifest, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.factories[id].Info)
	}
	return out
}
