package codex

import (
	"context"
	"encoding/json"
	"errors"
)

type writeEnvelope struct {
	payload []byte
	done    chan error
}

// writeRawMessage writes a pre-marshaled JSON-RPC message and trailing newline.
func (t *StdioTransport) writeRawMessage(data []byte) error {
	// Write message then newline delimiter, handling short writes.
	// The newline is written separately to avoid copying the entire
	// payload just to append one byte.
	for len(data) > 0 {
		n, err := t.writer.Write(data)
		if err != nil {
			return NewTransportError("write message", err)
		}
		if n == 0 {
			return NewTransportError("write message", errors.New("writer returned zero bytes written without error"))
		}
		data = data[n:]
	}

	delim := []byte{'\n'}
	for len(delim) > 0 {
		n, err := t.writer.Write(delim)
		if err != nil {
			return NewTransportError("write message", err)
		}
		if n == 0 {
			return NewTransportError("write message", errors.New("writer returned zero bytes written without error"))
		}
		delim = delim[n:]
	}

	return nil
}

func (t *StdioTransport) enqueueWrite(ctx context.Context, msg interface{}, op string, watchReaderStop bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := marshalStdioFrame(msg)
	if err != nil {
		return NewTransportError("marshal message", err)
	}
	env := writeEnvelope{
		payload: data,
		done:    make(chan error, 1),
	}

	if watchReaderStop {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return t.transportStopError(op)
		case <-t.readerStopped:
			return t.transportStopError(op)
		case t.writeQueue <- env:
		}
	} else {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return NewTransportError(op, errTransportClosed)
		case t.writeQueue <- env:
		}
	}

	if watchReaderStop {
		select {
		case err := <-env.done:
			return t.normalizeWriteCompletionError(op, err)
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return t.transportStopError(op)
		case <-t.readerStopped:
			return t.transportStopError(op)
		}
	}

	select {
	case err := <-env.done:
		return t.normalizeWriteCompletionError(op, err)
	case <-ctx.Done():
		return ctx.Err()
	case <-t.ctx.Done():
		return NewTransportError(op, errTransportClosed)
	}
}

func marshalStdioFrame(msg interface{}) ([]byte, error) {
	switch v := msg.(type) {
	case Request:
		return marshalStdioRequest(v)
	case *Request:
		if v == nil {
			return json.Marshal(v)
		}
		return marshalStdioRequest(*v)
	case Response:
		return marshalStdioResponse(v)
	case *Response:
		if v == nil {
			return json.Marshal(v)
		}
		return marshalStdioResponse(*v)
	case Notification:
		return marshalStdioNotification(v)
	case *Notification:
		if v == nil {
			return json.Marshal(v)
		}
		return marshalStdioNotification(*v)
	default:
		return json.Marshal(msg)
	}
}

func marshalStdioRequest(req Request) ([]byte, error) {
	type wireRequest struct {
		ID     RequestID       `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params,omitempty"`
	}
	return json.Marshal(wireRequest{
		ID:     req.ID,
		Method: req.Method,
		Params: req.Params,
	})
}

func marshalStdioResponse(resp Response) ([]byte, error) {
	type wireResponse struct {
		ID     RequestID       `json:"id"`
		Result json.RawMessage `json:"result,omitempty"`
		Error  *Error          `json:"error,omitempty"`
	}
	return json.Marshal(wireResponse{
		ID:     resp.ID,
		Result: resp.Result,
		Error:  resp.Error,
	})
}

func marshalStdioNotification(notif Notification) ([]byte, error) {
	type wireNotification struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params,omitempty"`
	}
	return json.Marshal(wireNotification{
		Method: notif.Method,
		Params: notif.Params,
	})
}

func (t *StdioTransport) normalizeWriteCompletionError(op string, err error) error {
	if err == nil {
		return nil
	}

	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return t.transportStopError(op)
	}

	select {
	case <-t.readerStopped:
		return t.transportStopError(op)
	case <-t.ctx.Done():
		return t.transportStopError(op)
	default:
		return err
	}
}

// writeMessage enqueues a JSON-RPC message for serialized writer-loop delivery.
func (t *StdioTransport) writeMessage(msg interface{}) error {
	return t.enqueueWrite(t.ctx, msg, "write message", false)
}

func recvWhileRunning[T any](ctx context.Context, queue <-chan T) (T, bool) {
	var zero T

	select {
	case <-ctx.Done():
		return zero, false
	case value, ok := <-queue:
		if !ok || ctx.Err() != nil {
			return zero, false
		}
		return value, true
	}
}

func (t *StdioTransport) writeLoop() {
	for {
		env, ok := recvWhileRunning(t.ctx, t.writeQueue)
		if !ok {
			return
		}

		err := t.writeRawMessage(env.payload)
		if err != nil {
			t.handleWriteFailure(err)
			env.done <- err
			return
		}
		env.done <- nil
	}
}
