package host

import (
	"bytes"
	"strings"
	"testing"
)

func TestWritePluginConfigSyncSummary(t *testing.T) {
	var out bytes.Buffer
	err := WritePluginConfigSyncSummary(&out, PluginConfigSyncResult{
		Changed:         2,
		Skipped:         []string{"weather"},
		UnknownDisabled: []string{"legacy"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"插件配置已同步：2 项更新",
		"外部插件待构建：weather",
		"未知插件已跳过：legacy（已禁用）",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}
