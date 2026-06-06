package sdk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tty00a381/anybot/core"
)

// ErrDataDirUnavailable 表示当前运行时没有配置插件数据目录。
var ErrDataDirUnavailable = errors.New("anybot: plugin data dir unavailable")

var appDataDirs sync.Map

// WithDataDir 注入插件数据根目录；宿主会在其下为每个插件创建独立目录。
func WithDataDir(root string) Option {
	return func(app *core.App) {
		if app == nil {
			return
		}
		root = strings.TrimSpace(root)
		if root == "" {
			appDataDirs.Delete(app)
			return
		}
		appDataDirs.Store(app, root)
	}
}

// DataDir 返回当前插件的私有数据目录，并确保目录已创建。
func (c *Context) DataDir() (string, error) {
	if c == nil || c.app == nil || c.manifest.Name == "" {
		return "", ErrDataDirUnavailable
	}
	root, _ := appDataDirs.Load(c.app)
	rootPath, ok := root.(string)
	if !ok || rootPath == "" {
		return "", ErrDataDirUnavailable
	}
	dir, err := pluginDataDir(rootPath, c.manifest.Name)
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
	if !safePluginPathName(name) {
		return "", fmt.Errorf("plugin name %q cannot be used as a data directory name", name)
	}
	return filepath.Join(root, "plugins", name), nil
}

func safePluginPathName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}
