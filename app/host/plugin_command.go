package host

import (
	"fmt"
	"io"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

// PluginCommandOptions 描述生成运行框架内置 plugin 子命令的运行参数。
type PluginCommandOptions struct {
	Args          []string
	Output        io.Writer
	ConfigPath    string
	WorkspacePath string
	Registry      absdk.Registry
}

// RunPluginCommand 执行生成运行框架内置的 plugin 子命令。
func RunPluginCommand(opts PluginCommandOptions) error {
	if len(opts.Args) == 0 {
		return fmt.Errorf("用法：plugin <sync|status|inspect|config|check|enable|disable>")
	}
	output := opts.Output
	if output == nil {
		output = io.Discard
	}
	configPath := strings.TrimSpace(opts.ConfigPath)
	if configPath == "" {
		configPath = "anybot.yaml"
	}
	workspacePath := strings.TrimSpace(opts.WorkspacePath)
	if workspacePath == "" {
		workspacePath = PluginWorkspaceFile
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	workspace, err := LoadPluginWorkspace(workspacePath)
	if err != nil {
		return err
	}
	switch opts.Args[0] {
	case "sync":
		result, err := SyncPluginConfigEntriesForWorkspace(configPath, opts.Registry, workspace)
		if err != nil {
			return err
		}
		return WritePluginConfigSyncSummary(output, result)
	case "status":
		return WritePluginStatusTable(output, PluginStatuses(cfg, opts.Registry, workspace))
	case "inspect":
		if len(opts.Args) < 2 {
			return fmt.Errorf("用法：plugin inspect <name>")
		}
		inspect, err := InspectPlugin(configPath, opts.Registry, workspace, opts.Args[1])
		if err != nil {
			return err
		}
		return WritePluginInspect(output, inspect)
	case "config":
		if len(opts.Args) < 3 {
			return fmt.Errorf("用法：plugin config <name> <key=value>...，或 plugin config <name> -reset <key>...")
		}
		name := opts.Args[1]
		if err := EnsureKnownPluginTarget(cfg, opts.Registry, workspace, name, true); err != nil {
			return err
		}
		change, err := ParsePluginConfigChanges(opts.Args[2:])
		if err != nil {
			return err
		}
		result, err := ApplyPluginConfigChange(configPath, name, change)
		if err != nil {
			return err
		}
		return writePluginConfigChangeResult(output, configPath, opts.Registry, name, result)
	case "enable", "disable":
		if len(opts.Args) < 2 {
			return fmt.Errorf("用法：plugin %s <name>", opts.Args[0])
		}
		enabled := opts.Args[0] == "enable"
		name := opts.Args[1]
		if err := EnsureKnownPluginTarget(cfg, opts.Registry, workspace, name, !enabled); err != nil {
			return err
		}
		changed, err := SetPluginEnabled(configPath, name, enabled)
		if err != nil {
			return err
		}
		return writePluginEnabledResult(output, configPath, opts.Registry, name, enabled, changed)
	case "check":
		checks := PluginConfigChecks(cfg, opts.Registry, workspace)
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

func writePluginConfigChangeResult(output io.Writer, configPath string, registry absdk.Registry, name string, result PluginConfigChangeResult) error {
	if result.Reset {
		if result.Changed {
			fmt.Fprintf(output, "插件配置已重置：%s（%d 项）\n", name, result.Count)
		} else {
			fmt.Fprintf(output, "插件配置未变化：%s\n", name)
		}
		sync, err := SyncPluginConfigEntry(configPath, registry, name)
		if err != nil {
			return err
		}
		if sync.Changed {
			fmt.Fprintln(output, "默认配置已同步：1 项更新")
		} else if !sync.Available {
			fmt.Fprintf(output, "默认配置待构建同步：%s（重新运行 anybot up 会完成）\n", name)
		}
		return nil
	}
	if result.Changed {
		fmt.Fprintf(output, "插件配置已更新：%s（%d 项）\n", name, result.Count)
	} else {
		fmt.Fprintf(output, "插件配置未变化：%s\n", name)
	}
	return nil
}

func writePluginEnabledResult(output io.Writer, configPath string, registry absdk.Registry, name string, enabled, changed bool) error {
	action := "启用"
	if !enabled {
		action = "禁用"
	}
	if changed {
		fmt.Fprintf(output, "插件已%s：%s\n", action, name)
	} else {
		fmt.Fprintf(output, "插件已处于%s状态：%s\n", action, name)
	}
	if !enabled {
		return nil
	}
	result, err := SyncPluginConfigEntry(configPath, registry, name)
	if err != nil {
		return err
	}
	if result.Changed {
		fmt.Fprintln(output, "默认配置已同步：1 项更新")
	} else if !result.Available {
		fmt.Fprintf(output, "默认配置待构建同步：%s（重新运行 anybot up 会完成）\n", name)
	}
	return nil
}
