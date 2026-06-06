package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/tty00a381/anybot/app/host"
	modmodule "golang.org/x/mod/module"
)

var managedModules = []string{
	"github.com/tty00a381/anybot",
}

type moduleDependency struct {
	Module  string
	Version string
	Replace string
}

var frameworkDependencies = detectFrameworkDependencies
var moduleVersionResolver = resolveModuleVersion

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	output := fs.String("o", "anybot-bot", "输出二进制")
	skipTidy := fs.Bool("skip-tidy", false, "跳过 go mod tidy")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := buildHost(buildOptions{dir: *dir, output: *output, skipTidy: *skipTidy}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "构建完成：%s\n", builtBinaryPath(*dir, *output))
	return nil
}

func runUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	dir := fs.String("dir", ".", "目标目录")
	output := fs.String("o", "anybot-bot", "输出二进制")
	skipTidy := fs.Bool("skip-tidy", false, "跳过 go mod tidy")
	skipBuild := fs.Bool("skip-build", false, "跳过构建")
	skipSync := fs.Bool("skip-sync", false, "跳过插件配置同步")
	skipCheck := fs.Bool("skip-check", false, "跳过插件配置检查")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*skipBuild {
		if err := buildHost(buildOptions{dir: *dir, output: *output, skipTidy: *skipTidy}); err != nil {
			return err
		}
	}
	binary := commandBinaryPath(*output)
	if !*skipSync {
		if err := commandRunner(*dir, binary, "plugin", "sync"); err != nil {
			return err
		}
	}
	if !*skipCheck {
		if err := commandRunner(*dir, binary, "plugin", "check"); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "启动：%s\n", builtBinaryPath(*dir, *output))
	return commandRunner(*dir, binary)
}

type buildOptions struct {
	dir      string
	output   string
	skipTidy bool
}

func buildHost(opts buildOptions) error {
	if opts.dir == "" {
		opts.dir = "."
	}
	if opts.output == "" {
		opts.output = "anybot-bot"
	}
	workspace, err := host.EnsurePluginWorkspace(opts.dir)
	if err != nil {
		return err
	}
	if err := syncFrameworkGoMod(opts.dir); err != nil {
		return err
	}
	if err := syncWorkspaceGoMod(opts.dir, workspace); err != nil {
		return err
	}
	if !opts.skipTidy {
		if err := commandRunner(opts.dir, "go", "mod", "tidy"); err != nil {
			return err
		}
	}
	return commandRunner(opts.dir, "go", "build", "-o", opts.output, ".")
}

func builtBinaryPath(dir, output string) string {
	if output == "" {
		output = "anybot-bot"
	}
	if filepath.IsAbs(output) {
		return output
	}
	if dir == "" {
		dir = "."
	}
	return localExecutablePath(filepath.Join(dir, output))
}

func commandBinaryPath(output string) string {
	if output == "" {
		output = "anybot-bot"
	}
	if filepath.IsAbs(output) {
		return output
	}
	return localExecutablePath(filepath.Clean(output))
}

func localExecutablePath(path string) string {
	if strings.ContainsAny(path, `/\`) {
		return path
	}
	return "." + string(filepath.Separator) + path
}

func runExternalCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func resolveModuleVersion(module, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		query = "latest"
	}
	spec := module + "@" + query
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Version}}", spec).CombinedOutput()
	if err != nil {
		if detail := strings.TrimSpace(string(out)); detail != "" {
			return "", fmt.Errorf("%w: %s", err, detail)
		}
		return "", err
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		return "", fmt.Errorf("go list did not return a version for %s", spec)
	}
	return version, nil
}

func syncWorkspaceGoMod(dir string, workspace host.PluginWorkspace) error {
	for _, plugin := range workspace.Plugins {
		if err := syncPluginGoMod(dir, plugin); err != nil {
			return err
		}
	}
	return nil
}

func syncFrameworkGoMod(dir string) error {
	for _, dep := range frameworkDependencies() {
		version := dep.Version
		if version == "" {
			version = "v0.0.0"
		}
		if err := commandRunner(dir, "go", "mod", "edit", "-require="+dep.Module+"@"+version); err != nil {
			return err
		}
		if dep.Replace != "" {
			if err := commandRunner(dir, "go", "mod", "edit", "-replace="+dep.Module+"="+dep.Replace); err != nil {
				return err
			}
			continue
		}
		if err := commandRunner(dir, "go", "mod", "edit", "-dropreplace="+dep.Module); err != nil {
			return err
		}
	}
	return nil
}

func detectFrameworkDependencies() []moduleDependency {
	out := make([]moduleDependency, 0, len(managedModules))
	for _, module := range managedModules {
		out = append(out, detectFrameworkDependency(module))
	}
	return out
}

func detectFrameworkDependency(module string) moduleDependency {
	if v := normalizedVersion(version); v != "" {
		return moduleDependency{Module: module, Version: v}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := normalizedVersion(info.Main.Version); v != "" {
			return moduleDependency{Module: module, Version: v}
		}
	}
	if root, ok := moduleSourceRoot(module); ok {
		return moduleDependency{Module: module, Version: "v0.0.0", Replace: root}
	}
	return moduleDependency{Module: module, Version: "v0.0.0"}
}

func normalizedVersion(v string) string {
	v = strings.TrimSpace(v)
	switch v {
	case "", "dev", "(devel)":
		return ""
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	if v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return ""
}

func moduleSourceRoot(module string) (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(file) {
		return "", false
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	candidates := []string{
		repo,
	}
	for _, root := range candidates {
		data, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "module" && fields[1] == module {
				return root, true
			}
		}
	}
	return "", false
}

func syncPluginGoMod(dir string, plugin host.PluginModule) error {
	version := plugin.Version
	if version == "" && plugin.Replace != "" {
		version = pluginReplacePlaceholderVersion(plugin.Module)
	}
	if version != "" {
		if err := commandRunner(dir, "go", "mod", "edit", "-require="+plugin.Module+"@"+version); err != nil {
			return err
		}
	}
	if plugin.Replace != "" {
		if err := commandRunner(dir, "go", "mod", "edit", "-replace="+plugin.Module+"="+plugin.Replace); err != nil {
			return err
		}
		return nil
	}
	return commandRunner(dir, "go", "mod", "edit", "-dropreplace="+plugin.Module)
}

func pluginReplacePlaceholderVersion(module string) string {
	_, pathMajor, ok := modmodule.SplitPathVersion(module)
	if !ok || pathMajor == "" {
		return "v0.0.0"
	}
	major := strings.TrimLeft(pathMajor, "/.")
	if major == "" {
		return "v0.0.0"
	}
	version := major + ".0.0"
	if err := modmodule.Check(module, version); err != nil {
		return "v0.0.0"
	}
	return version
}

func dropPluginGoMod(dir string, plugin host.PluginModule) error {
	if err := commandRunner(dir, "go", "mod", "edit", "-droprequire="+plugin.Module); err != nil {
		return err
	}
	if err := commandRunner(dir, "go", "mod", "edit", "-dropreplace="+plugin.Module); err != nil {
		return err
	}
	return nil
}
