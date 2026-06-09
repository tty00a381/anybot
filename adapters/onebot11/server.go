package onebot11

import (
	"context"
	"net/http"
	"time"
)

const (
	httpServerReadHeaderTimeout = 5 * time.Second
	httpServerReadTimeout       = 15 * time.Second
	httpServerIdleTimeout       = 60 * time.Second
	httpServerShutdownTimeout   = 5 * time.Second
)

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: httpServerReadHeaderTimeout,
		IdleTimeout:       httpServerIdleTimeout,
	}
}

func newWebhookServer(addr string, handler http.Handler) *http.Server {
	server := newHTTPServer(addr, handler)
	server.ReadTimeout = httpServerReadTimeout
	return server
}

func shutdownHTTPServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), httpServerShutdownTimeout)
	defer cancel()
	_ = server.Shutdown(ctx)
}
