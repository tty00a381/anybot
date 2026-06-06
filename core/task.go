package core

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// TaskFunc 是由 App 生命周期托管的后台任务函数。
type TaskFunc func(context.Context) error

// TaskOption 调整后台任务行为。
type TaskOption func(*taskOptions)

type taskOptions struct {
	critical  bool
	immediate bool
}

// TaskCritical 让任务失败时停止 App.Run，并把任务错误作为运行错误返回。
func TaskCritical() TaskOption {
	return func(opts *taskOptions) {
		opts.critical = true
	}
}

// TaskImmediate 让周期任务在启动后立即执行一次，然后再按间隔执行。
func TaskImmediate() TaskOption {
	return func(opts *taskOptions) {
		opts.immediate = true
	}
}

type taskKind uint8

const (
	taskOnce taskKind = iota
	taskEvery
)

type registeredTask struct {
	name     string
	kind     taskKind
	interval time.Duration
	fn       TaskFunc
	options  taskOptions
}

// Go 注册一个随 App 生命周期启动和停止的后台任务。
func (a *App) Go(name string, fn TaskFunc, opts ...TaskOption) {
	if a == nil || fn == nil {
		return
	}
	a.addTask(registeredTask{name: taskName(name), kind: taskOnce, fn: fn, options: applyTaskOptions(opts)})
}

// Every 注册一个随 App 生命周期托管的周期任务。
func (a *App) Every(name string, interval time.Duration, fn TaskFunc, opts ...TaskOption) {
	if a == nil || interval <= 0 || fn == nil {
		return
	}
	a.addTask(registeredTask{name: taskName(name), kind: taskEvery, interval: interval, fn: fn, options: applyTaskOptions(opts)})
}

func (a *App) addTask(task registeredTask) {
	a.taskMu.Lock()
	a.tasks = append(a.tasks, task)
	runner := a.taskRunner
	a.taskMu.Unlock()
	if runner != nil {
		runner.start(task)
	}
}

func (a *App) startTasks(ctx context.Context, cancel context.CancelFunc) *taskRunner {
	runner := &taskRunner{app: a, ctx: ctx, cancel: cancel}
	a.taskMu.Lock()
	a.taskRunner = runner
	tasks := append([]registeredTask(nil), a.tasks...)
	a.taskMu.Unlock()
	for _, task := range tasks {
		runner.start(task)
	}
	return runner
}

func (a *App) clearTaskRunner(runner *taskRunner) {
	a.taskMu.Lock()
	if a.taskRunner == runner {
		a.taskRunner = nil
	}
	a.taskMu.Unlock()
}

type taskRunner struct {
	app    *App
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	errMu sync.Mutex
	err   error
}

func (r *taskRunner) start(task registeredTask) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		switch task.kind {
		case taskEvery:
			r.runEvery(task)
		default:
			r.runOnce(task)
		}
	}()
}

func (r *taskRunner) stop() {
	r.wg.Wait()
}

func (r *taskRunner) fatalErr() error {
	r.errMu.Lock()
	defer r.errMu.Unlock()
	return r.err
}

func (r *taskRunner) setFatal(err error) {
	if err == nil {
		return
	}
	r.errMu.Lock()
	if r.err == nil {
		r.err = err
	}
	r.errMu.Unlock()
	r.cancel()
}

func (r *taskRunner) runOnce(task registeredTask) {
	err := r.call(task)
	r.handleTaskError(task, err)
}

func (r *taskRunner) runEvery(task registeredTask) {
	if task.options.immediate {
		if err := r.call(task); r.handleTaskError(task, err) {
			return
		}
	}
	timer := time.NewTimer(task.interval)
	defer timer.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-timer.C:
			if err := r.call(task); r.handleTaskError(task, err) {
				return
			}
			timer.Reset(task.interval)
		}
	}
}

func (r *taskRunner) call(task registeredTask) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &PanicError{Value: recovered, Stack: debug.Stack()}
		}
	}()
	return task.fn(r.ctx)
}

func (r *taskRunner) handleTaskError(task registeredTask, err error) bool {
	if err == nil || taskContextDone(r.ctx, err) {
		return false
	}
	r.app.logger.Error("后台任务失败", "task", task.name, "error", err)
	if task.options.critical {
		r.setFatal(fmt.Errorf("anybot: task %s failed: %w", task.name, err))
		return true
	}
	return false
}

func taskContextDone(ctx context.Context, err error) bool {
	if ctx == nil || ctx.Err() == nil {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ctx.Err())
}

func applyTaskOptions(opts []TaskOption) taskOptions {
	var out taskOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

func taskName(name string) string {
	if name == "" {
		return "task"
	}
	return name
}
