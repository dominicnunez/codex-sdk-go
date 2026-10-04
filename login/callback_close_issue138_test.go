package login

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCallbackServerCloseForceClosesTimedOutCallbackConnection(t *testing.T) {
	callback, active, closed, releaseHandler := startObservedCallbackServer(t, "expected-state", true)
	conn, err := net.DialTimeout("tcp", callback.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial callback listener: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set connection deadline: %v", err)
	}
	request := fmt.Sprintf("POST %s?code=synthetic-code&state=expected-state HTTP/1.1\r\nHost: %s\r\nContent-Length: 10\r\n\r\n", callbackPath, callback.Addr())
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatalf("write incomplete callback request: %v", err)
	}
	assertActiveCallbackConnection(t, active, conn)

	closeErr := callback.Close()
	// The forced Close succeeds, so preserve the original Shutdown error identity.
	if closeErr != context.DeadlineExceeded { //nolint:errorlint
		t.Fatalf("Close() error = %v, want the original graceful-shutdown deadline error", closeErr)
	}
	releaseHandler()
	assertCallbackConnectionClosed(t, conn, closed)
	probe, err := net.DialTimeout("tcp", callback.Addr(), 100*time.Millisecond)
	if err == nil {
		_ = probe.Close()
		t.Fatal("callback listener remained serviceable after concurrent Close")
	}
}

func TestCallbackServerCloseGracefulNilAndRepeated(t *testing.T) {
	var nilServer *CallbackServer
	if err := nilServer.Close(); err != nil {
		t.Fatalf("nil CallbackServer.Close() = %v, want nil", err)
	}
	if err := (&CallbackServer{}).Close(); err != nil {
		t.Fatalf("CallbackServer with nil http.Server Close() = %v, want nil", err)
	}

	callback, _, _, _ := startObservedCallbackServer(t, "expected-state", false)
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + callback.Addr() + callbackPath + "?code=synthetic-code&state=expected-state")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		t.Fatalf("callback status = %s, want 200 OK", response.Status)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close callback response body: %v", err)
	}
	if err := callback.Close(); err != nil {
		t.Fatalf("graceful CallbackServer.Close() = %v, want nil", err)
	}
	if err := callback.Close(); err != nil {
		t.Fatalf("repeated CallbackServer.Close() = %v, want nil", err)
	}

	concurrent, _, _, _ := startObservedCallbackServer(t, "expected-state", false)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			results <- concurrent.Close()
		}()
	}
	close(start)
	for range 2 {
		select {
		case <-results: // Preserve standard-library listener-close error variance.
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent Close() did not return")
		}
	}
	probe, err := net.DialTimeout("tcp", concurrent.Addr(), 100*time.Millisecond)
	if err == nil {
		_ = probe.Close()
		t.Fatal("callback listener remained serviceable after concurrent Close")
	}

}

func TestCallbackServerCloseGracefullyWaitsForAcceptedHandler(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	shutdownStarted := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})}
	server.RegisterOnShutdown(func() { close(shutdownStarted) })
	callback := &CallbackServer{server: server, listen: listener}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		_ = server.Close()
		select {
		case <-serveDone:
		case <-time.After(time.Second):
			t.Error("graceful callback Serve did not stop during cleanup")
		}
	})

	client := &http.Client{Timeout: 3 * time.Second}
	responseCh := make(chan *http.Response, 1)
	requestErr := make(chan error, 1)
	go func() {
		response, err := client.Get("http://" + listener.Addr().String() + "/held")
		if err != nil {
			requestErr <- err
			return
		}
		responseCh <- response
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("test handler did not admit the callback connection")
	}

	closeResult := make(chan error, 1)
	go func() { closeResult <- callback.Close() }()
	select {
	case <-shutdownStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("http.Server did not begin graceful shutdown")
	}
	select {
	case err := <-closeResult:
		t.Fatalf("Close returned before the admitted handler was released: %v", err)
	default:
	}
	releaseOnce.Do(func() { close(release) })

	select {
	case err := <-closeResult:
		if err != nil {
			t.Fatalf("graceful Close() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not finish after the admitted handler completed")
	}
	select {
	case err := <-requestErr:
		t.Fatalf("accepted handler request: %v", err)
	case response := <-responseCh:
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("handler status = %s, want 204 No Content", response.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("accepted handler did not finish its HTTP response")
	}
}

func startObservedCallbackServer(t *testing.T, state string, holdActive bool) (*CallbackServer, chan net.Conn, chan net.Conn, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	active := make(chan net.Conn, 1)
	closed := make(chan net.Conn, 1)
	codeCh := make(chan callbackResult, 1)
	activeGate := make(chan struct{})
	var releaseOnce sync.Once
	releaseActive := func() { releaseOnce.Do(func() { close(activeGate) }) }
	server := &http.Server{
		Handler:           callbackHandler(state, codeCh),
		ReadHeaderTimeout: callbackReadHeaderTimeout,
		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateActive:
				select {
				case active <- conn:
				default:
				}
				if holdActive {
					<-activeGate
				}
			case http.StateClosed:
				select {
				case closed <- conn:
				default:
				}
			}
		},
	}
	callback := &CallbackServer{server: server, listen: listener, codeCh: codeCh}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		releaseActive()
		_ = server.Close()
		select {
		case <-serveDone:
		case <-time.After(time.Second):
			t.Error("callback Serve did not stop during cleanup")
		}
	})
	return callback, active, closed, releaseActive
}

func assertActiveCallbackConnection(t *testing.T, active <-chan net.Conn, client net.Conn) {
	t.Helper()
	select {
	case accepted := <-active:
		if accepted.RemoteAddr().String() != client.LocalAddr().String() || accepted.LocalAddr().String() != client.RemoteAddr().String() {
			t.Fatalf("ConnState observed a different connection: local=%s remote=%s", accepted.LocalAddr(), accepted.RemoteAddr())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the exact callback connection did not reach StateActive")
	}
}

func assertCallbackConnectionClosed(t *testing.T, conn net.Conn, closed <-chan net.Conn) {
	t.Helper()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set post-close connection deadline: %v", err)
	}
	_, writeErr := io.WriteString(conn, "0123456789")
	var closedConn net.Conn
	select {
	case closedConn = <-closed:
	case <-time.After(time.Second):
		t.Error("server did not report the accepted connection closed after Close")
	}
	response, readErr := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodPost})
	if readErr == nil {
		_ = response.Body.Close()
		t.Fatalf("callback connection completed an HTTP response after Close: status=%s", response.Status)
	}
	var networkErr net.Error
	if errors.As(readErr, &networkErr) && networkErr.Timeout() {
		t.Fatalf("connection remained open instead of closing after Close (write error %v): %v", writeErr, readErr)
	}
	if closedConn != nil && (closedConn.RemoteAddr().String() != conn.LocalAddr().String() || closedConn.LocalAddr().String() != conn.RemoteAddr().String()) {
		t.Fatalf("StateClosed observed a different connection: local=%s remote=%s", closedConn.LocalAddr(), closedConn.RemoteAddr())
	}
}

func TestLoginCleanupForceClosesAcceptedCallbackConnection(t *testing.T) {
	callbackFailure := errors.New("synthetic authorization URL callback failure")
	var callback *CallbackServer
	var active chan net.Conn
	var closed chan net.Conn
	var callbackConn net.Conn
	var releaseHandler func()
	tokenRequests := 0
	start := func(_ context.Context, _ Config, state string) (*CallbackServer, error) {
		callback, active, closed, releaseHandler = startObservedCallbackServer(t, state, true)
		return callback, nil
	}
	client := &http.Client{Transport: auditTokenTransport(func(*http.Request) (*http.Response, error) {
		tokenRequests++
		return nil, fmt.Errorf("unexpected token exchange")
	})}
	opts := LoginOptions{
		Config: Config{HTTPClient: client},
		OnAuthURL: func(_ context.Context, authURL string) error {
			parsed, err := url.Parse(authURL)
			if err != nil {
				return fmt.Errorf("parse synthetic auth URL: %w", err)
			}
			conn, err := net.DialTimeout("tcp", callback.Addr(), time.Second)
			if err != nil {
				return fmt.Errorf("dial synthetic callback listener: %w", err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			callbackConn = conn
			if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				return fmt.Errorf("set synthetic callback deadline: %w", err)
			}
			request := fmt.Sprintf("POST %s?code=synthetic-code&state=%s HTTP/1.1\r\nHost: %s\r\nContent-Length: 10\r\n\r\n", callbackPath, url.QueryEscape(parsed.Query().Get("state")), callback.Addr())
			if _, err := io.WriteString(conn, request); err != nil {
				return fmt.Errorf("write incomplete synthetic callback request: %w", err)
			}
			assertActiveCallbackConnection(t, active, conn)
			return callbackFailure
		},
	}
	credentials, err := loginWithCallbackServer(context.Background(), opts, start)
	if err == nil || !errors.Is(err, callbackFailure) {
		t.Fatalf("Login error = %v, want wrapped OnAuthURL error", err)
	}
	if credentials.AccessToken != "" || credentials.RefreshToken != "" || !credentials.ExpiresAt.IsZero() || credentials.AccountID != "" || credentials.PlanType != nil {
		t.Fatalf("Login returned credentials after OnAuthURL failure: %+v", credentials.Redacted())
	}
	if tokenRequests != 0 {
		t.Fatalf("token endpoint received %d requests after OnAuthURL failure", tokenRequests)
	}
	if callback == nil || active == nil || closed == nil {
		t.Fatal("Login did not start the instrumented callback server")
	}
	if callbackConn == nil {
		t.Fatal("OnAuthURL did not retain its callback connection")
	}
	releaseHandler()
	assertCallbackConnectionClosed(t, callbackConn, closed)
}

func TestLoginPublicCallbackAndTokenExchangeStillSucceed(t *testing.T) {
	var tokenRequests atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests.Add(1)
		form := tokenRequestForm(t, r)
		if form.Get("code") != "synthetic-code" || form.Get("grant_type") != grantTypeAuthorizationCode {
			t.Errorf("token exchange form = %v", form)
		}
		writeTokenResponse(t, w, fakeAccessToken(t, "acct_callback", "plus"), "refresh_callback")
	}))
	t.Cleanup(tokenServer.Close)

	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve callback port: %v", err)
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	if err := reservation.Close(); err != nil {
		t.Fatalf("release callback port reservation: %v", err)
	}
	callbackURL := "http://127.0.0.1:" + strconv.Itoa(port) + callbackPath
	callbackClient := &http.Client{Timeout: 2 * time.Second}
	opts := LoginOptions{
		Config: Config{
			CallbackHost:  "127.0.0.1",
			CallbackPort:  port,
			RedirectURI:   callbackURL,
			TokenEndpoint: tokenServer.URL,
			HTTPClient:    tokenServer.Client(),
		},
		OnAuthURL: func(ctx context.Context, authURL string) error {
			authorize, err := url.Parse(authURL)
			if err != nil {
				return fmt.Errorf("parse auth URL: %w", err)
			}
			redirect, err := url.Parse(authorize.Query().Get("redirect_uri"))
			if err != nil {
				return fmt.Errorf("parse callback redirect: %w", err)
			}
			query := redirect.Query()
			query.Set("code", "synthetic-code")
			query.Set("state", authorize.Query().Get("state"))
			redirect.RawQuery = query.Encode()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, redirect.String(), nil)
			if err != nil {
				return err
			}
			response, err := callbackClient.Do(request)
			if err != nil {
				return fmt.Errorf("request local synthetic callback: %w", err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				return fmt.Errorf("callback status = %s", response.Status)
			}
			return nil
		},
	}
	credentials, err := Login(context.Background(), opts)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if tokenRequests.Load() != 1 {
		t.Fatalf("token request count = %d, want 1", tokenRequests.Load())
	}
	if credentials.AccountID != "acct_callback" || credentials.RefreshToken != "refresh_callback" || credentials.AccessToken == "" || credentials.ExpiresAt.IsZero() {
		t.Fatalf("Login credentials = %+v", credentials.Redacted())
	}
}
