package transport

import (
	"time"

	sdk "github.com/dominicnunez/codex-sdk-go/sdk"
)

const jsonrpcVersion = "2.0"

const (
	ErrCodeParseError     = sdk.ErrCodeParseError
	ErrCodeInvalidRequest = sdk.ErrCodeInvalidRequest
	ErrCodeMethodNotFound = sdk.ErrCodeMethodNotFound
	ErrCodeInvalidParams  = sdk.ErrCodeInvalidParams
	ErrCodeInternalError  = sdk.ErrCodeInternalError
)

var (
	ErrNilContext     = sdk.ErrNilContext
	ErrInvalidParams  = sdk.ErrInvalidParams
	NewTransportError = sdk.NewTransportError
)

var errInvalidParams = ErrInvalidParams

type (
	Client              = sdk.Client
	ClientOption        = sdk.ClientOption
	Error               = sdk.Error
	Notification        = sdk.Notification
	NotificationHandler = sdk.NotificationHandler
	Request             = sdk.Request
	RequestHandler      = sdk.RequestHandler
	RequestID           = sdk.RequestID
	Response            = sdk.Response
	TransportError      = sdk.TransportError

	ApplyPatchApprovalParams          = sdk.ApplyPatchApprovalParams
	ApplyPatchApprovalResponse        = sdk.ApplyPatchApprovalResponse
	ApprovalHandlers                  = sdk.ApprovalHandlers
	FileChangeRequestApprovalParams   = sdk.FileChangeRequestApprovalParams
	FileChangeRequestApprovalResponse = sdk.FileChangeRequestApprovalResponse
	ReviewDecisionWrapper             = sdk.ReviewDecisionWrapper
)

func NewClient(transport sdk.Transport, opts ...ClientOption) *Client {
	return sdk.NewClient(transport, opts...)
}

func WithRequestTimeout(timeout time.Duration) ClientOption {
	return sdk.WithRequestTimeout(timeout)
}
