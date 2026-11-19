package protocol

type AccountUpdateReason string

const (
	AccountUpdateReasonWorkspaceRoutingReady AccountUpdateReason = "workspaceRoutingReady"
	AccountUpdateReasonUnknown               AccountUpdateReason = "unknown"
)

var validAccountUpdateReasons = map[AccountUpdateReason]struct{}{AccountUpdateReasonWorkspaceRoutingReady: {}, AccountUpdateReasonUnknown: {}}

func (v AccountUpdateReason) MarshalJSON() ([]byte, error) {
	return marshalEnumString("account.reason", v, validAccountUpdateReasons)
}
