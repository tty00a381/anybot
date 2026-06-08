package sdk

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePluginID 校验本地插件 ID 能安全用于锁文件、配置文件、状态前缀和数据目录。
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
	if !validPluginIdentifier(id) {
		return fmt.Errorf("plugin id %q is invalid: use lowercase letters, digits, and underscores; start with a letter", id)
	}
	return nil
}

func validPluginIdentifier(id string) bool {
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		case r == '_':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
