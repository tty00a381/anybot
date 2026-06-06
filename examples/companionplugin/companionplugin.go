package companionplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Command       string        `yaml:"command"`
	ProviderURL   string        `yaml:"provider_url"`
	APIKey        string        `yaml:"api_key"`
	Model         string        `yaml:"model"`
	Persona       string        `yaml:"persona"`
	MemoryTTL     time.Duration `yaml:"memory_ttl"`
	MaxHistory    int           `yaml:"max_history"`
	AllowedGroups []string      `yaml:"allowed_groups"`
	Timeout       time.Duration `yaml:"timeout"`
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Command) == "" {
		return fmt.Errorf("companion.command 不能为空")
	}
	if cfg.MemoryTTL < 0 {
		return fmt.Errorf("companion.memory_ttl 不能小于 0")
	}
	if cfg.MaxHistory < 0 {
		return fmt.Errorf("companion.max_history 不能小于 0")
	}
	if cfg.Timeout < 0 {
		return fmt.Errorf("companion.timeout 不能小于 0")
	}
	return nil
}

var Module = absdk.Define(
	absdk.Manifest{Name: "companion", Version: "1.0.0", Description: "人格化聊天插件起步示例"},
	Config{
		Command:    "chat",
		Model:      "local",
		Persona:    "你是一个温暖、自然、有边界感的群聊伙伴。回答要短，先回应对方的情绪，再给出有用信息。",
		MemoryTTL:  24 * time.Hour,
		MaxHistory: 12,
		Timeout:    20 * time.Second,
	},
	func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).
			Name("companion.command").
			Use(absdk.RateLimit(12, time.Minute)).
			Handle(func(c *absdk.EventContext) error {
				text := strings.TrimSpace(c.Args())
				if text == "" {
					_, err := c.ReplyText("用法：/" + cfg.Command + " 想聊的话")
					return err
				}
				return replyCompanion(ctx, c, cfg, text)
			})

		ctx.OnMessage(absdk.All(absdk.ToMe(), absdk.NotFromSelf(), allowedGroups(cfg))).
			Name("companion.mention").
			Use(absdk.RateLimit(20, time.Minute)).
			Handle(func(c *absdk.EventContext) error {
				return replyCompanion(ctx, c, cfg, strings.TrimSpace(c.Text()))
			})

		return nil
	},
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func replyCompanion(ctx *absdk.Context, c *absdk.EventContext, cfg Config, text string) error {
	if text == "" {
		return c.Pass()
	}
	history := loadHistory(ctx, c)
	messages := buildMessages(cfg, history, text)
	answer, err := completeChat(c.Context, cfg, messages)
	if err != nil {
		ctx.Logger().Warn("companion provider failed", "error", err)
		answer = "我刚刚有点接不上外部服务，但我先把这句记下来了：" + trimForReply(text)
	}
	history = append(history, chatMessage{Role: "user", Content: text}, chatMessage{Role: "assistant", Content: answer})
	saveHistory(ctx, c, cfg, trimHistory(history, cfg.MaxHistory))
	_, err = c.ReplyText(answer)
	return err
}

func buildMessages(cfg Config, history []chatMessage, text string) []chatMessage {
	messages := []chatMessage{{Role: "system", Content: cfg.Persona}}
	messages = append(messages, trimHistory(history, cfg.MaxHistory)...)
	messages = append(messages, chatMessage{Role: "user", Content: text})
	return messages
}

func completeChat(ctx context.Context, cfg Config, messages []chatMessage) (string, error) {
	if strings.TrimSpace(cfg.ProviderURL) == "" {
		return "我听见了：" + trimForReply(messages[len(messages)-1].Content), nil
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(map[string]any{
		"model":    cfg.Model,
		"messages": messages,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, cfg.ProviderURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("provider status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		Choices []struct {
			Message chatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("provider returned empty choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func loadHistory(ctx *absdk.Context, c *absdk.EventContext) []chatMessage {
	var history []chatMessage
	_, _ = ctx.UserSession(c).LoadJSON(c.Context, "history", &history)
	return history
}

func saveHistory(ctx *absdk.Context, c *absdk.EventContext, cfg Config, history []chatMessage) {
	ttl := cfg.MemoryTTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	_ = ctx.UserSession(c).SaveJSON(c.Context, "history", history, ttl)
}

func trimHistory(history []chatMessage, max int) []chatMessage {
	if max == 0 {
		max = 12
	}
	if len(history) <= max {
		return append([]chatMessage(nil), history...)
	}
	return append([]chatMessage(nil), history[len(history)-max:]...)
}

func trimForReply(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= 80 {
		return text
	}
	return string(runes[:80]) + "..."
}

func allowedGroups(cfg Config) absdk.Rule {
	allowed := map[string]struct{}{}
	for _, id := range cfg.AllowedGroups {
		id = strings.TrimSpace(id)
		if id != "" {
			allowed[id] = struct{}{}
		}
	}
	return absdk.RuleFunc(func(_ context.Context, c *absdk.EventContext) (absdk.Match, bool) {
		if len(allowed) == 0 || c.GroupID() == "" {
			return absdk.Match{Reason: "companion.group:all", Score: 1}, true
		}
		_, ok := allowed[c.GroupID()]
		return absdk.Match{Reason: "companion.group", Score: 1}, ok
	})
}
