package host

import (
	"context"
	"fmt"
	"sync"

	absdk "github.com/tty00a381/anybot/sdk"
	"gopkg.in/yaml.v3"
)

type pluginConfigStore struct {
	path string
	mu   sync.Mutex
}

func newPluginConfigStore(path string) *pluginConfigStore {
	return &pluginConfigStore{path: path}
}

func (s *pluginConfigStore) SetPluginConfig(ctx context.Context, name string, assignments []absdk.ConfigAssignment) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	hostAssignments := make([]PluginConfigAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		node, err := configValueNode(assignment.Value)
		if err != nil {
			return fmt.Errorf("plugin config %s: %w", name, err)
		}
		hostAssignments = append(hostAssignments, PluginConfigAssignment{
			Path:  append([]string(nil), assignment.Path...),
			Value: node,
		})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := SetPluginConfigValues(s.path, name, hostAssignments)
	return err
}

func (s *pluginConfigStore) ResetPluginConfig(ctx context.Context, name string, paths [][]string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	copied := make([][]string, 0, len(paths))
	for _, path := range paths {
		copied = append(copied, append([]string(nil), path...))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := RemovePluginConfigValues(s.path, name, copied)
	return err
}

func configValueNode(value any) (yaml.Node, error) {
	var node yaml.Node
	data, err := yaml.Marshal(value)
	if err != nil {
		return yaml.Node{}, err
	}
	if err := yaml.Unmarshal(data, &node); err != nil {
		return yaml.Node{}, err
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return *node.Content[0], nil
	}
	return node, nil
}
