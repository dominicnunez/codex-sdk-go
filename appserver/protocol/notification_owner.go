package protocol

// These synced notifications use exact schema properties, matching transport
// ownership and the other typed notification decoders.
func decodeOwnedNotification[T any](data []byte, required []string) (T, error) {
	var decoded T
	err := unmarshalInboundObject(data, &decoded, required, required)
	return decoded, err
}

func (n *ExternalAgentConfigImportProgressNotification) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportProgressNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"importId", "itemTypeResults"})
	if err != nil {
		return err
	}
	*n = ExternalAgentConfigImportProgressNotification(decoded)
	return nil
}

func (n *ExternalAgentConfigImportCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportCompletedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"importId", "itemTypeResults"})
	if err != nil {
		return err
	}
	*n = ExternalAgentConfigImportCompletedNotification(decoded)
	return nil
}

func (n *ProjectChangedNotification) UnmarshalJSON(data []byte) error {
	type wire ProjectChangedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"changeType", "projectId"})
	if err != nil {
		return err
	}
	*n = ProjectChangedNotification(decoded)
	return nil
}

func (n *ThreadDeletedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadDeletedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"threadId"})
	if err != nil {
		return err
	}
	*n = ThreadDeletedNotification(decoded)
	return nil
}

func (n *ThreadRevertedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRevertedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"threadId"})
	if err != nil {
		return err
	}
	*n = ThreadRevertedNotification(decoded)
	return nil
}

func (n *ThreadQueueChangedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadQueueChangedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"threadId"})
	if err != nil {
		return err
	}
	*n = ThreadQueueChangedNotification(decoded)
	return nil
}

func (n *EnvironmentConnectionNotification) UnmarshalJSON(data []byte) error {
	type wire EnvironmentConnectionNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"environmentId", "threadId"})
	if err != nil {
		return err
	}
	*n = EnvironmentConnectionNotification(decoded)
	return nil
}

func (n *StrictReviewRequiredNotification) UnmarshalJSON(data []byte) error {
	type wire StrictReviewRequiredNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"startedAtMs", "threadId", "turnId"})
	if err != nil {
		return err
	}
	*n = StrictReviewRequiredNotification(decoded)
	return nil
}

func (n *TurnModerationMetadataNotification) UnmarshalJSON(data []byte) error {
	type wire TurnModerationMetadataNotification
	var decoded wire
	// metadata is open JSON and may be null; only its presence is required.
	if err := unmarshalInboundObject(data, &decoded, []string{"metadata", "threadId", "turnId"}, []string{"threadId", "turnId"}); err != nil {
		return err
	}
	*n = TurnModerationMetadataNotification(decoded)
	return nil
}

func (n *ModelSafetyBufferingUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire ModelSafetyBufferingUpdatedNotification
	var decoded struct {
		wire
		Reasons  nonNullStringList `json:"reasons"`
		UseCases nonNullStringList `json:"useCases"`
	}
	required := []string{"model", "reasons", "showBufferingUi", "threadId", "turnId", "useCases"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	decoded.wire.Reasons = []string(decoded.Reasons)
	decoded.wire.UseCases = []string(decoded.UseCases)
	*n = ModelSafetyBufferingUpdatedNotification(decoded.wire)
	return nil
}
