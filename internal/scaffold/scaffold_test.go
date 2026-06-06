package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	readmeData, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readmeData), "NapCat") || !strings.Contains(string(readmeData), "运行") {
		t.Fatalf("README.md 内容不符合预期:\n%s", readmeData)
	}
	if err := NewPlugin(PluginOptions{Dir: dir, Name: "hello-world"}); err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(dir, "plugins", "hello_world", "hello_world.go")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatal(err)
	}
	pluginData, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pluginData), "var Module = absdk.Define") ||
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
	if err := NewPlugin(PluginOptions{Dir: dir, Name: "hello-world"}); err != nil {
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
