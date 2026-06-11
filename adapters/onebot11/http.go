package onebot11

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tty00a381/anybot/core"
)

type httpTransport struct {
	apiURL     string
	listenAddr string
	opts       options
	client     *http.Client
}

func newHTTPTransport(apiURL, listenAddr string, opts options) *httpTransport {
	return &httpTransport{
		apiURL:     strings.TrimRight(apiURL, "/"),
		listenAddr: listenAddr,
		opts:       opts,
		client:     http.DefaultClient,
	}
}

func (t *httpTransport) Start(ctx context.Context, sink func(context.Context, *Event) error) error {
	if t.listenAddr != "" {
		if err := t.opts.checkPublicListener("http", t.listenAddr); err != nil {
			return err
		}
	}
	t.opts.emitAdapterState(ctx, core.AdapterState{
		Protocol:    Protocol,
		Kind:        core.AdapterStateReady,
		ActionReady: t.apiURL != "",
		Transport:   "http",
		Reason:      "http transport started",
	})
	if t.listenAddr == "" {
		<-ctx.Done()
		return ctx.Err()
	}
	server := newWebhookServer(t.listenAddr, t.handler(sink))
	errc := make(chan error, 1)
	go func() {
		t.opts.logger.Info("HTTP回调监听中", "addr", t.listenAddr, "path", t.opts.path)
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errc <- err
	}()
	select {
	case <-ctx.Done():
		shutdownHTTPServer(server)
		err := <-errc
		if err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errc:
		return err
	}
}

func (t *httpTransport) handler(sink func(context.Context, *Event) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(t.opts.path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !t.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, t.opts.maxEventBytes)
		defer r.Body.Close()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := sink(r.Context(), &event); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

func (t *httpTransport) authorized(r *http.Request) bool {
	if t.opts.accessToken == "" {
		return true
	}
	if r.Header.Get("Authorization") == "Bearer "+t.opts.accessToken {
		return true
	}
	return r.URL.Query().Get("access_token") == t.opts.accessToken
}

func (t *httpTransport) CallRaw(ctx context.Context, action string, params any) (*Response, error) {
	if t.apiURL == "" {
		return nil, errors.New("onebot11: http api url is required")
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.apiURL+"/"+action, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, values := range t.opts.header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if t.opts.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.opts.accessToken)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("onebot11: http status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out Response
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	out.Raw = append(out.Raw[:0], data...)
	return &out, nil
}
