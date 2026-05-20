package transport

import "github.com/dominicnunez/codex-sdk-go/appserver/protocol"

const (
	notifyAgentMessageDelta      = protocol.NotifyAgentMessageDelta
	notifyCommandExecOutputDelta = protocol.NotifyCommandExecOutputDelta
	notifyConfigWarning          = protocol.NotifyConfigWarning
	notifyItemCompleted          = protocol.NotifyItemCompleted
	notifyModelRerouted          = protocol.NotifyModelRerouted
	notifyTurnCompleted          = protocol.NotifyTurnCompleted
)
