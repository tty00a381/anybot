package main

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"text/template"
)

//go:embed templates/init/*.tmpl templates/init/plugins/*.tmpl
var commandTemplates embed.FS

const (
	defaultConfigTemplate        = "templates/init/anybot.yaml.tmpl"
	defaultReadmeTemplate        = "templates/init/README.md.tmpl"
	defaultEnvExampleTemplate    = "templates/init/env.example.tmpl"
	defaultHelpPluginTemplate    = "templates/init/plugins/help.yaml.tmpl"
	defaultEchoPluginTemplate    = "templates/init/plugins/echo.yaml.tmpl"
	defaultBuiltinPluginTemplate = "templates/init/plugins/default.yaml.tmpl"
)

func usage() {
	fmt.Fprintln(stdout, `anybot 命令：
  anybot init [目录] [-dir 目录] [-force]
  anybot run [-dir 目录|-config config/anybot.yaml]
  anybot doctor [-config config/anybot.yaml] [-connect]
  anybot build [-dir 目录] [-o anybot-bot] [-skip-tidy]
  anybot up [-dir 目录] [-o anybot-bot] [-skip-tidy] [-skip-build] [-skip-sync] [-skip-check]
  anybot plugins
  anybot plugin add <module[@version]> [-version 版本] [-replace 本地路径] [-dir 目录]
  anybot plugin update <id> [-version 版本] [-replace 本地路径|-clear-replace] [-dir 目录]
  anybot plugin remove <id> [-dir 目录|-config config/anybot.yaml]
  anybot plugin list [-dir 目录]
  anybot plugin status [-dir 目录|-config config/anybot.yaml]
  anybot plugin inspect <id> [-dir 目录|-config config/anybot.yaml]
  anybot plugin config <id> <key=value>... [-dir 目录|-config config/anybot.yaml]
  anybot plugin config <id> -reset <key>... [-dir 目录|-config config/anybot.yaml]
  anybot plugin check [-dir 目录|-config config/anybot.yaml]
  anybot plugin sync [-dir 目录|-config config/anybot.yaml]
  anybot plugin enable <id> [-dir 目录|-config config/anybot.yaml]
  anybot plugin disable <id> [-dir 目录|-config config/anybot.yaml]
  anybot dev plugin <名称> [-dir 目录] [-force] [-module 插件模块] [-anybot-version 版本] [-replace AnyBot源码路径]
  anybot dev new plugin <名称> [同 anybot dev plugin]
  anybot version`)
}

var defaultConfig = renderCommandTemplate(defaultConfigTemplate, nil)
var defaultReadme = renderCommandTemplate(defaultReadmeTemplate, nil)
var defaultEnvExample = renderCommandTemplate(defaultEnvExampleTemplate, nil)
var defaultHelpPluginConfig = renderCommandTemplate(defaultHelpPluginTemplate, nil)
var defaultEchoPluginConfig = renderCommandTemplate(defaultEchoPluginTemplate, nil)

func defaultBuiltinPluginConfig(source string) string {
	switch source {
	case "help":
		return defaultHelpPluginConfig
	case "echo":
		return defaultEchoPluginConfig
	default:
		return renderCommandTemplate(defaultBuiltinPluginTemplate, nil)
	}
}

func renderCommandTemplate(name string, data any) string {
	src, err := commandTemplates.ReadFile(name)
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
