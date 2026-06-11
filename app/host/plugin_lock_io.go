package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadPluginLock 读取插件安装锁；文件不存在时返回空锁。
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

// SavePluginLock 写入插件安装锁。
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
	return checkGeneratedGoModWritable(filepath.Join(dir, "go.mod"), force)
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

func checkGeneratedGoModWritable(path string, force bool) error {
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
	if generatedGoMod(data) {
		return nil
	}
	return fmt.Errorf("%s 已存在且不是 anybot 生成的 go.mod；使用 -force 接管，或换一个空目录", path)
}

func generatedGoMod(data []byte) bool {
	text := string(data)
	if strings.HasPrefix(text, generatedHeader) {
		return true
	}
	module, ok := goModModulePath(text)
	return ok && module == defaultPluginHostModule
}

func goModModulePath(text string) (string, bool) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1], true
		}
	}
	return "", false
}

func writeGeneratedGoModIfMissing(path, content string) error {
	if err := checkGeneratedGoModWritable(path, false); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(content), 0o644)
}

func writeGeneratedGoMod(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(content), 0o644)
}
