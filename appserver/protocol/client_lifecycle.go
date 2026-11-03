package protocol

import (
	"context"
	"errors"
	"sync"
)

var errClientClosed = errors.New("client closed")

// Done closes after the transport's accepted notification delivery finishes.
// For a transport without TransportLifecycle, only Client.Close is observable;
// delivery accepted by that fallback means dispatch has entered the client.
func (c *Client) Done() <-chan struct{} { return c.lifecycle.Done() }

// Err reports the terminal transport cause. Custom transports without
// TransportLifecycle report client closure; autonomous termination is not
// observable through the required Transport interface alone.
func (c *Client) Err() error { return c.lifecycle.Err() }

type clientCloseGate struct {
	mu      sync.Mutex
	active  int
	closing bool
	done    chan struct{}
}

func (g *clientCloseGate) Done() <-chan struct{} { return g.done }

func (g *clientCloseGate) Err() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return errClientClosed
	}
	return nil
}

func (g *clientCloseGate) claim() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return false
	}
	g.active++
	return true
}

func (g *clientCloseGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active--
	if g.closing && g.active == 0 {
		close(g.done)
	}
}

func (g *clientCloseGate) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return
	}
	g.closing = true
	if g.active == 0 {
		close(g.done)
	}
}

func (c *Client) handleLegacyNotification(ctx context.Context, notif Notification) {
	if !c.closeGate.claim() {
		return
	}
	defer c.closeGate.release()
	c.handleNotification(ctx, notif)
}
