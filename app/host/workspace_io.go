package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadPluginWorkspace 读取外部插件工作区清单；文件不存在时返回空工作区。
func LoadPluginWorkspace(path string) (PluginWorkspace, error) {
	if path == "" {
		path = PluginWorkspaceFile
	}
	if err := requireYAMLFile(path, "插件工作区清单"); err != nil {
		return PluginWorkspace{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultPluginWorkspace(), nil
	}
	if err != nil {
		return PluginWorkspace{}, err
	}
	var workspace PluginWorkspace
	if err := yaml.Unmarshal(data, &workspace); err != nil {
		return PluginWorkspace{}, err
	}
	workspace.applyDefaults()
	if err := ValidatePluginWorkspace(workspace); err != nil {
		return PluginWorkspace{}, err
	}
	return workspace, nil
}

// SavePluginWorkspace 写入外部插件工作区清单。
func SavePluginWorkspace(path string, workspace PluginWorkspace) error {
	if path == "" {
		path = PluginWorkspaceFile
	}
	if err := requireYAMLFile(path, "插件工作区清单"); err != nil {
		return err
	}
	workspace.applyDefaults()
	if err := ValidatePluginWorkspace(workspace); err != nil {
		return err
	}
	data, err := yaml.Marshal(workspace)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

// CheckPluginWorkspaceWritable 检查 anybot 生成文件是否可安全写入。
func CheckPluginWorkspaceWritable(dir string, force bool) error {
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
