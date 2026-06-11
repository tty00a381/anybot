package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tty00a381/anybot/app/host"
)

const (
	cmdTestHelpID      = "plg_aaaaaaaaaaaaaaaaaaaaaaaaaa"
	cmdTestEchoID      = "plg_bbbbbbbbbbbbbbbbbbbbbbbbbb"
	cmdTestRateLimitID = "plg_dddddddddddddddddddddddddd"
	cmdTestWeatherID   = "plg_eeeeeeeeeeeeeeeeeeeeeeeeee"
	cmdTestLegacyID    = "plg_ffffffffffffffffffffffffff"
	cmdTestGhostID     = "plg_22222222222222222222222222"
)

func TestRunInitDoctorAndPlugins(t *testing.T) {
	dir := t.TempDir()
	out, errOut, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"init", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "已初始化 AnyBot 工作目录") ||
		!strings.Contains(out.String(), "export ONEBOT_ACCESS_TOKEN=你的令牌") ||
		!strings.Contains(out.String(), "anybot doctor") ||
		!strings.Contains(out.String(), "anybot run") {
		t.Fatalf("init output:\n%s", out.String())
	}
	out.Reset()
	errOut.Reset()
	config := host.ConfigPath(dir)
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), `listen: "127.0.0.1:6700"`, `listen: "127.0.0.1:0"`))
	if err := os.WriteFile(config, data, 0o644); err != nil {
		t.Fatal(err)
	}
	helpID := pluginIDByBuiltin(t, dir, "help")

	if err := run([]string{"doctor", "-config", config}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "传输：reverse_ws") || !strings.Contains(out.String(), "插件："+helpID) {
		t.Fatalf("doctor output:\n%s", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor stderr:\n%s", errOut.String())
	}
	for _, name := range []string{host.PluginLockFile, "plugins.gen.go", "main.go", "go.mod", host.ConfigDirName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was not generated: %v", name, err)
		}
	}
	readme := readTestFile(t, filepath.Join(dir, "README.md"))
	if !strings.Contains(readme, "anybot plugin add github.com/acme/anybot-weather@v0.1.0\nanybot plugin status\nanybot plugin enable <id>\nanybot up") {
		t.Fatalf("README should run external plugins through anybot up:\n%s", readme)
	}
	out.Reset()
	if err := run([]string{"plugins"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "help\t1.0.2") || !strings.Contains(out.String(), "echo\t1.0.2") {
		t.Fatalf("plugins output:\n%s", out.String())
	}
}

func TestRunInitAcceptsDirectoryArgument(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "bot")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"init", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "已初始化 AnyBot 工作目录："+dir) ||
		!strings.Contains(out.String(), "cd "+dir) {
		t.Fatalf("init output:\n%s", out.String())
	}
	for _, name := range []string{host.PluginLockFile, "plugins.gen.go", "main.go", "go.mod", host.ConfigDirName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was not generated: %v", name, err)
		}
	}
	if _, err := os.Stat(host.ConfigPath(dir)); err != nil {
		t.Fatalf("%s was not generated: %v", host.DefaultConfigPath, err)
	}
}

func TestRunInitDefaultsToMybotDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"init"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "已初始化 AnyBot 工作目录：mybot") ||
		!strings.Contains(out.String(), "cd mybot") {
		t.Fatalf("init output:\n%s", out.String())
	}
	if _, err := os.Stat(host.ConfigPath(filepath.Join(root, "mybot"))); err != nil {
		t.Fatalf("default mybot directory was not generated: %v", err)
	}
}

func TestRunInitRejectsDirectoryArgumentWithDirFlag(t *testing.T) {
	err := run([]string{"init", "bot", "-dir", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "不能同时指定目录参数和 -dir") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunInitAcceptsEqualsFlagsWithDirectoryArgument(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "bot")
	if err := run([]string{"init", dir, "--force=true"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(host.ConfigPath(dir)); err != nil {
		t.Fatalf("bot directory was not generated: %v", err)
	}
}

func TestRunInitRejectsUnmanagedGeneratedFilesWithoutPartialWrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"init", "-dir", dir})
	if err == nil {
		t.Fatal("init should reject unmanaged main.go")
	}
	if !strings.Contains(err.Error(), "不是 anybot 生成文件") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(host.ConfigPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("anybot.yaml should not be partially written: %v", err)
	}
}

func TestRunInitForceTakesOverGeneratedHostFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, host.ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, host.ConfigDirName, cmdTestGhostID+".yaml"), []byte("enabled: true\nconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"init", dir, "-force"}); err != nil {
		t.Fatal(err)
	}
	mainGo := readTestFile(t, filepath.Join(dir, "main.go"))
	if !strings.Contains(mainGo, "Code generated by anybot") ||
		!strings.Contains(mainGo, "registerGeneratedPlugins(external)") {
		t.Fatalf("main.go:\n%s", mainGo)
	}
	if _, err := os.Stat(filepath.Join(dir, host.ConfigDirName, cmdTestGhostID+".yaml")); !os.IsNotExist(err) {
		t.Fatalf("stale plugin config should be removed, err=%v", err)
	}
}

func TestRunVersionAndHelp(t *testing.T) {
	old := version
	version = "v9.9.9"
	defer func() { version = old }()
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"version"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "anybot v9.9.9\n" {
		t.Fatalf("version output = %q", got)
	}
	out.Reset()
	if err := run([]string{"help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "anybot run") || !strings.Contains(out.String(), "anybot plugins") {
		t.Fatalf("help output:\n%s", out.String())
	}
	out.Reset()
	if err := run([]string{"dev"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "anybot dev plugin") ||
		!strings.Contains(out.String(), "anybot dev new plugin") ||
		strings.Contains(out.String(), "anybot dev init") ||
		strings.Contains(out.String(), "anybot dev doctor") ||
		strings.Contains(out.String(), "anybot dev run") {
		t.Fatalf("dev help output:\n%s", out.String())
	}
	out.Reset()
	if err := run([]string{"dev", "help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "anybot dev plugin") ||
		!strings.Contains(out.String(), "anybot dev new plugin") ||
		strings.Contains(out.String(), "anybot dev init") ||
		strings.Contains(out.String(), "anybot dev doctor") ||
		strings.Contains(out.String(), "anybot dev run") {
		t.Fatalf("dev help output:\n%s", out.String())
	}
	out.Reset()
	if err := run([]string{"plugin"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "anybot plugin add") ||
		!strings.Contains(out.String(), "anybot plugin status") ||
		!strings.Contains(out.String(), "anybot plugin inspect") {
		t.Fatalf("plugin help output:\n%s", out.String())
	}
	out.Reset()
	if err := run([]string{"plugin", "help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "anybot plugin enable") ||
		!strings.Contains(out.String(), "anybot plugin disable") ||
		!strings.Contains(out.String(), "anybot plugin inspect") {
		t.Fatalf("plugin help output:\n%s", out.String())
	}
}

func TestRunDevPluginModule(t *testing.T) {
	root := t.TempDir()
	anybotDir := filepath.Join(root, "anybot")
	setTestFrameworkDependencies(t, []moduleDependency{
		{Module: "github.com/tty00a381/anybot", Version: "v0.0.0", Replace: anybotDir},
	})
	dir := filepath.Join(root, "weather")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{
		"dev", "plugin", "daily-weather",
		"-dir", dir,
		"-module", "github.com/acme/anybot-weather",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "已生成插件模块：daily-weather (github.com/acme/anybot-weather)") ||
		!strings.Contains(out.String(), "go test ./...") ||
		!strings.Contains(out.String(), "anybot plugin add github.com/acme/anybot-weather -replace "+dir) ||
		!strings.Contains(out.String(), "anybot plugin status -dir <机器人工作目录>") ||
		!strings.Contains(out.String(), "anybot plugin enable <id> -dir <机器人工作目录>") ||
		!strings.Contains(out.String(), "anybot up -dir <机器人工作目录>") {
		t.Fatalf("dev plugin output:\n%s", out.String())
	}
	goMod := readTestFile(t, filepath.Join(dir, "go.mod"))
	if !strings.Contains(goMod, "module github.com/acme/anybot-weather") ||
		!strings.Contains(goMod, "require github.com/tty00a381/anybot v0.0.0") ||
		!strings.Contains(goMod, "replace github.com/tty00a381/anybot => ../anybot") {
		t.Fatalf("go.mod:\n%s", goMod)
	}
	plugin := readTestFile(t, filepath.Join(dir, "daily_weather.go"))
	if !strings.Contains(plugin, `absdk.Manifest{Name: "daily-weather"`) ||
		strings.Contains(plugin, `"github.com/tty00a381/anybot/core"`) {
		t.Fatalf("plugin scaffold:\n%s", plugin)
	}
	readme := readTestFile(t, filepath.Join(dir, "README.md"))
	if !strings.Contains(readme, "anybot plugin add github.com/acme/anybot-weather -replace <插件目录>") {
		t.Fatalf("README.md:\n%s", readme)
	}
}

func TestRunDevPluginDefaultsToModule(t *testing.T) {
	root := t.TempDir()
	anybotDir := filepath.Join(root, "anybot")
	setTestFrameworkDependencies(t, []moduleDependency{
		{Module: "github.com/tty00a381/anybot", Version: "v0.0.0", Replace: anybotDir},
	})
	dir := filepath.Join(root, "plugin")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"dev", "plugin", "hello-world", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "已生成插件模块：hello-world (example.com/anybot-plugin/hello-world)") ||
		!strings.Contains(out.String(), "当前 module 是示例路径") ||
		!strings.Contains(out.String(), "anybot plugin add example.com/anybot-plugin/hello-world") {
		t.Fatalf("dev plugin output:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugins", "hello_world", "hello_world.go")); !os.IsNotExist(err) {
		t.Fatalf("default plugin should be a module, project plugin err=%v", err)
	}
}

func TestRunDevPluginModuleReleaseVersion(t *testing.T) {
	setTestFrameworkDependencies(t, releaseFrameworkDependencies("v1.2.3"))
	dir := filepath.Join(t.TempDir(), "space dir", "weather")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{
		"dev", "plugin", "daily-weather",
		"-dir", dir,
		"-module", "github.com/acme/anybot-weather",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "go mod tidy") ||
		!strings.Contains(out.String(), shellQuote(dir)) {
		t.Fatalf("dev plugin output:\n%s", out.String())
	}
	goMod := readTestFile(t, filepath.Join(dir, "go.mod"))
	if !strings.Contains(goMod, "require github.com/tty00a381/anybot v1.2.3") ||
		strings.Contains(goMod, "replace github.com/tty00a381/anybot") {
		t.Fatalf("go.mod:\n%s", goMod)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.sum")); !os.IsNotExist(err) {
		t.Fatalf("go.sum should not be generated without local replace: %v", err)
	}
}

func TestRunDevPluginModuleRejectsUnknownFrameworkDependency(t *testing.T) {
	setTestFrameworkDependencies(t, []moduleDependency{{Module: "github.com/tty00a381/anybot", Version: "v0.0.0"}})
	err := run([]string{
		"dev", "plugin", "daily-weather",
		"-dir", t.TempDir(),
		"-module", "github.com/acme/anybot-weather",
	})
	if err == nil || !strings.Contains(err.Error(), "生成插件模块需要可解析的 AnyBot 版本") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunDoctorRejectsPublicReverseWSWithoutToken(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, `runtime:
  log_level: info
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "0.0.0.0:0"
`)
	out, errOut, restore := captureOutput(t)
	defer restore()
	err := run([]string{"doctor", "-config", config})
	if err == nil {
		t.Fatal("doctor should reject public reverse_ws without token")
	}
	if !strings.Contains(err.Error(), "监听非本机地址时必须配置可用访问令牌") {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("doctor output=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestRunDoctorRejectsPublicHTTPWebhookWithoutToken(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, `runtime:
  log_level: info
adapter:
  protocol: onebot11
  transport:
    type: http
    url: "http://127.0.0.1:5700"
    listen: "0.0.0.0:0"
`)
	_, _, restore := captureOutput(t)
	defer restore()
	err := run([]string{"doctor", "-config", config})
	if err == nil {
		t.Fatal("doctor should reject public http webhook without token")
	}
	if !strings.Contains(err.Error(), "监听非本机地址时必须配置可用访问令牌") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPluginAddAndList(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/weather" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "v1.2.3", nil
	})
	dir := t.TempDir()
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/weather", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	id := readOnlyPluginID(t, dir)
	shortID := host.ShortPluginID(id)
	if !strings.Contains(out.String(), "插件已添加："+id+" (github.com/acme/weather@v1.2.3)") ||
		!strings.Contains(out.String(), filepath.ToSlash(filepath.Join(dir, host.ConfigDirName, id+".yaml"))) ||
		!strings.Contains(out.String(), "下一步：") ||
		!strings.Contains(out.String(), "cd "+dir) ||
		!strings.Contains(out.String(), "anybot plugin enable "+shortID) ||
		!strings.Contains(out.String(), "anybot up") {
		t.Fatalf("add output:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "plugins.gen.go")); err != nil {
		t.Fatal(err)
	}
	config := readTestFile(t, host.ConfigPath(dir))
	if strings.Contains(config, "weather:") {
		t.Fatalf("config:\n%s", config)
	}
	pluginConfig := readTestFile(t, filepath.Join(dir, host.ConfigDirName, id+".yaml"))
	if !strings.Contains(pluginConfig, "enabled: false") || !strings.Contains(pluginConfig, "config: {}") {
		t.Fatalf("plugin config:\n%s", pluginConfig)
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if !strings.Contains(lockText, "id: "+id) ||
		!strings.Contains(lockText, "version: v1.2.3") ||
		strings.Contains(lockText, "name:") {
		t.Fatalf("lockText:\n%s", lockText)
	}
	out.Reset()
	if err := run([]string{"plugin", "list", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID\t模块") ||
		!strings.Contains(out.String(), shortID+"\tgithub.com/acme/weather@v1.2.3") {
		t.Fatalf("list output:\n%s", out.String())
	}
}

func TestRunPluginAddWithReplaceDoesNotResolveLatest(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		t.Fatalf("local replace should not resolve %s@%s", module, query)
		return "", nil
	})
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	root := t.TempDir()
	dir := filepath.Join(root, "bot")
	pluginDir := filepath.Join(root, "weather")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/weather", "-replace", pluginDir, "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		dir + " go mod edit -require=github.com/acme/weather@v0.0.0",
		dir + " go mod edit -replace=github.com/acme/weather=../weather",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	id := readOnlyPluginID(t, dir)
	if !strings.Contains(out.String(), "插件已添加："+id+" (github.com/acme/weather => ../weather)") {
		t.Fatalf("add output:\n%s", out.String())
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if strings.Contains(lockText, "version:") || !strings.Contains(lockText, "replace: ../weather") {
		t.Fatalf("lockText:\n%s", lockText)
	}
}

func TestRunPluginAddWithReplaceUsesPathMajorPlaceholder(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		t.Fatalf("local replace should not resolve %s@%s", module, query)
		return "", nil
	})
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	root := t.TempDir()
	dir := filepath.Join(root, "bot")
	pluginDir := filepath.Join(root, "weather")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/weather/v2", "-replace", pluginDir, "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		dir + " go mod edit -require=github.com/acme/weather/v2@v2.0.0",
		dir + " go mod edit -replace=github.com/acme/weather/v2=../weather",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	id := readOnlyPluginID(t, dir)
	if !strings.Contains(out.String(), "插件已添加："+id+" (github.com/acme/weather/v2 => ../weather)") {
		t.Fatalf("add output:\n%s", out.String())
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if !strings.Contains(lockText, "id: "+id) ||
		strings.Contains(lockText, "version:") ||
		!strings.Contains(lockText, "module: github.com/acme/weather/v2") {
		t.Fatalf("lockText:\n%s", lockText)
	}
}

func TestRunPluginAddReportsVersionResolutionErrorWithoutPartialWrite(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/weather" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "", errors.New("module not found")
	})
	dir := t.TempDir()
	err := run([]string{"plugin", "add", "github.com/acme/weather", "-dir", dir})
	if err == nil {
		t.Fatal("version resolution failure should reject plugin add")
	}
	if !strings.Contains(err.Error(), "解析插件版本 github.com/acme/weather@latest 失败") ||
		!strings.Contains(err.Error(), "module not found") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(host.PluginLockPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("lock should not be created, err=%v", err)
	}
}

func TestRunPluginAddRollsBackAfterConfigFailure(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		t.Fatalf("local replace should not resolve %s@%s", module, query)
		return "", nil
	})
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls int
	commandRunner = func(dir, name string, args ...string) error {
		calls++
		if calls == 2 {
			return errors.New("go mod edit failed")
		}
		return nil
	}
	dir := t.TempDir()
	pluginDir := filepath.Join(t.TempDir(), "weather")
	err := run([]string{
		"plugin", "add", "github.com/acme/weather",
		"-replace", pluginDir,
		"-dir", dir,
	})
	if err == nil || !strings.Contains(err.Error(), "go mod edit failed") {
		t.Fatalf("err = %v", err)
	}
	for _, name := range []string{host.PluginLockFile, "plugins.gen.go", "main.go", "go.mod"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should be rolled back, err=%v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, host.ConfigDirName)); !os.IsNotExist(err) {
		t.Fatalf("config should be rolled back, err=%v", err)
	}
}

func TestJoinRollbackErrorMergesRollbackFailure(t *testing.T) {
	err := errors.New("go mod edit failed")
	joinRollbackError(&err, false, func() error {
		return errors.New("restore failed")
	})
	if !strings.Contains(err.Error(), "go mod edit failed") ||
		!strings.Contains(err.Error(), "回滚失败") ||
		!strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPluginAddSupportsVersionAndReplace(t *testing.T) {
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	root := t.TempDir()
	dir := filepath.Join(root, "bot")
	pluginDir := filepath.Join(root, "weather")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/weather@v1.2.3", "-replace", pluginDir, "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		dir + " go mod edit -require=github.com/acme/weather@v1.2.3",
		dir + " go mod edit -replace=github.com/acme/weather=../weather",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	id := readOnlyPluginID(t, dir)
	if !strings.Contains(out.String(), "插件已添加："+id+" (github.com/acme/weather@v1.2.3 => ../weather)") {
		t.Fatalf("add output:\n%s", out.String())
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if !strings.Contains(lockText, "version: v1.2.3") || !strings.Contains(lockText, "replace: ../weather") {
		t.Fatalf("lockText:\n%s", lockText)
	}
}

func TestRunPluginUpdatePreservesConfigAndSyncsGoMod(t *testing.T) {
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	root := t.TempDir()
	dir := filepath.Join(root, "bot")
	pluginDir := filepath.Join(root, "weather")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/weather@v1.2.3", "-replace", pluginDir, "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	id := readOnlyPluginID(t, dir)
	pluginConfig := writeTestPluginConfig(t, dir, id, "enabled: true\nconfig:\n  city: Hangzhou\n")
	calls = nil
	out.Reset()
	if err := run([]string{"plugin", "update", host.ShortPluginID(id), "-version", "v1.3.0", "-clear-replace", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		dir + " go mod edit -require=github.com/acme/weather@v1.3.0",
		dir + " go mod edit -dropreplace=github.com/acme/weather",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(out.String(), "插件已更新："+id+" (github.com/acme/weather@v1.3.0)") {
		t.Fatalf("update output:\n%s", out.String())
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if !strings.Contains(lockText, "version: v1.3.0") ||
		strings.Contains(lockText, "replace:") {
		t.Fatalf("lockText:\n%s", lockText)
	}
	if config := readTestFile(t, pluginConfig); !strings.Contains(config, "city: Hangzhou") {
		t.Fatalf("plugin config should be preserved:\n%s", config)
	}
}

func TestRunPluginUpdateRollsBackAfterGoModFailure(t *testing.T) {
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls int
	commandRunner = func(dir, name string, args ...string) error {
		calls++
		if calls == 2 {
			return errors.New("go mod edit failed")
		}
		return nil
	}
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{
		Dir:     dir,
		ID:      cmdTestWeatherID,
		Module:  "github.com/acme/weather",
		Version: "v1.2.3",
		Replace: "../weather",
	}); err != nil {
		t.Fatal(err)
	}
	lockPath := host.PluginLockPath(dir)
	beforeLockText := readTestFile(t, lockPath)
	beforeGenerated := readTestFile(t, filepath.Join(dir, "plugins.gen.go"))
	beforeMain := readTestFile(t, filepath.Join(dir, "main.go"))
	beforeGoMod := readTestFile(t, filepath.Join(dir, "go.mod"))
	err := run([]string{"plugin", "update", host.ShortPluginID(cmdTestWeatherID), "-version", "v1.3.0", "-clear-replace", "-dir", dir})
	if err == nil || !strings.Contains(err.Error(), "go mod edit failed") {
		t.Fatalf("err = %v", err)
	}
	if got := readTestFile(t, lockPath); got != beforeLockText {
		t.Fatalf("lockText should roll back:\n%s", got)
	}
	if got := readTestFile(t, filepath.Join(dir, "plugins.gen.go")); got != beforeGenerated {
		t.Fatalf("plugins.gen.go should roll back:\n%s", got)
	}
	if got := readTestFile(t, filepath.Join(dir, "main.go")); got != beforeMain {
		t.Fatalf("main.go should roll back:\n%s", got)
	}
	if got := readTestFile(t, filepath.Join(dir, "go.mod")); got != beforeGoMod {
		t.Fatalf("go.mod should roll back:\n%s", got)
	}
}

func TestRunPluginUpdatePinsLatestBeforeWriting(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/weather" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "v1.4.0", nil
	})
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{
		Dir:     dir,
		ID:      cmdTestWeatherID,
		Module:  "github.com/acme/weather",
		Version: "v1.2.3",
	}); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "update", host.ShortPluginID(cmdTestWeatherID), "-version", "latest", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		dir + " go mod edit -require=github.com/acme/weather@v1.4.0",
		dir + " go mod edit -dropreplace=github.com/acme/weather",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(out.String(), "插件已更新："+cmdTestWeatherID+" (github.com/acme/weather@v1.4.0)") {
		t.Fatalf("update output:\n%s", out.String())
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if !strings.Contains(lockText, "version: v1.4.0") || strings.Contains(lockText, "latest") {
		t.Fatalf("lockText:\n%s", lockText)
	}
}

func TestRunPluginUpdateClearReplacePinsLatest(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/weather" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "v1.4.0", nil
	})
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{
		Dir:     dir,
		ID:      cmdTestWeatherID,
		Module:  "github.com/acme/weather",
		Replace: "../weather",
	}); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "update", host.ShortPluginID(cmdTestWeatherID), "-clear-replace", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		dir + " go mod edit -require=github.com/acme/weather@v1.4.0",
		dir + " go mod edit -dropreplace=github.com/acme/weather",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(out.String(), "插件已更新："+cmdTestWeatherID+" (github.com/acme/weather@v1.4.0)") {
		t.Fatalf("update output:\n%s", out.String())
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if !strings.Contains(lockText, "version: v1.4.0") ||
		strings.Contains(lockText, "replace:") {
		t.Fatalf("lockText:\n%s", lockText)
	}
}

func TestRunPluginUpdateVersionResolutionErrorDoesNotWrite(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/weather" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "", errors.New("module not found")
	})
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	commandRunner = func(dir, name string, args ...string) error {
		t.Fatalf("go command should not run after resolution failure: %s %s", name, strings.Join(args, " "))
		return nil
	}
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{
		Dir:     dir,
		ID:      cmdTestWeatherID,
		Module:  "github.com/acme/weather",
		Version: "v1.2.3",
	}); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"plugin", "update", host.ShortPluginID(cmdTestWeatherID), "-version", "latest", "-dir", dir})
	if err == nil {
		t.Fatal("version resolution failure should reject plugin update")
	}
	if !strings.Contains(err.Error(), "解析插件版本 github.com/acme/weather@latest 失败") ||
		!strings.Contains(err.Error(), "module not found") {
		t.Fatalf("err = %v", err)
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if !strings.Contains(lockText, "version: v1.2.3") ||
		strings.Contains(lockText, "latest") {
		t.Fatalf("lockText should remain unchanged:\n%s", lockText)
	}
}

func TestRunPluginUpdateRejectsBuiltinInstall(t *testing.T) {
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestHelpID, Builtin: "help"}); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"plugin", "update", host.ShortPluginID(cmdTestHelpID), "-version", "v1.0.1", "-dir", dir})
	if err == nil || !strings.Contains(err.Error(), "cannot be updated as a Go module") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginVersionNeedsResolution(t *testing.T) {
	tests := []struct {
		module  string
		version string
		want    bool
	}{
		{module: "github.com/acme/weather", version: "", want: true},
		{module: "github.com/acme/weather", version: "latest", want: true},
		{module: "github.com/acme/weather", version: "main", want: true},
		{module: "github.com/acme/weather", version: "abcdef123456", want: true},
		{module: "github.com/acme/weather", version: "v1", want: true},
		{module: "github.com/acme/weather", version: "v1.2", want: true},
		{module: "github.com/acme/weather", version: "v1.2.x", want: true},
		{module: "github.com/acme/weather", version: "v1.2.3foo", want: true},
		{module: "github.com/acme/weather", version: "v1.2.3", want: false},
		{module: "github.com/acme/weather", version: "v1.2.3-rc.1", want: false},
		{module: "github.com/acme/weather", version: "v1.2.3+build.1", want: false},
		{module: "github.com/acme/weather", version: "v0.0.0-20260101000000-abcdefabcdef", want: false},
		{module: "github.com/acme/weather", version: "v2.0.0+incompatible", want: false},
		{module: "github.com/acme/weather/v2", version: "v2.0.0", want: false},
		{module: "github.com/acme/weather/v2", version: "v1.2.3", want: true},
		{module: "github.com/acme/weather/v2", version: "v2", want: true},
		{module: "github.com/acme/weather/v2", version: "v2.0", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.module+"@"+tt.version, func(t *testing.T) {
			if got := pluginVersionNeedsResolution(tt.module, tt.version); got != tt.want {
				t.Fatalf("pluginVersionNeedsResolution(%q, %q) = %v, want %v", tt.module, tt.version, got, tt.want)
			}
		})
	}
}

func TestNormalizeReplacePath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "bot")
	pluginDir := filepath.Join(root, "plugins", "weather")
	got, err := normalizeReplacePath(dir, pluginDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "../plugins/weather" {
		t.Fatalf("replace = %q", got)
	}
}

func TestNormalizeReplacePathResolvesSymlinkedDirectories(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	if err := os.MkdirAll(filepath.Join(real, "bot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(real, "plugins", "weather"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	got, err := normalizeReplacePath(filepath.Join(link, "bot"), filepath.Join(real, "plugins", "weather"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "../plugins/weather" {
		t.Fatalf("replace = %q", got)
	}
}

func TestNormalizeReplacePathResolvesSymlinkedMissingLeaf(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	got, err := normalizeReplacePath(filepath.Join(link, "plugin"), filepath.Join(real, "anybot"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "../anybot" {
		t.Fatalf("replace = %q", got)
	}
}

func TestRunPluginAddAllowsModuleNamedLikeBuiltin(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/help" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "v1.2.3", nil
	})
	dir := t.TempDir()
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/help", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	id := readOnlyPluginID(t, dir)
	if !strings.Contains(out.String(), "插件已添加："+id) {
		t.Fatalf("add output:\n%s", out.String())
	}
}

func TestRunPluginAddAllowsMultipleInstancesOfSameModule(t *testing.T) {
	resolveCalls := 0
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		resolveCalls++
		if module != "github.com/acme/help" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "v1.2.3", nil
	})
	dir := t.TempDir()
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/help", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run([]string{"plugin", "add", "github.com/acme/help", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if resolveCalls != 1 {
		t.Fatalf("duplicate module should reuse locked version, resolveCalls=%d", resolveCalls)
	}
	lock, err := host.LoadPluginLock(host.PluginLockPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Plugins) != 2 ||
		lock.Plugins[0].Module != "github.com/acme/help" ||
		lock.Plugins[1].Module != "github.com/acme/help" ||
		lock.Plugins[0].ID == lock.Plugins[1].ID {
		t.Fatalf("lock = %#v", lock)
	}
	if err := run([]string{"plugin", "list", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), host.ShortPluginID(lock.Plugins[0].ID)+"\tgithub.com/acme/help@v1.2.3") ||
		!strings.Contains(out.String(), host.ShortPluginID(lock.Plugins[1].ID)+"\tgithub.com/acme/help@v1.2.3") {
		t.Fatalf("list output:\n%s", out.String())
	}
}

func TestRunPluginStatus(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"init", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	helpID := pluginIDByBuiltin(t, dir, "help")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestWeatherID, Module: "github.com/acme/weather"}); err != nil {
		t.Fatal(err)
	}
	if _, err := host.EnsurePluginConfigEntry(host.ConfigPath(dir), cmdTestWeatherID); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "status", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID\t名称\t来源\t配置\t启用\t可加载\t版本\t模块") ||
		!strings.Contains(out.String(), host.ShortPluginID(helpID)+"\thelp\t内置\t是\t是\t是\t1.0.2\t-") ||
		!strings.Contains(out.String(), host.ShortPluginID(cmdTestWeatherID)+"\t-\t外部\t是\t否\t否\t-\tgithub.com/acme/weather") {
		t.Fatalf("status output:\n%s", out.String())
	}
}

func TestRunPluginStatusUsesConfigPathDirectoryForLock(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestHelpID, Builtin: "help"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestHelpID, "enabled: true\nconfig: {}\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: other, ID: cmdTestWeatherID, Module: "github.com/acme/weather"}); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "status", "-config", config}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), host.ShortPluginID(cmdTestHelpID)+"\thelp\t内置") ||
		strings.Contains(out.String(), host.ShortPluginID(cmdTestWeatherID)) {
		t.Fatalf("status output should use config directory lock:\n%s", out.String())
	}
}

func TestRunPluginInspect(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"init", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	helpID := pluginIDByBuiltin(t, dir, "help")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "inspect", host.ShortPluginID(helpID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "名称：help") ||
		!strings.Contains(out.String(), "ID："+helpID) ||
		!strings.Contains(out.String(), "配置文件："+filepath.Join(dir, host.ConfigDirName, helpID+".yaml")) ||
		!strings.Contains(out.String(), "当前配置：") ||
		!strings.Contains(out.String(), "默认配置：") ||
		!strings.Contains(out.String(), "command: help") {
		t.Fatalf("inspect output:\n%s", out.String())
	}
}

func TestRunPluginRejectsDirAndConfigTogether(t *testing.T) {
	err := run([]string{"plugin", "status", "-dir", t.TempDir(), "-config", host.ConfigPath(t.TempDir())})
	if err == nil || !strings.Contains(err.Error(), "不能同时指定 -dir 和 -config") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPluginCheck(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestHelpID, Builtin: "help"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestHelpID, "enabled: true\nconfig: {}\n")
	writeTestPluginConfig(t, dir, cmdTestLegacyID, "enabled: false\nconfig: {}\n")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "check", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID\t来源\t状态\t说明") ||
		!strings.Contains(out.String(), host.ShortPluginID(cmdTestHelpID)+"\t内置\t可用\t-") ||
		!strings.Contains(out.String(), host.ShortPluginID(cmdTestLegacyID)+"\t配置\t禁用\t-") {
		t.Fatalf("check output:\n%s", out.String())
	}

	out.Reset()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestRateLimitID, Builtin: "ratelimit"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestRateLimitID, "enabled: true\nconfig:\n  limit: 0\n  window: 1m\n")
	err := run([]string{"plugin", "check", "-dir", dir})
	if err == nil || !strings.Contains(err.Error(), "插件配置检查失败") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), host.ShortPluginID(cmdTestRateLimitID)+"\t内置\t无效") ||
		!strings.Contains(out.String(), "ratelimit.limit") {
		t.Fatalf("invalid check output:\n%s", out.String())
	}
}

func TestRunPluginRemove(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/weather" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "v1.2.3", nil
	})
	dir := t.TempDir()
	if err := run([]string{"plugin", "add", "github.com/acme/weather", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	id := readOnlyPluginID(t, dir)
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "remove", host.ShortPluginID(id), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已移除："+id) ||
		!strings.Contains(out.String(), "配置已移除："+filepath.ToSlash(filepath.Join(dir, host.ConfigDirName, id+".yaml"))) {
		t.Fatalf("remove output:\n%s", out.String())
	}
	config := readTestFile(t, host.ConfigPath(dir))
	if strings.Contains(config, "weather:") {
		t.Fatalf("config:\n%s", config)
	}
	if _, err := os.Stat(filepath.Join(dir, host.ConfigDirName, id+".yaml")); !os.IsNotExist(err) {
		t.Fatalf("plugin config should be removed: %v", err)
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if strings.Contains(lockText, "github.com/acme/weather") {
		t.Fatalf("lockText:\n%s", lockText)
	}
}

func TestRunPluginRemoveUsesConfigPathDirectoryForLock(t *testing.T) {
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	dir := t.TempDir()
	other := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestWeatherID, Module: "github.com/acme/weather"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestWeatherID, "enabled: false\nconfig: {}\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: other, ID: cmdTestGhostID, Module: "github.com/acme/ghost"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"plugin", "remove", host.ShortPluginID(cmdTestWeatherID), "-config", config}); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || !strings.HasPrefix(calls[0], dir+" go mod edit") {
		t.Fatalf("calls should run in config directory: %#v", calls)
	}
	if _, err := host.LoadPluginLock(host.PluginLockPath(dir)); err != nil {
		t.Fatal(err)
	}
	lockText := readTestFile(t, host.PluginLockPath(dir))
	if strings.Contains(lockText, "github.com/acme/weather") {
		t.Fatalf("config directory lock should remove weather:\n%s", lockText)
	}
	otherLockText := readTestFile(t, host.PluginLockPath(other))
	if !strings.Contains(otherLockText, "github.com/acme/ghost") {
		t.Fatalf("other directory lock should be untouched:\n%s", otherLockText)
	}
}

func TestRunPluginRemoveDropsVersionedGoModEntries(t *testing.T) {
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{
		Dir:     dir,
		ID:      cmdTestWeatherID,
		Module:  "github.com/acme/weather",
		Version: "v1.2.3",
		Replace: "../weather",
	}); err != nil {
		t.Fatal(err)
	}
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	writeTestPluginConfig(t, dir, cmdTestWeatherID, "enabled: false\nconfig: {}\n")
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	if err := run([]string{"plugin", "remove", host.ShortPluginID(cmdTestWeatherID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		dir + " go mod edit -droprequire=github.com/acme/weather",
		dir + " go mod edit -dropreplace=github.com/acme/weather",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestRunPluginRemoveKeepsSharedModuleGoModEntries(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{cmdTestWeatherID, cmdTestGhostID} {
		if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{
			Dir:     dir,
			ID:      id,
			Module:  "github.com/acme/weather",
			Version: "v1.2.3",
			Replace: "../weather",
		}); err != nil {
			t.Fatal(err)
		}
	}
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	writeTestPluginConfig(t, dir, cmdTestWeatherID, "enabled: false\nconfig: {}\n")
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	if err := run([]string{"plugin", "remove", host.ShortPluginID(cmdTestWeatherID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("shared module should stay in go.mod, calls = %#v", calls)
	}
	lock, err := host.LoadPluginLock(host.PluginLockPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Plugins) != 1 || lock.Plugins[0].ID != cmdTestGhostID || lock.Plugins[0].Module != "github.com/acme/weather" {
		t.Fatalf("lock = %#v", lock)
	}
}

func TestRunPluginRemoveRollsBackAfterGoModFailure(t *testing.T) {
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls int
	commandRunner = func(dir, name string, args ...string) error {
		calls++
		if calls == 2 {
			return errors.New("go mod edit failed")
		}
		return nil
	}
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{
		Dir:     dir,
		ID:      cmdTestWeatherID,
		Module:  "github.com/acme/weather",
		Version: "v1.2.3",
		Replace: "../weather",
	}); err != nil {
		t.Fatal(err)
	}
	configPath := host.ConfigPath(dir)
	writeTestConfig(t, configPath, "runtime:\n  log_level: info\n")
	pluginConfig := writeTestPluginConfig(t, dir, cmdTestWeatherID, "enabled: true\nconfig:\n  city: Hangzhou\n")
	lockPath := host.PluginLockPath(dir)
	beforeLockText := readTestFile(t, lockPath)
	beforeConfig := readTestFile(t, pluginConfig)
	beforeGoMod := readTestFile(t, filepath.Join(dir, "go.mod"))
	err := run([]string{"plugin", "remove", host.ShortPluginID(cmdTestWeatherID), "-dir", dir})
	if err == nil || !strings.Contains(err.Error(), "go mod edit failed") {
		t.Fatalf("err = %v", err)
	}
	if got := readTestFile(t, lockPath); got != beforeLockText {
		t.Fatalf("lockText should roll back:\n%s", got)
	}
	if got := readTestFile(t, pluginConfig); got != beforeConfig {
		t.Fatalf("config should roll back:\n%s", got)
	}
	if got := readTestFile(t, filepath.Join(dir, "go.mod")); got != beforeGoMod {
		t.Fatalf("go.mod should roll back:\n%s", got)
	}
}

func TestRunPluginRemoveAllowsBuiltinInstall(t *testing.T) {
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestHelpID, Builtin: "help"}); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "remove", host.ShortPluginID(cmdTestHelpID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已移除："+cmdTestHelpID+" (内置:help)") {
		t.Fatalf("remove output:\n%s", out.String())
	}
}

func TestRunPluginSync(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestEchoID, Builtin: "echo"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestEchoID, "enabled: false\nconfig: {}\n")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "sync", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件配置已同步：") {
		t.Fatalf("sync output:\n%s", out.String())
	}
	updated := readTestFile(t, filepath.Join(dir, host.ConfigDirName, cmdTestEchoID+".yaml"))
	if !strings.Contains(updated, "command: echo") {
		t.Fatalf("config:\n%s", updated)
	}
}

func TestRunPluginSyncSkipsDisabledUnknownPlugin(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestEchoID, Builtin: "echo"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestEchoID, "enabled: false\nconfig: {}\n")
	writeTestPluginConfig(t, dir, cmdTestLegacyID, "enabled: false\nconfig: {}\n")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "sync", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "未知插件已跳过："+cmdTestLegacyID+"（已禁用）") {
		t.Fatalf("sync output:\n%s", out.String())
	}
	updated := readTestFile(t, filepath.Join(dir, host.ConfigDirName, cmdTestEchoID+".yaml"))
	if !strings.Contains(updated, "command: echo") {
		t.Fatalf("plugin config:\n%s", updated)
	}
}

func TestRunPluginSyncSkipsExternalLockPlugin(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/weather" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "v1.2.3", nil
	})
	dir := t.TempDir()
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/weather", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run([]string{"plugin", "sync", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	id := readOnlyPluginID(t, dir)
	if !strings.Contains(out.String(), "插件配置已同步：") ||
		!strings.Contains(out.String(), "外部插件待构建："+id) {
		t.Fatalf("sync output:\n%s", out.String())
	}
}

func TestRunPluginConfigResetSyncsBuiltinDefaults(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestHelpID, Builtin: "help"}); err != nil {
		t.Fatal(err)
	}
	pluginPath := writeTestPluginConfig(t, dir, cmdTestHelpID, "enabled: true\nconfig:\n  command: docs\n  lines:\n    - custom\n")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "config", host.ShortPluginID(cmdTestHelpID), "-reset", "command", "lines", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件配置已重置："+cmdTestHelpID+"（2 项）") ||
		!strings.Contains(out.String(), "默认配置已同步：1 项更新") {
		t.Fatalf("reset output:\n%s", out.String())
	}
	updated := readTestFile(t, pluginPath)
	if !strings.Contains(updated, "command: help") ||
		!strings.Contains(updated, "/help 显示帮助") ||
		strings.Contains(updated, "command: docs") ||
		strings.Contains(updated, "custom") {
		t.Fatalf("config:\n%s", updated)
	}
}

func TestRunPluginConfigResetUsesPluginConfigDir(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	pluginPath := filepath.Join(dir, host.ConfigDirName, cmdTestHelpID+".yaml")
	if err := os.Mkdir(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestHelpID, Builtin: "help"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, []byte(`enabled: true
config:
  command: docs
  lines:
    - custom
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "config", host.ShortPluginID(cmdTestHelpID), "-reset", "command", "lines", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件配置已重置："+cmdTestHelpID+"（2 项）") ||
		!strings.Contains(out.String(), "默认配置已同步：1 项更新") {
		t.Fatalf("reset output:\n%s", out.String())
	}
	updated := readTestFile(t, pluginPath)
	if !strings.Contains(updated, "command: help") ||
		!strings.Contains(updated, "/help 显示帮助") ||
		strings.Contains(updated, "command: docs") ||
		strings.Contains(updated, "custom") {
		t.Fatalf("plugin config:\n%s", updated)
	}
	main := readTestFile(t, config)
	if strings.Contains(main, "command:") || strings.Contains(main, "lines:") {
		t.Fatalf("main config should not receive split config:\n%s", main)
	}
}

func TestRunPluginConfigResetMustFollowPluginName(t *testing.T) {
	err := run([]string{"plugin", "config", "-reset", "command", "help"})
	if err == nil || !strings.Contains(err.Error(), "-reset 必须写在插件 ID 之后") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPluginConfigResetRejectsMixedSetAndReset(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestHelpID, Builtin: "help"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestHelpID, "enabled: true\nconfig: {}\n")
	err := run([]string{"plugin", "config", host.ShortPluginID(cmdTestHelpID), "-reset", "command", "command=docs", "-dir", dir})
	if err == nil || !strings.Contains(err.Error(), "不能同时设置和重置插件配置") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPluginConfigResetLockPluginWaitsForGeneratedHost(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	pluginPath := writeTestPluginConfig(t, dir, cmdTestWeatherID, "enabled: true\nconfig:\n  city: Hangzhou\n  unit: c\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestWeatherID, Module: "github.com/acme/weather"}); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "config", host.ShortPluginID(cmdTestWeatherID), "-reset", "city", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件配置已重置："+cmdTestWeatherID+"（1 项）") ||
		!strings.Contains(out.String(), "默认配置待构建同步："+cmdTestWeatherID) {
		t.Fatalf("reset output:\n%s", out.String())
	}
	updated := readTestFile(t, pluginPath)
	if strings.Contains(updated, "city: Hangzhou") || !strings.Contains(updated, "unit: c") {
		t.Fatalf("config:\n%s", updated)
	}
}

func TestRunPluginEnableDisable(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestEchoID, Builtin: "echo"}); err != nil {
		t.Fatal(err)
	}
	pluginPath := writeTestPluginConfig(t, dir, cmdTestEchoID, "enabled: false\nconfig: {}\n")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "enable", host.ShortPluginID(cmdTestEchoID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已启用："+cmdTestEchoID) {
		t.Fatalf("enable output:\n%s", out.String())
	}
	if updated := readTestFile(t, pluginPath); !strings.Contains(updated, "enabled: true") {
		t.Fatalf("config:\n%s", updated)
	}
	out.Reset()
	if err := run([]string{"plugin", "disable", host.ShortPluginID(cmdTestEchoID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已禁用："+cmdTestEchoID) {
		t.Fatalf("disable output:\n%s", out.String())
	}
	if updated := readTestFile(t, pluginPath); !strings.Contains(updated, "enabled: false") {
		t.Fatalf("config:\n%s", updated)
	}
}

func TestRunPluginEnableSyncsBuiltinDefaults(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestRateLimitID, Builtin: "ratelimit"}); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "enable", host.ShortPluginID(cmdTestRateLimitID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已启用："+cmdTestRateLimitID) ||
		!strings.Contains(out.String(), "默认配置已同步：1 项更新") {
		t.Fatalf("enable output:\n%s", out.String())
	}
	updated := readTestFile(t, filepath.Join(dir, host.ConfigDirName, cmdTestRateLimitID+".yaml"))
	if !strings.Contains(updated, "enabled: true") ||
		!strings.Contains(updated, "limit: 5") ||
		!strings.Contains(updated, "window: 1m") {
		t.Fatalf("config:\n%s", updated)
	}
}

func TestRunPluginEnableRejectsUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	err := run([]string{"plugin", "enable", host.ShortPluginID(cmdTestGhostID), "-dir", dir})
	if err == nil {
		t.Fatal("unknown plugin should be rejected")
	}
	if !strings.Contains(err.Error(), "未知插件 ID \""+host.ShortPluginID(cmdTestGhostID)+"\"") {
		t.Fatalf("err = %v", err)
	}
	if updated := readTestFile(t, config); strings.Contains(updated, cmdTestGhostID) {
		t.Fatalf("config was polluted:\n%s", updated)
	}
}

func TestRunPluginEnableRejectsUnknownConfiguredTarget(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	pluginPath := writeTestPluginConfig(t, dir, cmdTestGhostID, "enabled: false\nconfig: {}\n")
	err := run([]string{"plugin", "enable", host.ShortPluginID(cmdTestGhostID), "-dir", dir})
	if err == nil {
		t.Fatal("unknown configured plugin should be rejected")
	}
	if !strings.Contains(err.Error(), "未知插件 ID \""+host.ShortPluginID(cmdTestGhostID)+"\"") {
		t.Fatalf("err = %v", err)
	}
	if updated := readTestFile(t, pluginPath); strings.Contains(updated, "enabled: true") {
		t.Fatalf("unknown plugin should not be enabled:\n%s", updated)
	}
}

func TestRunPluginDisableAllowsUnknownConfiguredTarget(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	pluginPath := writeTestPluginConfig(t, dir, cmdTestLegacyID, "enabled: true\nconfig: {}\n")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "disable", host.ShortPluginID(cmdTestLegacyID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已禁用："+cmdTestLegacyID) {
		t.Fatalf("disable output:\n%s", out.String())
	}
	if updated := readTestFile(t, pluginPath); !strings.Contains(updated, "enabled: false") {
		t.Fatalf("config:\n%s", updated)
	}
}

func TestRunPluginEnableAllowsLockPlugin(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestWeatherID, Module: "github.com/acme/weather"}); err != nil {
		t.Fatal(err)
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "enable", host.ShortPluginID(cmdTestWeatherID), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "插件已启用："+cmdTestWeatherID) ||
		!strings.Contains(out.String(), "默认配置待构建同步："+cmdTestWeatherID) {
		t.Fatalf("enable output:\n%s", out.String())
	}
	if updated := readTestFile(t, filepath.Join(dir, host.ConfigDirName, cmdTestWeatherID+".yaml")); !strings.Contains(updated, "enabled: true") {
		t.Fatalf("config:\n%s", updated)
	}
}

func TestRunDoctorSkipsGeneratedExternalPluginChecks(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		if module != "github.com/acme/weather" || query != "latest" {
			t.Fatalf("resolve %s@%s", module, query)
		}
		return "v1.2.3", nil
	})
	dir := t.TempDir()
	out, errOut, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/weather", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	id := readOnlyPluginID(t, dir)
	if err := run([]string{"plugin", "enable", host.ShortPluginID(id), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	config := host.ConfigPath(dir)
	data := readTestFile(t, config)
	data = strings.ReplaceAll(data, `listen: "127.0.0.1:6700"`, `listen: "127.0.0.1:0"`)
	writeTestConfig(t, config, data)
	if err := run([]string{"doctor", "-config", config}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "传输：reverse_ws") ||
		!strings.Contains(out.String(), "插件："+id) ||
		!strings.Contains(out.String(), "外部插件：基础配置已检查") ||
		!strings.Contains(out.String(), "anybot up -dir "+dir) ||
		!strings.Contains(out.String(), "./anybot-bot plugin check") {
		t.Fatalf("doctor output:\n%s", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("doctor stderr:\n%s", errOut.String())
	}
}

func TestRunDoctorStillRejectsUnknownConfiguredPlugin(t *testing.T) {
	setTestModuleVersionResolver(t, func(module, query string) (string, error) {
		return "v1.2.3", nil
	})
	dir := t.TempDir()
	_, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "github.com/acme/weather", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	id := readOnlyPluginID(t, dir)
	if err := run([]string{"plugin", "enable", host.ShortPluginID(id), "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestGhostID, "enabled: true\nconfig: {}\n")
	config := host.ConfigPath(dir)
	data := readTestFile(t, config)
	data = strings.ReplaceAll(data, `listen: "127.0.0.1:6700"`, `listen: "127.0.0.1:0"`)
	writeTestConfig(t, config, data)
	err := run([]string{"doctor", "-config", config})
	if err == nil || !strings.Contains(err.Error(), "未知插件") || !strings.Contains(err.Error(), cmdTestGhostID) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunBuildInvokesGoTool(t *testing.T) {
	setTestFrameworkDependencies(t, releaseFrameworkDependencies("v9.9.9"))
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	dir := t.TempDir()
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"build", "-dir", dir, "-o", "bot"}); err != nil {
		t.Fatal(err)
	}
	want := append(frameworkGoModCalls(dir, releaseFrameworkDependencies("v9.9.9")),
		dir+" go mod tidy",
		dir+" go build -o bot .",
	)
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(out.String(), "构建完成："+filepath.Join(dir, "bot")) {
		t.Fatalf("build output:\n%s", out.String())
	}
	if _, err := os.Stat(host.PluginLockPath(dir)); err != nil {
		t.Fatal(err)
	}
}

func TestRunBuildSyncsPluginHostGoMod(t *testing.T) {
	setTestFrameworkDependencies(t, releaseFrameworkDependencies("v9.9.9"))
	dir := t.TempDir()
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{
		Dir:     dir,
		ID:      cmdTestWeatherID,
		Module:  "github.com/acme/weather",
		Version: "v1.2.3",
		Replace: "../weather",
	}); err != nil {
		t.Fatal(err)
	}
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	if err := run([]string{"build", "-dir", dir, "-o", "bot"}); err != nil {
		t.Fatal(err)
	}
	want := append(frameworkGoModCalls(dir, releaseFrameworkDependencies("v9.9.9")),
		dir+" go mod edit -require=github.com/acme/weather@v1.2.3",
		dir+" go mod edit -replace=github.com/acme/weather=../weather",
		dir+" go mod tidy",
		dir+" go build -o bot .",
	)
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestRunBuildGeneratedHostWithLocalPlugin(t *testing.T) {
	anybotRoot, ok := detectModuleSourceRoot("github.com/tty00a381/anybot")
	if !ok {
		t.Fatal("cannot locate anybot source root")
	}
	root := t.TempDir()
	cacheDir := filepath.Join(root, "cache")
	modCacheDir := filepath.Join(root, "modcache")
	pluginDir := filepath.Join(root, "weather-plugin")
	botDir := filepath.Join(root, "bot")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte(`module example.com/weather

go 1.24.0

require github.com/tty00a381/anybot v0.0.0

replace github.com/tty00a381/anybot => `+filepath.ToSlash(anybotRoot)+`
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "weather.go"), []byte(`package weather

import (
	"fmt"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Command string `+"`yaml:\"command\"`"+`
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Command) == "" {
		return fmt.Errorf("command is required")
	}
	return nil
}

var Plugin = absdk.Define(absdk.Spec[Config]{
	Manifest: absdk.Manifest{Name: "weather", Version: "0.1.0"},
	DefaultConfig: Config{Command: "weather"},
	Setup: func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).Handle(func(c *absdk.EventContext) error {
			_, err := c.ReplyText("sunny")
			return err
		})
		return nil
	},
})
`), 0o644); err != nil {
		t.Fatal(err)
	}
	setTestFrameworkDependencies(t, []moduleDependency{{Module: "github.com/tty00a381/anybot", Version: "v0.0.0", Replace: anybotRoot}})
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	commandRunner = func(dir, name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GOCACHE="+cacheDir,
			"GOMODCACHE="+modCacheDir,
			"GOFLAGS=-modcacherw",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if detail := strings.TrimSpace(string(out)); detail != "" {
				return errors.Join(err, errors.New(detail))
			}
			return err
		}
		return nil
	}
	_, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"plugin", "add", "example.com/weather", "-replace", pluginDir, "-dir", botDir}); err != nil {
		t.Fatal(err)
	}
	id := readOnlyPluginID(t, botDir)
	if err := run([]string{"plugin", "enable", host.ShortPluginID(id), "-dir", botDir}); err != nil {
		t.Fatal(err)
	}
	configPath := host.ConfigPath(botDir)
	config := readTestFile(t, configPath)
	config = strings.ReplaceAll(config, `listen: "127.0.0.1:6700"`, `listen: "127.0.0.1:0"`)
	writeTestConfig(t, configPath, config)
	if err := run([]string{"build", "-dir", botDir}); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(botDir, "anybot-bot")
	check := exec.Command(binary, "plugin", "check")
	check.Dir = botDir
	out, err := check.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), host.ShortPluginID(id)) || !strings.Contains(string(out), "可用") {
		t.Fatalf("plugin check output:\n%s", out)
	}
	inspect := exec.Command(binary, "plugin", "inspect", host.ShortPluginID(id))
	inspect.Dir = botDir
	out, err = inspect.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin inspect failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "名称：weather") ||
		!strings.Contains(string(out), "command: weather") {
		t.Fatalf("plugin inspect output:\n%s", out)
	}
}

func TestRunBuildReportsAbsoluteOutput(t *testing.T) {
	setTestFrameworkDependencies(t, releaseFrameworkDependencies("v9.9.9"))
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	commandRunner = func(dir, name string, args ...string) error {
		return nil
	}
	dir := t.TempDir()
	out, _, restore := captureOutput(t)
	defer restore()
	output := filepath.Join(t.TempDir(), "bot")
	if err := run([]string{"build", "-dir", dir, "-o", output}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "构建完成："+output) {
		t.Fatalf("build output:\n%s", out.String())
	}
}

func TestRunBuildSyncsLocalFrameworkReplace(t *testing.T) {
	deps := []moduleDependency{
		{Module: "github.com/tty00a381/anybot", Version: "v0.0.0", Replace: "../anybot"},
	}
	setTestFrameworkDependencies(t, deps)
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	dir := t.TempDir()
	if err := run([]string{"build", "-dir", dir, "-o", "bot", "-skip-tidy"}); err != nil {
		t.Fatal(err)
	}
	want := append(frameworkGoModCalls(dir, deps),
		dir+" go build -o bot .",
	)
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestRunBuildRejectsUnresolvedFrameworkVersion(t *testing.T) {
	setTestFrameworkDependencies(t, []moduleDependency{{Module: "github.com/tty00a381/anybot", Version: "v0.0.0"}})
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	commandRunner = func(dir, name string, args ...string) error {
		t.Fatalf("go command should not run with unresolved framework version: %s %s", name, strings.Join(args, " "))
		return nil
	}
	err := run([]string{"build", "-dir", t.TempDir(), "-o", "bot"})
	if err == nil || !strings.Contains(err.Error(), "构建生成宿主需要可解析的 AnyBot 发布版本") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunDelegatesEnabledExternalPluginToGeneratedHost(t *testing.T) {
	setTestFrameworkDependencies(t, releaseFrameworkDependencies("v9.9.9"))
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}

	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestWeatherID, Module: "github.com/acme/weather", Version: "v1.2.3"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestWeatherID, "enabled: true\nconfig: {}\n")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"run", "-config", config}); err != nil {
		t.Fatal(err)
	}
	binary := "." + string(filepath.Separator) + "anybot-bot"
	want := append(frameworkGoModCalls(dir, releaseFrameworkDependencies("v9.9.9")),
		dir+" go mod edit -require=github.com/acme/weather@v1.2.3",
		dir+" go mod edit -dropreplace=github.com/acme/weather",
		dir+" go mod tidy",
		dir+" go build -o anybot-bot .",
		dir+" "+binary+" plugin sync",
		dir+" "+binary+" plugin check",
		dir+" "+binary+" ",
	)
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(out.String(), "启动："+filepath.Join(dir, "anybot-bot")) {
		t.Fatalf("run output:\n%s", out.String())
	}
}

func TestRunDirDelegatesEnabledExternalPluginToGeneratedHost(t *testing.T) {
	setTestFrameworkDependencies(t, releaseFrameworkDependencies("v9.9.9"))
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}

	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestWeatherID, Module: "github.com/acme/weather", Version: "v1.2.3"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestWeatherID, "enabled: true\nconfig: {}\n")
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"run", "-dir", dir}); err != nil {
		t.Fatal(err)
	}
	binary := "." + string(filepath.Separator) + "anybot-bot"
	want := append(frameworkGoModCalls(dir, releaseFrameworkDependencies("v9.9.9")),
		dir+" go mod edit -require=github.com/acme/weather@v1.2.3",
		dir+" go mod edit -dropreplace=github.com/acme/weather",
		dir+" go mod tidy",
		dir+" go build -o anybot-bot .",
		dir+" "+binary+" plugin sync",
		dir+" "+binary+" plugin check",
		dir+" "+binary+" ",
	)
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(out.String(), "启动："+filepath.Join(dir, "anybot-bot")) {
		t.Fatalf("run output:\n%s", out.String())
	}
}

func TestRunRejectsDirAndConfigTogether(t *testing.T) {
	err := run([]string{"run", "-dir", t.TempDir(), "-config", host.ConfigPath(t.TempDir())})
	if err == nil || !strings.Contains(err.Error(), "不能同时指定 -dir 和 -config") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunIgnoresDisabledExternalPlugin(t *testing.T) {
	dir := t.TempDir()
	config := host.ConfigPath(dir)
	writeTestConfig(t, config, "runtime:\n  log_level: info\nadapter:\n  transport:\n    listen: \"127.0.0.1:0\"\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestWeatherID, Module: "github.com/acme/weather", Version: "v1.2.3"}); err != nil {
		t.Fatal(err)
	}
	writeTestPluginConfig(t, dir, cmdTestWeatherID, "enabled: false\nconfig: {}\n")
	cfg, err := host.LoadConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := host.LoadPluginLock(host.PluginLockPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if lockHasEnabledExternalPlugins(cfg, lock) {
		t.Fatal("disabled external plugin should not trigger generated host")
	}
}

func TestRunRejectsExternalPluginWithNonDefaultConfigPath(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "bot.yaml")
	writeTestConfig(t, config, "runtime:\n  log_level: info\n")
	if _, err := host.AddPluginInstall(host.AddPluginInstallOptions{Dir: dir, ID: cmdTestWeatherID, Module: "github.com/acme/weather", Version: "v1.2.3"}); err != nil {
		t.Fatal(err)
	}
	writeTestConfig(t, filepath.Join(dir, cmdTestWeatherID+".yaml"), "enabled: true\nconfig: {}\n")
	err := run([]string{"run", "-config", config})
	if err == nil || !strings.Contains(err.Error(), "需要使用工作目录中的 config/anybot.yaml") {
		t.Fatalf("err = %v", err)
	}
}

func TestDetectFrameworkDependencyFindsSourceRoot(t *testing.T) {
	dep := detectFrameworkDependency("github.com/tty00a381/anybot")
	if dep.Module != "github.com/tty00a381/anybot" || dep.Version != "v0.0.0" {
		t.Fatalf("dependency = %#v", dep)
	}
	if dep.Replace == "" {
		t.Fatalf("expected local replace, got %#v", dep)
	}
	if _, err := os.Stat(filepath.Join(dep.Replace, "go.mod")); err != nil {
		t.Fatalf("replace root %q: %v", dep.Replace, err)
	}
}

func TestDetectFrameworkDependencyPrefersLocalSourceOverPseudoVersion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/tty00a381/anybot\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	setTestModuleSourceRoot(t, root, true)
	setTestBuildInfoMainVersion(t, "v1.0.1-0.20260608195323-a5dba06356cb")

	dep := detectFrameworkDependency("github.com/tty00a381/anybot")
	if dep.Version != "v0.0.0" || dep.Replace != root {
		t.Fatalf("dependency = %#v", dep)
	}
}

func TestDetectFrameworkDependencyUsesBuildVersionForModuleCacheSource(t *testing.T) {
	root := filepath.Join(t.TempDir(), "go", "pkg", "mod", "github.com", "tty00a381", "anybot@v1.2.3")
	setTestModuleSourceRoot(t, root, true)
	setTestBuildInfoMainVersion(t, "v1.2.3")

	dep := detectFrameworkDependency("github.com/tty00a381/anybot")
	if dep.Version != "v1.2.3" || dep.Replace != "" {
		t.Fatalf("dependency = %#v", dep)
	}
}

func TestBinaryPathHelpers(t *testing.T) {
	absOutput := filepath.Join(string(filepath.Separator), "tmp", "bot")
	tests := []struct {
		name    string
		dir     string
		output  string
		built   string
		command string
	}{
		{
			name:    "default output in current directory",
			dir:     ".",
			output:  "",
			built:   "." + string(filepath.Separator) + "anybot-bot",
			command: "." + string(filepath.Separator) + "anybot-bot",
		},
		{
			name:    "relative lock output",
			dir:     "bot",
			output:  "anybot-bot",
			built:   filepath.Join("bot", "anybot-bot"),
			command: "." + string(filepath.Separator) + "anybot-bot",
		},
		{
			name:    "relative output with directory",
			dir:     ".",
			output:  filepath.Join("bin", "bot"),
			built:   filepath.Join("bin", "bot"),
			command: filepath.Join("bin", "bot"),
		},
		{
			name:    "absolute output",
			dir:     "/lock",
			output:  absOutput,
			built:   absOutput,
			command: absOutput,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := builtBinaryPath(tt.dir, tt.output); got != tt.built {
				t.Fatalf("builtBinaryPath() = %q, want %q", got, tt.built)
			}
			if got := commandBinaryPath(tt.output); got != tt.command {
				t.Fatalf("commandBinaryPath() = %q, want %q", got, tt.command)
			}
		})
	}
}

func TestRunUpBuildsSyncsAndRuns(t *testing.T) {
	setTestFrameworkDependencies(t, releaseFrameworkDependencies("v9.9.9"))
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	dir := t.TempDir()
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"up", "-dir", dir, "-o", "bot"}); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "bot")
	command := "." + string(filepath.Separator) + "bot"
	want := append(frameworkGoModCalls(dir, releaseFrameworkDependencies("v9.9.9")),
		dir+" go mod tidy",
		dir+" go build -o bot .",
		dir+" "+command+" plugin sync",
		dir+" "+command+" plugin check",
		dir+" "+command+" ",
	)
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(out.String(), "启动："+binary) {
		t.Fatalf("up output:\n%s", out.String())
	}
}

func TestRunUpUsesLocalBinaryForDefaultOutput(t *testing.T) {
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	out, _, restore := captureOutput(t)
	defer restore()
	if err := run([]string{"up", "-skip-build"}); err != nil {
		t.Fatal(err)
	}
	binary := "." + string(filepath.Separator) + "anybot-bot"
	want := []string{
		". " + binary + " plugin sync",
		". " + binary + " plugin check",
		". " + binary + " ",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(out.String(), "启动："+binary) {
		t.Fatalf("up output:\n%s", out.String())
	}
}

func TestRunUpCanSkipPluginCheck(t *testing.T) {
	oldRunner := commandRunner
	defer func() { commandRunner = oldRunner }()
	var calls []string
	commandRunner = func(dir, name string, args ...string) error {
		calls = append(calls, dir+" "+name+" "+strings.Join(args, " "))
		return nil
	}
	if err := run([]string{"up", "-skip-build", "-skip-check"}); err != nil {
		t.Fatal(err)
	}
	binary := "." + string(filepath.Separator) + "anybot-bot"
	want := []string{
		". " + binary + " plugin sync",
		". " + binary + " ",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
}

func captureOutput(t *testing.T) (*bytes.Buffer, *bytes.Buffer, func()) {
	t.Helper()
	oldStdout := stdout
	oldStderr := stderr
	var out bytes.Buffer
	var errOut bytes.Buffer
	stdout = &out
	stderr = &errOut
	return &out, &errOut, func() {
		stdout = oldStdout
		stderr = oldStderr
	}
}

func setTestFrameworkDependencies(t *testing.T, deps []moduleDependency) {
	t.Helper()
	old := frameworkDependencies
	frameworkDependencies = func() []moduleDependency {
		return append([]moduleDependency(nil), deps...)
	}
	t.Cleanup(func() {
		frameworkDependencies = old
	})
}

func setTestBuildInfoMainVersion(t *testing.T, version string) {
	t.Helper()
	old := buildInfoMainVersion
	buildInfoMainVersion = func() string {
		return version
	}
	t.Cleanup(func() {
		buildInfoMainVersion = old
	})
}

func setTestModuleSourceRoot(t *testing.T, root string, ok bool) {
	t.Helper()
	old := moduleSourceRoot
	moduleSourceRoot = func(string) (string, bool) {
		return root, ok
	}
	t.Cleanup(func() {
		moduleSourceRoot = old
	})
}

func setTestModuleVersionResolver(t *testing.T, resolver func(module, query string) (string, error)) {
	t.Helper()
	old := moduleVersionResolver
	moduleVersionResolver = resolver
	t.Cleanup(func() {
		moduleVersionResolver = old
	})
}

func releaseFrameworkDependencies(version string) []moduleDependency {
	return []moduleDependency{
		{Module: "github.com/tty00a381/anybot", Version: version},
	}
}

func frameworkGoModCalls(dir string, deps []moduleDependency) []string {
	var calls []string
	for _, dep := range deps {
		version := dep.Version
		if version == "" {
			version = "v0.0.0"
		}
		calls = append(calls, dir+" go mod edit -require="+dep.Module+"@"+version)
		if dep.Replace != "" {
			calls = append(calls, dir+" go mod edit -replace="+dep.Module+"="+dep.Replace)
		} else {
			calls = append(calls, dir+" go mod edit -dropreplace="+dep.Module)
		}
	}
	return calls
}

func readOnlyPluginID(t *testing.T, dir string) string {
	t.Helper()
	lock, err := host.LoadPluginLock(host.PluginLockPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Plugins) != 1 {
		t.Fatalf("lock plugins = %#v", lock.Plugins)
	}
	return lock.Plugins[0].ID
}

func pluginIDByBuiltin(t *testing.T, dir, source string) string {
	t.Helper()
	lock, err := host.LoadPluginLock(host.PluginLockPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range lock.Plugins {
		if item.Builtin == source {
			return item.ID
		}
	}
	t.Fatalf("builtin %q not found in lock: %#v", source, lock.Plugins)
	return ""
}

func onlyExternalPluginID(t *testing.T, dir string) string {
	t.Helper()
	lock, err := host.LoadPluginLock(host.PluginLockPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, item := range lock.Plugins {
		if item.Module != "" {
			if id != "" {
				t.Fatalf("multiple external plugins in lock: %#v", lock.Plugins)
			}
			id = item.ID
		}
	}
	if id == "" {
		t.Fatalf("no external plugin in lock: %#v", lock.Plugins)
	}
	return id
}

func writeTestPluginConfig(t *testing.T, dir, id, content string) string {
	t.Helper()
	path := filepath.Join(dir, host.ConfigDirName, id+".yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestConfig(t, path, content)
	return path
}

func writeTestConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
