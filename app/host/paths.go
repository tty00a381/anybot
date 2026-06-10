package host

import "path/filepath"

// ConfigPath 返回机器人工作目录中的默认框架配置路径。
func ConfigPath(dir string) string {
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, ConfigDirName, ConfigFileName)
}

// PluginLockPath 返回机器人工作目录中的插件锁路径。
func PluginLockPath(dir string) string {
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, PluginLockFile)
}

// WorkDirForConfig 返回配置文件所属的机器人工作目录。
func WorkDirForConfig(configPath string) string {
	if configPath == "" {
		configPath = DefaultConfigPath
	}
	clean := filepath.Clean(configPath)
	dir := filepath.Dir(clean)
	if filepath.Base(clean) == ConfigFileName && filepath.Base(dir) == ConfigDirName {
		root := filepath.Dir(dir)
		if root == "" {
			return "."
		}
		return root
	}
	if dir == "" {
		return "."
	}
	return dir
}
