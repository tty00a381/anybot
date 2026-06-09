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

func TestInitProjectAndPlugin(t *testing.T) {
	dir := t.TempDir()
	if err := InitProject(ProjectOptions{Dir: dir, Module: "example.com/demo"}); err != nil {
		t.Fatal(err)
	}
	mainData, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainData), "onebot11.LoadAdapter") {
		t.Fatalf("main.go does not contain adapter setup:\n%s", mainData)
	}
	if !strings.Contains(string(mainData), "--help") {
		t.Fatalf("main.go 缺少帮助入口:\n%s", mainData)
	}
	configData, err := os.ReadFile(filepath.Join(dir, "core.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configData), "action_timeout: 10s") {
		t.Fatalf("core.yaml 内容不符合预期:\n%s", configData)
	}
	if !strings.Contains(string(configData), "max_event_bytes: 1048576") {
		t.Fatalf("core.yaml 缺少事件大小上限:\n%s", configData)
	}
	readmeData, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readmeData), "NapCat") ||
		!strings.Contains(string(readmeData), "运行") ||
		!strings.Contains(string(readmeData), "export ONEBOT_ACCESS_TOKEN=你的令牌") {
		t.Fatalf("README.md 内容不符合预期:\n%s", readmeData)
	}
	readme := string(readmeData)
	for _, want := range []string{
		"anybot dev plugin hello -in-project",
		"anybot dev plugin hello -dir ../anybot-hello -module example.com/hello",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README.md 缺少插件命令 %q:\n%s", want, readme)
		}
	}
	result, err := NewPlugin(PluginOptions{Dir: dir, Name: "hello-world"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "hello_world" || result.Package != "hello_world" || result.Standalone {
		t.Fatalf("plugin result = %#v", result)
	}
	pluginPath := filepath.Join(dir, "plugins", "hello_world", "hello_world.go")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatal(err)
	}
	pluginData, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pluginData), "var Plugin = absdk.Define") ||
		!strings.Contains(string(pluginData), "absdk.EventContext") ||
		!strings.Contains(string(pluginData), "func (cfg Config) Validate() error") ||
		!strings.Contains(string(pluginData), `Command string `+"`yaml:\"command\"`") ||
		strings.Contains(string(pluginData), "github.com/tty00a381/anybot/core") {
		t.Fatalf("插件模板缺少 SDK 模块入口:\n%s", pluginData)
	}
}

func TestInitProjectConflictDoesNotPartiallyWrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("已有说明"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InitProject(ProjectOptions{Dir: dir, Module: "example.com/demo"}); err == nil {
		t.Fatal("存在文件时应拒绝生成")
	}
	for _, name := range []string{"go.mod", "main.go", "core.yaml", ".env.example"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Fatalf("%s 不应被部分写入", name)
		}
	}
}

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

func TestGeneratedProjectSmoke(t *testing.T) {
	dir := t.TempDir()
	if err := InitProject(ProjectOptions{Dir: dir, Module: "example.com/demo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPlugin(PluginOptions{Dir: dir, Name: "hello-world"}); err != nil {
		t.Fatal(err)
	}
	root := repoRoot(t)
	goMod := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(goMod)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\nrequire github.com/tty00a381/anybot v0.0.0\nreplace github.com/tty00a381/anybot => "+root+"\n")...)
	if err := os.WriteFile(goMod, data, 0o644); err != nil {
		t.Fatal(err)
	}
	runGo(t, dir, "mod", "tidy")
	runGo(t, dir, "test", "./...")
	out := runGo(t, dir, "run", ".", "--help")
	if !strings.Contains(out, "用法：go run .") {
		t.Fatalf("help 输出不符合预期:\n%s", out)
	}
}

func TestGeneratedStandalonePluginSmoke(t *testing.T) {
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
	if !result.Standalone || result.Name != "hello_world" || result.Module != "example.com/hello-world" {
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
		!strings.Contains(plugin, "absdk.Define") {
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

func TestGeneratedStandalonePluginWithReleaseVersionRequiresTidy(t *testing.T) {
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

func TestGeneratedStandalonePluginRejectsUnknownFrameworkVersion(t *testing.T) {
	_, err := NewPlugin(PluginOptions{
		Dir:    t.TempDir(),
		Name:   "broken-plugin",
		Module: "example.com/broken-plugin",
	})
	if err == nil || !strings.Contains(err.Error(), "独立插件需要有效 AnyBot 版本") {
		t.Fatalf("err = %v", err)
	}
}

func TestGeneratedStandalonePluginSanitizesDisplayName(t *testing.T) {
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

func TestGeneratedStandalonePluginQuotesReplaceWithSpaces(t *testing.T) {
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
