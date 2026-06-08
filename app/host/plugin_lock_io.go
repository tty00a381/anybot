package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadPluginLock 读取外部插件锁；文件不存在时返回空锁。
func LoadPluginLock(path string) (PluginLock, error) {
	if path == "" {
		path = PluginLockFile
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultPluginLock(), nil
	}
	if err != nil {
		return PluginLock{}, err
	}
	var lock PluginLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return PluginLock{}, err
	}
	lock.applyDefaults()
	if err := ValidatePluginLock(lock); err != nil {
		return PluginLock{}, err
	}
	return lock, nil
}

// SavePluginLock 写入外部插件锁。
func SavePluginLock(path string, lock PluginLock) error {
	if path == "" {
		path = PluginLockFile
	}
	lock.applyDefaults()
	if err := ValidatePluginLock(lock); err != nil {
		return err
	}
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

// CheckGeneratedHostWritable 检查 anybot 生成文件是否可安全写入。
func CheckGeneratedHostWritable(dir string, force bool) error {
	if dir == "" {
		dir = "."
	}
	for _, name := range []string{generatedPlugins, generatedMain} {
		if err := checkGeneratedWritable(filepath.Join(dir, name), force); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerated(path, content string, force bool) error {
	if err := checkGeneratedWritable(path, force); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(content), 0o644)
}

func checkGeneratedWritable(path string, force bool) error {
	if force {
		return nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.HasPrefix(string(data), generatedHeader) {
		return nil
	}
	return fmt.Errorf("%s 已存在且不是 anybot 生成文件；使用 -force 接管，或换一个空目录", path)
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return writeGenerated(path, content, false)
}
