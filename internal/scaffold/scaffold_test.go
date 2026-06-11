package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestPackageName(t *testing.T) {
	tests := map[string]string{
		"hello-world":     "hello_world",
		"hello---world":   "hello_world",
		"123 hello world": "hello_world",
		"type":            "type_plugin",
		"你好":              "plugin",
	}
	for input, want := range tests {
		if got := packageName(input); got != want {
			t.Fatalf("packageName(%q)=%q want %q", input, got, want)
		}
	}
}

func TestGeneratedPluginModuleSmoke(t *testing.T) {
	dir := t.TempDir()
	root := repoRoot(t)
	result, err := NewPlugin(PluginOptions{
		Dir:           dir,
		Name:          "hello-world",
		Module:        "example.com/hello-world",
		AnyBotVersion: "v0.0.0",
		AnyBotReplace: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "hello-world" || result.Module != "example.com/hello-world" {
		t.Fatalf("plugin result = %#v", result)
	}
	if !result.TestReady {
		t.Fatalf("plugin should be test-ready with local replace: %#v", result)
	}
	for _, name := range []string{"go.mod", "go.sum", "hello_world.go", "hello_world_test.go", "README.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was not generated: %v", name, err)
		}
	}
	goMod := readFile(t, filepath.Join(dir, "go.mod"))
	if !strings.Contains(goMod, "module example.com/hello-world") ||
		!strings.Contains(goMod, "replace github.com/tty00a381/anybot => "+root) ||
		!strings.Contains(goMod, "gopkg.in/yaml.v3 v3.0.1 // indirect") {
		t.Fatalf("go.mod:\n%s", goMod)
	}
	goSum := readFile(t, filepath.Join(dir, "go.sum"))
	if !strings.Contains(goSum, "gopkg.in/yaml.v3 v3.0.1 h1:") {
		t.Fatalf("go.sum:\n%s", goSum)
	}
	plugin := readFile(t, filepath.Join(dir, "hello_world.go"))
	if strings.Contains(plugin, "github.com/tty00a381/anybot/core") ||
		!strings.Contains(plugin, "absdk.Define") ||
		!strings.Contains(plugin, "absdk.Spec[Config]") ||
		!strings.Contains(plugin, `absdk.Manifest{Name: "hello-world"`) ||
		!strings.Contains(plugin, `DefaultConfig: Config{Command: "hello_world"}`) ||
		!strings.Contains(plugin, "Setup: func") ||
		!strings.Contains(plugin, `ReplyText("hello-world 已启动")`) {
		t.Fatalf("plugin scaffold:\n%s", plugin)
	}
	testFile := readFile(t, filepath.Join(dir, "hello_world_test.go"))
	if strings.Contains(testFile, "github.com/tty00a381/anybot/core") ||
		strings.Contains(testFile, "absdk.NewApp") ||
		strings.Contains(testFile, "recordClient") ||
		!strings.Contains(testFile, "github.com/tty00a381/anybot/sdk/testkit") ||
		!strings.Contains(testFile, "testkit.NewApp()") {
		t.Fatalf("plugin test scaffold:\n%s", testFile)
	}
	readme := readFile(t, filepath.Join(dir, "README.md"))
	if !strings.Contains(readme, "./anybot-bot plugin check") {
		t.Fatalf("README should suggest generated-host plugin check:\n%s", readme)
	}
	if !strings.Contains(readme, "anybot plugin add example.com/hello-world@v0.1.0\nanybot plugin status\nanybot plugin enable <id>\nanybot up") {
		t.Fatalf("README should run fixed-version installs through anybot up:\n%s", readme)
	}
	runGo(t, dir, "test", "./...")
}

func TestGeneratedPluginModuleWithReleaseVersionRequiresTidy(t *testing.T) {
	dir := t.TempDir()
	result, err := NewPlugin(PluginOptions{
		Dir:           dir,
		Name:          "release-plugin",
		Module:        "example.com/release-plugin",
		AnyBotVersion: "v1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TestReady {
		t.Fatalf("release-version scaffold should require tidy: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.sum")); !os.IsNotExist(err) {
		t.Fatalf("go.sum should not be generated without local replace: %v", err)
	}
	goMod := readFile(t, filepath.Join(dir, "go.mod"))
	if !strings.Contains(goMod, "require github.com/tty00a381/anybot v1.2.3") ||
		strings.Contains(goMod, "replace github.com/tty00a381/anybot") {
		t.Fatalf("go.mod:\n%s", goMod)
	}
}

func TestGeneratedPluginModuleRejectsUnknownFrameworkVersion(t *testing.T) {
	_, err := NewPlugin(PluginOptions{
		Dir:    t.TempDir(),
		Name:   "broken-plugin",
		Module: "example.com/broken-plugin",
	})
	if err == nil || !strings.Contains(err.Error(), "插件模块需要有效 AnyBot 版本") {
		t.Fatalf("err = %v", err)
	}
}

func TestGeneratedPluginModuleSanitizesDisplayName(t *testing.T) {
	dir := t.TempDir()
	root := repoRoot(t)
	if _, err := NewPlugin(PluginOptions{
		Dir:           dir,
		Name:          `hello-"world"`,
		Module:        "example.com/quoted-plugin",
		AnyBotVersion: "v0.0.0",
		AnyBotReplace: root,
	}); err != nil {
		t.Fatal(err)
	}
	plugin := readFile(t, filepath.Join(dir, "hello_world.go"))
	if strings.Contains(plugin, `"world"`) {
		t.Fatalf("plugin should use sanitized display text:\n%s", plugin)
	}
	runGo(t, dir, "test", "./...")
}

func TestGeneratedPluginModuleKeepsNonASCIIDisplayName(t *testing.T) {
	dir := t.TempDir()
	root := repoRoot(t)
	result, err := NewPlugin(PluginOptions{
		Dir:           dir,
		Name:          "天气提醒",
		Module:        "example.com/weather-reminder",
		AnyBotVersion: "v0.0.0",
		AnyBotReplace: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "天气提醒" || result.Package != "plugin" {
		t.Fatalf("plugin result = %#v", result)
	}
	plugin := readFile(t, filepath.Join(dir, "plugin.go"))
	if !strings.Contains(plugin, `absdk.Manifest{Name: "天气提醒"`) ||
		!strings.Contains(plugin, `ReplyText("天气提醒 已启动")`) ||
		!strings.Contains(plugin, "absdk.Spec[Config]") ||
		!strings.Contains(plugin, `Config{Command: "plugin"}`) {
		t.Fatalf("plugin scaffold:\n%s", plugin)
	}
	readme := readFile(t, filepath.Join(dir, "README.md"))
	if !strings.Contains(readme, "# 天气提醒") ||
		!strings.Contains(readme, "command: plugin") {
		t.Fatalf("README.md:\n%s", readme)
	}
	runGo(t, dir, "test", "./...")
}

func TestGeneratedPluginModuleQuotesReplaceWithSpaces(t *testing.T) {
	root := t.TempDir()
	replaceDir := filepath.Join(root, "AnyBot Source")
	if err := os.MkdirAll(replaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "plugin")
	if _, err := NewPlugin(PluginOptions{
		Dir:           dir,
		Name:          "space-path",
		Module:        "example.com/space-path",
		AnyBotVersion: "v0.0.0",
		AnyBotReplace: replaceDir,
	}); err != nil {
		t.Fatal(err)
	}
	goMod := readFile(t, filepath.Join(dir, "go.mod"))
	if _, err := modfile.Parse("go.mod", []byte(goMod), nil); err != nil {
		t.Fatalf("go.mod should be parseable: %v\n%s", err, goMod)
	}
	if !strings.Contains(goMod, `replace github.com/tty00a381/anybot => "`) ||
		!strings.Contains(goMod, `AnyBot Source"`) {
		t.Fatalf("go.mod should quote replace path containing spaces:\n%s", goMod)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位测试文件")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runGo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s 失败: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
