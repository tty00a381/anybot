package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tty00a381/anybot/app/host"
	modmodule "golang.org/x/mod/module"
)

func runPlugins(args []string) error {
	fs := flag.NewFlagSet("plugins", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	for _, manifest := range host.DefaultRegistry().Plugins() {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", manifest.Name, manifest.Version, manifest.Description)
	}
	return nil
}

func runPlugin(args []string) error {
	if len(args) == 0 {
		pluginUsage()
		return nil
	}
	switch args[0] {
	case "add":
		return runPluginAdd(args[1:])
	case "update":
		return runPluginUpdate(args[1:])
	case "remove":
		return runPluginRemove(args[1:])
	case "list":
		return runPluginList(args[1:])
	case "status":
		return runPluginStatus(args[1:])
	case "inspect":
		return runPluginInspect(args[1:])
	case "config":
		return runPluginConfig(args[1:])
	case "check":
		return runPluginCheck(args[1:])
	case "sync":
		return runPluginSync(args[1:])
	case "enable":
		return runPluginSetEnabled(args[1:], true)
	case "disable":
		return runPluginSetEnabled(args[1:], false)
	case "help", "-h", "--help":
		pluginUsage()
		return nil
	default:
		return fmt.Errorf("未知插件命令 %q", args[0])
	}
}

func pluginUsage() {
	fmt.Fprintln(stdout, `anybot plugin 命令：
  anybot plugin add <module[@version]> [-name 名称] [-symbol Module] [-version 版本] [-replace 本地路径] [-dir 目录]
  anybot plugin update <name> [-version 版本] [-symbol Module] [-replace 本地路径|-clear-replace] [-dir 目录]
  anybot plugin remove <name> [-dir 目录] [-config anybot.yaml]
  anybot plugin list [-dir 目录]
  anybot plugin status [-dir 目录] [-config anybot.yaml]
  anybot plugin inspect <name> [-dir 目录] [-config anybot.yaml]
  anybot plugin config <name> <key=value>... [-dir 目录] [-config anybot.yaml]
  anybot plugin check [-dir 目录] [-config anybot.yaml]
  anybot plugin sync [-dir 目录] [-config anybot.yaml]
  anybot plugin enable <name> [-dir 目录] [-config anybot.yaml]
  anybot plugin disable <name> [-dir 目录] [-config anybot.yaml]`)
}

func runPluginAdd(args []string) error {
	fs := flag.NewFlagSet("plugin add", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	name := fs.String("name", "", "插件配置名称")
	symbol := fs.String("symbol", "Module", "插件模块导出的 Spec 变量名")
	version := fs.String("version", "", "插件模块版本")
	replace := fs.String("replace", "", "本地模块替换路径")
	module, flagArgs, err := splitPluginAddArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if module == "" {
		return fmt.Errorf("用法：anybot plugin add <module[@version]> [-name 名称] [-symbol Module] [-version 版本] [-replace 本地路径] [-dir 目录]")
	}
	modulePath, moduleVersion, err := host.ParsePluginModuleSpec(module)
	if err != nil {
		return err
	}
	if *version != "" && moduleVersion != "" && *version != moduleVersion {
		return fmt.Errorf("插件版本冲突：%s 与 %s", moduleVersion, *version)
	}
	if *version == "" {
		*version = moduleVersion
	}
	replacePath, err := normalizeReplacePath(*dir, *replace)
	if err != nil {
		return err
	}
	pluginName := *name
	if pluginName == "" {
		pluginName = host.DefaultPluginName(modulePath)
	}
	if _, exists := host.DefaultRegistry().Factory(pluginName); exists {
		return fmt.Errorf("插件名 %q 已被内置插件占用；请使用 -name 指定其他名称", pluginName)
	}
	pinnedVersion, err := pinPluginModuleVersion(modulePath, *version, replacePath)
	if err != nil {
		return err
	}
	*version = pinnedVersion
	workspace, err := host.AddPluginModule(host.AddPluginOptions{
		Dir:     *dir,
		Name:    *name,
		Module:  modulePath,
		Version: *version,
		Replace: replacePath,
		Symbol:  *symbol,
	})
	if err != nil {
		return err
	}
	var item host.PluginModule
	for _, plugin := range workspace.Plugins {
		if plugin.Module == modulePath {
			item = plugin
			break
		}
	}
	if err := syncPluginGoMod(*dir, item); err != nil {
		return err
	}
	configPath := filepath.Join(*dir, "anybot.yaml")
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		if err := writeFile(configPath, defaultConfig, false); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	changed, err := host.EnsurePluginConfigEntry(configPath, item.Name)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "插件已添加：%s (%s.%s)\n", item.Name, pluginModuleRef(item), item.Symbol)
	if changed {
		fmt.Fprintf(stdout, "配置已添加：%s（默认禁用）\n", item.Name)
	}
	printNextSteps(*dir, "anybot plugin enable "+item.Name, "anybot up")
	return nil
}

func runPluginUpdate(args []string) error {
	fs := flag.NewFlagSet("plugin update", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	version := fs.String("version", "", "插件模块版本")
	symbol := fs.String("symbol", "", "插件模块导出的 Spec 变量名")
	replace := fs.String("replace", "", "本地模块替换路径")
	clearReplace := fs.Bool("clear-replace", false, "清除本地模块替换路径")
	name, flagArgs, err := splitPluginUpdateArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("用法：anybot plugin update <name> [-version 版本] [-symbol Module] [-replace 本地路径|-clear-replace] [-dir 目录]")
	}
	var setVersion, setSymbol, setReplace bool
	fs.Visit(func(flag *flag.Flag) {
		switch flag.Name {
		case "version":
			setVersion = true
		case "symbol":
			setSymbol = true
		case "replace":
			setReplace = true
		}
	})
	replacePath := *replace
	if setReplace {
		replacePath, err = normalizeReplacePath(*dir, *replace)
		if err != nil {
			return err
		}
	}
	target, err := previewPluginModuleUpdate(host.UpdatePluginOptions{
		Dir:          *dir,
		Name:         name,
		Version:      *version,
		Replace:      replacePath,
		Symbol:       *symbol,
		SetVersion:   setVersion,
		SetReplace:   setReplace,
		ClearReplace: *clearReplace,
		SetSymbol:    setSymbol,
	})
	if err != nil {
		return err
	}
	updateVersion := *version
	updateSetVersion := setVersion
	pinnedVersion, err := pinPluginModuleVersion(target.Module, target.Version, target.Replace)
	if err != nil {
		return err
	}
	if pinnedVersion != target.Version {
		updateVersion = pinnedVersion
		updateSetVersion = true
	}
	updated, _, changed, err := host.UpdatePluginModule(host.UpdatePluginOptions{
		Dir:          *dir,
		Name:         name,
		Version:      updateVersion,
		Replace:      replacePath,
		Symbol:       *symbol,
		SetVersion:   updateSetVersion,
		SetReplace:   setReplace,
		ClearReplace: *clearReplace,
		SetSymbol:    setSymbol,
	})
	if err != nil {
		return err
	}
	if err := syncPluginGoMod(*dir, updated); err != nil {
		return err
	}
	if changed {
		fmt.Fprintf(stdout, "插件已更新：%s (%s.%s)\n", updated.Name, pluginModuleRef(updated), updated.Symbol)
	} else {
		fmt.Fprintf(stdout, "插件配置未变化：%s (%s.%s)\n", updated.Name, pluginModuleRef(updated), updated.Symbol)
	}
	return nil
}

func runPluginRemove(args []string) error {
	fs := flag.NewFlagSet("plugin remove", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	config := fs.String("config", "", "配置文件")
	name, flagArgs, err := splitPluginConfigArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("用法：anybot plugin remove <name> [-dir 目录] [-config anybot.yaml]")
	}
	if _, exists := host.DefaultRegistry().Factory(name); exists {
		return fmt.Errorf("内置插件 %q 不能移除；请使用 anybot plugin disable %s", name, name)
	}
	removed, _, err := host.RemovePluginModule(host.RemovePluginOptions{Dir: *dir, Name: name})
	if err != nil {
		return err
	}
	if err := dropPluginGoMod(*dir, removed); err != nil {
		return err
	}
	configPath := *config
	if configPath == "" {
		configPath = filepath.Join(*dir, "anybot.yaml")
	}
	changed := false
	if _, err := os.Stat(configPath); err == nil {
		changed, err = host.RemovePluginConfigEntry(configPath, removed.Name)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Fprintf(stdout, "插件已移除：%s (%s.%s)\n", removed.Name, pluginModuleRef(removed), removed.Symbol)
	if changed {
		fmt.Fprintf(stdout, "配置已移除：%s\n", removed.Name)
	}
	return nil
}

func previewPluginModuleUpdate(opts host.UpdatePluginOptions) (host.PluginModule, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Name == "" {
		return host.PluginModule{}, fmt.Errorf("plugin name is required")
	}
	if opts.SetReplace && opts.ClearReplace {
		return host.PluginModule{}, fmt.Errorf("-replace and -clear-replace cannot be used together")
	}
	workspace, err := host.LoadPluginWorkspace(filepath.Join(opts.Dir, host.PluginWorkspaceFile))
	if err != nil {
		return host.PluginModule{}, err
	}
	var updated host.PluginModule
	found := false
	for _, item := range workspace.Plugins {
		if item.Name == opts.Name {
			updated = item
			found = true
			break
		}
	}
	if !found {
		return host.PluginModule{}, fmt.Errorf("plugin %s not found", opts.Name)
	}
	if opts.SetVersion {
		updated.Version = strings.TrimSpace(opts.Version)
	}
	if opts.SetReplace {
		updated.Replace = strings.TrimSpace(opts.Replace)
	}
	if opts.ClearReplace {
		updated.Replace = ""
	}
	if opts.SetSymbol {
		updated.Symbol = strings.TrimSpace(opts.Symbol)
		if updated.Symbol == "" {
			updated.Symbol = "Module"
		}
	}
	return updated, nil
}

func pinPluginModuleVersion(module, version, replace string) (string, error) {
	version = strings.TrimSpace(version)
	if strings.TrimSpace(replace) != "" {
		if pluginVersionNeedsResolution(module, version) {
			return "", nil
		}
		return version, nil
	}
	if !pluginVersionNeedsResolution(module, version) {
		return version, nil
	}
	query := version
	if query == "" {
		query = "latest"
	}
	resolved, err := moduleVersionResolver(module, query)
	if err != nil {
		return "", fmt.Errorf("解析插件版本 %s@%s 失败: %w", module, query, err)
	}
	return resolved, nil
}

func pluginVersionNeedsResolution(module, version string) bool {
	version = strings.TrimSpace(version)
	return !pinnedPluginVersion(module, version)
}

func pinnedPluginVersion(module, version string) bool {
	if version == "" || version == "latest" {
		return false
	}
	if err := modmodule.Check(module, version); err != nil {
		return false
	}
	core := version
	if i := strings.IndexAny(core, "-+"); i >= 0 {
		core = core[:i]
	}
	return strings.Count(core, ".") == 2
}

func splitPluginAddArgs(args []string) (string, []string, error) {
	var module string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir" ||
			arg == "-name" || arg == "--name" ||
			arg == "-symbol" || arg == "--symbol" ||
			arg == "-version" || arg == "--version" ||
			arg == "-replace" || arg == "--replace":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s 需要值", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		case strings.HasPrefix(arg, "-dir=") || strings.HasPrefix(arg, "--dir=") ||
			strings.HasPrefix(arg, "-name=") || strings.HasPrefix(arg, "--name=") ||
			strings.HasPrefix(arg, "-symbol=") || strings.HasPrefix(arg, "--symbol=") ||
			strings.HasPrefix(arg, "-version=") || strings.HasPrefix(arg, "--version=") ||
			strings.HasPrefix(arg, "-replace=") || strings.HasPrefix(arg, "--replace="):
			flagArgs = append(flagArgs, arg)
		case strings.HasPrefix(arg, "-"):
			flagArgs = append(flagArgs, arg)
		default:
			if module != "" {
				return "", nil, fmt.Errorf("只能指定一个插件模块")
			}
			module = arg
		}
	}
	return module, flagArgs, nil
}

func splitPluginUpdateArgs(args []string) (string, []string, error) {
	var name string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir" ||
			arg == "-version" || arg == "--version" ||
			arg == "-symbol" || arg == "--symbol" ||
			arg == "-replace" || arg == "--replace":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s 需要值", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		case arg == "-clear-replace" || arg == "--clear-replace":
			flagArgs = append(flagArgs, arg)
		case strings.HasPrefix(arg, "-dir=") || strings.HasPrefix(arg, "--dir=") ||
			strings.HasPrefix(arg, "-version=") || strings.HasPrefix(arg, "--version=") ||
			strings.HasPrefix(arg, "-symbol=") || strings.HasPrefix(arg, "--symbol=") ||
			strings.HasPrefix(arg, "-replace=") || strings.HasPrefix(arg, "--replace="):
			flagArgs = append(flagArgs, arg)
		case strings.HasPrefix(arg, "-"):
			flagArgs = append(flagArgs, arg)
		default:
			if name != "" {
				return "", nil, fmt.Errorf("只能指定一个插件名")
			}
			name = arg
		}
	}
	return name, flagArgs, nil
}

func normalizeReplacePath(dir, replace string) (string, error) {
	replace = strings.TrimSpace(replace)
	if replace == "" {
		return "", nil
	}
	if dir == "" {
		dir = "."
	}
	targetDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	replacePath := replace
	if !filepath.IsAbs(replacePath) {
		replacePath, err = filepath.Abs(replacePath)
		if err != nil {
			return "", err
		}
	}
	rel, err := filepath.Rel(targetDir, replacePath)
	if err != nil {
		return replace, nil
	}
	return filepath.ToSlash(rel), nil
}

func runPluginList(args []string) error {
	fs := flag.NewFlagSet("plugin list", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	if err := fs.Parse(args); err != nil {
		return err
	}
	workspace, err := host.LoadPluginWorkspace(filepath.Join(*dir, host.PluginWorkspaceFile))
	if err != nil {
		return err
	}
	if len(workspace.Plugins) == 0 {
		fmt.Fprintln(stdout, "外部插件：无")
		return nil
	}
	for _, item := range workspace.Plugins {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", item.Name, pluginModuleRef(item), item.Symbol)
	}
	return nil
}

func pluginModuleRef(item host.PluginModule) string {
	out := item.Module
	if item.Version != "" {
		out += "@" + item.Version
	}
	if item.Replace != "" {
		out += " => " + item.Replace
	}
	return out
}

func runPluginStatus(args []string) error {
	fs := flag.NewFlagSet("plugin status", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	config := fs.String("config", "", "配置文件")
	if err := fs.Parse(args); err != nil {
		return err
	}
	configPath := *config
	if configPath == "" {
		configPath = filepath.Join(*dir, "anybot.yaml")
	}
	cfg, err := host.LoadConfig(configPath)
	if err != nil {
		return err
	}
	workspace, err := host.LoadPluginWorkspace(filepath.Join(*dir, host.PluginWorkspaceFile))
	if err != nil {
		return err
	}
	statuses := host.PluginStatuses(cfg, host.DefaultRegistry(), workspace)
	return host.WritePluginStatusTable(stdout, statuses)
}

func runPluginInspect(args []string) error {
	fs := flag.NewFlagSet("plugin inspect", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	config := fs.String("config", "", "配置文件")
	name, flagArgs, err := splitPluginConfigArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("用法：anybot plugin inspect <name> [-dir 目录] [-config anybot.yaml]")
	}
	configPath := *config
	if configPath == "" {
		configPath = filepath.Join(*dir, "anybot.yaml")
	}
	workspace, err := host.LoadPluginWorkspace(filepath.Join(*dir, host.PluginWorkspaceFile))
	if err != nil {
		return err
	}
	inspect, err := host.InspectPlugin(configPath, host.DefaultRegistry(), workspace, name)
	if err != nil {
		return err
	}
	return host.WritePluginInspect(stdout, inspect)
}

func runPluginConfig(args []string) error {
	fs := flag.NewFlagSet("plugin config", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	config := fs.String("config", "", "配置文件")
	name, assignmentArgs, flagArgs, err := splitPluginConfigSetArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if name == "" || len(assignmentArgs) == 0 {
		return fmt.Errorf("用法：anybot plugin config <name> <key=value>... [-dir 目录] [-config anybot.yaml]")
	}
	configPath := *config
	if configPath == "" {
		configPath = filepath.Join(*dir, "anybot.yaml")
	}
	if err := ensureKnownPluginTarget(configPath, filepath.Join(*dir, host.PluginWorkspaceFile), name, true); err != nil {
		return err
	}
	assignments := make([]host.PluginConfigAssignment, 0, len(assignmentArgs))
	for _, arg := range assignmentArgs {
		assignment, err := host.ParsePluginConfigAssignment(arg)
		if err != nil {
			return err
		}
		assignments = append(assignments, assignment)
	}
	changed, err := host.SetPluginConfigValues(configPath, name, assignments)
	if err != nil {
		return err
	}
	if changed {
		fmt.Fprintf(stdout, "插件配置已更新：%s（%d 项）\n", name, len(assignments))
	} else {
		fmt.Fprintf(stdout, "插件配置未变化：%s\n", name)
	}
	return nil
}

func runPluginSync(args []string) error {
	fs := flag.NewFlagSet("plugin sync", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	config := fs.String("config", "", "配置文件")
	if err := fs.Parse(args); err != nil {
		return err
	}
	configPath := *config
	if configPath == "" {
		configPath = filepath.Join(*dir, "anybot.yaml")
	}
	workspace, err := host.LoadPluginWorkspace(filepath.Join(*dir, host.PluginWorkspaceFile))
	if err != nil {
		return err
	}
	result, err := host.SyncPluginConfigEntriesForWorkspace(configPath, host.DefaultRegistry(), workspace)
	if err != nil {
		return err
	}
	return host.WritePluginConfigSyncSummary(stdout, result)
}

func runPluginCheck(args []string) error {
	fs := flag.NewFlagSet("plugin check", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	config := fs.String("config", "", "配置文件")
	if err := fs.Parse(args); err != nil {
		return err
	}
	configPath := *config
	if configPath == "" {
		configPath = filepath.Join(*dir, "anybot.yaml")
	}
	cfg, err := host.LoadConfig(configPath)
	if err != nil {
		return err
	}
	workspace, err := host.LoadPluginWorkspace(filepath.Join(*dir, host.PluginWorkspaceFile))
	if err != nil {
		return err
	}
	checks := host.PluginConfigChecks(cfg, host.DefaultRegistry(), workspace)
	if err := host.WritePluginConfigCheckTable(stdout, checks); err != nil {
		return err
	}
	if host.PluginConfigCheckFailed(checks) {
		return fmt.Errorf("插件配置检查失败")
	}
	return nil
}

func runPluginSetEnabled(args []string, enabled bool) error {
	fs := flag.NewFlagSet("plugin enabled", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	config := fs.String("config", "", "配置文件")
	name, flagArgs, err := splitPluginConfigArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if name == "" {
		action := "enable"
		if !enabled {
			action = "disable"
		}
		return fmt.Errorf("用法：anybot plugin %s <name> [-dir 目录] [-config anybot.yaml]", action)
	}
	configPath := *config
	if configPath == "" {
		configPath = filepath.Join(*dir, "anybot.yaml")
	}
	if err := ensureKnownPluginTarget(configPath, filepath.Join(*dir, host.PluginWorkspaceFile), name, !enabled); err != nil {
		return err
	}
	changed, err := host.SetPluginEnabled(configPath, name, enabled)
	if err != nil {
		return err
	}
	action := "启用"
	if !enabled {
		action = "禁用"
	}
	if changed {
		fmt.Fprintf(stdout, "插件已%s：%s\n", action, name)
	} else {
		fmt.Fprintf(stdout, "插件已处于%s状态：%s\n", action, name)
	}
	if enabled {
		if err := syncEnabledPluginDefaults(configPath, filepath.Join(*dir, host.PluginWorkspaceFile), name); err != nil {
			return err
		}
	}
	return nil
}

func syncEnabledPluginDefaults(configPath, workspacePath, name string) error {
	result, err := host.SyncPluginConfigEntry(configPath, host.DefaultRegistry(), name)
	if err != nil {
		return err
	}
	if result.Changed {
		fmt.Fprintln(stdout, "默认配置已同步：1 项更新")
		return nil
	}
	if result.Available {
		return nil
	}
	workspace, err := host.LoadPluginWorkspace(workspacePath)
	if err != nil {
		return err
	}
	for _, item := range workspace.Plugins {
		if item.Name == name {
			fmt.Fprintf(stdout, "默认配置待构建同步：%s（anybot up 会完成）\n", name)
			break
		}
	}
	return nil
}

func ensureKnownPluginTarget(configPath, workspacePath, name string, allowConfigured bool) error {
	cfg, err := host.LoadConfig(configPath)
	if err != nil {
		return err
	}
	if _, ok := cfg.Plugins[name]; ok && allowConfigured {
		return nil
	}
	if _, ok := host.DefaultRegistry().Factory(name); ok {
		return nil
	}
	workspace, err := host.LoadPluginWorkspace(workspacePath)
	if err != nil {
		return err
	}
	for _, item := range workspace.Plugins {
		if item.Name == name {
			return nil
		}
	}
	return fmt.Errorf("未知插件 %q；请先用 anybot plugins 查看内置插件，或用 anybot plugin add <module> 添加外部插件", name)
}

func splitPluginConfigArgs(args []string) (string, []string, error) {
	var name string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir" || arg == "-config" || arg == "--config":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s 需要值", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		case strings.HasPrefix(arg, "-dir=") || strings.HasPrefix(arg, "--dir=") ||
			strings.HasPrefix(arg, "-config=") || strings.HasPrefix(arg, "--config="):
			flagArgs = append(flagArgs, arg)
		case strings.HasPrefix(arg, "-"):
			flagArgs = append(flagArgs, arg)
		default:
			if name != "" {
				return "", nil, fmt.Errorf("只能指定一个插件名")
			}
			name = arg
		}
	}
	return name, flagArgs, nil
}

func splitPluginConfigSetArgs(args []string) (string, []string, []string, error) {
	var name string
	var assignments []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir" || arg == "-config" || arg == "--config":
			if i+1 >= len(args) {
				return "", nil, nil, fmt.Errorf("%s 需要值", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		case strings.HasPrefix(arg, "-dir=") || strings.HasPrefix(arg, "--dir=") ||
			strings.HasPrefix(arg, "-config=") || strings.HasPrefix(arg, "--config="):
			flagArgs = append(flagArgs, arg)
		default:
			if name == "" {
				name = arg
				continue
			}
			assignments = append(assignments, arg)
		}
	}
	return name, assignments, flagArgs, nil
}
