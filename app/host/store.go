package host

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tty00a381/anybot/core"
)

func newRuntimeStore(cfg RuntimeConfig, raw RuntimeConfig, configPath string) (core.Store, error) {
	if !runtimeDataConfigured(raw, configPath) {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Store.Type)) {
	case "", "memory", "none", "off", "false":
		return nil, nil
	case "file":
		path, err := runtimeStorePath(cfg, configPath)
		if err != nil {
			return nil, err
		}
		store, err := core.NewFileStore(path)
		if err != nil {
			return nil, fmt.Errorf("runtime.store.path %s: %w", path, err)
		}
		return store, nil
	default:
		return nil, fmt.Errorf("runtime.store.type 不支持 %q", cfg.Store.Type)
	}
}

func runtimeDataDir(cfg RuntimeConfig, raw RuntimeConfig, configPath string) (string, bool, error) {
	if !runtimeDataConfigured(raw, configPath) {
		return "", false, nil
	}
	dir := strings.TrimSpace(cfg.DataDir)
	if dir == "" {
		return "", false, fmt.Errorf("runtime.data_dir 不能为空")
	}
	return resolveConfigRelativePath(configPath, dir), true, nil
}

func runtimeDataConfigured(raw RuntimeConfig, configPath string) bool {
	return configPath != "" ||
		strings.TrimSpace(raw.DataDir) != "" ||
		strings.TrimSpace(raw.Store.Type) != "" ||
		strings.TrimSpace(raw.Store.Path) != ""
}

func runtimeStorePath(cfg RuntimeConfig, configPath string) (string, error) {
	path := strings.TrimSpace(cfg.Store.Path)
	if path == "" {
		return "", fmt.Errorf("runtime.store.path 不能为空")
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	dataDir := resolveConfigRelativePath(configPath, strings.TrimSpace(cfg.DataDir))
	if dataDir == "" {
		return "", fmt.Errorf("runtime.data_dir 不能为空")
	}
	if !localRuntimePath(path) {
		return "", fmt.Errorf("runtime.store.path %q 必须位于 runtime.data_dir 内，或使用绝对路径", path)
	}
	return filepath.Join(dataDir, filepath.Clean(path)), nil
}

func localRuntimePath(path string) bool {
	clean := filepath.Clean(path)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
