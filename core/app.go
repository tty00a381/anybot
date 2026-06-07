package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// App 是 AnyBot 的运行时入口，负责连接适配器、调度事件并执行路由链。
type App struct {
	adapter Adapter
	router  *Router
	store   Store
	logger  *slog.Logger

	workers       int
	buffer        int
	errorHandlers []ErrorHandler
	serial        EventKeyFunc
	locks         *keyedLocks

	startHooks    []Hook
	readyHooks    []Hook
	shutdownHooks []Hook
	superUsers    []string

	stateMu      sync.RWMutex
	adapterState AdapterState
	stateHooks   []AdapterStateHook
	stateWaiters []chan struct{}

	taskMu     sync.Mutex
	tasks      []registeredTask
	taskRunner *taskRunner

	observerMu            sync.RWMutex
	observers             []*Observer
	observerErrorHandlers []ObserverErrorHandler
	observerSlots         chan struct{}
	observerContext       context.Context
}

// New 创建新的运行时实例，并应用传入的配置选项。
func New(opts ...Option) *App {
	app := &App{
		store:   NewMemoryStore(),
		logger:  defaultLogger(),
		workers: defaultWorkers(),
		buffer:  128,
		locks:   newKeyedLocks(),
	}
	app.observerSlots = make(chan struct{}, defaultObserverBuffer())
	app.router = newRouter(app, nil, nil)
	for _, opt := range opts {
		opt(app)
	}
	return app
}

// Adapter 返回当前运行时配置的协议适配器；未配置时返回 nil。
func (a *App) Adapter() Adapter {
	return a.adapter
}

// Client 返回适配器提供的动作客户端；运行时或适配器为空时返回 nil。
func (a *App) Client() ActionClient {
	if a == nil || a.adapter == nil {
		return nil
	}
	return a.adapter.Client()
}

// Store 返回运行时使用的会话存储。
func (a *App) Store() Store {
	return a.store
}

// Logger 返回运行时使用的日志器。
func (a *App) Logger() *slog.Logger {
	return a.logger
}

// Router 返回根路由器，适合在需要细分路由组时使用。
func (a *App) Router() *Router {
	return a.router
}

// Run 启动生命周期钩子、适配器和事件调度器，并在 ctx 结束后退出。
func (a *App) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.adapter == nil {
		return errors.New("anybot: no adapter configured")
	}
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	a.setObserverContext(runCtx)
	defer a.setObserverContext(nil)
	a.bindAdapterState(runCtx)
	var runErr error
	defer func() {
		a.stopAdapterState(context.Background(), runErr)
	}()

	if err := runHooks(runCtx, a.startHooks); err != nil {
		runErr = fmt.Errorf("start hook: %w", err)
		return runErr
	}
	defer func() {
		if err := runHooks(context.Background(), reverseHooks(a.shutdownHooks)); err != nil {
			a.logger.Error("关闭钩子失败", "error", err)
		}
	}()

	if a.workers <= 0 {
		if err := runHooks(runCtx, a.readyHooks); err != nil {
			runErr = fmt.Errorf("ready hook: %w", err)
			return runErr
		}
		tasks := a.startTasks(runCtx, cancelRun)
		defer func() {
			cancelRun()
			a.clearTaskRunner(tasks)
			tasks.stop()
		}()
		err := a.adapter.Start(runCtx, a.dispatch)
		if fatal := tasks.fatalErr(); fatal != nil && ctx.Err() == nil {
			runErr = fatal
			return fatal
		}
		runErr = err
		return err
	}

	type envelope struct {
		ctx   context.Context
		event *Event
	}
	events := make(chan envelope, a.buffer)
	var wg sync.WaitGroup
	for range a.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range events {
				_ = a.dispatch(item.ctx, item.event)
			}
		}()
	}

	if err := runHooks(runCtx, a.readyHooks); err != nil {
		close(events)
		wg.Wait()
		runErr = fmt.Errorf("ready hook: %w", err)
		return runErr
	}
	tasks := a.startTasks(runCtx, cancelRun)
	defer func() {
		cancelRun()
		a.clearTaskRunner(tasks)
		tasks.stop()
	}()

	err := a.adapter.Start(runCtx, func(eventCtx context.Context, event *Event) error {
		select {
		case events <- envelope{ctx: eventCtx, event: event}:
			return nil
		case <-runCtx.Done():
			return runCtx.Err()
		case <-eventCtx.Done():
			return eventCtx.Err()
		}
	})
	close(events)
	wg.Wait()
	if fatal := tasks.fatalErr(); fatal != nil && ctx.Err() == nil {
		runErr = fatal
		return fatal
	}
	runErr = err
	return err
}

// Dispatch 将标准化事件直接投递给运行时，常用于单元测试或自定义适配器。
func (a *App) Dispatch(ctx context.Context, event *Event) error {
	return a.dispatch(ctx, event)
}

func (a *App) dispatch(ctx context.Context, event *Event) error {
	if unlock := a.lockEvent(event); unlock != nil {
		defer unlock()
	}
	c := newContext(ctx, a, event)
	a.dispatchObservers(c)
	for _, route := range a.router.snapshot() {
		if route.handler == nil {
			continue
		}
		c.clearMatch()
		match, ok := route.match(c)
		if !ok {
			continue
		}
		c.applyMatch(route, match)
		c.stopped = false
		err := route.chain()(c)
		switch {
		case errors.Is(err, ErrPass):
			continue
		case errors.Is(err, ErrStop):
		case err != nil:
			a.handleError(c, err)
		}
		if c.Stopped() || errors.Is(err, ErrStop) {
			break
		}
	}
	return nil
}

func (a *App) lockEvent(event *Event) func() {
	if a == nil || a.serial == nil || a.locks == nil {
		return nil
	}
	key := a.serial(event)
	if key == "" {
		return nil
	}
	return a.locks.lock(key)
}

func (a *App) handleError(c *Context, err error) {
	if len(a.errorHandlers) > 0 {
		for _, handler := range a.errorHandlers {
			if handler != nil {
				handler(c, err)
			}
		}
		return
	}
	fields := []any{"error", err}
	if name := routeName(c.Route()); name != "" {
		fields = append(fields, "route", name)
	}
	if event := c.Event(); event != nil {
		fields = append(fields,
			"protocol", event.Protocol,
			"type", event.Type,
			"detail_type", event.DetailType,
			"user_id", event.UserID,
			"group_id", event.GroupID,
		)
	}
	a.logger.Error("处理失败", fields...)
}

// Use 注册全局中间件。全局中间件会包裹所有路由处理函数。
func (a *App) Use(middleware ...Middleware) {
	a.router.Use(middleware...)
}

// Group 创建共享基础规则的路由组，便于把同类路由收束在一起。
func (a *App) Group(rules ...Rule) *Router {
	return a.router.Group(rules...)
}

// On 注册通用事件路由。
func (a *App) On(rules ...Rule) *Route {
	return a.router.On(rules...)
}

// OnMessage 注册消息事件路由，相当于在规则前追加 MessageEvent。
func (a *App) OnMessage(rules ...Rule) *Route {
	return a.router.OnMessage(rules...)
}

// Command 注册命令路由，默认识别 /、!、. 三种命令前缀。
func (a *App) Command(names ...string) *Route {
	return a.router.Command(names...)
}

// Regex 注册文本正则路由，并把匹配结果写入 Context 的匹配变量。
func (a *App) Regex(pattern string) *Route {
	return a.router.Regex(pattern)
}
