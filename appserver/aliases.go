// Package appserver starts and manages a Codex app-server process.
package appserver

import (
	"time"

	protocol "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const sdkVersion = "0.3.0"

type (
	Client                 = protocol.Client
	ClientInfo             = protocol.ClientInfo
	ClientOption           = protocol.ClientOption
	InitializeCapabilities = protocol.InitializeCapabilities
	InitializeParams       = protocol.InitializeParams
	InitializeResponse     = protocol.InitializeResponse
	Notification           = protocol.Notification
	Transport              = protocol.Transport
)

func NewClient(transport Transport, opts ...ClientOption) *Client {
	return protocol.NewClient(transport, opts...)
}

func WithRequestTimeout(timeout time.Duration) ClientOption {
	return protocol.WithRequestTimeout(timeout)
}

func WithHandlerErrorCallback(cb func(method string, err error)) ClientOption {
	return protocol.WithHandlerErrorCallback(cb)
}
