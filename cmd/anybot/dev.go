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
  anybot dev plugin <名称> [-dir 目录] [-force]
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
	fmt.Fprintf(stdout, "已生成 core 项目：%s\n", cleanDisplayDir(*dir))
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
	force := fs.Bool("force", false, "覆盖已有文件")
	name, flagArgs, err := splitDevPluginArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("用法：anybot dev plugin <名称>")
	}
	if err := scaffold.NewPlugin(scaffold.PluginOptions{Dir: *dir, Name: name, Force: *force}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "已生成插件骨架：%s\n", name)
	printNextSteps(*dir, "go test ./...")
	return nil
}

func splitDevPluginArgs(args []string) (string, []string, error) {
	var name string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-dir" || arg == "--dir":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s 需要值", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		case strings.HasPrefix(arg, "-dir=") || strings.HasPrefix(arg, "--dir="):
			flagArgs = append(flagArgs, arg)
		case arg == "-force" || arg == "--force":
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
