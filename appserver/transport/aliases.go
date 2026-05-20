package transport

import (
	"time"

	protocol "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const jsonrpcVersion = "2.0"

const (
	ErrCodeParseError     = protocol.ErrCodeParseError
	ErrCodeInvalidRequest = protocol.ErrCodeInvalidRequest
	ErrCodeMethodNotFound = protocol.ErrCodeMethodNotFound
	ErrCodeInvalidParams  = protocol.ErrCodeInvalidParams
	ErrCodeInternalError  = protocol.ErrCodeInternalError
)

var (
	ErrNilContext     = protocol.ErrNilContext
	ErrInvalidParams  = protocol.ErrInvalidParams
	NewTransportError = protocol.NewTransportError
)

var errInvalidParams = ErrInvalidParams

type (
	Client              = protocol.Client
	ClientOption        = protocol.ClientOption
	Error               = protocol.Error
	Notification        = protocol.Notification
	NotificationHandler = protocol.NotificationHandler
	Request             = protocol.Request
	RequestHandler      = protocol.RequestHandler
	RequestID           = protocol.RequestID
	Response            = protocol.Response
	TransportError      = protocol.TransportError

	ApplyPatchApprovalParams          = protocol.ApplyPatchApprovalParams
	ApplyPatchApprovalResponse        = protocol.ApplyPatchApprovalResponse
	ApprovalHandlers                  = protocol.ApprovalHandlers
	FileChangeRequestApprovalParams   = protocol.FileChangeRequestApprovalParams
	FileChangeRequestApprovalResponse = protocol.FileChangeRequestApprovalResponse
	ReviewDecisionWrapper             = protocol.ReviewDecisionWrapper
)

func NewClient(transport protocol.Transport, opts ...ClientOption) *Client {
	return protocol.NewClient(transport, opts...)
}

func WithRequestTimeout(timeout time.Duration) ClientOption {
	return protocol.WithRequestTimeout(timeout)
}
