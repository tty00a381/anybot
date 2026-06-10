package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"golang.org/x/mod/modfile"
	modmodule "golang.org/x/mod/module"
)

//go:embed templates/plugin/*.tmpl
var scaffoldTemplates embed.FS

const (
	pluginFileTemplate   = "templates/plugin/plugin.go.tmpl"
	pluginTestTemplate   = "templates/plugin/plugin_test.go.tmpl"
	pluginReadmeTemplate = "templates/plugin/README.md.tmpl"
	pluginGoModTemplate  = "templates/plugin/go.mod.tmpl"
	pluginGoSumTemplate  = "templates/plugin/go.sum.tmpl"
)

// PluginOptions 配置插件脚手架的目标项目、插件名和覆盖策略。
type PluginOptions struct {
	Dir           string
	Name          string
	Module        string
	Force         bool
	AnyBotVersion string
	AnyBotReplace string
}

// PluginResult 描述插件脚手架写入结果。
type PluginResult struct {
	Name      string // 面向用户展示的插件名。
	Package   string // 生成的 Go 包名和文件名前缀。
	Module    string
	TestReady bool
	Files     []string
}

type scaffoldFile struct {
	Name    string
	Content string
}

type pluginScaffoldData struct {
	Package       string
	Manifest      string
	DisplayName   string
	Command       string
	Module        string
	AnyBotVersion string
	AnyBotReplace string
}

// NewPlugin 写入插件模块 starter。
func NewPlugin(opts PluginOptions) (PluginResult, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Name == "" {
		return PluginResult{}, fmt.Errorf("插件名不能为空")
	}
	opts.Module = strings.TrimSpace(opts.Module)
	if opts.Module == "" {
		return PluginResult{}, fmt.Errorf("插件模块路径不能为空")
	}
	pkg := packageName(opts.Name)
	displayName := pluginDisplayName(opts.Name, pkg)
	data := pluginScaffoldData{
		Package:       pkg,
		Manifest:      displayName,
		DisplayName:   displayName,
		Command:       pkg,
		Module:        opts.Module,
		AnyBotVersion: anybotVersion(opts.AnyBotVersion),
		AnyBotReplace: strings.TrimSpace(opts.AnyBotReplace),
	}
	return newPluginModule(opts, data)
}

func newPluginModule(opts PluginOptions, data pluginScaffoldData) (PluginResult, error) {
	if err := modmodule.CheckPath(opts.Module); err != nil {
		return PluginResult{}, fmt.Errorf("插件模块路径 %q 无效: %w", opts.Module, err)
	}
	goMod, err := pluginGoMod(data)
	if err != nil {
		return PluginResult{}, err
	}
	files := []scaffoldFile{
		{Name: "go.mod", Content: goMod},
		{Name: data.Package + ".go", Content: mustFormat(renderTemplate(pluginFileTemplate, data))},
		{Name: data.Package + "_test.go", Content: mustFormat(renderTemplate(pluginTestTemplate, data))},
		{Name: "README.md", Content: renderTemplate(pluginReadmeTemplate, data)},
	}
	testReady := data.AnyBotReplace != ""
	if testReady {
		files = insertScaffoldFile(files, 1, scaffoldFile{Name: "go.sum", Content: renderTemplate(pluginGoSumTemplate, data)})
	}
	if err := checkFileConflicts(opts.Dir, files, opts.Force); err != nil {
		return PluginResult{}, err
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(opts.Dir, file.Name)
		if err := writeFile(path, file.Content, opts.Force); err != nil {
			return PluginResult{}, err
		}
		paths = append(paths, path)
	}
	return PluginResult{Name: data.Manifest, Package: data.Package, Module: opts.Module, TestReady: testReady, Files: paths}, nil
}

func insertScaffoldFile(files []scaffoldFile, index int, file scaffoldFile) []scaffoldFile {
	files = append(files, scaffoldFile{})
	copy(files[index+1:], files[index:])
	files[index] = file
	return files
}

func anybotVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "v0.0.0"
	}
	return version
}

func pluginGoMod(data pluginScaffoldData) (string, error) {
	if data.AnyBotReplace == "" && data.AnyBotVersion == "v0.0.0" {
		return "", fmt.Errorf("插件模块需要有效 AnyBot 版本，或使用 -replace 指向本地 AnyBot 源码")
	}
	goMod := renderTemplate(pluginGoModTemplate, data)
	file, err := modfile.Parse("go.mod", []byte(goMod), nil)
	if err != nil {
		return "", fmt.Errorf("插件 go.mod 无效: %w", err)
	}
	if data.AnyBotReplace != "" {
		if err := file.AddReplace("github.com/tty00a381/anybot", "", data.AnyBotReplace, ""); err != nil {
			return "", fmt.Errorf("插件 go.mod replace 无效: %w", err)
		}
	}
	out, err := file.Format()
	if err != nil {
		return "", fmt.Errorf("插件 go.mod 格式化失败: %w", err)
	}
	return string(out), nil
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

func checkFileConflicts(dir string, files []scaffoldFile, force bool) error {
	if force {
		return nil
	}
	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s 已存在；使用 -force 覆盖", path)
		}
	}
	return nil
}

func renderTemplate(name string, data any) string {
	src, err := scaffoldTemplates.ReadFile(name)
	if err != nil {
		panic(err)
	}
	tpl := template.Must(template.New(filepath.Base(name)).Parse(string(src)))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}

func mustFormat(src string) string {
	out, err := format.Source([]byte(src))
	if err != nil {
		return src
	}
	return string(out)
}

func packageName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	lastSep := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastSep = false
		case r >= '0' && r <= '9' && b.Len() > 0:
			b.WriteRune(r)
			lastSep = false
		case r == '_' || r == '-' || r == ' ':
			if b.Len() > 0 && !lastSep {
				b.WriteByte('_')
				lastSep = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "plugin"
	}
	if token.Lookup(out).IsKeyword() {
		return out + "_plugin"
	}
	return out
}

func pluginDisplayName(name, fallback string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	lastSep := false
	for _, r := range name {
		switch {
		case r == '"' || r == '\\' || r < 0x20 || r == 0x7f:
			if b.Len() > 0 && !lastSep {
				b.WriteByte('-')
				lastSep = true
			}
		case r == '_' || r == '-' || unicode.IsSpace(r):
			if b.Len() > 0 && !lastSep {
				b.WriteByte('-')
				lastSep = true
			}
		default:
			b.WriteRune(r)
			lastSep = false
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return fallback
	}
	return out
}
