package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/tty00a381/anybot/app/host"
)

var version = "dev"
var stdout io.Writer = os.Stdout
var stderr io.Writer = os.Stderr
var commandRunner = runExternalCommand

type initFile struct {
	name    string
	content string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "run":
		return runHost(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "build":
		return runBuild(args[1:])
	case "up":
		return runUp(args[1:])
	case "plugins":
		return runPlugins(args[1:])
	case "plugin":
		return runPlugin(args[1:])
	case "dev":
		return runDev(args[1:])
	case "version":
		fmt.Fprintf(stdout, "anybot %s\n", version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("未知命令 %q", args[0])
	}
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	force := fs.Bool("force", false, "覆盖已有文件")
	if err := fs.Parse(args); err != nil {
		return err
	}
	files := []initFile{
		{name: "anybot.yaml", content: defaultConfig},
		{name: host.PluginLockFile},
		{name: ".env.example", content: "ONEBOT_ACCESS_TOKEN=\n"},
		{name: "README.md", content: defaultReadme},
	}
	if err := checkInitFiles(*dir, files, *force); err != nil {
		return err
	}
	if err := host.CheckGeneratedHostWritable(*dir, *force); err != nil {
		return err
	}
	if err := checkInitDir(*dir, "plugins.d"); err != nil {
		return err
	}
	for _, file := range files {
		if err := writeFile(filepath.Join(*dir, file.name), file.content, *force); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(*dir, "plugins.d"), 0o755); err != nil {
		return err
	}
	lock, err := host.NewDefaultPluginLock()
	if err != nil {
		return err
	}
	if err := host.SavePluginLock(filepath.Join(*dir, host.PluginLockFile), lock); err != nil {
		return err
	}
	for _, item := range lock.Plugins {
		content := defaultBuiltinPluginConfig(item.Builtin)
		if content == "" {
			continue
		}
		if err := writeFile(filepath.Join(*dir, "plugins.d", item.ID+".yaml"), content, *force); err != nil {
			return err
		}
	}
	if _, err := host.EnsurePluginHostForce(*dir, *force); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "已初始化 AnyBot 工作目录：%s\n", cleanDisplayDir(*dir))
	printNextSteps(*dir, "anybot doctor", "anybot run")
	return nil
}

func cleanDisplayDir(dir string) string {
	if dir == "" {
		return "."
	}
	return filepath.Clean(dir)
}

func printNextSteps(dir string, commands ...string) {
	fmt.Fprintln(stdout, "下一步：")
	if clean := cleanDisplayDir(dir); clean != "." {
		fmt.Fprintf(stdout, "  cd %s\n", clean)
	}
	for _, command := range commands {
		if command != "" {
			fmt.Fprintf(stdout, "  %s\n", command)
		}
	}
}

func checkInitFiles(dir string, files []initFile, force bool) error {
	if force {
		return nil
	}
	for _, file := range files {
		path := filepath.Join(dir, file.name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在；使用 -force 覆盖", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func checkInitDir(dir, name string) error {
	path := filepath.Join(dir, name)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 已存在且不是目录", path)
	}
	return nil
}

func runHost(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", "anybot.yaml", "配置文件")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := host.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	lock, err := host.LoadPluginLock(filepath.Join(filepath.Dir(*configPath), host.PluginLockFile))
	if err != nil {
		return err
	}
	if lockHasEnabledExternalPlugins(cfg, lock) {
		dir := filepath.Dir(*configPath)
		if filepath.Base(*configPath) != "anybot.yaml" {
			return fmt.Errorf("已启用外部插件时 anybot run 需要使用工作目录中的 anybot.yaml；自定义配置请先执行 anybot build -dir %s 后运行生成宿主", cleanDisplayDir(dir))
		}
		return runGeneratedHost(generatedHostRunOptions{dir: dir, output: "anybot-bot"})
	}
	logger, err := host.NewLogger(cfg.Runtime.LogLevel, stderr)
	if err != nil {
		return err
	}
	app, err := host.NewApp(cfg, host.EmptyRegistry(), logger, host.WithConfigPath(*configPath), host.WithRuntimeState(), host.WithPluginLock(lock))
	if err != nil {
		return withExternalPluginHint(err, filepath.Dir(*configPath))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func lockHasEnabledExternalPlugins(cfg host.Config, lock host.PluginLock) bool {
	for _, item := range lock.Plugins {
		if item.Module == "" {
			continue
		}
		entry, ok := cfg.Plugins[item.ID]
		if ok && cliPluginEntryEnabled(entry) {
			return true
		}
	}
	return false
}

func cliPluginEntryEnabled(entry host.PluginEntry) bool {
	return entry.Enabled == nil || *entry.Enabled
}

func writeFile(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在；使用 -force 覆盖", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func withExternalPluginHint(err error, dir string) error {
	var unknown host.UnknownPluginError
	if !errors.As(err, &unknown) {
		return err
	}
	lock, lockErr := host.LoadPluginLock(filepath.Join(dir, host.PluginLockFile))
	if lockErr != nil {
		return err
	}
	for _, item := range lock.Plugins {
		if item.ID == unknown.ID && item.Module != "" {
			return fmt.Errorf("%w；%s 是插件锁中的外部插件，请使用 anybot run/anybot up 构建并运行生成宿主，或先执行 anybot plugin disable %s", err, unknown.ID, host.ShortPluginID(unknown.ID))
		}
	}
	return err
}
