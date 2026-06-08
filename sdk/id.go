package sdk

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	pluginIDPrefix       = "plg_"
	pluginIDRandomLength = 26
)

// ValidatePluginID 校验 PluginID 是否符合 AnyBot 生成的随机安装 ID 形态。
func ValidatePluginID(id string) error {
	if id == "" {
		return fmt.Errorf("plugin id is required")
	}
	if strings.TrimSpace(id) != id ||
		strings.ContainsAny(id, `/\`) ||
		filepath.IsAbs(id) ||
		id == "." ||
		id == ".." ||
		strings.HasPrefix(id, ".") {
		return fmt.Errorf("plugin id %q is invalid", id)
	}
	if !strings.HasPrefix(id, pluginIDPrefix) || len(strings.TrimPrefix(id, pluginIDPrefix)) != pluginIDRandomLength {
		return fmt.Errorf("plugin id %q is invalid: use an AnyBot-generated random PluginID", id)
	}
	if !validPluginIDRandomPart(strings.TrimPrefix(id, pluginIDPrefix)) {
		return fmt.Errorf("plugin id %q is invalid: generated PluginID contains invalid characters", id)
	}
	return nil
}

func validPluginIDRandomPart(id string) bool {
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '2' && r <= '7':
		default:
			return false
		}
	}
	return true
}
