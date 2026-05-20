// Package appserver starts and manages a Codex app-server process.
package appserver

import (
	"time"

	sdk "github.com/dominicnunez/codex-sdk-go/sdk"
)

const sdkVersion = "0.3.0"

type (
	Client                 = sdk.Client
	ClientInfo             = sdk.ClientInfo
	ClientOption           = sdk.ClientOption
	InitializeCapabilities = sdk.InitializeCapabilities
	InitializeParams       = sdk.InitializeParams
	InitializeResponse     = sdk.InitializeResponse
	Notification           = sdk.Notification
	Transport              = sdk.Transport
)

func NewClient(transport Transport, opts ...ClientOption) *Client {
	return sdk.NewClient(transport, opts...)
}

func WithRequestTimeout(timeout time.Duration) ClientOption {
	return sdk.WithRequestTimeout(timeout)
}

func WithHandlerErrorCallback(cb func(method string, err error)) ClientOption {
	return sdk.WithHandlerErrorCallback(cb)
}
