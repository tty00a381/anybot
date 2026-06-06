package core

import (
	"context"
	"runtime/debug"
	"time"
)

// ObserverHandler 异步观察事件，不参与路由控制。
type ObserverHandler func(context.Context, *Event) error

// ObserverErrorHandler 处理事件观察者返回的错误或 panic。
type ObserverErrorHandler func(context.Context, *Event, error)

// Observer 表示一条事件旁路观察规则。
type Observer struct {
	app *App

	name    string
	rules   []Rule
	handler ObserverHandler
}

// Observe 注册事件旁路观察者。观察者不会参与 route ordering、Stop 或 Pass。
func (a *App) Observe(rules ...Rule) *Observer {
	if a == nil {
		return nil
	}
	observer := &Observer{app: a, rules: append([]Rule(nil), rules...)}
	a.observerMu.Lock()
	a.observers = append(a.observers, observer)
	a.observerMu.Unlock()
	return observer
}

// OnObserverError 注册观察者错误处理器。
func (a *App) OnObserverError(handler ObserverErrorHandler) {
	if a == nil || handler == nil {
		return
	}
	a.observerMu.Lock()
	a.observerErrorHandlers = append(a.observerErrorHandlers, handler)
	a.observerMu.Unlock()
}

// Name 设置观察者名称，便于日志诊断。
func (o *Observer) Name(name string) *Observer {
	if o != nil {
		o.name = name
	}
	return o
}

// Handle 设置观察者处理函数。
func (o *Observer) Handle(handler ObserverHandler) *Observer {
	if o != nil {
		o.handler = handler
	}
	return o
}

func (a *App) dispatchObservers(c *Context) {
	if a == nil || c == nil || c.event == nil {
		return
	}
	observers := a.snapshotObservers()
	for _, observer := range observers {
		if observer == nil || observer.handler == nil {
			continue
		}
		c.clearMatch()
		if _, ok := observer.match(c); !ok {
			continue
		}
		a.enqueueObserver(c.Context, observer, c.event.Clone())
	}
	c.clearMatch()
}

func (a *App) snapshotObservers() []*Observer {
	a.observerMu.RLock()
	observers := append([]*Observer(nil), a.observers...)
	a.observerMu.RUnlock()
	return observers
}

func (o *Observer) match(c *Context) (Match, bool) {
	var merged Match
	for _, rule := range o.rules {
		if rule == nil {
			continue
		}
		match, ok := rule.Match(c.Context, c)
		if !ok {
			return Match{}, false
		}
		merged = mergeMatch(merged, match)
	}
	return merged, true
}

func (a *App) enqueueObserver(ctx context.Context, observer *Observer, event *Event) {
	if ctx == nil {
		ctx = context.Background()
	}
	observerCtx := a.observerHandlerContext(ctx)
	select {
	case a.observerSlots <- struct{}{}:
		go a.runObserver(observerCtx, observer, event)
	default:
		a.logger.Warn("观察者队列已满", "observer", observerName(observer))
	}
}

func (a *App) setObserverContext(ctx context.Context) {
	if a == nil {
		return
	}
	a.observerMu.Lock()
	a.observerContext = ctx
	a.observerMu.Unlock()
}

func (a *App) observerHandlerContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	values := context.WithoutCancel(ctx)
	a.observerMu.RLock()
	lifecycle := a.observerContext
	a.observerMu.RUnlock()
	if lifecycle == nil {
		return values
	}
	return observerContext{values: values, lifecycle: lifecycle}
}

func (a *App) runObserver(ctx context.Context, observer *Observer, event *Event) {
	defer func() {
		<-a.observerSlots
		if recovered := recover(); recovered != nil {
			a.handleObserverError(ctx, observer, event, &PanicError{Value: recovered, Stack: debug.Stack()})
		}
	}()
	if err := observer.handler(ctx, event); err != nil {
		a.handleObserverError(ctx, observer, event, err)
	}
}

func (a *App) handleObserverError(ctx context.Context, observer *Observer, event *Event, err error) {
	if err == nil {
		return
	}
	a.observerMu.RLock()
	handlers := append([]ObserverErrorHandler(nil), a.observerErrorHandlers...)
	a.observerMu.RUnlock()
	if len(handlers) > 0 {
		for _, handler := range handlers {
			if handler != nil {
				handler(ctx, event, err)
			}
		}
		return
	}
	a.logger.Error("观察者失败", "observer", observerName(observer), "error", err)
}

func observerName(observer *Observer) string {
	if observer == nil || observer.name == "" {
		return "observer"
	}
	return observer.name
}

type observerContext struct {
	values    context.Context
	lifecycle context.Context
}

func (c observerContext) Deadline() (time.Time, bool) {
	return c.lifecycle.Deadline()
}

func (c observerContext) Done() <-chan struct{} {
	return c.lifecycle.Done()
}

func (c observerContext) Err() error {
	return c.lifecycle.Err()
}

func (c observerContext) Value(key any) any {
	return c.values.Value(key)
}
