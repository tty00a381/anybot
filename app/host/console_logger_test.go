package host

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewLoggerConsoleOmitsNilErrorAndEmptyFields(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger("info", "console", &buf)
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("路由完成", "route", "chat.command", "error", nil, "group_id", "")
	out := buf.String()
	if !strings.Contains(out, "INFO  路由完成") || !strings.Contains(out, "route=chat.command") {
		t.Fatalf("console output = %q", out)
	}
	for _, noisy := range []string{"error=", "group_id="} {
		if strings.Contains(out, noisy) {
			t.Fatalf("console output contains noisy field %q: %q", noisy, out)
		}
	}
}

func TestConsoleHandlerHonorsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger("warn", "console", &buf)
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("hidden")
	logger.Warn("shown")
	if out := buf.String(); strings.Contains(out, "hidden") || !strings.Contains(out, "WARN  shown") {
		t.Fatalf("console output = %q", out)
	}
}

func TestNewLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger("info", "json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("hello", "route", "chat.command")
	if out := buf.String(); !strings.Contains(out, `"msg":"hello"`) || !strings.Contains(out, `"route":"chat.command"`) {
		t.Fatalf("json output = %q", out)
	}
}

func TestConsoleHandlerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newConsoleHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.WithGroup("plugin").Info("ok", "id", "plg_1", "at", time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC))
	if out := buf.String(); !strings.Contains(out, "plugin.id=plg_1") {
		t.Fatalf("grouped output = %q", out)
	}
}
