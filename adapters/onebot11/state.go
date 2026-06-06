package onebot11

import (
	"context"
	"sync"
	"time"

	"github.com/tty00a381/anybot/core"
)

type adapterState struct {
	mu    sync.RWMutex
	state core.AdapterState
	sink  core.AdapterStateSink
}

func newAdapterState() *adapterState {
	return &adapterState{state: core.AdapterState{
		Protocol: core.ProtocolOneBot11,
		Kind:     core.AdapterStateUnknown,
	}}
}

func (s *adapterState) State() core.AdapterState {
	if s == nil {
		return core.AdapterState{Protocol: core.ProtocolOneBot11, Kind: core.AdapterStateUnknown}
	}
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()
	return state
}

func (s *adapterState) SetSink(sink core.AdapterStateSink) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.sink = sink
	state := s.state
	s.mu.Unlock()
	if sink != nil && state.Kind != core.AdapterStateUnknown {
		sink(context.Background(), state)
	}
}

func (s *adapterState) Set(ctx context.Context, state core.AdapterState) {
	if s == nil {
		return
	}
	if state.Protocol == "" {
		state.Protocol = core.ProtocolOneBot11
	}
	if state.At.IsZero() {
		state.At = time.Now()
	}
	s.mu.Lock()
	s.state = state
	sink := s.sink
	s.mu.Unlock()
	if sink != nil {
		sink(ctx, state)
	}
}

func (opts options) emitAdapterState(ctx context.Context, state core.AdapterState) {
	if opts.state != nil {
		opts.state.Set(ctx, state)
	}
}

func (opts options) emitConnectionState(ctx context.Context, event ConnectionEvent) {
	state := core.AdapterState{
		Protocol:  core.ProtocolOneBot11,
		Transport: event.Transport,
		Reason:    string(event.State),
	}
	switch event.State {
	case ConnectionConnected:
		state.Kind = core.AdapterStateReady
		state.ActionReady = true
	case ConnectionDisconnected:
		state.Kind = core.AdapterStateDisconnected
	default:
		state.Kind = core.AdapterStateUnknown
	}
	opts.emitAdapterState(ctx, state)
}
