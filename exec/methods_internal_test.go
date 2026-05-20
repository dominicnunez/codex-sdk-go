package exec

import "github.com/dominicnunez/codex-sdk-go/appserver/protocol"

const (
	methodInitialize = protocol.MethodInitialize

	notifyAgentMessageDelta      = protocol.NotifyAgentMessageDelta
	notifyCommandExecOutputDelta = protocol.NotifyCommandExecOutputDelta
	notifyConfigWarning          = protocol.NotifyConfigWarning
	notifyItemCompleted          = protocol.NotifyItemCompleted
	notifyModelRerouted          = protocol.NotifyModelRerouted
	notifyTurnCompleted          = protocol.NotifyTurnCompleted
)
