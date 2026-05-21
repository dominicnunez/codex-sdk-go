package login

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const callbackReadHeaderTimeout = 5 * time.Second
const callbackShutdownTimeout = 5 * time.Second

type CallbackServer struct {
	server *http.Server
	listen net.Listener
	codeCh chan callbackResult
	once   sync.Once
}

type callbackResult struct {
	code AuthorizationCode
	err  error
}

func StartCallbackServer(ctx context.Context, cfg Config, state string) (*CallbackServer, error) {
	cfg = cfg.withDefaults()
	address := net.JoinHostPort(cfg.CallbackHost, strconv.Itoa(cfg.CallbackPort))
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen for OAuth callback: %w", err)
	}

	server := &CallbackServer{
		listen: listener,
		codeCh: make(chan callbackResult, 1),
	}
	server.server = &http.Server{
		Handler:           callbackHandler(state, server.codeCh),
		ReadHeaderTimeout: callbackReadHeaderTimeout,
	}

	go func() {
		if err := server.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			server.send(callbackResult{err: err})
		}
	}()
	return server, nil
}

func (s *CallbackServer) Addr() string {
	if s == nil || s.listen == nil {
		return ""
	}
	return s.listen.Addr().String()
}

func (s *CallbackServer) Wait(ctx context.Context) (AuthorizationCode, error) {
	if s == nil {
		return AuthorizationCode{}, ErrMissingAuthorizationCode
	}
	select {
	case result := <-s.codeCh:
		return result.code, result.err
	case <-ctx.Done():
		return AuthorizationCode{}, ctx.Err()
	}
}

func (s *CallbackServer) Close() error {
	if s == nil || s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), callbackShutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *CallbackServer) send(result callbackResult) {
	s.once.Do(func() {
		s.codeCh <- result
	})
}

func callbackHandler(expectedState string, codeCh chan<- callbackResult) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		code, state, err := parseCallbackQuery(r, expectedState)
		if err != nil {
			writeCallbackHTML(w, http.StatusBadRequest, err.Error())
			return
		}
		writeCallbackHTML(w, http.StatusOK, "OpenAI authentication completed. You can close this window.")
		select {
		case codeCh <- callbackResult{code: AuthorizationCode{Code: code, State: state}}:
		default:
		}
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != callbackPath {
			writeCallbackHTML(w, http.StatusNotFound, "Callback route not found.")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func parseCallbackQuery(r *http.Request, expectedState string) (string, string, error) {
	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	if expectedState != "" && state != expectedState {
		return "", "", ErrStateMismatch
	}
	if code == "" {
		return "", "", ErrMissingAuthorizationCode
	}
	return code, state, nil
}

func writeCallbackHTML(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<!doctype html><html><body><p>%s</p></body></html>", message)
}
