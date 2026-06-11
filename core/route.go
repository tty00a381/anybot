package core

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Handler 处理已经匹配成功的事件。
type Handler func(*Context) error

// Middleware 包装处理函数，用于实现日志、鉴权、限速等横切能力。
type Middleware func(Handler) Handler

// Match 描述规则匹配结果，包含评分、诊断原因和传递给处理函数的变量。
type Match struct {
	Score  float64
	Reason string
	Vars   map[string]any
}

// WithVar 向匹配结果追加变量，并返回更新后的 Match。
func (m Match) WithVar(key string, value any) Match {
	if m.Vars == nil {
		m.Vars = map[string]any{}
	}
	m.Vars[key] = value
	return m
}

// Rule 判断路由是否应处理当前事件。
type Rule interface {
	Match(context.Context, *Context) (Match, bool)
}

// RuleFunc 将普通函数适配为 Rule。
type RuleFunc func(context.Context, *Context) (Match, bool)

// Match 调用底层函数完成规则匹配。
func (f RuleFunc) Match(ctx context.Context, c *Context) (Match, bool) {
	return f(ctx, c)
}

// Router 组织路由、中间件和共享基础规则。
type Router struct {
	app *App

	mu         sync.RWMutex
	routes     []*Route
	sorted     []*Route
	dirty      bool
	middleware []Middleware
	nextOrder  int64

	parent     *Router
	baseRules  []Rule
	baseMiddle []Middleware
}

func newRouter(app *App, parent *Router, rules []Rule) *Router {
	return &Router{app: app, parent: parent, baseRules: append([]Rule(nil), rules...)}
}

// Use 追加路由器中间件。根路由器上的中间件作用于所有路由。
func (r *Router) Use(middleware ...Middleware) {
	root := r.root()
	root.mu.Lock()
	defer root.mu.Unlock()
	if r.parent != nil {
		r.baseMiddle = append(r.baseMiddle, middleware...)
		return
	}
	r.middleware = append(r.middleware, middleware...)
}

// Group 创建继承当前基础规则的子路由器。
func (r *Router) Group(rules ...Rule) *Router {
	return &Router{
		app:       r.app,
		parent:    r,
		baseRules: append(append([]Rule(nil), r.baseRules...), rules...),
	}
}

// On 注册一条通用事件路由。
func (r *Router) On(rules ...Rule) *Route {
	root := r.root()
	root.mu.Lock()
	defer root.mu.Unlock()

	root.nextOrder++
	route := &Route{
		router: r,
		rules:  append(append([]Rule(nil), r.baseRules...), rules...),
		order:  root.nextOrder,
	}
	root.routes = append(root.routes, route)
	root.dirty = true
	return route
}

// OnMessage 注册消息事件路由。
func (r *Router) OnMessage(rules ...Rule) *Route {
	return r.On(append([]Rule{MessageEvent()}, rules...)...)
}

// Command 注册命令路由，默认识别 /、!、. 三种前缀。
func (r *Router) Command(names ...string) *Route {
	route := r.OnMessage(CommandRule(names...))
	if len(names) == 0 {
		route.name = "command:*"
	} else {
		route.name = "command:" + strings.Join(names, ",")
	}
	return route
}

// Regex 注册文本正则路由。
func (r *Router) Regex(pattern string) *Route {
	route := r.OnMessage(RegexpRule(regexp.MustCompile(pattern)))
	route.name = "regex:" + pattern
	return route
}

func (r *Router) root() *Router {
	for r.parent != nil {
		r = r.parent
	}
	return r
}

func (r *Router) snapshot() []*Route {
	r.mu.RLock()
	if !r.dirty {
		routes := append([]*Route(nil), r.sorted...)
		r.mu.RUnlock()
		return routes
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dirty {
		routes := append([]*Route(nil), r.routes...)
		sort.SliceStable(routes, func(i, j int) bool {
			if routes[i].priority == routes[j].priority {
				return routes[i].order < routes[j].order
			}
			return routes[i].priority > routes[j].priority
		})
		r.sorted = routes
		r.dirty = false
	}
	return append([]*Route(nil), r.sorted...)
}

// Route 表示一条可配置的事件路由。
type Route struct {
	router *Router

	name       string
	rules      []Rule
	handler    Handler
	middleware []Middleware
	priority   int
	order      int64
}

// Name 设置路由名称，便于日志、追踪和错误诊断。
func (r *Route) Name(name string) *Route {
	r.name = name
	return r
}

// Priority 设置路由优先级；数值越大越先执行，同优先级保持注册顺序。
func (r *Route) Priority(priority int) *Route {
	if r.router != nil {
		root := r.router.root()
		root.mu.Lock()
		r.priority = priority
		root.dirty = true
		root.mu.Unlock()
		return r
	}
	r.priority = priority
	return r
}

// Use 追加只作用于当前路由的中间件。
func (r *Route) Use(middleware ...Middleware) *Route {
	if r.router != nil {
		root := r.router.root()
		root.mu.Lock()
		r.middleware = append(r.middleware, middleware...)
		root.mu.Unlock()
		return r
	}
	r.middleware = append(r.middleware, middleware...)
	return r
}

// Handle 设置路由处理函数。
func (r *Route) Handle(handler Handler) *Route {
	r.handler = handler
	return r
}

func (r *Route) match(c *Context) (Match, bool) {
	var merged Match
	for _, rule := range r.rules {
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

func (r *Route) chain() Handler {
	handler := r.handler
	middleware := r.middlewareChain()
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

func (r *Route) middlewareChain() []Middleware {
	if r == nil || r.router == nil {
		return append([]Middleware(nil), r.middleware...)
	}
	root := r.router.root()
	root.mu.RLock()
	defer root.mu.RUnlock()

	var routers []*Router
	for router := r.router; router != nil; router = router.parent {
		routers = append(routers, router)
	}
	middleware := make([]Middleware, 0, len(r.middleware))
	for i := len(routers) - 1; i >= 0; i-- {
		router := routers[i]
		if router.parent == nil {
			middleware = append(middleware, router.middleware...)
		} else {
			middleware = append(middleware, router.baseMiddle...)
		}
	}
	middleware = append(middleware, r.middleware...)
	return middleware
}
