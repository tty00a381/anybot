package onebot11

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 描述可从 YAML 读取的 OneBot v11 适配器配置。
type Config struct {
	Protocol  string          `yaml:"protocol"`
	Transport TransportConfig `yaml:"transport"`
}

// TransportConfig 描述 OneBot v11 传输层配置。
type TransportConfig struct {
	Type                 string            `yaml:"type"`
	Listen               string            `yaml:"listen"`
	Path                 string            `yaml:"path"`
	URL                  string            `yaml:"url"`
	AccessToken          string            `yaml:"access_token"`
	AccessTokenEnv       string            `yaml:"access_token_env"`
	Headers              map[string]string `yaml:"headers"`
	DialTimeout          string            `yaml:"dial_timeout"`
	ActionTimeout        string            `yaml:"action_timeout"`
	ReconnectInterval    string            `yaml:"reconnect_interval"`
	ReconnectMaxInterval string            `yaml:"reconnect_max_interval"`
}

// LoadConfig 从 YAML 文件读取 OneBot v11 配置；path 为空时读取 anybot.yaml。
func LoadConfig(path string) (Config, error) {
	if path == "" {
		path = "anybot.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// LoadAdapter 从 YAML 文件读取配置并创建 OneBot v11 适配器。
func LoadAdapter(path string, extra ...Option) (*Adapter, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return AdapterFromConfig(cfg, extra...)
}

// AdapterFromConfig 根据配置创建 OneBot v11 适配器，并追加额外选项。
func AdapterFromConfig(cfg Config, extra ...Option) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	opts, err := cfg.Options(extra...)
	if err != nil {
		return nil, err
	}
	switch cfg.Transport.Type {
	case "reverse_ws":
		return ReverseWS(cfg.Transport.Listen, opts...), nil
	case "websocket":
		return WebSocket(cfg.Transport.URL, opts...), nil
	case "http":
		return HTTP(cfg.Transport.URL, cfg.Transport.Listen, opts...), nil
	default:
		return nil, fmt.Errorf("onebot11: 不支持传输 %q", cfg.Transport.Type)
	}
}

// Validate 检查协议、传输类型、URL scheme 和必要字段是否有效。
func (cfg Config) Validate() error {
	if cfg.Protocol != "" && cfg.Protocol != "onebot11" {
		return fmt.Errorf("onebot11: 不支持协议 %q", cfg.Protocol)
	}
	if cfg.Transport.Path != "" && !strings.HasPrefix(cfg.Transport.Path, "/") {
		return fmt.Errorf("onebot11: transport.path 必须以 / 开头")
	}
	switch cfg.Transport.Type {
	case "reverse_ws":
		if cfg.Transport.Listen == "" {
			return fmt.Errorf("onebot11: reverse_ws 需要配置 transport.listen")
		}
	case "websocket", "http":
		if cfg.Transport.URL == "" {
			return fmt.Errorf("onebot11: %s 需要配置 transport.url", cfg.Transport.Type)
		}
		parsed, err := url.ParseRequestURI(cfg.Transport.URL)
		if err != nil {
			return err
		}
		if err := validateTransportScheme(cfg.Transport.Type, parsed.Scheme); err != nil {
			return err
		}
	default:
		return fmt.Errorf("onebot11: 不支持传输 %q", cfg.Transport.Type)
	}
	return nil
}

func validateTransportScheme(transport, scheme string) error {
	switch transport {
	case "websocket":
		if scheme == "ws" || scheme == "wss" {
			return nil
		}
		return fmt.Errorf("onebot11: websocket transport.url 必须使用 ws 或 wss")
	case "http":
		if scheme == "http" || scheme == "https" {
			return nil
		}
		return fmt.Errorf("onebot11: http transport.url 必须使用 http 或 https")
	default:
		return nil
	}
}

// Options 将配置转换为传输选项，并把 extra 追加到结果末尾。
func (cfg Config) Options(extra ...Option) ([]Option, error) {
	var opts []Option
	if cfg.Transport.Path != "" {
		opts = append(opts, WithPath(cfg.Transport.Path))
	}
	token := cfg.Transport.AccessToken
	if token == "" && cfg.Transport.AccessTokenEnv != "" {
		token = os.Getenv(cfg.Transport.AccessTokenEnv)
	}
	if token != "" {
		opts = append(opts, WithAccessToken(token))
	}
	for key, value := range cfg.Transport.Headers {
		opts = append(opts, WithHeader(key, value))
	}
	if cfg.Transport.DialTimeout != "" {
		timeout, err := time.ParseDuration(cfg.Transport.DialTimeout)
		if err != nil {
			return nil, fmt.Errorf("onebot11: dial_timeout 无效: %w", err)
		}
		opts = append(opts, WithDialTimeout(timeout))
	}
	if cfg.Transport.ActionTimeout != "" {
		timeout, err := time.ParseDuration(cfg.Transport.ActionTimeout)
		if err != nil {
			return nil, fmt.Errorf("onebot11: action_timeout 无效: %w", err)
		}
		opts = append(opts, WithActionTimeout(timeout))
	}
	if cfg.Transport.ReconnectInterval != "" {
		interval, err := time.ParseDuration(cfg.Transport.ReconnectInterval)
		if err != nil {
			return nil, fmt.Errorf("onebot11: reconnect_interval 无效: %w", err)
		}
		opts = append(opts, WithReconnectInterval(interval))
	}
	if cfg.Transport.ReconnectMaxInterval != "" {
		interval, err := time.ParseDuration(cfg.Transport.ReconnectMaxInterval)
		if err != nil {
			return nil, fmt.Errorf("onebot11: reconnect_max_interval 无效: %w", err)
		}
		opts = append(opts, WithReconnectMaxInterval(interval))
	}
	return append(opts, extra...), nil
}
