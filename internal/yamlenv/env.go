package yamlenv

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Resolve 返回一个已展开 !env 标记的 YAML 节点副本。
func Resolve(node yaml.Node, subject string) (*yaml.Node, error) {
	out := Clone(&node)
	if err := ResolveInPlace(out, subject); err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveInPlace 原地展开 YAML 节点中的 !env 标记。
func ResolveInPlace(node *yaml.Node, subject string) error {
	if node == nil {
		return nil
	}
	if subject == "" {
		subject = "config"
	}
	if node.Tag == "!env" {
		if node.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s !env must be a scalar", subject)
		}
		name := strings.TrimSpace(node.Value)
		if name == "" {
			return fmt.Errorf("%s !env requires a variable name", subject)
		}
		node.Tag = "!!str"
		node.Value = os.Getenv(name)
		node.Style = 0
		return nil
	}
	for _, child := range node.Content {
		if err := ResolveInPlace(child, subject); err != nil {
			return err
		}
	}
	return nil
}

// Clone 深拷贝 YAML 节点。
func Clone(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	out := *node
	if len(node.Content) > 0 {
		out.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			out.Content[i] = Clone(child)
		}
	}
	return &out
}
