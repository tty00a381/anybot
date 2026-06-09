package sdk

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrConfigStoreUnavailable 表示当前运行时没有配置可写的插件配置存储。
var ErrConfigStoreUnavailable = errors.New("anybot: plugin config store unavailable")

// ConfigAssignment 描述一次当前插件 config 下的字段写入。
type ConfigAssignment struct {
	Path  []string
	Value any
}

// ConfigStore 是运行框架注入的插件配置写回能力。方法的 plugin 参数是当前
// 插件的 PluginID，不是 Manifest.Name。
type ConfigStore interface {
	SetPluginConfig(context.Context, string, []ConfigAssignment) error
	ResetPluginConfig(context.Context, string, [][]string) error
}

// ConfigHandle 是当前插件配置写回入口。
type ConfigHandle struct {
	plugin string
	store  ConfigStore
}

// Config 返回当前插件的配置写回入口。
func (c *Context) Config() ConfigHandle {
	if c == nil || c.app == nil {
		return ConfigHandle{}
	}
	return ConfigHandle{plugin: c.PluginID(), store: c.env.ConfigStore}
}

// Available 判断当前框架是否支持运行期写回插件配置。
func (h ConfigHandle) Available() bool {
	return h.plugin != "" && h.store != nil
}

// Set 写入当前插件 config 下的单个字段。
func (h ConfigHandle) Set(ctx context.Context, key string, value any) error {
	path, err := ParseConfigPath(key)
	if err != nil {
		return err
	}
	return h.SetAll(ctx, ConfigAssignment{Path: path, Value: value})
}

// SetAll 批量写入当前插件 config 下的字段。
func (h ConfigHandle) SetAll(ctx context.Context, assignments ...ConfigAssignment) error {
	if h.plugin == "" || h.store == nil {
		return ErrConfigStoreUnavailable
	}
	if len(assignments) == 0 {
		return fmt.Errorf("plugin config assignment is required")
	}
	for _, assignment := range assignments {
		if err := validateConfigPath(assignment.Path); err != nil {
			return err
		}
	}
	return h.store.SetPluginConfig(ctx, h.plugin, assignments)
}

// Reset 删除当前插件 config 下的字段覆盖。
func (h ConfigHandle) Reset(ctx context.Context, keys ...string) error {
	if h.plugin == "" || h.store == nil {
		return ErrConfigStoreUnavailable
	}
	if len(keys) == 0 {
		return fmt.Errorf("plugin config key is required")
	}
	paths := make([][]string, 0, len(keys))
	for _, key := range keys {
		path, err := ParseConfigPath(key)
		if err != nil {
			return err
		}
		paths = append(paths, path)
	}
	return h.store.ResetPluginConfig(ctx, h.plugin, paths)
}

// ParseConfigPath 解析插件 config 下的点分字段路径。
func ParseConfigPath(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("plugin config key is required")
	}
	parts := strings.Split(input, ".")
	if err := validateConfigPath(parts); err != nil {
		return nil, fmt.Errorf("plugin config key %q is invalid", input)
	}
	return parts, nil
}

func validateConfigPath(parts []string) error {
	if len(parts) == 0 {
		return fmt.Errorf("plugin config key is required")
	}
	for _, part := range parts {
		if strings.TrimSpace(part) != part || part == "" || strings.Contains(part, "=") || strings.HasPrefix(part, "-") {
			return fmt.Errorf("plugin config key is invalid")
		}
	}
	return nil
}
