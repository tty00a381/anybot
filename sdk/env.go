package sdk

import (
	"github.com/tty00a381/anybot/internal/yamlenv"
	"gopkg.in/yaml.v3"
)

func resolveConfigEnv(node yaml.Node) (*yaml.Node, error) {
	return yamlenv.Resolve(node, "plugin config")
}

func resolveConfigEnvNode(node *yaml.Node) error {
	return yamlenv.ResolveInPlace(node, "plugin config")
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	return yamlenv.Clone(node)
}
