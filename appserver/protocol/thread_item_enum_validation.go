package protocol

var validAgentMessageDeliveries = map[AgentMessageDelivery]struct{}{
	AgentMessageDeliveryAsync: {},
}

var validCommandExecutionSources = map[CommandExecutionSource]struct{}{
	CommandExecutionSourceAgent:                  {},
	CommandExecutionSourceUserShell:              {},
	CommandExecutionSourceUnifiedExecStartup:     {},
	CommandExecutionSourceUnifiedExecInteraction: {},
}

var validSubAgentActivityKinds = map[SubAgentActivityKind]struct{}{
	SubAgentActivityKindStarted:     {},
	SubAgentActivityKindInteracted:  {},
	SubAgentActivityKindInterrupted: {},
	SubAgentActivityKindCompleted:   {},
}

// Validate every occurrence after the existing branch decoder succeeds. Public
// scalar and record types retain ordinary decoding outside the item union.
func validateItemEnumField[T ~string](data []byte, field string, allowed map[T]struct{}, nullable bool) error {
	return validateOptionalFields(data, func(_ string, raw []byte) error {
		if isNullJSONValue(raw) {
			if nullable {
				return nil
			}
			return ErrNullResultField
		}
		var value T
		return unmarshalEnumString(raw, "thread item."+field, allowed, &value)
	}, field)
}
