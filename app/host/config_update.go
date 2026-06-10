package host

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
	"gopkg.in/yaml.v3"
)

// PluginConfigSyncResult 描述插件配置同步结果。
type PluginConfigSyncResult struct {
	Changed         int
	Skipped         []string
	UnknownDisabled []string
}

// PluginConfigEntrySyncResult 描述单个插件配置同步结果。
type PluginConfigEntrySyncResult struct {
	Changed   bool
	Available bool
}

// PluginConfigAssignment 描述一次插件 config 下的字段写入。
type PluginConfigAssignment struct {
	Path  []string
	Value yaml.Node
}

// PluginConfigChange 描述一次插件配置命令中的写入或重置。
type PluginConfigChange struct {
	Assignments []PluginConfigAssignment
	ResetPaths  [][]string
}

// PluginConfigChangeResult 描述一次插件配置变更的执行结果。
type PluginConfigChangeResult struct {
	Reset   bool
	Changed bool
	Count   int
}

// Empty 判断配置命令是否没有实际变更。
func (change PluginConfigChange) Empty() bool {
	return len(change.Assignments) == 0 && len(change.ResetPaths) == 0
}

// ParsePluginConfigChanges 解析 plugin config 的变更参数。
func ParsePluginConfigChanges(args []string) (PluginConfigChange, error) {
	var change PluginConfigChange
	resetMode := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-reset" || arg == "--reset":
			if i+1 >= len(args) {
				return PluginConfigChange{}, fmt.Errorf("%s 需要值", arg)
			}
			path, err := ParsePluginConfigPath(args[i+1])
			if err != nil {
				return PluginConfigChange{}, err
			}
			change.ResetPaths = append(change.ResetPaths, path)
			resetMode = true
			i++
		case strings.HasPrefix(arg, "-reset=") || strings.HasPrefix(arg, "--reset="):
			_, value, _ := strings.Cut(arg, "=")
			path, err := ParsePluginConfigPath(value)
			if err != nil {
				return PluginConfigChange{}, err
			}
			change.ResetPaths = append(change.ResetPaths, path)
			resetMode = true
		case resetMode:
			if strings.Contains(arg, "=") {
				return PluginConfigChange{}, fmt.Errorf("不能同时设置和重置插件配置")
			}
			path, err := ParsePluginConfigPath(arg)
			if err != nil {
				return PluginConfigChange{}, err
			}
			change.ResetPaths = append(change.ResetPaths, path)
		default:
			assignment, err := ParsePluginConfigAssignment(arg)
			if err != nil {
				return PluginConfigChange{}, err
			}
			change.Assignments = append(change.Assignments, assignment)
		}
	}
	if len(change.Assignments) > 0 && len(change.ResetPaths) > 0 {
		return PluginConfigChange{}, fmt.Errorf("不能同时设置和重置插件配置")
	}
	return change, nil
}

// ApplyPluginConfigChange 执行一次插件配置写入或重置。
func ApplyPluginConfigChange(path, id string, change PluginConfigChange) (PluginConfigChangeResult, error) {
	if len(change.Assignments) > 0 && len(change.ResetPaths) > 0 {
		return PluginConfigChangeResult{}, fmt.Errorf("不能同时设置和重置插件配置")
	}
	if len(change.ResetPaths) > 0 {
		count, err := removePluginConfigValues(path, id, change.ResetPaths)
		return PluginConfigChangeResult{Reset: true, Changed: count > 0, Count: count}, err
	}
	if len(change.Assignments) > 0 {
		changed, err := SetPluginConfigValues(path, id, change.Assignments)
		return PluginConfigChangeResult{Changed: changed, Count: len(change.Assignments)}, err
	}
	return PluginConfigChangeResult{}, fmt.Errorf("plugin config change is required")
}

// ParsePluginConfigAssignment 解析 key=value 形式的插件配置写入项。value 使用 YAML 语义解析。
func ParsePluginConfigAssignment(input string) (PluginConfigAssignment, error) {
	key, value, ok := strings.Cut(input, "=")
	if !ok {
		return PluginConfigAssignment{}, fmt.Errorf("plugin config assignment %q must be key=value", input)
	}
	path, err := ParsePluginConfigPath(key)
	if err != nil {
		return PluginConfigAssignment{}, err
	}
	node, err := parseYAMLValue(value)
	if err != nil {
		return PluginConfigAssignment{}, fmt.Errorf("plugin config %s: %w", strings.Join(path, "."), err)
	}
	return PluginConfigAssignment{Path: path, Value: *node}, nil
}

// ParsePluginConfigPath 解析插件 config 下的点分路径。
func ParsePluginConfigPath(input string) ([]string, error) {
	return absdk.ParseConfigPath(input)
}

// EnsurePluginConfigEntry 确保 config/<id>.yaml 占位项存在。
func EnsurePluginConfigEntry(path, id string) (bool, error) {
	entryPath, exists, err := pluginConfigEntryPathForUpdate(PluginConfigDir(path), id)
	if err != nil || exists {
		return false, err
	}
	doc := yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{pluginEntryNode(emptyConfigNode())}}
	if err := saveYAMLDocument(entryPath, doc); err != nil {
		return false, err
	}
	return true, nil
}

// SetPluginEnabled 设置 config/<id>.yaml 中的 enabled。
func SetPluginEnabled(path, id string, enabled bool) (bool, error) {
	entryPath, _, err := pluginConfigEntryPathForUpdate(PluginConfigDir(path), id)
	if err != nil {
		return false, err
	}
	doc, entry, err := loadPluginEntryDocument(entryPath)
	if err != nil {
		return false, err
	}
	changed, err := setPluginEntryEnabled(entry, id, enabled)
	if err != nil || !changed {
		return changed, err
	}
	return true, saveYAMLDocument(entryPath, doc)
}

// SetPluginConfigValues 写入 config/<id>.yaml 中的 config 字段。
func SetPluginConfigValues(path, id string, assignments []PluginConfigAssignment) (bool, error) {
	if len(assignments) == 0 {
		return false, fmt.Errorf("plugin config assignment is required")
	}
	entryPath, _, err := pluginConfigEntryPathForUpdate(PluginConfigDir(path), id)
	if err != nil {
		return false, err
	}
	doc, entry, err := loadPluginEntryDocument(entryPath)
	if err != nil {
		return false, err
	}
	changed, err := setPluginConfigAssignments(id, entry, assignments)
	if err != nil || !changed {
		return changed, err
	}
	return true, saveYAMLDocument(entryPath, doc)
}

// RemovePluginConfigValues 删除 config/<id>.yaml 里的 config 字段覆盖。
func RemovePluginConfigValues(path, id string, paths [][]string) (bool, error) {
	count, err := removePluginConfigValues(path, id, paths)
	return count > 0, err
}

func removePluginConfigValues(path, id string, paths [][]string) (int, error) {
	if len(paths) == 0 {
		return 0, fmt.Errorf("plugin config path is required")
	}
	entryPath, exists, err := pluginConfigEntryPathForUpdate(PluginConfigDir(path), id)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	doc, entry, err := loadPluginEntryDocument(entryPath)
	if err != nil {
		return 0, err
	}
	count, err := removePluginConfigPaths(id, entry, paths)
	if err != nil || count == 0 {
		return count, err
	}
	return count, saveYAMLDocument(entryPath, doc)
}

// RemovePluginConfigEntry 删除 config/<id>.yaml。
func RemovePluginConfigEntry(path, id string) (bool, error) {
	entryPath, exists, err := pluginConfigEntryPathForUpdate(PluginConfigDir(path), id)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	return true, os.Remove(entryPath)
}

// SyncPluginConfigEntries 把 registry 中的插件默认配置同步到插件配置文件。
func SyncPluginConfigEntries(path string, registry absdk.Registry) (int, error) {
	result, err := syncPluginConfigEntries(path, registry, nil)
	return result.Changed, err
}

// SyncPluginConfigEntriesForLock 同步插件配置，并允许插件锁中的外部插件等待生成宿主补全默认配置。
func SyncPluginConfigEntriesForLock(path string, registry absdk.Registry, lock PluginLock) (PluginConfigSyncResult, error) {
	lock.applyDefaults()
	allowUnavailable := map[string]struct{}{}
	for _, item := range lock.Plugins {
		if item.ID != "" {
			allowUnavailable[item.ID] = struct{}{}
		}
	}
	return syncPluginConfigEntries(path, registry, allowUnavailable)
}

// SyncPluginConfigEntry 同步单个可加载插件的默认配置。
func SyncPluginConfigEntry(path string, registry absdk.Registry, id string) (PluginConfigEntrySyncResult, error) {
	if path == "" {
		path = DefaultConfigPath
	}
	if id == "" {
		return PluginConfigEntrySyncResult{}, fmt.Errorf("plugin id is required")
	}
	if err := ValidatePluginID(id); err != nil {
		return PluginConfigEntrySyncResult{}, err
	}
	factory, ok := registry.Factory(id)
	if !ok {
		return PluginConfigEntrySyncResult{}, nil
	}
	entryPath, exists, err := pluginConfigEntryPathForUpdate(PluginConfigDir(path), id)
	if err != nil {
		return PluginConfigEntrySyncResult{}, err
	}
	if !exists {
		return PluginConfigEntrySyncResult{Available: true}, nil
	}
	doc, entry, err := loadPluginEntryDocument(entryPath)
	if err != nil {
		return PluginConfigEntrySyncResult{}, err
	}
	changed, err := syncPluginConfigNode(id, entry, factory)
	if err != nil {
		return PluginConfigEntrySyncResult{}, err
	}
	if changed {
		if err := saveYAMLDocument(entryPath, doc); err != nil {
			return PluginConfigEntrySyncResult{}, err
		}
	}
	return PluginConfigEntrySyncResult{Changed: changed, Available: true}, nil
}

func syncPluginConfigEntries(path string, registry absdk.Registry, allowUnavailable map[string]struct{}) (PluginConfigSyncResult, error) {
	if path == "" {
		path = DefaultConfigPath
	}
	locations, err := pluginConfigEntryLocations(path)
	if err != nil {
		return PluginConfigSyncResult{}, err
	}
	var result PluginConfigSyncResult
	changedFiles := map[string]*yaml.Node{}
	for _, location := range locations {
		factory, ok := registry.Factory(location.id)
		if !ok {
			if _, allowed := allowUnavailable[location.id]; allowed {
				result.Skipped = append(result.Skipped, location.id)
				continue
			}
			if pluginEntryDisabled(location.entry) {
				result.UnknownDisabled = append(result.UnknownDisabled, location.id)
				continue
			}
			return PluginConfigSyncResult{}, fmt.Errorf("unknown plugin %q", location.id)
		}
		pluginChanged, err := syncPluginConfigNode(location.id, location.entry, factory)
		if err != nil {
			return PluginConfigSyncResult{}, err
		}
		if pluginChanged {
			result.Changed++
			changedFiles[location.path] = location.doc
		}
	}
	sort.Strings(result.Skipped)
	sort.Strings(result.UnknownDisabled)
	paths := make([]string, 0, len(changedFiles))
	for path := range changedFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := saveYAMLDocument(path, *changedFiles[path]); err != nil {
			return PluginConfigSyncResult{}, err
		}
	}
	return result, nil
}

type pluginConfigEntryLocation struct {
	id    string
	path  string
	doc   *yaml.Node
	entry *yaml.Node
}

func pluginConfigEntryLocations(configPath string) ([]pluginConfigEntryLocation, error) {
	dir := PluginConfigDir(configPath)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	locations := make([]pluginConfigEntryLocation, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		id, ok, err := pluginConfigID(entry.Name())
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if err := ValidatePluginID(id); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, entry.Name()), err)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("plugin %s is configured more than once", id)
		}
		seen[id] = struct{}{}
		entryPath := filepath.Join(dir, entry.Name())
		entryDoc, entryRoot, err := loadPluginEntryDocument(entryPath)
		if err != nil {
			return nil, err
		}
		locations = append(locations, pluginConfigEntryLocation{
			id:    id,
			path:  entryPath,
			doc:   &entryDoc,
			entry: entryRoot,
		})
	}
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].id < locations[j].id
	})
	return locations, nil
}

func syncPluginConfigNode(id string, entry *yaml.Node, factory absdk.Factory) (bool, error) {
	defaultConfig, err := defaultConfigNode(factory.Default)
	if err != nil {
		return false, fmt.Errorf("plugin %s default config: %w", id, err)
	}
	pluginChanged := false
	if filled, err := fillMissingPluginEnabled(entry); err != nil {
		return false, fmt.Errorf("plugin %s enabled: %w", id, err)
	} else if filled {
		pluginChanged = true
	}
	if filled, err := fillPluginConfigDefaults(entry, defaultConfig); err != nil {
		return false, fmt.Errorf("plugin %s config: %w", id, err)
	} else if filled {
		pluginChanged = true
	}
	return pluginChanged, nil
}

func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
		}
		return doc.Content[0]
	}
	return doc
}

func pluginConfigEntryPath(dir, id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("plugin id is required")
	}
	if err := ValidatePluginID(id); err != nil {
		return "", fmt.Errorf("plugin id %q cannot be used as a config file name: %w", id, err)
	}
	return filepath.Join(dir, id+".yaml"), nil
}

func pluginConfigEntryPathForUpdate(dir, id string) (string, bool, error) {
	defaultPath, err := pluginConfigEntryPath(dir, id)
	if err != nil {
		return "", false, err
	}
	for _, candidate := range pluginConfigEntryCandidates(defaultPath) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true, nil
		} else if !os.IsNotExist(err) {
			return "", false, err
		}
	}
	return defaultPath, false, nil
}

func pluginConfigEntryCandidates(path string) []string {
	candidates := []string{path}
	if filepath.Ext(path) == ".yaml" {
		candidates = append(candidates, strings.TrimSuffix(path, ".yaml")+".yml")
	}
	return candidates
}

func ensureMapping(root *yaml.Node, key string) (*yaml.Node, error) {
	value := mappingValue(root, key)
	if value == nil {
		value = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, scalar(key), value)
		return value, nil
	}
	if value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
		value.Kind = yaml.MappingNode
		value.Tag = "!!map"
		value.Value = ""
		value.Content = nil
		return value, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a YAML mapping", key)
	}
	return value, nil
}

func ensurePluginEntryMapping(entry *yaml.Node) error {
	if entry.Kind == yaml.ScalarNode && entry.Tag == "!!null" {
		entry.Kind = yaml.MappingNode
		entry.Tag = "!!map"
		entry.Value = ""
		entry.Content = nil
		return nil
	}
	if entry.Kind != yaml.MappingNode {
		return fmt.Errorf("plugin entry must be a YAML mapping")
	}
	return nil
}

func loadPluginEntryDocument(path string) (yaml.Node, *yaml.Node, error) {
	doc, err := loadYAMLDocument(path)
	if os.IsNotExist(err) {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{pluginEntryNode(emptyConfigNode())}}
		return doc, documentRoot(&doc), nil
	}
	if err != nil {
		return yaml.Node{}, nil, err
	}
	root := documentRoot(&doc)
	if root.Kind == 0 {
		root.Kind = yaml.MappingNode
		root.Tag = "!!map"
	}
	return doc, root, nil
}

func setPluginEntryEnabled(entry *yaml.Node, id string, enabled bool) (bool, error) {
	if err := ensurePluginEntryMapping(entry); err != nil {
		return false, fmt.Errorf("plugin %s entry must be a YAML mapping", id)
	}
	enabledNode := mappingValue(entry, "enabled")
	if enabledNode != nil && enabledNode.Kind == yaml.ScalarNode && enabledNode.Tag == "!!bool" && enabledNode.Value == boolValue(enabled) {
		return false, nil
	}
	if enabledNode == nil {
		entry.Content = append(entry.Content, scalar("enabled"), boolNode(enabled))
	} else {
		*enabledNode = *boolNode(enabled)
	}
	return true, nil
}

func setPluginConfigAssignments(id string, entry *yaml.Node, assignments []PluginConfigAssignment) (bool, error) {
	if err := ensurePluginEntryMapping(entry); err != nil {
		return false, fmt.Errorf("plugin %s entry must be a YAML mapping", id)
	}
	config := mappingValue(entry, "config")
	if emptyYAMLNode(config) {
		if config == nil {
			config = emptyConfigNode()
			entry.Content = append(entry.Content, scalar("config"), config)
		} else {
			*config = *emptyConfigNode()
		}
	}
	if config.Kind != yaml.MappingNode {
		return false, fmt.Errorf("plugin %s config must be a YAML mapping", id)
	}
	var changed bool
	for _, assignment := range assignments {
		if len(assignment.Path) == 0 {
			return false, fmt.Errorf("plugin config path is required")
		}
		if set, err := setMappingPath(config, assignment.Path, &assignment.Value); err != nil {
			return false, fmt.Errorf("plugin %s config.%s: %w", id, strings.Join(assignment.Path, "."), err)
		} else if set {
			changed = true
		}
	}
	return changed, nil
}

func removePluginConfigPaths(id string, entry *yaml.Node, paths [][]string) (int, error) {
	if err := ensurePluginEntryMapping(entry); err != nil {
		return 0, fmt.Errorf("plugin %s entry must be a YAML mapping", id)
	}
	config := mappingValue(entry, "config")
	if emptyYAMLNode(config) {
		return 0, nil
	}
	if config.Kind != yaml.MappingNode {
		return 0, fmt.Errorf("plugin %s config must be a YAML mapping", id)
	}
	var count int
	for _, path := range paths {
		if len(path) == 0 {
			return 0, fmt.Errorf("plugin config path is required")
		}
		if removed, err := removeMappingPath(config, path); err != nil {
			return 0, fmt.Errorf("plugin %s config.%s: %w", id, strings.Join(path, "."), err)
		} else if removed {
			count++
		}
	}
	return count, nil
}

func setMappingPath(mapping *yaml.Node, path []string, value *yaml.Node) (bool, error) {
	if mapping.Kind != yaml.MappingNode {
		return false, fmt.Errorf("parent must be a YAML mapping")
	}
	key := path[0]
	existing := mappingValue(mapping, key)
	if len(path) == 1 {
		value = cloneYAMLNode(value)
		if existing == nil {
			mapping.Content = append(mapping.Content, scalar(key), value)
			return true, nil
		}
		if yamlNodeEqual(existing, value) {
			return false, nil
		}
		*existing = *value
		return true, nil
	}
	if emptyYAMLNode(existing) {
		if existing == nil {
			existing = emptyConfigNode()
			mapping.Content = append(mapping.Content, scalar(key), existing)
		} else {
			*existing = *emptyConfigNode()
		}
	}
	if existing.Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s must be a YAML mapping", key)
	}
	return setMappingPath(existing, path[1:], value)
}

func removeMappingPath(mapping *yaml.Node, path []string) (bool, error) {
	if mapping.Kind != yaml.MappingNode {
		return false, fmt.Errorf("parent must be a YAML mapping")
	}
	key := path[0]
	if len(path) == 1 {
		return removeMappingKey(mapping, key), nil
	}
	existing := mappingValue(mapping, key)
	if emptyYAMLNode(existing) {
		return false, nil
	}
	if existing.Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s must be a YAML mapping", key)
	}
	removed, err := removeMappingPath(existing, path[1:])
	if err != nil || !removed {
		return removed, err
	}
	if len(existing.Content) == 0 {
		removeMappingKey(mapping, key)
	}
	return true, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func mappingKeys(mapping *yaml.Node) []string {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(mapping.Content)/2)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keys = append(keys, mapping.Content[i].Value)
	}
	return keys
}

func removeMappingKey(mapping *yaml.Node, key string) bool {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return true
		}
	}
	return false
}

func loadYAMLDocument(path string) (yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return yaml.Node{}, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return yaml.Node{}, err
	}
	return doc, nil
}

func saveYAMLDocument(path string, doc yaml.Node) error {
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		_ = enc.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, out.Bytes(), 0o644)
}

func pluginEntryNode(config *yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			scalar("enabled"),
			{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"},
			scalar("config"),
			config,
		},
	}
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func boolNode(value bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: boolValue(value)}
}

func boolValue(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func emptyConfigNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

func parseYAMLValue(input string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input+"\n"), &doc); err != nil {
		return nil, err
	}
	root := documentRoot(&doc)
	if root.Kind == 0 {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: ""}, nil
	}
	return cloneYAMLNode(root), nil
}

func defaultConfigNode(config any) (*yaml.Node, error) {
	if config == nil {
		return emptyConfigNode(), nil
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	root := documentRoot(&doc)
	if root.Kind == 0 || (root.Kind == yaml.ScalarNode && root.Tag == "!!null") {
		return emptyConfigNode(), nil
	}
	return cloneYAMLNode(root), nil
}

func fillPluginConfigDefaults(entry, defaultConfig *yaml.Node) (bool, error) {
	if err := ensurePluginEntryMapping(entry); err != nil {
		return false, err
	}
	config := mappingValue(entry, "config")
	if emptyYAMLNode(config) {
		if config == nil {
			entry.Content = append(entry.Content, scalar("config"), cloneYAMLNode(defaultConfig))
			return true, nil
		}
		*config = *cloneYAMLNode(defaultConfig)
		return true, nil
	}
	if config.Kind == yaml.MappingNode && defaultConfig.Kind == yaml.MappingNode {
		return mergeMappingDefaults(config, defaultConfig), nil
	}
	return false, nil
}

func mergeMappingDefaults(target, defaults *yaml.Node) bool {
	var changed bool
	for i := 0; i+1 < len(defaults.Content); i += 2 {
		key := defaults.Content[i]
		value := defaults.Content[i+1]
		existing := mappingValue(target, key.Value)
		if existing == nil {
			target.Content = append(target.Content, cloneYAMLNode(key), cloneYAMLNode(value))
			changed = true
			continue
		}
		if existing.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			if mergeMappingDefaults(existing, value) {
				changed = true
			}
			continue
		}
		if existing.Kind == yaml.ScalarNode && existing.Tag == "!!null" {
			*existing = *cloneYAMLNode(value)
			changed = true
		}
	}
	return changed
}

func fillMissingPluginEnabled(entry *yaml.Node) (bool, error) {
	if err := ensurePluginEntryMapping(entry); err != nil {
		return false, err
	}
	enabled := mappingValue(entry, "enabled")
	if enabled == nil {
		entry.Content = append([]*yaml.Node{scalar("enabled"), boolNode(true)}, entry.Content...)
		return true, nil
	}
	if enabled.Kind == yaml.ScalarNode && enabled.Tag == "!!null" {
		*enabled = *boolNode(true)
		return true, nil
	}
	return false, nil
}

func pluginEntryDisabled(entry *yaml.Node) bool {
	if entry == nil || entry.Kind != yaml.MappingNode {
		return false
	}
	enabled := mappingValue(entry, "enabled")
	if enabled == nil || enabled.Kind != yaml.ScalarNode || enabled.Tag != "!!bool" {
		return false
	}
	var value bool
	if err := enabled.Decode(&value); err != nil {
		return false
	}
	return !value
}

func emptyYAMLNode(node *yaml.Node) bool {
	return node == nil ||
		node.Kind == 0 ||
		(node.Kind == yaml.ScalarNode && node.Tag == "!!null") ||
		(node.Kind == yaml.MappingNode && len(node.Content) == 0)
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

func yamlNodeEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	adata, aerr := yaml.Marshal(a)
	bdata, berr := yaml.Marshal(b)
	return aerr == nil && berr == nil && bytes.Equal(adata, bdata)
}
