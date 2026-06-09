package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/internal/scaffold"
)

func runDev(args []string) error {
	if len(args) == 0 {
		devUsage()
		return nil
	}
	switch args[0] {
	case "init":
		return runDevInit(args[1:])
	case "plugin":
		return runDevPlugin(args[1:])
	case "new":
		return runDevNew(args[1:])
	case "doctor":
		return runDevDoctor(args[1:])
	case "run":
		return runDevRun(args[1:])
	case "help", "-h", "--help":
		devUsage()
		return nil
	default:
		return fmt.Errorf("未知开发者命令 %q", args[0])
	}
}

func devUsage() {
	fmt.Fprintln(stdout, `anybot dev 命令：
  anybot dev init [-module 模块名] [-dir 目录] [-force]
  anybot dev plugin <名称> [-dir 目录] [-force] [-module 插件模块] [-anybot-version 版本] [-replace AnyBot源码路径]
  anybot dev plugin <名称> -in-project [-dir 目录] [-force]
  anybot dev new plugin <名称> [同 anybot dev plugin]
  anybot dev doctor [-config core.yaml] [-connect]
  anybot dev run [go run 参数...]`)
}

func runDevInit(args []string) error {
	fs := flag.NewFlagSet("dev init", flag.ContinueOnError)
	module := fs.String("module", "example.com/bot", "Go 模块路径")
	dir := fs.String("dir", ".", "目标目录")
	force := fs.Bool("force", false, "覆盖已有文件")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := scaffold.InitProject(scaffold.ProjectOptions{Dir: *dir, Module: *module, Force: *force}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "已生成核心库项目：%s\n", cleanDisplayDir(*dir))
	printNextSteps(*dir, "go mod tidy", "anybot dev doctor", "go run .")
	return nil
}

func runDevNew(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法：anybot dev new plugin <名称>")
	}
	switch args[0] {
	case "plugin":
		return runDevPlugin(args[1:])
	default:
		return fmt.Errorf("未知开发者生成器 %q", args[0])
	}
}

func runDevPlugin(args []string) error {
	fs := flag.NewFlagSet("dev plugin", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标项目目录")
	module := fs.String("module", "", "独立插件 Go 模块路径；为空时使用 example.com/anybot-plugin/<名称>")
	anybotVersion := fs.String("anybot-version", "", "独立插件依赖的 AnyBot 版本")
	replace := fs.String("replace", "", "独立插件 go.mod 中的 AnyBot 本地源码替换路径")
	inProject := fs.Bool("in-project", false, "生成到现有核心库项目的 plugins/ 目录")
	force := fs.Bool("force", false, "覆盖已有文件")
	name, flagArgs, err := splitDevPluginArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("用法：anybot dev plugin <名称> [-dir 目录] [-module 插件模块] [-anybot-version 版本] [-replace AnyBot源码路径]，或 anybot dev plugin <名称> -in-project")
	}
	if *inProject && strings.TrimSpace(*module) != "" {
		return fmt.Errorf("-in-project 不能与 -module 同时使用")
	}
	if !*inProject && strings.TrimSpace(*module) == "" {
		*module = defaultPluginModule(name)
	}
	opts := scaffold.PluginOptions{
		Dir:           *dir,
		Name:          name,
		Module:        *module,
		AnyBotVersion: *anybotVersion,
		AnyBotReplace: *replace,
		Force:         *force,
	}
	if err := fillDevPluginDependency(&opts); err != nil {
		return err
	}
	result, err := scaffold.NewPlugin(opts)
	if err != nil {
		return err
	}
	if result.Standalone {
		fmt.Fprintf(stdout, "已生成独立插件模块：%s (%s)\n", result.Name, result.Module)
		printStandalonePluginNextSteps(*dir, result)
		return nil
	}
	fmt.Fprintf(stdout, "已生成插件骨架：%s\n", result.Name)
	printNextSteps(*dir, "在 Go 入口中 import ./plugins/"+result.Package+" 并使用 absdk.InstallDefault(app, "+result.Package+".Plugin)", "go test ./...")
	return nil
}

func defaultPluginModule(name string) string {
	suffix := modulePathName(name)
	if suffix == "" {
		suffix = "plugin"
	}
	return "example.com/anybot-plugin/" + suffix
}

func modulePathName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastSep := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastSep = false
		case r >= '0' && r <= '9':
			if b.Len() == 0 {
				b.WriteString("plugin-")
			}
			b.WriteRune(r)
			lastSep = false
		default:
			if b.Len() > 0 && !lastSep {
				b.WriteByte('-')
				lastSep = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func printStandalonePluginNextSteps(dir string, result scaffold.PluginResult) {
	fmt.Fprintln(stdout, "测试插件：")
	if clean := cleanDisplayDir(dir); clean != "." {
		fmt.Fprintf(stdout, "  cd %s\n", shellQuote(clean))
	}
	if result.TestReady {
		fmt.Fprintln(stdout, "  go test ./...")
	} else {
		fmt.Fprintln(stdout, "  go mod tidy")
		fmt.Fprintln(stdout, "  go test ./...")
	}
	if absDir, err := filepath.Abs(dir); err == nil {
		fmt.Fprintln(stdout, "在机器人工作目录中安装本地插件：")
		fmt.Fprintf(stdout, "  anybot plugin add %s -replace %s -dir <机器人工作目录>\n", result.Module, shellQuote(absDir))
		fmt.Fprintln(stdout, "  anybot plugin status -dir <机器人工作目录>")
		fmt.Fprintln(stdout, "  anybot plugin enable <id> -dir <机器人工作目录>")
		fmt.Fprintln(stdout, "  anybot run -dir <机器人工作目录>")
	}
}

func fillDevPluginDependency(opts *scaffold.PluginOptions) error {
	if opts == nil || strings.TrimSpace(opts.Module) == "" {
		return nil
	}
	opts.AnyBotVersion = strings.TrimSpace(opts.AnyBotVersion)
	opts.AnyBotReplace = strings.TrimSpace(opts.AnyBotReplace)
	if opts.AnyBotVersion == "" && opts.AnyBotReplace == "" {
		dep := devPluginFrameworkDependency("github.com/tty00a381/anybot")
		opts.AnyBotVersion = dep.Version
		opts.AnyBotReplace = dep.Replace
	}
	if opts.AnyBotReplace == "" {
		if opts.AnyBotVersion == "" || opts.AnyBotVersion == "v0.0.0" {
			return fmt.Errorf("生成独立插件需要可解析的 AnyBot 版本；请传 -anybot-version vX.Y.Z，或传 -replace /path/to/anybot 使用本地源码")
		}
		return nil
	}
	replace, err := normalizeReplacePath(opts.Dir, opts.AnyBotReplace)
	if err != nil {
		return err
	}
	opts.AnyBotReplace = replace
	return nil
}

func devPluginFrameworkDependency(module string) moduleDependency {
	for _, dep := range frameworkDependencies() {
		if dep.Module == module {
			return dep
		}
	}
	return detectFrameworkDependency(module)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$`!*?[]{}()<>|&;") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func splitDevPluginArgs(args []string) (string, []string, error) {
	var name string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir" ||
			arg == "-module" || arg == "--module" ||
			arg == "-anybot-version" || arg == "--anybot-version" ||
			arg == "-replace" || arg == "--replace":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s 需要值", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		case strings.HasPrefix(arg, "-dir=") || strings.HasPrefix(arg, "--dir=") ||
			strings.HasPrefix(arg, "-module=") || strings.HasPrefix(arg, "--module=") ||
			strings.HasPrefix(arg, "-anybot-version=") || strings.HasPrefix(arg, "--anybot-version=") ||
			strings.HasPrefix(arg, "-replace=") || strings.HasPrefix(arg, "--replace="):
			flagArgs = append(flagArgs, arg)
		case arg == "-force" || arg == "--force" ||
			arg == "-in-project" || arg == "--in-project":
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

func runDevDoctor(args []string) error {
	fs := flag.NewFlagSet("dev doctor", flag.ContinueOnError)
	configPath := fs.String("config", "core.yaml", "配置文件")
	connect := fs.Bool("connect", false, "检查远端动作接口是否可连接")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := onebot11.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := checkListen(cfg); err != nil {
		return err
	}
	for _, warning := range doctorWarnings(cfg) {
		fmt.Fprintf(stderr, "警告：%s\n", warning)
	}
	if *connect {
		if err := checkRemote(cfg); err != nil {
			return err
		}
	}
	abs, _ := filepath.Abs(*configPath)
	printDevDoctorSummary(abs, cfg)
	fmt.Fprintf(stdout, "配置可用：%s\n", abs)
	return nil
}

func printDevDoctorSummary(path string, cfg onebot11.Config) {
	fmt.Fprintf(stdout, "配置文件：%s\n", path)
	fmt.Fprintf(stdout, "协议：%s\n", cfg.Protocol)
	fmt.Fprintf(stdout, "传输：%s\n", cfg.Transport.Type)
	switch cfg.Transport.Type {
	case "reverse_ws":
		fmt.Fprintf(stdout, "监听：%s\n", cfg.Transport.Listen)
		fmt.Fprintf(stdout, "路径：%s\n", doctorPath(cfg.Transport.Path))
	case "http", "websocket":
		fmt.Fprintf(stdout, "URL：%s\n", cfg.Transport.URL)
	}
}

func runDevRun(args []string) error {
	cmdArgs := append([]string{"run", "."}, args...)
	return commandRunner(".", "go", cmdArgs...)
}
