package host

import (
	"fmt"
	"io"
	"strings"
)

// WritePluginConfigSyncSummary 输出插件配置同步结果摘要。
func WritePluginConfigSyncSummary(w io.Writer, result PluginConfigSyncResult) error {
	if _, err := fmt.Fprintf(w, "插件配置已同步：%d 项更新\n", result.Changed); err != nil {
		return err
	}
	if len(result.Skipped) > 0 {
		if _, err := fmt.Fprintf(w, "外部插件待构建：%s（anybot run 会同步默认配置）\n", strings.Join(result.Skipped, ", ")); err != nil {
			return err
		}
	}
	if len(result.UnknownDisabled) > 0 {
		if _, err := fmt.Fprintf(w, "未知插件已跳过：%s（已禁用）\n", strings.Join(result.UnknownDisabled, ", ")); err != nil {
			return err
		}
	}
	return nil
}
