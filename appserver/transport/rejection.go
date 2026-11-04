package transport

import (
	"errors"
	"sync"
)

const maxRetainedRejectionBytes = 64 * 1024 * 1024

var errRejectionByteLimit = errors.New("retained rejection reply byte limit exceeded")

// Reader-owned rejections enter the existing serialized writer queue without
// waiting for the external writer. Admission is not successful delivery: the
// writer still owns actual completion and terminal failure handling.
func (t *StdioTransport) enqueueRejection(id RequestID, code int64, message string) {
	data, err := marshalStdioFrame(errorResponse(id, code, message))
	if err != nil {
		t.handleWriteFailure(NewTransportError("marshal rejection", err))
		return
	}
	reservation, err := t.rejectionBudget.reserve(cap(data))
	if err != nil {
		t.handleWriteFailure(NewTransportError("queue rejection", err))
		return
	}
	if _, err := t.queueReservedPayload(t.ctx, data, "queue rejection", false, reservation); err != nil {
		reservation.release()
		t.handleWriteFailure(err)
	}
}

type rejectionBudget struct {
	mu    sync.Mutex
	bytes int
}

type rejectionReservation struct {
	budget *rejectionBudget
	bytes  int
	once   sync.Once
}

func (b *rejectionBudget) reserve(size int) (*rejectionReservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if size > maxRetainedRejectionBytes-b.bytes {
		return nil, errRejectionByteLimit
	}
	b.bytes += size
	return &rejectionReservation{budget: b, bytes: size}, nil
}

func (r *rejectionReservation) release() {
	r.once.Do(func() {
		r.budget.mu.Lock()
		r.budget.bytes -= r.bytes
		r.budget.mu.Unlock()
	})
}
