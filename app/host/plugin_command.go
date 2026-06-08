package host

import (
	"fmt"
	"io"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

// PluginCommandOptions 描述生成运行框架内置 plugin 子命令的运行参数。
type PluginCommandOptions struct {
	Args       []string
	Output     io.Writer
	ConfigPath string
	LockPath   string
	Registry   absdk.Registry
}

// RunPluginCommand 执行生成运行框架内置的 plugin 子命令。
func RunPluginCommand(opts PluginCommandOptions) error {
	if len(opts.Args) == 0 {
		return fmt.Errorf("用法：plugin <sync|status|inspect|config|check|enable|disable>")
	}
	if err := validatePluginCommandArgs(opts.Args); err != nil {
		return err
	}
	output := opts.Output
	if output == nil {
		output = io.Discard
	}
	configPath := strings.TrimSpace(opts.ConfigPath)
	if configPath == "" {
		configPath = "anybot.yaml"
	}
	lockPath := strings.TrimSpace(opts.LockPath)
	if lockPath == "" {
		lockPath = PluginLockFile
	}
	registry := opts.Registry
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	lock, err := LoadPluginLock(lockPath)
	if err != nil {
		return err
	}
	registry, err = RegistryForLock(lock, registry)
	if err != nil {
		return err
	}
	switch opts.Args[0] {
	case "sync":
		result, err := SyncPluginConfigEntriesForLock(configPath, registry, lock)
		if err != nil {
			return err
		}
		return WritePluginConfigSyncSummary(output, result)
	case "status":
		return WritePluginStatusTable(output, PluginStatuses(cfg, registry, lock))
	case "inspect":
		id, err := ResolvePluginID(cfg, registry, lock, opts.Args[1], true)
		if err != nil {
			return err
		}
		inspect, err := InspectPlugin(configPath, registry, lock, id)
		if err != nil {
			return err
		}
		return WritePluginInspect(output, inspect)
	case "config":
		if len(opts.Args) < 3 {
			return fmt.Errorf("用法：plugin config <id> <key=value>...，或 plugin config <id> -reset <key>...")
		}
		id, err := ResolvePluginID(cfg, registry, lock, opts.Args[1], true)
		if err != nil {
			return err
		}
		change, err := ParsePluginConfigChanges(opts.Args[2:])
		if err != nil {
			return err
		}
		result, err := ApplyPluginConfigChange(configPath, id, change)
		if err != nil {
			return err
		}
		return writePluginConfigChangeResult(output, configPath, registry, id, result)
	case "enable", "disable":
		enabled := opts.Args[0] == "enable"
		id, err := ResolvePluginID(cfg, registry, lock, opts.Args[1], !enabled)
		if err != nil {
			return err
		}
		changed, err := SetPluginEnabled(configPath, id, enabled)
		if err != nil {
			return err
		}
		return writePluginEnabledResult(output, configPath, registry, id, enabled, changed)
	case "check":
		checks := PluginConfigChecks(cfg, registry, lock)
		if err := WritePluginConfigCheckTable(output, checks); err != nil {
			return err
		}
		if PluginConfigCheckFailed(checks) {
			return fmt.Errorf("插件配置检查失败")
		}
		return nil
	default:
		return fmt.Errorf("未知插件命令 %q", opts.Args[0])
	}
}

func validatePluginCommandArgs(args []string) error {
	switch args[0] {
	case "sync", "status", "check":
		if len(args) != 1 {
			return fmt.Errorf("用法：plugin %s", args[0])
		}
	case "inspect", "enable", "disable":
		if len(args) != 2 {
			return fmt.Errorf("用法：plugin %s <id>", args[0])
		}
	case "config":
		if len(args) < 3 {
			return fmt.Errorf("用法：plugin config <id> <key=value>...，或 plugin config <id> -reset <key>...")
		}
	default:
		return fmt.Errorf("未知插件命令 %q", args[0])
	}
	return nil
}

func writePluginConfigChangeResult(output io.Writer, configPath string, registry absdk.Registry, id string, result PluginConfigChangeResult) error {
	if result.Reset {
		if result.Changed {
			fmt.Fprintf(output, "插件配置已重置：%s（%d 项）\n", id, result.Count)
		} else {
			fmt.Fprintf(output, "插件配置未变化：%s\n", id)
		}
		sync, err := SyncPluginConfigEntry(configPath, registry, id)
		if err != nil {
			return err
		}
		if sync.Changed {
			fmt.Fprintln(output, "默认配置已同步：1 项更新")
		} else if !sync.Available {
			fmt.Fprintf(output, "默认配置待构建同步：%s（重新运行 anybot up 会完成）\n", id)
		}
		return nil
	}
	if result.Changed {
		fmt.Fprintf(output, "插件配置已更新：%s（%d 项）\n", id, result.Count)
	} else {
		fmt.Fprintf(output, "插件配置未变化：%s\n", id)
	}
	return nil
}

func writePluginEnabledResult(output io.Writer, configPath string, registry absdk.Registry, id string, enabled, changed bool) error {
	action := "启用"
	if !enabled {
		action = "禁用"
	}
	if changed {
		fmt.Fprintf(output, "插件已%s：%s\n", action, id)
	} else {
		fmt.Fprintf(output, "插件已处于%s状态：%s\n", action, id)
	}
	if !enabled {
		return nil
	}
	result, err := SyncPluginConfigEntry(configPath, registry, id)
	if err != nil {
		return err
	}
	if result.Changed {
		fmt.Fprintln(output, "默认配置已同步：1 项更新")
	} else if !result.Available {
		fmt.Fprintf(output, "默认配置待构建同步：%s（重新运行 anybot up 会完成）\n", id)
	}
	return nil
}
