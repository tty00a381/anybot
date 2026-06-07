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
		return runHostPluginCommand(args)
	case "inspect":
		return runHostPluginCommand(args)
	case "config":
		return runHostPluginCommand(args)
	case "check":
		return runHostPluginCommand(args)
	case "sync":
		return runHostPluginCommand(args)
	case "enable":
		return runHostPluginCommand(args)
	case "disable":
		return runHostPluginCommand(args)
	case "help", "-h", "--help":
		pluginUsage()
		return nil
	default:
		return fmt.Errorf("未知插件命令 %q", args[0])
	}
}

func pluginUsage() {
	fmt.Fprintln(stdout, `anybot plugin 命令：
  anybot plugin add <module[@version]> [-name 名称] [-symbol Plugin] [-version 版本] [-replace 本地路径] [-dir 目录]
  anybot plugin update <name> [-version 版本] [-symbol Plugin] [-replace 本地路径|-clear-replace] [-dir 目录]
  anybot plugin remove <name> [-dir 目录] [-config anybot.yaml]
  anybot plugin list [-dir 目录]
  anybot plugin status [-dir 目录] [-config anybot.yaml]
  anybot plugin inspect <name> [-dir 目录] [-config anybot.yaml]
  anybot plugin config <name> <key=value>... [-dir 目录] [-config anybot.yaml]
  anybot plugin config <name> -reset <key>... [-dir 目录] [-config anybot.yaml]
  anybot plugin check [-dir 目录] [-config anybot.yaml]
  anybot plugin sync [-dir 目录] [-config anybot.yaml]
  anybot plugin enable <name> [-dir 目录] [-config anybot.yaml]
  anybot plugin disable <name> [-dir 目录] [-config anybot.yaml]`)
}

func runPluginAdd(args []string) (err error) {
	fs := flag.NewFlagSet("plugin add", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	name := fs.String("name", "", "插件配置名称")
	symbol := fs.String("symbol", "Plugin", "插件模块导出的 Definition 变量名")
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
		return fmt.Errorf("用法：anybot plugin add <module[@version]> [-name 名称] [-symbol Plugin] [-version 版本] [-replace 本地路径] [-dir 目录]")
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
	if err := host.ValidatePluginName(pluginName); err != nil {
		return err
	}
	pinnedVersion, err := pinPluginModuleVersion(modulePath, *version, replacePath)
	if err != nil {
		return err
	}
	*version = pinnedVersion
	rollback, err := snapshotFiles(pluginAddTouchedFiles(*dir, pluginName))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		joinRollbackError(&err, committed, rollback)
	}()
	workspace, err := host.AddPluginModule(host.AddPluginOptions{
		Dir:     *dir,
		Name:    pluginName,
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
	committed = true
	return nil
}

func pluginAddTouchedFiles(dir, name string) []string {
	if dir == "" {
		dir = "."
	}
	return append(pluginWorkspaceTouchedFiles(dir), pluginConfigTouchedFiles(filepath.Join(dir, "anybot.yaml"), name)...)
}

func pluginWorkspaceTouchedFiles(dir string) []string {
	if dir == "" {
		dir = "."
	}
	return []string{
		filepath.Join(dir, host.PluginWorkspaceFile),
		filepath.Join(dir, "plugins.gen.go"),
		filepath.Join(dir, "main.go"),
		filepath.Join(dir, "go.mod"),
	}
}

func pluginConfigTouchedFiles(configPath, name string) []string {
	files := []string{configPath}
	if name == "" {
		return files
	}
	base := filepath.Dir(configPath)
	if base == "" {
		base = "."
	}
	files = append(files, filepath.Join(base, "plugins.d"), filepath.Join(base, "plugins.d", name+".yaml"))
	if _, err := os.Stat(configPath); err != nil {
		return files
	}
	cfg, err := host.LoadConfig(configPath)
	if err != nil || cfg.PluginConfigDir == "" {
		return files
	}
	dir := cfg.PluginConfigDir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(base, dir)
	}
	files = append(files, dir, filepath.Join(dir, name+".yaml"))
	return files
}

func snapshotFiles(paths []string) (func() error, error) {
	type snapshot struct {
		path   string
		data   []byte
		mode   os.FileMode
		exists bool
		dir    bool
	}
	snapshots := make([]snapshot, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		item := snapshot{path: clean}
		info, err := os.Stat(clean)
		if errors.Is(err, os.ErrNotExist) {
			snapshots = append(snapshots, item)
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			item.exists = true
			item.dir = true
			item.mode = info.Mode().Perm()
			snapshots = append(snapshots, item)
			continue
		}
		item.exists = true
		item.mode = info.Mode().Perm()
		item.data, err = os.ReadFile(clean)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, item)
	}
	return func() error {
		var first error
		for i := len(snapshots) - 1; i >= 0; i-- {
			item := snapshots[i]
			if !item.exists {
				if err := removeCreatedPath(item.path); err != nil && first == nil {
					first = err
				}
				continue
			}
			if item.dir {
				if err := os.MkdirAll(item.path, item.mode); err != nil {
					if first == nil {
						first = err
					}
					continue
				}
				if err := os.Chmod(item.path, item.mode); err != nil && first == nil {
					first = err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(item.path), 0o755); err != nil {
				if first == nil {
					first = err
				}
				continue
			}
			if err := os.WriteFile(item.path, item.data, item.mode); err != nil && first == nil {
				first = err
			}
		}
		return first
	}, nil
}

func joinRollbackError(result *error, committed bool, rollback func() error) {
	if committed || rollback == nil {
		return
	}
	if err := rollback(); err != nil {
		*result = errors.Join(*result, fmt.Errorf("回滚失败: %w", err))
	}
}

func removeCreatedPath(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	info, statErr := os.Stat(path)
	if statErr == nil && info.IsDir() {
		return nil
	}
	return err
}

func runPluginUpdate(args []string) (err error) {
	fs := flag.NewFlagSet("plugin update", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	version := fs.String("version", "", "插件模块版本")
	symbol := fs.String("symbol", "", "插件模块导出的 Definition 变量名")
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
		return fmt.Errorf("用法：anybot plugin update <name> [-version 版本] [-symbol Plugin] [-replace 本地路径|-clear-replace] [-dir 目录]")
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
	rollback, err := snapshotFiles(pluginWorkspaceTouchedFiles(*dir))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		joinRollbackError(&err, committed, rollback)
	}()
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
	committed = true
	return nil
}

func runPluginRemove(args []string) (err error) {
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
	configPath := *config
	if configPath == "" {
		configPath = filepath.Join(*dir, "anybot.yaml")
	}
	rollback, err := snapshotFiles(append(pluginWorkspaceTouchedFiles(*dir), pluginConfigTouchedFiles(configPath, name)...))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		joinRollbackError(&err, committed, rollback)
	}()
	removed, _, err := host.RemovePluginModule(host.RemovePluginOptions{Dir: *dir, Name: name})
	if err != nil {
		return err
	}
	if err := dropPluginGoMod(*dir, removed); err != nil {
		return err
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
	committed = true
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
			updated.Symbol = "Plugin"
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
	targetDir, err := stablePath(dir)
	if err != nil {
		return "", err
	}
	replacePath, err := stablePath(replace)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(targetDir, replacePath)
	if err != nil {
		return filepath.ToSlash(replacePath), nil
	}
	return filepath.ToSlash(rel), nil
}

func stablePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	clean := filepath.Clean(abs)
	var rest []string
	for {
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, rest[i])
			}
			return resolved, nil
		}
		parent := filepath.Dir(clean)
		if parent == clean {
			return abs, nil
		}
		rest = append(rest, filepath.Base(clean))
		clean = parent
	}
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

func runHostPluginCommand(args []string) error {
	dir, configPath, commandArgs, err := parseHostPluginCommandArgs(args)
	if err != nil {
		return err
	}
	return host.RunPluginCommand(host.PluginCommandOptions{
		Args:          commandArgs,
		Output:        stdout,
		ConfigPath:    configPath,
		WorkspacePath: filepath.Join(dir, host.PluginWorkspaceFile),
		Registry:      host.DefaultRegistry(),
	})
}

func parseHostPluginCommandArgs(args []string) (dir string, configPath string, commandArgs []string, err error) {
	dir = "."
	command := ""
	seenConfigTarget := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("%s 需要值", arg)
			}
			dir = args[i+1]
			i++
		case arg == "-config" || arg == "--config":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("%s 需要值", arg)
			}
			configPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-dir="):
			dir = strings.TrimPrefix(arg, "-dir=")
		case strings.HasPrefix(arg, "--dir="):
			dir = strings.TrimPrefix(arg, "--dir=")
		case strings.HasPrefix(arg, "-config="):
			configPath = strings.TrimPrefix(arg, "-config=")
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "-reset" || arg == "--reset" || strings.HasPrefix(arg, "-reset=") || strings.HasPrefix(arg, "--reset="):
			if command == "config" && !seenConfigTarget {
				return "", "", nil, fmt.Errorf("-reset 必须写在插件名之后")
			}
			commandArgs = append(commandArgs, arg)
		default:
			if command == "" {
				command = arg
			} else if command == "config" && !strings.HasPrefix(arg, "-") && !seenConfigTarget {
				seenConfigTarget = true
			}
			commandArgs = append(commandArgs, arg)
		}
	}
	if command == "config" && (!seenConfigTarget || len(commandArgs) < 3) {
		return "", "", nil, fmt.Errorf("用法：anybot plugin config <name> <key=value>... [-dir 目录] [-config anybot.yaml]，或 anybot plugin config <name> -reset <key>...")
	}
	if configPath == "" {
		configPath = filepath.Join(dir, "anybot.yaml")
	}
	return dir, configPath, commandArgs, nil
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
