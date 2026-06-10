package host

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/generated/*.tmpl
var hostTemplates embed.FS

const (
	generatedPluginsTemplate = "templates/generated/plugins.gen.go.tmpl"
	generatedMainTemplate    = "templates/generated/main.go.tmpl"
	generatedGoModTemplate   = "templates/generated/go.mod.tmpl"
)

// RenderPluginHost 重写生成宿主插件注册代码，并在缺失时创建最小生成宿主。
func RenderPluginHost(dir string, lock PluginLock) error {
	return renderPluginHost(dir, lock, false)
}

func renderPluginHost(dir string, lock PluginLock, force bool) error {
	if dir == "" {
		dir = "."
	}
	lock.applyDefaults()
	if err := ValidatePluginLock(lock); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := CheckGeneratedHostWritable(dir, force); err != nil {
		return err
	}
	if err := writeGenerated(filepath.Join(dir, generatedPlugins), renderPlugins(lock), force); err != nil {
		return err
	}
	if err := writeGenerated(filepath.Join(dir, generatedMain), renderMain(), force); err != nil {
		return err
	}
	if force {
		return writeGeneratedGoMod(filepath.Join(dir, "go.mod"), renderGoMod(lock))
	}
	return writeGeneratedGoModIfMissing(filepath.Join(dir, "go.mod"), renderGoMod(lock))
}

func renderPlugins(lock PluginLock) string {
	type pluginImport struct {
		Alias  string
		ID     string
		Module string
	}
	data := struct {
		Plugins []pluginImport
	}{}
	for _, item := range lock.Plugins {
		if item.Module == "" {
			continue
		}
		i := len(data.Plugins)
		data.Plugins = append(data.Plugins, pluginImport{
			Alias:  fmt.Sprintf("plugin%d", i),
			ID:     item.ID,
			Module: item.Module,
		})
	}
	return mustFormat(renderHostTemplate(generatedPluginsTemplate, data))
}

func renderMain() string {
	return mustFormat(renderHostTemplate(generatedMainTemplate, nil))
}

func renderGoMod(lock PluginLock) string {
	lock.applyDefaults()
	return renderHostTemplate(generatedGoModTemplate, struct {
		Module string
	}{Module: lock.Module})
}

func renderHostTemplate(name string, data any) string {
	src, err := hostTemplates.ReadFile(name)
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
