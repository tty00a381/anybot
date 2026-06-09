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
	for _, builtin := range host.BuiltinPlugins() {
		info := builtin.Factory.Info
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", builtin.Source, info.Name, info.Version, info.Description)
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
	case "status", "inspect", "config", "check", "sync", "enable", "disable":
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
  anybot plugin add <module[@version]> [-version 版本] [-replace 本地路径] [-dir 目录]
  anybot plugin update <id> [-version 版本] [-replace 本地路径|-clear-replace] [-dir 目录]
  anybot plugin remove <id> [-dir 目录|-config anybot.yaml]
  anybot plugin list [-dir 目录]
  anybot plugin status [-dir 目录|-config anybot.yaml]
  anybot plugin inspect <id> [-dir 目录|-config anybot.yaml]
  anybot plugin config <id> <key=value>... [-dir 目录|-config anybot.yaml]
  anybot plugin config <id> -reset <key>... [-dir 目录|-config anybot.yaml]
  anybot plugin check [-dir 目录|-config anybot.yaml]
  anybot plugin sync [-dir 目录|-config anybot.yaml]
  anybot plugin enable <id> [-dir 目录|-config anybot.yaml]
  anybot plugin disable <id> [-dir 目录|-config anybot.yaml]`)
}

func runPluginAdd(args []string) (err error) {
	fs := flag.NewFlagSet("plugin add", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	version := fs.String("version", "", "插件模块版本")
	replace := fs.String("replace", "", "本地模块替换路径")
	module, flagArgs, err := splitPluginModuleArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if module == "" {
		return fmt.Errorf("用法：anybot plugin add <module[@version]> [-version 版本] [-replace 本地路径] [-dir 目录]")
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
	pinnedVersion, err := pinPluginModuleVersion(modulePath, *version, replacePath)
	if err != nil {
		return err
	}
	pluginID, err := host.NewPluginID()
	if err != nil {
		return err
	}
	configPath := filepath.Join(*dir, "anybot.yaml")
	rollback, err := snapshotFiles(pluginAddTouchedFiles(*dir, configPath, pluginID))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		joinRollbackError(&err, committed, rollback)
	}()
	lock, err := host.AddPluginInstall(host.AddPluginInstallOptions{
		Dir:     *dir,
		ID:      pluginID,
		Module:  modulePath,
		Version: pinnedVersion,
		Replace: replacePath,
	})
	if err != nil {
		return err
	}
	item, ok := pluginModuleByID(lock, pluginID)
	if !ok {
		return fmt.Errorf("plugin %s not found after add", pluginID)
	}
	if err := syncPluginGoMod(*dir, item); err != nil {
		return err
	}
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		if err := writeFile(configPath, defaultConfig, false); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	changed, err := host.EnsurePluginConfigEntry(configPath, item.ID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "插件已添加：%s (%s)\n", item.ID, pluginModuleRef(item))
	if changed {
		fmt.Fprintf(stdout, "配置已添加：%s（默认禁用）\n", pluginConfigDisplayPath(configPath, item.ID))
	}
	printNextSteps(*dir, "anybot plugin enable "+host.ShortPluginID(item.ID), "anybot up")
	committed = true
	return nil
}

func runPluginUpdate(args []string) (err error) {
	fs := flag.NewFlagSet("plugin update", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	version := fs.String("version", "", "插件模块版本")
	replace := fs.String("replace", "", "本地模块替换路径")
	clearReplace := fs.Bool("clear-replace", false, "清除本地模块替换路径")
	target, flagArgs, err := splitPluginIDArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if target == "" {
		return fmt.Errorf("用法：anybot plugin update <id> [-version 版本] [-replace 本地路径|-clear-replace] [-dir 目录]")
	}
	var setVersion, setReplace bool
	fs.Visit(func(flag *flag.Flag) {
		switch flag.Name {
		case "version":
			setVersion = true
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
	targetModule, lock, err := previewPluginModuleUpdate(*dir, target, host.UpdatePluginInstallOptions{
		Version:      *version,
		Replace:      replacePath,
		SetVersion:   setVersion,
		SetReplace:   setReplace,
		ClearReplace: *clearReplace,
	})
	if err != nil {
		return err
	}
	if targetModule.Module == "" {
		return fmt.Errorf("builtin plugin %s cannot be updated as a Go module", targetModule.ID)
	}
	updateVersion := *version
	updateSetVersion := setVersion
	pinnedVersion, err := pinPluginModuleVersion(targetModule.Module, targetModule.Version, targetModule.Replace)
	if err != nil {
		return err
	}
	if pinnedVersion != targetModule.Version {
		updateVersion = pinnedVersion
		updateSetVersion = true
	}
	rollback, err := snapshotFiles(pluginHostTouchedFiles(*dir))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		joinRollbackError(&err, committed, rollback)
	}()
	updated, _, changed, err := host.UpdatePluginInstall(host.UpdatePluginInstallOptions{
		Dir:          *dir,
		ID:           targetModule.ID,
		Version:      updateVersion,
		Replace:      replacePath,
		SetVersion:   updateSetVersion,
		SetReplace:   setReplace,
		ClearReplace: *clearReplace,
	})
	if err != nil {
		return err
	}
	if err := syncPluginGoMod(*dir, updated); err != nil {
		return err
	}
	if changed {
		fmt.Fprintf(stdout, "插件已更新：%s (%s)\n", updated.ID, pluginModuleRef(updated))
	} else {
		fmt.Fprintf(stdout, "插件配置未变化：%s (%s)\n", updated.ID, pluginModuleRef(updated))
	}
	_ = lock
	committed = true
	return nil
}

func runPluginRemove(args []string) (err error) {
	fs := flag.NewFlagSet("plugin remove", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	config := fs.String("config", "", "配置文件")
	target, flagArgs, err := splitPluginIDArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if target == "" {
		return fmt.Errorf("用法：anybot plugin remove <id> [-dir 目录|-config anybot.yaml]")
	}
	cfgTarget, err := newPluginConfigTarget(*dir, *config, flagSetExplicit(fs, "dir"), flagSetExplicit(fs, "config"))
	if err != nil {
		return err
	}
	lock, err := host.LoadPluginLock(cfgTarget.lockPath)
	if err != nil {
		return err
	}
	pluginID, err := resolveLockPluginID(lock, target)
	if err != nil {
		return err
	}
	configPath := cfgTarget.configPath
	rollback, err := snapshotFiles(append(pluginHostTouchedFiles(cfgTarget.dir), pluginConfigTouchedFiles(configPath, pluginID)...))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		joinRollbackError(&err, committed, rollback)
	}()
	removed, _, err := host.RemovePluginInstall(host.RemovePluginInstallOptions{Dir: cfgTarget.dir, ID: pluginID})
	if err != nil {
		return err
	}
	if err := dropPluginGoMod(cfgTarget.dir, removed); err != nil {
		return err
	}
	changed := false
	if _, err := os.Stat(configPath); err == nil {
		changed, err = host.RemovePluginConfigEntry(configPath, removed.ID)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Fprintf(stdout, "插件已移除：%s (%s)\n", removed.ID, pluginModuleRef(removed))
	if changed {
		fmt.Fprintf(stdout, "配置已移除：%s\n", pluginConfigDisplayPath(configPath, removed.ID))
	}
	committed = true
	return nil
}

func runPluginList(args []string) error {
	fs := flag.NewFlagSet("plugin list", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	if err := fs.Parse(args); err != nil {
		return err
	}
	lock, err := host.LoadPluginLock(filepath.Join(*dir, host.PluginLockFile))
	if err != nil {
		return err
	}
	var externals []host.PluginInstall
	for _, item := range lock.Plugins {
		if item.Module != "" {
			externals = append(externals, item)
		}
	}
	if len(externals) == 0 {
		fmt.Fprintln(stdout, "外部插件：无")
		return nil
	}
	fmt.Fprintln(stdout, "ID\t模块")
	for _, item := range externals {
		fmt.Fprintf(stdout, "%s\t%s\n", host.ShortPluginID(item.ID), pluginModuleRef(item))
	}
	return nil
}

func splitPluginModuleArgs(args []string) (string, []string, error) {
	return splitPluginTargetArgs(args, map[string]bool{
		"dir":     true,
		"version": true,
		"replace": true,
	}, nil, "只能指定一个插件模块")
}

func splitPluginIDArgs(args []string) (string, []string, error) {
	return splitPluginTargetArgs(args, map[string]bool{
		"dir":     true,
		"config":  true,
		"version": true,
		"replace": true,
	}, map[string]struct{}{
		"clear-replace": {},
	}, "只能指定一个插件 ID")
}

func splitPluginTargetArgs(args []string, valueFlags map[string]bool, boolFlags map[string]struct{}, duplicateMessage string) (string, []string, error) {
	var target string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, hasValue, isFlag := splitFlagArg(arg)
		if isFlag {
			if _, ok := boolFlags[name]; ok && !hasValue {
				flagArgs = append(flagArgs, arg)
				continue
			}
			if valueFlags[name] {
				if hasValue {
					flagArgs = append(flagArgs, arg)
					continue
				}
				if i+1 >= len(args) {
					return "", nil, fmt.Errorf("%s 需要值", arg)
				}
				flagArgs = append(flagArgs, arg, args[i+1])
				i++
				continue
			}
			flagArgs = append(flagArgs, arg)
			continue
		}
		if target != "" {
			return "", nil, errors.New(duplicateMessage)
		}
		target = arg
	}
	return target, flagArgs, nil
}

func splitFlagArg(arg string) (name string, hasValue bool, ok bool) {
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		return "", false, false
	}
	trimmed := strings.TrimLeft(arg, "-")
	name, _, hasValue = strings.Cut(trimmed, "=")
	return name, hasValue, name != ""
}

func pluginAddTouchedFiles(dir, configPath, id string) []string {
	if dir == "" {
		dir = "."
	}
	return append(pluginHostTouchedFiles(dir), pluginConfigTouchedFiles(configPath, id)...)
}

func pluginHostTouchedFiles(dir string) []string {
	if dir == "" {
		dir = "."
	}
	return []string{
		filepath.Join(dir, host.PluginLockFile),
		filepath.Join(dir, "plugins.gen.go"),
		filepath.Join(dir, "main.go"),
		filepath.Join(dir, "go.mod"),
	}
}

func pluginConfigTouchedFiles(configPath, id string) []string {
	files := []string{configPath, host.PluginConfigDir(configPath)}
	if id == "" {
		return files
	}
	files = append(files,
		filepath.Join(host.PluginConfigDir(configPath), id+".yaml"),
		filepath.Join(host.PluginConfigDir(configPath), id+".yml"),
	)
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

func previewPluginModuleUpdate(dir, target string, opts host.UpdatePluginInstallOptions) (host.PluginInstall, host.PluginLock, error) {
	if dir == "" {
		dir = "."
	}
	if opts.SetReplace && opts.ClearReplace {
		return host.PluginInstall{}, host.PluginLock{}, fmt.Errorf("-replace and -clear-replace cannot be used together")
	}
	lock, err := host.LoadPluginLock(filepath.Join(dir, host.PluginLockFile))
	if err != nil {
		return host.PluginInstall{}, host.PluginLock{}, err
	}
	id, err := resolveLockPluginID(lock, target)
	if err != nil {
		return host.PluginInstall{}, host.PluginLock{}, err
	}
	updated, ok := pluginModuleByID(lock, id)
	if !ok {
		return host.PluginInstall{}, host.PluginLock{}, fmt.Errorf("plugin %s not found", id)
	}
	updated.ID = id
	if opts.SetVersion {
		updated.Version = strings.TrimSpace(opts.Version)
	}
	if opts.SetReplace {
		updated.Replace = strings.TrimSpace(opts.Replace)
	}
	if opts.ClearReplace {
		updated.Replace = ""
	}
	return updated, lock, nil
}

func resolveLockPluginID(lock host.PluginLock, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("plugin id is required")
	}
	lockIDs := make([]string, 0, len(lock.Plugins))
	for _, item := range lock.Plugins {
		if item.ID != "" {
			lockIDs = append(lockIDs, item.ID)
		}
	}
	for _, id := range lockIDs {
		if id == target {
			return id, nil
		}
	}
	matches := make([]string, 0, 4)
	for _, id := range lockIDs {
		if strings.HasPrefix(id, target) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("plugin %s not found", target)
	default:
		short := make([]string, 0, len(matches))
		for _, id := range matches {
			short = append(short, host.ShortPluginID(id))
		}
		return "", fmt.Errorf("插件 ID 前缀 %q 不唯一：%s", target, strings.Join(short, ", "))
	}
}

func pluginModuleByID(lock host.PluginLock, id string) (host.PluginInstall, bool) {
	for _, item := range lock.Plugins {
		if item.ID == id {
			return item, true
		}
	}
	return host.PluginInstall{}, false
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

func pluginModuleRef(item host.PluginInstall) string {
	if item.Module == "" && item.Builtin != "" {
		return "内置:" + item.Builtin
	}
	out := item.Module
	if item.Version != "" {
		out += "@" + item.Version
	}
	if item.Replace != "" {
		out += " => " + item.Replace
	}
	return out
}

func pluginConfigDisplayPath(configPath, id string) string {
	return filepath.ToSlash(filepath.Join(host.PluginConfigDir(configPath), id+".yaml"))
}

func runHostPluginCommand(args []string) error {
	target, commandArgs, err := parseHostPluginCommandArgs(args)
	if err != nil {
		return err
	}
	return host.RunPluginCommand(host.PluginCommandOptions{
		Args:       commandArgs,
		Output:     stdout,
		ConfigPath: target.configPath,
		LockPath:   target.lockPath,
		Registry:   host.EmptyRegistry(),
	})
}

type pluginConfigTarget struct {
	dir        string
	configPath string
	lockPath   string
}

func newPluginConfigTarget(dir, configPath string, dirSet, configSet bool) (pluginConfigTarget, error) {
	if dir == "" {
		dir = "."
	}
	if dirSet && configSet {
		return pluginConfigTarget{}, fmt.Errorf("anybot plugin 不能同时指定 -dir 和 -config")
	}
	if configPath == "" {
		configPath = filepath.Join(dir, "anybot.yaml")
	} else {
		dir = filepath.Dir(configPath)
	}
	return pluginConfigTarget{
		dir:        dir,
		configPath: configPath,
		lockPath:   filepath.Join(dir, host.PluginLockFile),
	}, nil
}

func flagSetExplicit(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			set = true
		}
	})
	return set
}

func parseHostPluginCommandArgs(args []string) (target pluginConfigTarget, commandArgs []string, err error) {
	dir := "."
	configPath := ""
	dirSet := false
	configSet := false
	command := ""
	seenConfigTarget := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir":
			if i+1 >= len(args) {
				return pluginConfigTarget{}, nil, fmt.Errorf("%s 需要值", arg)
			}
			dir = args[i+1]
			dirSet = true
			i++
		case arg == "-config" || arg == "--config":
			if i+1 >= len(args) {
				return pluginConfigTarget{}, nil, fmt.Errorf("%s 需要值", arg)
			}
			configPath = args[i+1]
			configSet = true
			i++
		case strings.HasPrefix(arg, "-dir="):
			dir = strings.TrimPrefix(arg, "-dir=")
			dirSet = true
		case strings.HasPrefix(arg, "--dir="):
			dir = strings.TrimPrefix(arg, "--dir=")
			dirSet = true
		case strings.HasPrefix(arg, "-config="):
			configPath = strings.TrimPrefix(arg, "-config=")
			configSet = true
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
			configSet = true
		case arg == "-reset" || arg == "--reset" || strings.HasPrefix(arg, "-reset=") || strings.HasPrefix(arg, "--reset="):
			if command == "config" && !seenConfigTarget {
				return pluginConfigTarget{}, nil, fmt.Errorf("-reset 必须写在插件 ID 之后")
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
		return pluginConfigTarget{}, nil, fmt.Errorf("用法：anybot plugin config <id> <key=value>... [-dir 目录|-config anybot.yaml]，或 anybot plugin config <id> -reset <key>...")
	}
	target, err = newPluginConfigTarget(dir, configPath, dirSet, configSet)
	if err != nil {
		return pluginConfigTarget{}, nil, err
	}
	return target, commandArgs, nil
}
