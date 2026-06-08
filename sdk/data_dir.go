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

// DataDir 返回当前插件 ID 的私有数据目录，并确保目录已创建。
func (c *Context) DataDir() (string, error) {
	if c == nil || c.app == nil || c.PluginID() == "" {
		return "", ErrDataDirUnavailable
	}
	rootPath := strings.TrimSpace(c.env.DataDir)
	if rootPath == "" {
		return "", ErrDataDirUnavailable
	}
	dir, err := pluginDataDir(rootPath, c.PluginID())
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func pluginDataDir(root, id string) (string, error) {
	if root == "" {
		return "", ErrDataDirUnavailable
	}
	id = strings.TrimSpace(id)
	if err := ValidatePluginID(id); err != nil {
		return "", fmt.Errorf("plugin id %q cannot be used as a data directory name", id)
	}
	return filepath.Join(root, "plugins", id), nil
}
