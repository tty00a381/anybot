package sdk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrDataDirUnavailable 表示当前运行时没有配置插件数据目录。
var ErrDataDirUnavailable = errors.New("anybot: plugin data dir unavailable")

// DataDir 返回当前插件实例的私有数据目录，并确保目录已创建。
func (c *Context) DataDir() (string, error) {
	if c == nil || c.app == nil || c.InstanceID() == "" {
		return "", ErrDataDirUnavailable
	}
	rootPath := strings.TrimSpace(c.env.DataDir)
	if rootPath == "" {
		return "", ErrDataDirUnavailable
	}
	dir, err := pluginDataDir(rootPath, c.InstanceID())
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func pluginDataDir(root, name string) (string, error) {
	if root == "" {
		return "", ErrDataDirUnavailable
	}
	name = strings.TrimSpace(name)
	if err := ValidatePluginName(name); err != nil {
		return "", fmt.Errorf("plugin name %q cannot be used as a data directory name", name)
	}
	return filepath.Join(root, "plugins", name), nil
}
