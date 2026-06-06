package sdk

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func resolveConfigEnv(node yaml.Node) (*yaml.Node, error) {
	out := cloneYAMLNode(&node)
	if err := resolveConfigEnvNode(out); err != nil {
		return nil, err
	}
	return out, nil
}

func resolveConfigEnvNode(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.ScalarNode && node.Tag == "!env" {
		name := strings.TrimSpace(node.Value)
		if name == "" {
			return fmt.Errorf("plugin config !env requires a variable name")
		}
		value, ok := os.LookupEnv(name)
		if !ok {
			return fmt.Errorf("plugin config environment variable %s is not set", name)
		}
		node.Tag = "!!str"
		node.Value = value
		node.Style = 0
		return nil
	}
	for _, child := range node.Content {
		if err := resolveConfigEnvNode(child); err != nil {
			return err
		}
	}
	return nil
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	out := *node
	if len(node.Content) > 0 {
		out.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			out.Content[i] = cloneYAMLNode(child)
		}
	}
	return &out
}
