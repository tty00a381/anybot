# 维护者 API 索引

这份索引按包列出当前主要类型、函数和方法。它不是 GoDoc 的替代品，而是维护者快速判断“能力在哪一层”的地图。

## `cmd/anybot`

CLI 入口包，不作为库导入。

命令：

- `anybot init [-dir 目录] [-force]`：生成最终用户工作目录。
- `anybot run [-config anybot.yaml]`：按配置运行基础框架。
- `anybot doctor [-config anybot.yaml] [-connect]`：静态检查配置，可选连接检查。
- `anybot build [-dir 目录] [-o anybot-bot] [-skip-tidy]`：构建包含外部插件的运行框架。
- `anybot up [-dir 目录] [-o anybot-bot] [-skip-tidy] [-skip-build] [-skip-sync] [-skip-check]`：构建、同步、检查并运行。
- `anybot plugins`：列出基础二进制内置插件。
- `anybot plugin add/update/remove/list/status/inspect/config/check/sync/enable/disable`：管理插件。
- `anybot dev init/plugin/new plugin/doctor/run`：开发者脚手架和直接核心库项目辅助命令。
- `anybot version`：打印版本。

关键实现文件：

- `main.go`：命令分发、`init`、`run`。
- `plugin.go`：插件管理、失败回滚、配置命令。
- `build.go`：构建生成框架、同步 `go.mod`。
- `dev.go`：开发者脚手架。
- `doctor.go`：配置检查。
- `defaults.go`：CLI 帮助、默认配置、工作目录 README。

## `app/host`

运行框架层。

### 配置类型

- `Config`：顶层框架配置，字段为 `Runtime`、`Adapter`、`Security`、`PluginConfigDir`、`Plugins`。
- `RuntimeConfig`：`LogLevel`、`Workers`、`Buffer`、`Serial`、`DataDir`、`Store`。
- `StoreConfig`：`Type`、`Path`。
- `AdapterConfig`：`Protocol`、`Transport`。
- `SecurityConfig`：`SuperUsers`。
- `PluginEntry`：插件 `Enabled` 与原始 YAML `Config`。

函数：

- `LoadConfig(path string) (Config, error)`：读取 `.yaml`，应用默认值并加载 `plugins.d`。
- `ValidateConfig(cfg Config, registry sdk.Registry) error`：静态校验配置和插件配置。
- `NewLogger(level string, out io.Writer) (*slog.Logger, error)`：创建日志器。
- `NewApp(cfg Config, registry sdk.Registry, logger *slog.Logger, opts ...AppOption) (*core.App, error)`：装配运行时。
- `InstallPlugins(app *core.App, cfg Config, registry sdk.Registry, env sdk.Environment) error`：按配置安装插件。
- `EnabledPlugins(cfg Config, registry sdk.Registry) ([]string, error)`：返回启用插件名。

App option：

- `AppOptions`：`ConfigPath`、`RuntimeState`。
- `WithConfigPath(path string)`：注入配置写回路径。
- `WithRuntimeState()`：启用文件存储和插件数据目录。

### 外部插件工作区

常量：

- `PluginWorkspaceFile = "anybot.plugins.yaml"`。

类型：

- `PluginWorkspace`：`Module`、`Plugins`。
- `PluginModule`：`Name`、`Module`、`Version`、`Replace`、`Symbol`。
- `AddPluginOptions`
- `UpdatePluginOptions`
- `RemovePluginOptions`

函数：

- `EnsurePluginWorkspace(dir string) (PluginWorkspace, error)`：确保工作区与生成文件存在。
- `EnsurePluginWorkspaceForce(dir string, force bool) (PluginWorkspace, error)`：允许接管同名非生成文件。
- `LoadPluginWorkspace(path string) (PluginWorkspace, error)`：读取清单，文件不存在返回空工作区。
- `SavePluginWorkspace(path string, workspace PluginWorkspace) error`：校验并原子写入清单。
- `RenderPluginWorkspace(dir string, workspace PluginWorkspace) error`：重写 `plugins.gen.go` 与 `main.go`。
- `AddPluginModule(opts AddPluginOptions) (PluginWorkspace, error)`：添加外部插件。
- `UpdatePluginModule(opts UpdatePluginOptions) (PluginModule, PluginWorkspace, bool, error)`：更新版本、替换路径或符号。
- `RemovePluginModule(opts RemovePluginOptions) (PluginModule, PluginWorkspace, error)`：移除外部插件。
- `CheckPluginWorkspaceWritable(dir string, force bool) error`：检查生成文件是否可写。
- `ParsePluginModuleSpec(spec string) (module, version string, err error)`：解析 `module@version`。
- `DefaultPluginName(module string) string`：从 module 推导插件名。
- `ValidatePluginWorkspace(workspace PluginWorkspace) error`
- `ValidatePluginModule(item PluginModule) error`
- `ValidatePluginName(name string) error`

实现文件：

- `workspace.go`：工作区模型、增删改和校验。
- `workspace_io.go`：清单读写、生成文件写入保护。
- `workspace_render.go`：`plugins.gen.go`、生成 `main.go` 和 `go.mod` 的渲染。

### 生成框架内置命令

类型：

- `PluginCommandOptions`：`Args`、`Output`、`ConfigPath`、`WorkspacePath`、`Registry`。

函数：

- `RunPluginCommand(opts PluginCommandOptions) error`：执行生成运行框架内置的 `plugin` 子命令。

### 插件配置更新

类型：

- `PluginConfigAssignment`：`Path []string`、`Value yaml.Node`。
- `PluginConfigChange`：`Assignments`、`ResetPaths`。
- `PluginConfigChangeResult`：`Reset`、`Changed`、`Count`。
- `PluginConfigSyncResult`：`Changed`、`Skipped`、`UnknownDisabled`。
- `PluginConfigEntrySyncResult`：`Changed`、`Available`。

函数：

- `ParsePluginConfigPath(input string) ([]string, error)`：解析点分路径。
- `ParsePluginConfigAssignment(input string) (PluginConfigAssignment, error)`：解析 `key=value`。
- `ParsePluginConfigChanges(args []string) (PluginConfigChange, error)`：解析 CLI 参数。
- `ApplyPluginConfigChange(path, name string, change PluginConfigChange) (PluginConfigChangeResult, error)`：应用设置或重置。
- `EnsurePluginConfigEntry(path, name string) (bool, error)`：确保插件配置项存在。
- `SetPluginEnabled(path, name string, enabled bool) (bool, error)`：启用或禁用。
- `SetPluginConfigValues(path, name string, assignments []PluginConfigAssignment) (bool, error)`：写入配置值。
- `RemovePluginConfigValues(path, name string, paths [][]string) (bool, error)`：删除配置字段覆盖。
- `RemovePluginConfigEntry(path, name string) (bool, error)`：移除插件配置。
- `SyncPluginConfigEntry(path string, registry sdk.Registry, name string) (PluginConfigEntrySyncResult, error)`：同步单个插件默认配置。
- `SyncPluginConfigEntries(path string, registry sdk.Registry) (int, error)`：同步注册表内插件。
- `SyncPluginConfigEntriesForWorkspace(path string, registry sdk.Registry, workspace PluginWorkspace) (PluginConfigSyncResult, error)`：允许外部插件待构建的同步。
- `WritePluginConfigSyncSummary(w io.Writer, result PluginConfigSyncResult) error`。

### 插件状态、检查、详情

类型：

- `PluginStatus`：插件运行视图。
- `PluginConfigCheck`：配置检查结果。
- `PluginInspect`：单插件完整视图。
- `UnknownPluginError`：未知插件错误。

函数：

- `PluginStatuses(cfg Config, registry sdk.Registry, workspace PluginWorkspace) []PluginStatus`
- `WritePluginStatusTable(w io.Writer, statuses []PluginStatus) error`
- `PluginConfigChecks(cfg Config, registry sdk.Registry, workspace PluginWorkspace) []PluginConfigCheck`
- `PluginConfigCheckFailed(checks []PluginConfigCheck) bool`
- `WritePluginConfigCheckTable(w io.Writer, checks []PluginConfigCheck) error`
- `InspectPlugin(configPath string, registry sdk.Registry, workspace PluginWorkspace, name string) (PluginInspect, error)`
- `WritePluginInspect(w io.Writer, inspect PluginInspect) error`
- `EnsureKnownPluginTarget(cfg Config, registry sdk.Registry, workspace PluginWorkspace, name string, allowConfigured bool) error`

### 内置插件注册表

- `DefaultRegistry() sdk.Registry`：注册 `help`、`echo`、`admin`、`ratelimit`。

## `app/plugins/help`

类型：

- `Config`：`Command`、`Lines`。

变量：

- `Plugin`：`sdk.Definition`，默认命令 `/help`。

## `app/plugins/echo`

类型：

- `Config`：`Command`。

变量：

- `Plugin`：复读命令插件，默认命令 `/echo`，默认禁用由工作目录配置决定。

## `app/plugins/admin`

类型：

- `Config`：`Users`。

变量：

- `Plugin`：超级用户命令插件。`Users` 为空时使用框架 `security.superusers`。

## `app/plugins/ratelimit`

类型：

- `Config`：`Limit`、`Window`。

方法：

- `Config.Validate() error`：校验限额和窗口。

变量：

- `Plugin`：注册框架级 `RateLimit` 中间件。

## `sdk`

插件作者主入口。

### 插件定义

- `Plugin`：已配置插件实例接口，方法 `Manifest() Manifest`、`Setup(*Context) error`。
- `Definition`：插件定义接口，可转成运行框架工厂，也可用默认配置构建插件实例。
- `SetupFunc[T]`：typed config 安装函数。
- `Define`：用类型参数 `T` 创建 `Definition`，参数为 `Manifest`、默认配置和 `SetupFunc[T]`。
- `Definition.Manifest() Manifest`
- `Definition.Build() (Plugin, error)`：按默认配置构建插件实例。
- `Definition.Factory() Factory`：转为运行框架可注册工厂。
- `Environment`：`DataDir`、`ConfigStore`。
- `Install(app *App, plugins ...Plugin) error`：把已配置插件实例安装到运行时。
- `InstallWith(app *App, env Environment, plugins ...Plugin) error`：用显式宿主能力安装插件实例。
- `InstallDefault(app *App, definitions ...Definition) error`：按默认配置安装插件定义。
- `InstallDefaultWith(app *App, env Environment, definitions ...Definition) error`：按默认配置和显式宿主能力安装插件定义。

### 注册表

- `Factory`：`Info`、`Default`、`Build func(yaml.Node) (Plugin, error)`。
- `Factory.WithName(name string) Factory`：为外部插件配置别名。
- `Registry`：插件工厂表。
- `NewRegistry() Registry`
- `Registry.Register(factory Factory) error`
- `Registry.Factory(name string) (Factory, bool)`
- `Registry.Plugins() []Manifest`

### 安装上下文

- `Context`：插件安装上下文。
- `NewContext(app *App, manifest Manifest, opts ...InstallOption) *Context`
- `Context.Manifest() Manifest`
- `Context.Logger() *slog.Logger`
- `Context.Store() Store`
- `Context.Client() ActionClient`
- `Context.Send(ctx, target, chain) (MessageReceipt, error)`
- `Context.SendText(ctx, target, text) (MessageReceipt, error)`
- `Context.Use(...)`
- `Context.UseGlobal(...)`
- `Context.On(...)`
- `Context.OnMessage(...)`
- `Context.Command(...)`
- `Context.Observe(...)`
- `Context.Go(...)`
- `Context.Every(...)`
- `Context.OnStart(...)`
- `Context.OnReady(...)`
- `Context.OnShutdown(...)`
- `Context.WaitActionReady(ctx) error`
- `Context.OnAdapterState(hook)`

### 会话和数据

- `Context.Session(event *EventContext) *Session`
- `Context.UserSession(event *EventContext) *Session`
- `Context.GroupSession(event *EventContext) *Session`
- `Context.SessionBy(key string) *Session`
- `WithDataDir(root string) InstallOption`
- `Context.DataDir() (string, error)`
- `ErrDataDirUnavailable`

### 配置写回

- `ConfigAssignment`：`Path`、`Value`。
- `ConfigStore`：`SetPluginConfig`、`ResetPluginConfig`。
- `ConfigHandle`：当前插件配置写回入口。
- `WithConfigStore(store ConfigStore) InstallOption`
- `Context.Config() ConfigHandle`
- `ConfigHandle.Available() bool`
- `ConfigHandle.Set(ctx, key, value) error`
- `ConfigHandle.SetAll(ctx, assignments...) error`
- `ConfigHandle.Reset(ctx, keys...) error`
- `ParseConfigPath(input string) ([]string, error)`
- `ErrConfigStoreUnavailable`

### 多轮对话

- `DialogueScope`：`DialogueScopeConversation`、`DialogueScopeUser`、`DialogueScopeGroup`。
- `DialogueOption`
- `DialogueWithTTL(ttl)`
- `DialogueWithPriority(priority)`
- `DialogueWithRules(rules...)`
- `DialogueWithScope(scope)`
- `Dialogue`
- `Context.Dialogue(name string, opts ...DialogueOption) *Dialogue`
- `Dialogue.Name() string`
- `Dialogue.Route() *Route`
- `Dialogue.Step(name string, handler DialogueHandler) *Dialogue`
- `Dialogue.Begin(c, step, data) error`
- `Dialogue.BeginText(c, step, data, prompt) error`
- `Dialogue.End(c) error`
- `Dialogue.Active(c) (DialogueSnapshot, bool, error)`
- `DialogueTurn.Step() string`
- `DialogueTurn.State() DialogueSnapshot`
- `DialogueTurn.Load(out any) error`
- `DialogueTurn.Next(step, data) error`
- `DialogueTurn.NextText(step, data, prompt) error`
- `DialogueTurn.End() error`
- `DialogueTurn.EndText(text) error`

### 规则和中间件

SDK 重新导出核心规则：

- `Any`
- `All`
- `AnyOf`
- `Not`
- `EventType`
- `DetailType`
- `MessageEvent`
- `Group`
- `Private`
- `FromUser`
- `FromSelf`
- `NotFromSelf`
- `InGroup`
- `Mentioned`
- `ToMe`
- `Contains`
- `Prefix`
- `CommandRule`
- `CommandWithPrefixes`
- `RegexRule`
- `RegexpRule`

SDK 自有规则：

- `AllowedGroups(ids ...string) Rule`

中间件：

- `Recover`
- `Trace`
- `Timeout`
- `OnlyPrivate`
- `OnlyGroup`
- `SuperUser`
- `RequireSuperUser`
- `RequireAdmin`
- `RateLimit`
- `RateLimitBy`

### core 类型别名

SDK 重新导出 `core` 的常用类型：`App`、`Option`、`Adapter`、`EmitFunc`、`Event`、`EventContext`、`Handler`、`ErrorHandler`、`ObserverHandler`、`Hook`、`Middleware`、`Match`、`Rule`、`RuleFunc`、`Store`、`Session`、`MemoryStore`、`FileStore`、`ActionClient`、`ReplyTarget`、`MessageReceipt`、`Protocol`、`PanicError`、`TaskFunc`、`TaskOption`、`AdapterState`、`AdapterStateHook`。

SDK 重新导出函数：`NewApp`、`WithAdapter`、`WithStore`、`WithSuperUsers`、`NewTestContext`、`NewSession`、`NewMemoryStore`、`NewFileStore`、`TaskCritical`、`TaskImmediate`。

## `sdk/message`

插件作者使用的消息链包，转发自 `core/message`。

- `type Chain`
- `type Segment`
- `New(segments ...Segment) Chain`
- `Text(text string) Segment`
- `Image(file string) Segment`
- `Video(file string) Segment`
- `Raw(kind string, data map[string]any) Segment`

## `core`

协议无关核心库。

### App 与快捷入口

- `New(opts ...Option) *App`
- `Default() *App`
- `Configure(opts ...Option)`
- `ResetDefault(opts ...Option)`
- `Run(ctxs ...context.Context) error`
- `Use(...)`
- `OnStart`、`OnReady`、`OnShutdown`、`OnStop`
- `OnError`
- `OnAdapterState`
- `OnObserverError`
- `On`、`OnMessage`、`Command`、`Regex`、`Observe`
- `Go`、`Every`
- `CurrentAdapterState`
- `WaitActionReady`

### App 方法

- `Adapter() Adapter`
- `Client() ActionClient`
- `Store() Store`
- `Logger() *slog.Logger`
- `Router() *Router`
- `Run(ctx context.Context) error`
- `Dispatch(ctx context.Context, event *Event) error`
- `Use(middleware...)`
- `Group(rules...) *Router`
- `On(rules...) *Route`
- `OnMessage(rules...) *Route`
- `Command(names...) *Route`
- `Regex(pattern) *Route`
- `Observe(rules...) *Observer`
- `OnObserverError(handler)`
- `Go(name, fn, opts...)`
- `Every(name, interval, fn, opts...)`
- `OnStart`、`OnReady`、`OnError`、`OnShutdown`、`OnStop`
- `OnAdapterState(hook)`
- `AdapterState() AdapterState`
- `WaitActionReady(ctx) error`
- `SuperUsers() []string`
- `IsSuperUser(id any) bool`

### Options

- `WithAdapter(adapter Adapter)`
- `WithLogger(logger *slog.Logger)`
- `WithStore(store Store)`
- `WithSuperUsers(ids ...string)`
- `WithWorkers(workers int)`
- `WithBuffer(size int)`
- `WithObserverBuffer(size int)`
- `WithErrorHandler(handler ErrorHandler)`
- `WithSerialBy(fn EventKeyFunc)`
- `WithSerialByConversation()`

### Event 与 Context

- `Event`：`ID`、`Protocol`、`SelfID`、`Type`、`DetailType`、`SubType`、`UserID`、`GroupID`、`GuildID`、`ChannelID`、`Text`、`Message`、`Raw`、`Data`。
- `Event.Clone() *Event`
- `Event.DecodeRaw(out any) error`
- `Event.ConversationID() string`
- `Event.UserSessionID() string`
- `Event.GroupSessionID() string`
- `Event.Target() ReplyTarget`
- `Event.IsMessage() bool`

`Context` 方法：

- `App`、`Adapter`、`Event`、`Client`、`Store`、`Logger`
- `Match`、`Route`、`RouteName`
- `Set`、`Get`、`String`
- `Command`、`Args`、`Argv`
- `ConversationID`、`UserID`、`SelfID`、`GroupID`
- `IsPrivate`、`IsGroup`、`IsSuperUser`
- `RawEvent`、`Text`、`Target`
- `Stop`、`Stopped`、`Pass`、`StopError`
- `Session`、`UserSession`、`GroupSession`、`SessionBy`
- `Reply`、`ReplyText`

### Adapter 与动作

- `Protocol`
- `Adapter`：`Protocol()`、`Start(ctx, emit)`、`Client()`。
- `EmitFunc`
- `ActionClient`：`Send`。
- `ReplyTarget`
- `MessageReceipt`

### Adapter state

- `AdapterStateKind`：`unknown`、`starting`、`ready`、`disconnected`、`stopped`。
- `AdapterState`
- `AdapterState.Ready() bool`
- `AdapterStateHook`
- `AdapterStateSink`
- `StatefulAdapter`

### Router、Route、Rule

- `Handler`
- `Middleware`
- `Match`
- `Match.WithVar(key, value)`
- `Rule`
- `RuleFunc`
- `Router.Use`
- `Router.Group`
- `Router.On`
- `Router.OnMessage`
- `Router.Command`
- `Router.Regex`
- `Route.Name`
- `Route.Priority`
- `Route.Use`
- `Route.Handle`

规则列表同 SDK。

### Middleware

- `Recover`
- `Trace`
- `Timeout`
- `OnlyPrivate`
- `OnlyGroup`
- `SuperUser`
- `RequireSuperUser`
- `RateLimit`
- `RateLimitBy`

错误：

- `ErrPass`
- `ErrStop`
- `ErrUnauthorized`
- `ErrRateLimited`
- `ErrActionUnavailable`
- `PanicError`

### 生命周期

- `Hook`
- `TaskFunc`
- `TaskOption`
- `TaskCritical`
- `TaskImmediate`

### Store

- `Store`：`Get`、`Set`、`Delete`。
- `MemoryStore`
- `NewMemoryStore()`
- `MemoryStore.Get`、`Set`、`Delete`、`Sweep`
- `FileStore`
- `NewFileStore(path)`
- `FileStore.Path`
- `FileStore.Get`、`Set`、`Delete`、`Sweep`
- `Session`
- `NewSession(store, key)`
- `Session.Key`
- `Session.Get`、`Set`、`Delete`
- `Session.LoadJSON`
- `Session.SaveJSON`

## `core/message`

- `Segment`：`Type`、`Data`。
- `Chain []Segment`
- `New`
- `Raw`
- `Text`
- `Image`
- `Video`
- `Chain.Append`
- `Chain.Clone`
- `Chain.Text`
- `Chain.IsZero`
- `Chain.MarshalJSON`

## `adapters/onebot11`

### Adapter 与配置

- `Config`：`Protocol`、`Transport`。
- `TransportConfig`：`Type`、`Listen`、`Path`、`URL`、`AccessToken`、`AccessTokenEnv`、`Headers`、`DialTimeout`、`ActionTimeout`、`ReconnectInterval`、`ReconnectMaxInterval`。
- `LoadConfig(path) (Config, error)`
- `Config.Validate() error`
- `Config.Options(extra...) ([]Option, error)`
- `LoadAdapter(path, extra...) (*Adapter, error)`
- `AdapterFromConfig(cfg, extra...) (*Adapter, error)`
- `New(transport Transport) *Adapter`
- `ReverseWS(addr, opts...) *Adapter`
- `WebSocket(url, opts...) *Adapter`
- `HTTP(apiURL, listenAddr, opts...) *Adapter`
- `Adapter.Protocol`
- `Adapter.Start`
- `Adapter.Client`
- `Adapter.State`
- `Adapter.SetStateSink`

Options：

- `WithAccessToken`
- `WithPath`
- `WithHeader`
- `WithLogger`
- `WithConnectionHook`
- `WithDialTimeout`
- `WithActionTimeout`
- `WithReconnectInterval`
- `WithReconnectMaxInterval`

连接状态：

- `ConnectionState`：`connected`、`disconnected`。
- `ConnectionEvent`
- `ConnectionHook`

### 消息

- `type Message = message.Chain`
- `Text`
- `At`
- `Reply`
- `Image`
- `Face`
- `Record`
- `Video`
- `JSON`
- `XML`
- `Share`
- `Music`
- `CustomMusic`
- `Node`
- `CustomNode`
- `Custom`
- `ParseCQ`
- `CQString`

### 事件与上下文

- `Event`
- `EventFrom(c *core.Context) (*Event, bool)`
- `Event.UnmarshalJSON`
- `Event.Normalize() *core.Event`
- `Sender`

### Client 动作

通用调用：

- `ClientFrom`
- `MustClient`
- `CallRaw`
- `Call`
- `Send`
- `SendPrivateMessage`
- `SendGroupMessage`
- `SendMessage`

消息与资料：

- `DeleteMessage`
- `GetMessage`
- `GetForwardMessage`
- `SendLike`
- `GetLoginInfo`
- `GetStrangerInfo`
- `GetFriendList`
- `GetGroupList`
- `GetGroupInfo`
- `GetGroupMemberInfo`
- `GetGroupMemberList`
- `GetStatus`
- `GetVersionInfo`

群管理：

- `SetGroupKick`
- `SetGroupBan`
- `SetGroupAnonymousBan`
- `SetGroupWholeBan`
- `SetGroupAdmin`
- `SetGroupAnonymous`
- `SetGroupCard`
- `SetGroupName`
- `SetGroupLeave`
- `SetGroupSpecialTitle`

请求处理：

- `SetFriendAddRequest`
- `SetGroupAddRequest`

文件：

- `UploadGroupFile`
- `UploadPrivateFile`
- `GetGroupFileSystemInfo`
- `GetGroupRootFiles`
- `GetGroupFilesByFolder`
- `CreateGroupFileFolder`
- `DeleteGroupFolder`
- `DeleteGroupFile`
- `GetGroupFileURL`
- `GetImage`
- `GetRecord`

其他：

- `CanSendImage`
- `CanSendRecord`
- `GetCookies`
- `GetCSRFToken`
- `GetCredentials`
- `CleanCache`
- `SetRestart`

数据结构包括 `LoginInfo`、`FriendInfo`、`GroupInfo`、`GroupMemberInfo`、`MessageInfo`、`FileInfo`、`GroupFileSystemInfo`、`GroupFiles`、`GroupFile`、`GroupFolder`、`GroupHonorInfo`、`HonorItem`、`StatusInfo`、`VersionInfo`、`Credentials`、`Response`。

## `adapters/onebot11/napcat`

NapCat 专属扩展动作辅助包。

- `API`：NapCat HTTP API 客户端。
- `Client`：NapCat 所需的 OneBot v11 调用能力。
- `New(client Client) *API`
- `API.Call(ctx, action, params, out) error`
- `API.CallRaw(ctx, action, params) (*onebot11.Response, error)`
- `API.GetGroupMessageHistory(ctx, groupID, messageSeq, count)`
- `API.GetFriendMessageHistory(ctx, userID, messageSeq, count)`
- `API.SetMessageEmojiLike(ctx, messageID, emojiID)`
- `API.DownloadFile(ctx, url, threadCount, headers)`
- `API.GetFile(ctx, fileID)`
- `MessageSummary`
- `FileInfo`

## `internal/scaffold`

内部脚手架，不是公共 API。

- `DefaultPluginTemplate = "basic"`
- `ProjectOptions`：`Dir`、`Module`、`Force`。
- `InitProject(opts ProjectOptions) error`
- `PluginOptions`：`Dir`、`Name`、`Module`、`Template`、`AnyBotVersion`、`AnyBotReplace`、`Force`。
- `PluginResult`：`Name`、`Package`、`Module`、`Template`、`Standalone`、`TestReady`。
- `NewPlugin(opts PluginOptions) (PluginResult, error)`
- `PluginTemplates() []string`

模板：

- `basic`
- `companion`
- `minecraft`
