package host

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
)

type consoleHandler struct {
	out   io.Writer
	opts  slog.HandlerOptions
	attrs []slog.Attr
	group string
	mu    *sync.Mutex
}

func newConsoleHandler(out io.Writer, opts *slog.HandlerOptions) slog.Handler {
	if out == nil {
		out = io.Discard
	}
	var copied slog.HandlerOptions
	if opts != nil {
		copied = *opts
	}
	return &consoleHandler{out: out, opts: copied, mu: &sync.Mutex{}}
}

func (h *consoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	min := slog.LevelInfo
	if h.opts.Level != nil {
		min = h.opts.Level.Level()
	}
	return level >= min
}

func (h *consoleHandler) Handle(ctx context.Context, record slog.Record) error {
	var fields []string
	for _, attr := range h.attrs {
		if field := h.formatAttr(attr); field != "" {
			fields = append(fields, field)
		}
	}
	record.Attrs(func(attr slog.Attr) bool {
		if field := h.formatAttr(h.withCurrentGroup(attr)); field != "" {
			fields = append(fields, field)
		}
		return true
	})
	line := record.Time.Format("15:04:05") + " " + levelLabel(record.Level) + " " + record.Message
	if len(fields) > 0 {
		line += "  " + strings.Join(fields, " ")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := fmt.Fprintln(h.out, line)
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = slices.Clone(h.attrs)
	for _, attr := range attrs {
		next.attrs = append(next.attrs, h.withCurrentGroup(attr))
	}
	return &next
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	name = strings.TrimSpace(name)
	if name == "" {
		return h
	}
	next := *h
	if next.group == "" {
		next.group = name
	} else {
		next.group += "." + name
	}
	return &next
}

func (h *consoleHandler) formatAttr(attr slog.Attr) string {
	attr.Value = attr.Value.Resolve()
	if isEmptyAttr(attr) {
		return ""
	}
	key := strings.TrimSpace(attr.Key)
	if key == "" {
		return ""
	}
	return key + "=" + formatConsoleValue(attr.Value)
}

func (h *consoleHandler) withCurrentGroup(attr slog.Attr) slog.Attr {
	if h.group == "" || strings.TrimSpace(attr.Key) == "" {
		return attr
	}
	attr.Key = h.group + "." + attr.Key
	return attr
}

func isEmptyAttr(attr slog.Attr) bool {
	if strings.TrimSpace(attr.Key) == "" {
		return true
	}
	switch attr.Value.Kind() {
	case slog.KindAny:
		return attr.Value.Any() == nil
	case slog.KindString:
		return strings.TrimSpace(attr.Value.String()) == ""
	case slog.KindDuration:
		return attr.Value.Duration() == 0
	case slog.KindTime:
		return attr.Value.Time().IsZero()
	case slog.KindGroup:
		return len(attr.Value.Group()) == 0
	default:
		return false
	}
}

func formatConsoleValue(value slog.Value) string {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return quoteConsole(value.String())
	case slog.KindDuration:
		return value.Duration().Round(time.Millisecond).String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339)
	case slog.KindBool:
		if value.Bool() {
			return "true"
		}
		return "false"
	case slog.KindInt64:
		return fmt.Sprint(value.Int64())
	case slog.KindUint64:
		return fmt.Sprint(value.Uint64())
	case slog.KindFloat64:
		return fmt.Sprint(value.Float64())
	case slog.KindGroup:
		var parts []string
		for _, attr := range value.Group() {
			if isEmptyAttr(attr) {
				continue
			}
			parts = append(parts, attr.Key+":"+formatConsoleValue(attr.Value))
		}
		return "{" + strings.Join(parts, ",") + "}"
	default:
		return quoteConsole(fmt.Sprint(value.Any()))
	}
}

func quoteConsole(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\r=") {
		return fmt.Sprintf("%q", value)
	}
	return value
}

func levelLabel(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return "DEBUG"
	case level < slog.LevelWarn:
		return "INFO "
	case level < slog.LevelError:
		return "WARN "
	default:
		return "ERROR"
	}
}
