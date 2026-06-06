package core

import (
	"context"
	"time"
)

// AdapterStateKind 描述适配器动作通道的当前运行状态。
type AdapterStateKind string

const (
	// AdapterStateUnknown 表示运行时尚未收到适配器状态。
	AdapterStateUnknown AdapterStateKind = "unknown"
	// AdapterStateStarting 表示适配器正在启动。
	AdapterStateStarting AdapterStateKind = "starting"
	// AdapterStateReady 表示动作客户端当前可用。
	AdapterStateReady AdapterStateKind = "ready"
	// AdapterStateDisconnected 表示动作客户端暂不可用，但运行时仍可能继续等待重连。
	AdapterStateDisconnected AdapterStateKind = "disconnected"
	// AdapterStateStopped 表示适配器已经停止。
	AdapterStateStopped AdapterStateKind = "stopped"
)

// AdapterState 是协议无关的适配器动作通道状态。
type AdapterState struct {
	Protocol    Protocol
	Kind        AdapterStateKind
	ActionReady bool
	Transport   string
	Reason      string
	Err         error
	At          time.Time
}

// Ready 判断动作客户端当前是否可用。
func (s AdapterState) Ready() bool {
	return s.ActionReady
}

// AdapterStateHook 接收适配器动作通道状态变化。
type AdapterStateHook func(context.Context, AdapterState)

// AdapterStateSink 是适配器向运行时报告状态变化的函数。
type AdapterStateSink func(context.Context, AdapterState)

// StatefulAdapter 是可向运行时暴露动作通道状态的适配器扩展接口。
type StatefulAdapter interface {
	State() AdapterState
	SetStateSink(AdapterStateSink)
}

// OnAdapterState 注册适配器状态变化钩子。
func (a *App) OnAdapterState(hook AdapterStateHook) {
	if a == nil || hook == nil {
		return
	}
	a.stateMu.Lock()
	a.stateHooks = append(a.stateHooks, hook)
	a.stateMu.Unlock()
}

// AdapterState 返回最近一次已知的适配器状态。
func (a *App) AdapterState() AdapterState {
	if a == nil {
		return normalizeAdapterState(AdapterState{})
	}
	a.stateMu.RLock()
	state := a.adapterState
	a.stateMu.RUnlock()
	return normalizeAdapterState(state)
}

// WaitActionReady 阻塞直到动作客户端可用，或 ctx 结束。
func (a *App) WaitActionReady(ctx context.Context) error {
	if a == nil {
		return ErrActionUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		ch, state := a.waitAdapterStateChange()
		if state.ActionReady {
			return nil
		}
		if state.Kind == AdapterStateStopped {
			return ErrActionUnavailable
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (a *App) bindAdapterState(ctx context.Context) {
	if a == nil || a.adapter == nil {
		return
	}
	a.updateAdapterState(ctx, AdapterState{
		Protocol: a.adapter.Protocol(),
		Kind:     AdapterStateStarting,
		Reason:   "adapter starting",
	})
	if stateful, ok := a.adapter.(StatefulAdapter); ok {
		stateful.SetStateSink(a.updateAdapterState)
	}
}

func (a *App) stopAdapterState(ctx context.Context, err error) {
	if a == nil || a.adapter == nil {
		return
	}
	state := AdapterState{
		Protocol: a.adapter.Protocol(),
		Kind:     AdapterStateStopped,
		Reason:   "adapter stopped",
		Err:      err,
	}
	a.updateAdapterState(ctx, state)
	if stateful, ok := a.adapter.(StatefulAdapter); ok {
		stateful.SetStateSink(nil)
	}
}

func (a *App) updateAdapterState(ctx context.Context, state AdapterState) {
	if a == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	state = normalizeAdapterState(state)

	a.stateMu.Lock()
	a.adapterState = state
	hooks := append([]AdapterStateHook(nil), a.stateHooks...)
	waiters := a.stateWaiters
	a.stateWaiters = nil
	a.stateMu.Unlock()

	for _, ch := range waiters {
		close(ch)
	}
	for _, hook := range hooks {
		if hook != nil {
			hook(ctx, state)
		}
	}
}

func (a *App) waitAdapterStateChange() (<-chan struct{}, AdapterState) {
	if a == nil {
		ch := make(chan struct{})
		close(ch)
		return ch, normalizeAdapterState(AdapterState{})
	}
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	state := normalizeAdapterState(a.adapterState)
	if state.ActionReady {
		ch := make(chan struct{})
		close(ch)
		return ch, state
	}
	ch := make(chan struct{})
	a.stateWaiters = append(a.stateWaiters, ch)
	return ch, state
}

func normalizeAdapterState(state AdapterState) AdapterState {
	if state.Kind == "" {
		state.Kind = AdapterStateUnknown
	}
	if state.At.IsZero() {
		state.At = time.Now()
	}
	if state.Kind != AdapterStateReady {
		state.ActionReady = false
	}
	return state
}
