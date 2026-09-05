package protocol

import (
	"encoding/json"
	"fmt"
)

type CommandExecutionApprovalKind string

const (
	CommandExecutionApprovalKindCommand    CommandExecutionApprovalKind = "command"
	CommandExecutionApprovalKindWriteStdin CommandExecutionApprovalKind = "writeStdin"
)

var validCommandExecutionApprovalKindValues = map[CommandExecutionApprovalKind]struct{}{CommandExecutionApprovalKindCommand: {}, CommandExecutionApprovalKindWriteStdin: {}}

func (v CommandExecutionApprovalKind) MarshalJSON() ([]byte, error) {
	return marshalEnumString("CommandExecutionApprovalKind", v, validCommandExecutionApprovalKindValues)
}
func (v *CommandExecutionApprovalKind) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "CommandExecutionApprovalKind", validCommandExecutionApprovalKindValues, v)
}

type McpServerConnectionStatus string

const (
	McpServerConnectionStatusNotStarted             McpServerConnectionStatus = "notStarted"
	McpServerConnectionStatusStarting               McpServerConnectionStatus = "starting"
	McpServerConnectionStatusConnected              McpServerConnectionStatus = "connected"
	McpServerConnectionStatusAuthenticationRequired McpServerConnectionStatus = "authenticationRequired"
	McpServerConnectionStatusFailed                 McpServerConnectionStatus = "failed"
	McpServerConnectionStatusCancelled              McpServerConnectionStatus = "cancelled"
	McpServerConnectionStatusDisabled               McpServerConnectionStatus = "disabled"
)

var validMcpServerConnectionStatusValues = map[McpServerConnectionStatus]struct{}{McpServerConnectionStatusNotStarted: {}, McpServerConnectionStatusStarting: {}, McpServerConnectionStatusConnected: {}, McpServerConnectionStatusAuthenticationRequired: {}, McpServerConnectionStatusFailed: {}, McpServerConnectionStatusCancelled: {}, McpServerConnectionStatusDisabled: {}}

func (v McpServerConnectionStatus) MarshalJSON() ([]byte, error) {
	return marshalEnumString("McpServerConnectionStatus", v, validMcpServerConnectionStatusValues)
}
func (v *McpServerConnectionStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "McpServerConnectionStatus", validMcpServerConnectionStatusValues, v)
}

// AsyncUserInputQuestion follows the upstream AsyncUserInputQuestion schema.
type AsyncUserInputQuestion struct {
	Options *[]string `json:"options,omitempty"`
	Title   string    `json:"title"`
}

func (v *AsyncUserInputQuestion) UnmarshalJSON(data []byte) error {
	type wire AsyncUserInputQuestion
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"title"}, []string{"title"}); err != nil {
		return err
	}
	*v = AsyncUserInputQuestion(decoded)
	return nil
}

// MisalignmentSteer follows the upstream MisalignmentSteer schema.
type MisalignmentSteer struct {
	Message string `json:"message"`
}

func (v *MisalignmentSteer) UnmarshalJSON(data []byte) error {
	type wire MisalignmentSteer
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"message"}, []string{"message"}); err != nil {
		return err
	}
	*v = MisalignmentSteer(decoded)
	return nil
}

// MisalignmentErrorDetails follows the upstream MisalignmentErrorDetails schema.
type MisalignmentErrorDetails struct {
	DetailedExplanation *string            `json:"detailedExplanation,omitempty"`
	ErrorType           *string            `json:"errorType,omitempty"`
	Steer               *MisalignmentSteer `json:"steer,omitempty"`
}

// ResponseUsageMetadata follows the upstream ResponseUsageMetadata schema.
type ResponseUsageMetadata struct {
	Amount   *string         `json:"amount,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type AllowDenyRequirement string

const (
	AllowDenyRequirementAllow AllowDenyRequirement = "allow"
	AllowDenyRequirementDeny  AllowDenyRequirement = "deny"
)

var validAllowDenyRequirementValues = map[AllowDenyRequirement]struct{}{AllowDenyRequirementAllow: {}, AllowDenyRequirementDeny: {}}

func (v AllowDenyRequirement) MarshalJSON() ([]byte, error) {
	return marshalEnumString("AllowDenyRequirement", v, validAllowDenyRequirementValues)
}
func (v *AllowDenyRequirement) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "AllowDenyRequirement", validAllowDenyRequirementValues, v)
}

// BrowserUseOriginPolicyConfig follows the upstream BrowserUseOriginPolicyConfig schema.
type BrowserUseOriginPolicyConfig struct {
	Access        *AllowDenyRequirement `json:"access,omitempty"`
	Downloads     *AllowDenyRequirement `json:"downloads,omitempty"`
	FullCDPAccess *AllowDenyRequirement `json:"full_cdp_access,omitempty"`
	Uploads       *AllowDenyRequirement `json:"uploads,omitempty"`
}

// BrowserUseConfig follows the upstream BrowserUseConfig schema.
type BrowserUseConfig struct {
	AllowHistoryAccess  *bool                                    `json:"allow_history_access,omitempty"`
	DefaultOriginPolicy *BrowserUseOriginPolicyConfig            `json:"default_origin_policy,omitempty"`
	Origins             *map[string]BrowserUseOriginPolicyConfig `json:"origins,omitempty"`
}

// ComputerUseMacosConfig follows the upstream ComputerUseMacosConfig schema.
type ComputerUseMacosConfig struct {
	BundleIDs *map[string]AllowDenyRequirement `json:"bundle_ids,omitempty"`
}

// ComputerUseWindowsExeConfig follows the upstream ComputerUseWindowsExeConfig schema.
type ComputerUseWindowsExeConfig struct {
	Access        AllowDenyRequirement `json:"access"`
	BinaryName    *string              `json:"binary_name,omitempty"`
	ProductName   string               `json:"product_name"`
	PublisherName string               `json:"publisher_name"`
}

func (v *ComputerUseWindowsExeConfig) UnmarshalJSON(data []byte) error {
	type wire ComputerUseWindowsExeConfig
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"access", "product_name", "publisher_name"}, []string{"access", "product_name", "publisher_name"}); err != nil {
		return err
	}
	*v = ComputerUseWindowsExeConfig(decoded)
	return nil
}

// ComputerUseWindowsConfig follows the upstream ComputerUseWindowsConfig schema.
type ComputerUseWindowsConfig struct {
	Aumids *map[string]AllowDenyRequirement `json:"aumids,omitempty"`
	Exes   *[]ComputerUseWindowsExeConfig   `json:"exes,omitempty"`
}

// ComputerUseConfig follows the upstream ComputerUseConfig schema.
type ComputerUseConfig struct {
	DefaultAppAccess *AllowDenyRequirement     `json:"default_app_access,omitempty"`
	Macos            *ComputerUseMacosConfig   `json:"macos,omitempty"`
	Windows          *ComputerUseWindowsConfig `json:"windows,omitempty"`
}

// InAppBrowserRequirements follows the upstream InAppBrowserRequirements schema.
type InAppBrowserRequirements struct {
	AllowExternalBrowserSettingsImport *bool `json:"allowExternalBrowserSettingsImport,omitempty"`
}

// ComputerUseMacosRequirements follows the upstream ComputerUseMacosRequirements schema.
type ComputerUseMacosRequirements struct {
	BundleIDs *map[string]AllowDenyRequirement `json:"bundleIds,omitempty"`
}

// ComputerUseWindowsExeRequirement follows the upstream ComputerUseWindowsExeRequirement schema.
type ComputerUseWindowsExeRequirement struct {
	Access        AllowDenyRequirement `json:"access"`
	BinaryName    *string              `json:"binaryName,omitempty"`
	ProductName   string               `json:"productName"`
	PublisherName string               `json:"publisherName"`
}

func (v *ComputerUseWindowsExeRequirement) UnmarshalJSON(data []byte) error {
	type wire ComputerUseWindowsExeRequirement
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"access", "productName", "publisherName"}, []string{"access", "productName", "publisherName"}); err != nil {
		return err
	}
	*v = ComputerUseWindowsExeRequirement(decoded)
	return nil
}

// ComputerUseWindowsRequirements follows the upstream ComputerUseWindowsRequirements schema.
type ComputerUseWindowsRequirements struct {
	Aumids *map[string]AllowDenyRequirement    `json:"aumids,omitempty"`
	Exes   *[]ComputerUseWindowsExeRequirement `json:"exes,omitempty"`
}

type BrowserUseAccessApprovalLifetime string

const (
	BrowserUseAccessApprovalLifetimeTurn   BrowserUseAccessApprovalLifetime = "turn"
	BrowserUseAccessApprovalLifetimeThread BrowserUseAccessApprovalLifetime = "thread"
)

var validBrowserUseAccessApprovalLifetimeValues = map[BrowserUseAccessApprovalLifetime]struct{}{BrowserUseAccessApprovalLifetimeTurn: {}, BrowserUseAccessApprovalLifetimeThread: {}}

func (v BrowserUseAccessApprovalLifetime) MarshalJSON() ([]byte, error) {
	return marshalEnumString("BrowserUseAccessApprovalLifetime", v, validBrowserUseAccessApprovalLifetimeValues)
}
func (v *BrowserUseAccessApprovalLifetime) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "BrowserUseAccessApprovalLifetime", validBrowserUseAccessApprovalLifetimeValues, v)
}

// BrowserUseOriginPolicy follows the upstream BrowserUseOriginPolicy schema.
type BrowserUseOriginPolicy struct {
	Access                 *AllowDenyRequirement             `json:"access,omitempty"`
	AccessApprovalLifetime *BrowserUseAccessApprovalLifetime `json:"accessApprovalLifetime,omitempty"`
	AutoReview             *AllowDenyRequirement             `json:"autoReview,omitempty"`
	Downloads              *AllowDenyRequirement             `json:"downloads,omitempty"`
	FullCDPAccess          *AllowDenyRequirement             `json:"fullCdpAccess,omitempty"`
	PersistentApproval     *bool                             `json:"persistentApproval,omitempty"`
	Uploads                *AllowDenyRequirement             `json:"uploads,omitempty"`
}

// BrowserUseRequirements follows the upstream BrowserUseRequirements schema.
type BrowserUseRequirements struct {
	AllowGlobalPersistentApproval *bool                              `json:"allowGlobalPersistentApproval,omitempty"`
	AllowHistoryAccess            *bool                              `json:"allowHistoryAccess,omitempty"`
	DefaultOriginPolicy           *BrowserUseOriginPolicy            `json:"defaultOriginPolicy,omitempty"`
	DisableAutoReview             *bool                              `json:"disableAutoReview,omitempty"`
	Origins                       *map[string]BrowserUseOriginPolicy `json:"origins,omitempty"`
}

// TurnToolOutput follows the upstream TurnToolOutput schema.
type TurnToolOutput struct {
	Name      string                 `json:"name"`
	Namespace *string                `json:"namespace,omitempty"`
	Output    FunctionCallOutputBody `json:"output"`
}

func (v *TurnToolOutput) UnmarshalJSON(data []byte) error {
	type wire TurnToolOutput
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"name", "output"}, []string{"name", "output"}); err != nil {
		return err
	}
	*v = TurnToolOutput(decoded)
	return nil
}

type FunctionCallOutputContentItem interface{ isFunctionCallOutputContentItem() }
type FunctionCallOutputContentItemWrapper struct{ Value FunctionCallOutputContentItem }

// InputTextFunctionCallOutputContentItem follows the upstream InputTextFunctionCallOutputContentItem schema.
type InputTextFunctionCallOutputContentItem struct {
	Text string `json:"text"`
}

func (v *InputTextFunctionCallOutputContentItem) UnmarshalJSON(data []byte) error {
	type wire InputTextFunctionCallOutputContentItem
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"text"}, []string{"text"}); err != nil {
		return err
	}
	*v = InputTextFunctionCallOutputContentItem(decoded)
	return nil
}

func (*InputTextFunctionCallOutputContentItem) isFunctionCallOutputContentItem() {}
func (v *InputTextFunctionCallOutputContentItem) MarshalJSON() ([]byte, error) {
	type wire InputTextFunctionCallOutputContentItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"input_text", (*wire)(v)})
}

type ImageDetail string

const (
	ImageDetailAuto     ImageDetail = "auto"
	ImageDetailLow      ImageDetail = "low"
	ImageDetailHigh     ImageDetail = "high"
	ImageDetailOriginal ImageDetail = "original"
)

var validImageDetailValues = map[ImageDetail]struct{}{ImageDetailAuto: {}, ImageDetailLow: {}, ImageDetailHigh: {}, ImageDetailOriginal: {}}

func (v ImageDetail) MarshalJSON() ([]byte, error) {
	return marshalEnumString("ImageDetail", v, validImageDetailValues)
}
func (v *ImageDetail) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "ImageDetail", validImageDetailValues, v)
}

// InputImageFunctionCallOutputContentItem follows the upstream InputImageFunctionCallOutputContentItem schema.
type InputImageFunctionCallOutputContentItem struct {
	Detail   *ImageDetail `json:"detail,omitempty"`
	ImageURL string       `json:"image_url"`
}

func (v *InputImageFunctionCallOutputContentItem) UnmarshalJSON(data []byte) error {
	type wire InputImageFunctionCallOutputContentItem
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"image_url"}, []string{"image_url"}); err != nil {
		return err
	}
	*v = InputImageFunctionCallOutputContentItem(decoded)
	return nil
}

func (*InputImageFunctionCallOutputContentItem) isFunctionCallOutputContentItem() {}
func (v *InputImageFunctionCallOutputContentItem) MarshalJSON() ([]byte, error) {
	type wire InputImageFunctionCallOutputContentItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"input_image", (*wire)(v)})
}

// InputAudioFunctionCallOutputContentItem follows the upstream InputAudioFunctionCallOutputContentItem schema.
type InputAudioFunctionCallOutputContentItem struct {
	AudioURL string `json:"audio_url"`
}

func (v *InputAudioFunctionCallOutputContentItem) UnmarshalJSON(data []byte) error {
	type wire InputAudioFunctionCallOutputContentItem
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"audio_url"}, []string{"audio_url"}); err != nil {
		return err
	}
	*v = InputAudioFunctionCallOutputContentItem(decoded)
	return nil
}

func (*InputAudioFunctionCallOutputContentItem) isFunctionCallOutputContentItem() {}
func (v *InputAudioFunctionCallOutputContentItem) MarshalJSON() ([]byte, error) {
	type wire InputAudioFunctionCallOutputContentItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"input_audio", (*wire)(v)})
}

// EncryptedContentFunctionCallOutputContentItem follows the upstream EncryptedContentFunctionCallOutputContentItem schema.
type EncryptedContentFunctionCallOutputContentItem struct {
	EncryptedContent string `json:"encrypted_content"`
}

func (v *EncryptedContentFunctionCallOutputContentItem) UnmarshalJSON(data []byte) error {
	type wire EncryptedContentFunctionCallOutputContentItem
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"encrypted_content"}, []string{"encrypted_content"}); err != nil {
		return err
	}
	*v = EncryptedContentFunctionCallOutputContentItem(decoded)
	return nil
}

func (*EncryptedContentFunctionCallOutputContentItem) isFunctionCallOutputContentItem() {}
func (v *EncryptedContentFunctionCallOutputContentItem) MarshalJSON() ([]byte, error) {
	type wire EncryptedContentFunctionCallOutputContentItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"encrypted_content", (*wire)(v)})
}

type UnknownFunctionCallOutputContentItem struct{ Raw json.RawMessage }

func (*UnknownFunctionCallOutputContentItem) isFunctionCallOutputContentItem() {}
func (v *UnknownFunctionCallOutputContentItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Raw)
}
func (w *FunctionCallOutputContentItemWrapper) UnmarshalJSON(data []byte) error {
	tag, err := decodeRequiredObjectTypeField(data, "FunctionCallOutputContentItem")
	if err != nil {
		return err
	}
	switch tag {
	case "input_text":
		var v InputTextFunctionCallOutputContentItem
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "input_image":
		var v InputImageFunctionCallOutputContentItem
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "input_audio":
		var v InputAudioFunctionCallOutputContentItem
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "encrypted_content":
		var v EncryptedContentFunctionCallOutputContentItem
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	default:
		w.Value = &UnknownFunctionCallOutputContentItem{Raw: append(json.RawMessage(nil), data...)}
	}
	return nil
}
func (w FunctionCallOutputContentItemWrapper) MarshalJSON() ([]byte, error) {
	if isNilInterfaceValue(w.Value) {
		return nil, fmt.Errorf("missing FunctionCallOutputContentItem")
	}
	return json.Marshal(w.Value)
}

type ThreadRealtimeItem interface{ isThreadRealtimeItem() }
type ThreadRealtimeItemWrapper struct{ Value ThreadRealtimeItem }

// RealtimeSessionStartedThreadRealtimeItem follows the upstream RealtimeSessionStartedThreadRealtimeItem schema.
type RealtimeSessionStartedThreadRealtimeItem struct {
	ID                string `json:"id"`
	RealtimeSessionID string `json:"realtimeSessionId"`
}

func (v *RealtimeSessionStartedThreadRealtimeItem) UnmarshalJSON(data []byte) error {
	type wire RealtimeSessionStartedThreadRealtimeItem
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"id", "realtimeSessionId"}, []string{"id", "realtimeSessionId"}); err != nil {
		return err
	}
	*v = RealtimeSessionStartedThreadRealtimeItem(decoded)
	return nil
}

func (*RealtimeSessionStartedThreadRealtimeItem) isThreadRealtimeItem() {}
func (v *RealtimeSessionStartedThreadRealtimeItem) MarshalJSON() ([]byte, error) {
	type wire RealtimeSessionStartedThreadRealtimeItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"realtimeSessionStarted", (*wire)(v)})
}

type ThreadRealtimeTranscriptRole string

const (
	ThreadRealtimeTranscriptRoleUser      ThreadRealtimeTranscriptRole = "user"
	ThreadRealtimeTranscriptRoleAssistant ThreadRealtimeTranscriptRole = "assistant"
)

var validThreadRealtimeTranscriptRoleValues = map[ThreadRealtimeTranscriptRole]struct{}{ThreadRealtimeTranscriptRoleUser: {}, ThreadRealtimeTranscriptRoleAssistant: {}}

func (v ThreadRealtimeTranscriptRole) MarshalJSON() ([]byte, error) {
	return marshalEnumString("ThreadRealtimeTranscriptRole", v, validThreadRealtimeTranscriptRoleValues)
}
func (v *ThreadRealtimeTranscriptRole) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "ThreadRealtimeTranscriptRole", validThreadRealtimeTranscriptRoleValues, v)
}

// TranscriptSegmentThreadRealtimeItem follows the upstream TranscriptSegmentThreadRealtimeItem schema.
type TranscriptSegmentThreadRealtimeItem struct {
	ID                string                       `json:"id"`
	RealtimeSessionID string                       `json:"realtimeSessionId"`
	Role              ThreadRealtimeTranscriptRole `json:"role"`
	Text              string                       `json:"text"`
}

func (v *TranscriptSegmentThreadRealtimeItem) UnmarshalJSON(data []byte) error {
	type wire TranscriptSegmentThreadRealtimeItem
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"id", "realtimeSessionId", "role", "text"}, []string{"id", "realtimeSessionId", "role", "text"}); err != nil {
		return err
	}
	*v = TranscriptSegmentThreadRealtimeItem(decoded)
	return nil
}

func (*TranscriptSegmentThreadRealtimeItem) isThreadRealtimeItem() {}
func (v *TranscriptSegmentThreadRealtimeItem) MarshalJSON() ([]byte, error) {
	type wire TranscriptSegmentThreadRealtimeItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"transcriptSegment", (*wire)(v)})
}

type ThreadRealtimeBemItemPresentation interface{ isThreadRealtimeBemItemPresentation() }
type ThreadRealtimeBemItemPresentationWrapper struct {
	Value ThreadRealtimeBemItemPresentation
}

// WholeItemThreadRealtimeBemItemPresentation follows the upstream WholeItemThreadRealtimeBemItemPresentation schema.
type WholeItemThreadRealtimeBemItemPresentation struct {
}

func (*WholeItemThreadRealtimeBemItemPresentation) isThreadRealtimeBemItemPresentation() {}
func (v *WholeItemThreadRealtimeBemItemPresentation) MarshalJSON() ([]byte, error) {
	type wire WholeItemThreadRealtimeBemItemPresentation
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"wholeItem", (*wire)(v)})
}

// InlineMarkdownThreadRealtimeBemItemPresentation follows the upstream InlineMarkdownThreadRealtimeBemItemPresentation schema.
type InlineMarkdownThreadRealtimeBemItemPresentation struct {
}

func (*InlineMarkdownThreadRealtimeBemItemPresentation) isThreadRealtimeBemItemPresentation() {}
func (v *InlineMarkdownThreadRealtimeBemItemPresentation) MarshalJSON() ([]byte, error) {
	type wire InlineMarkdownThreadRealtimeBemItemPresentation
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"inlineMarkdown", (*wire)(v)})
}

// InlineVisualizationThreadRealtimeBemItemPresentation follows the upstream InlineVisualizationThreadRealtimeBemItemPresentation schema.
type InlineVisualizationThreadRealtimeBemItemPresentation struct {
	Index uint32 `json:"index"`
}

func (v *InlineVisualizationThreadRealtimeBemItemPresentation) UnmarshalJSON(data []byte) error {
	type wire InlineVisualizationThreadRealtimeBemItemPresentation
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"index"}, []string{"index"}); err != nil {
		return err
	}
	*v = InlineVisualizationThreadRealtimeBemItemPresentation(decoded)
	return nil
}

func (*InlineVisualizationThreadRealtimeBemItemPresentation) isThreadRealtimeBemItemPresentation() {}
func (v *InlineVisualizationThreadRealtimeBemItemPresentation) MarshalJSON() ([]byte, error) {
	type wire InlineVisualizationThreadRealtimeBemItemPresentation
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"inlineVisualization", (*wire)(v)})
}

type UnknownThreadRealtimeBemItemPresentation struct{ Raw json.RawMessage }

func (*UnknownThreadRealtimeBemItemPresentation) isThreadRealtimeBemItemPresentation() {}
func (v *UnknownThreadRealtimeBemItemPresentation) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Raw)
}
func (w *ThreadRealtimeBemItemPresentationWrapper) UnmarshalJSON(data []byte) error {
	tag, err := decodeRequiredObjectTypeField(data, "ThreadRealtimeBemItemPresentation")
	if err != nil {
		return err
	}
	switch tag {
	case "wholeItem":
		var v WholeItemThreadRealtimeBemItemPresentation
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "inlineMarkdown":
		var v InlineMarkdownThreadRealtimeBemItemPresentation
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "inlineVisualization":
		var v InlineVisualizationThreadRealtimeBemItemPresentation
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	default:
		w.Value = &UnknownThreadRealtimeBemItemPresentation{Raw: append(json.RawMessage(nil), data...)}
	}
	return nil
}
func (w ThreadRealtimeBemItemPresentationWrapper) MarshalJSON() ([]byte, error) {
	if isNilInterfaceValue(w.Value) {
		return nil, fmt.Errorf("missing ThreadRealtimeBemItemPresentation")
	}
	return json.Marshal(w.Value)
}

// BemItemPromotedThreadRealtimeItem follows the upstream BemItemPromotedThreadRealtimeItem schema.
type BemItemPromotedThreadRealtimeItem struct {
	ID                string                                   `json:"id"`
	RealtimeSessionID string                                   `json:"realtimeSessionId"`
	ItemID            string                                   `json:"item_id"`
	Presentation      ThreadRealtimeBemItemPresentationWrapper `json:"presentation"`
	TurnID            string                                   `json:"turn_id"`
}

func (v *BemItemPromotedThreadRealtimeItem) UnmarshalJSON(data []byte) error {
	type wire BemItemPromotedThreadRealtimeItem
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"id", "realtimeSessionId", "item_id", "presentation", "turn_id"}, []string{"id", "realtimeSessionId", "item_id", "presentation", "turn_id"}); err != nil {
		return err
	}
	*v = BemItemPromotedThreadRealtimeItem(decoded)
	return nil
}

func (*BemItemPromotedThreadRealtimeItem) isThreadRealtimeItem() {}
func (v *BemItemPromotedThreadRealtimeItem) MarshalJSON() ([]byte, error) {
	type wire BemItemPromotedThreadRealtimeItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"bemItemPromoted", (*wire)(v)})
}

type ThreadRealtimeSessionOutcome string

const (
	ThreadRealtimeSessionOutcomeEnded  ThreadRealtimeSessionOutcome = "ended"
	ThreadRealtimeSessionOutcomeFailed ThreadRealtimeSessionOutcome = "failed"
)

var validThreadRealtimeSessionOutcomeValues = map[ThreadRealtimeSessionOutcome]struct{}{ThreadRealtimeSessionOutcomeEnded: {}, ThreadRealtimeSessionOutcomeFailed: {}}

func (v ThreadRealtimeSessionOutcome) MarshalJSON() ([]byte, error) {
	return marshalEnumString("ThreadRealtimeSessionOutcome", v, validThreadRealtimeSessionOutcomeValues)
}
func (v *ThreadRealtimeSessionOutcome) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "ThreadRealtimeSessionOutcome", validThreadRealtimeSessionOutcomeValues, v)
}

// RealtimeSessionClosedThreadRealtimeItem follows the upstream RealtimeSessionClosedThreadRealtimeItem schema.
type RealtimeSessionClosedThreadRealtimeItem struct {
	ID                string                       `json:"id"`
	RealtimeSessionID string                       `json:"realtimeSessionId"`
	Outcome           ThreadRealtimeSessionOutcome `json:"outcome"`
}

func (v *RealtimeSessionClosedThreadRealtimeItem) UnmarshalJSON(data []byte) error {
	type wire RealtimeSessionClosedThreadRealtimeItem
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"id", "realtimeSessionId", "outcome"}, []string{"id", "realtimeSessionId", "outcome"}); err != nil {
		return err
	}
	*v = RealtimeSessionClosedThreadRealtimeItem(decoded)
	return nil
}

func (*RealtimeSessionClosedThreadRealtimeItem) isThreadRealtimeItem() {}
func (v *RealtimeSessionClosedThreadRealtimeItem) MarshalJSON() ([]byte, error) {
	type wire RealtimeSessionClosedThreadRealtimeItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"realtimeSessionClosed", (*wire)(v)})
}

type UnknownThreadRealtimeItem struct{ Raw json.RawMessage }

func (*UnknownThreadRealtimeItem) isThreadRealtimeItem()          {}
func (v *UnknownThreadRealtimeItem) MarshalJSON() ([]byte, error) { return json.Marshal(v.Raw) }
func (w *ThreadRealtimeItemWrapper) UnmarshalJSON(data []byte) error {
	tag, err := decodeRequiredObjectTypeField(data, "ThreadRealtimeItem")
	if err != nil {
		return err
	}
	switch tag {
	case "realtimeSessionStarted":
		var v RealtimeSessionStartedThreadRealtimeItem
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "transcriptSegment":
		var v TranscriptSegmentThreadRealtimeItem
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "bemItemPromoted":
		var v BemItemPromotedThreadRealtimeItem
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "realtimeSessionClosed":
		var v RealtimeSessionClosedThreadRealtimeItem
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	default:
		w.Value = &UnknownThreadRealtimeItem{Raw: append(json.RawMessage(nil), data...)}
	}
	return nil
}
func (w ThreadRealtimeItemWrapper) MarshalJSON() ([]byte, error) {
	if isNilInterfaceValue(w.Value) {
		return nil, fmt.Errorf("missing ThreadRealtimeItem")
	}
	return json.Marshal(w.Value)
}

// AuthRecoveryNotification follows the upstream AuthRecoveryNotification schema.
type AuthRecoveryNotification struct {
	Message  string `json:"message"`
	Provider string `json:"provider"`
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

func (v *AuthRecoveryNotification) UnmarshalJSON(data []byte) error {
	type wire AuthRecoveryNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"message", "provider", "threadId", "turnId"}, []string{"message", "provider", "threadId", "turnId"}); err != nil {
		return err
	}
	*v = AuthRecoveryNotification(decoded)
	return nil
}

// McpServerEventNotification follows the upstream McpServerEventNotification schema.
type McpServerEventNotification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (v *McpServerEventNotification) UnmarshalJSON(data []byte) error {
	type wire McpServerEventNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"method", "params"}, []string{"method"}); err != nil {
		return err
	}
	*v = McpServerEventNotification(decoded)
	return nil
}

// McpServerEventStreamNotification follows the upstream McpServerEventStreamNotification schema.
type McpServerEventStreamNotification struct {
	Notification   McpServerEventNotification `json:"notification"`
	SubscriptionID string                     `json:"subscriptionId"`
}

func (v *McpServerEventStreamNotification) UnmarshalJSON(data []byte) error {
	type wire McpServerEventStreamNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"notification", "subscriptionId"}, []string{"notification", "subscriptionId"}); err != nil {
		return err
	}
	*v = McpServerEventStreamNotification(decoded)
	return nil
}

// ThreadRealtimeItemStartedNotification follows the upstream ThreadRealtimeItemStartedNotification schema.
type ThreadRealtimeItemStartedNotification struct {
	Item     ThreadRealtimeItemWrapper `json:"item"`
	ThreadID string                    `json:"threadId"`
}

func (v *ThreadRealtimeItemStartedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeItemStartedNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"item", "threadId"}, []string{"item", "threadId"}); err != nil {
		return err
	}
	*v = ThreadRealtimeItemStartedNotification(decoded)
	return nil
}

// ThreadRealtimeItemCompletedNotification follows the upstream ThreadRealtimeItemCompletedNotification schema.
type ThreadRealtimeItemCompletedNotification struct {
	Item     ThreadRealtimeItemWrapper `json:"item"`
	ThreadID string                    `json:"threadId"`
}

func (v *ThreadRealtimeItemCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeItemCompletedNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"item", "threadId"}, []string{"item", "threadId"}); err != nil {
		return err
	}
	*v = ThreadRealtimeItemCompletedNotification(decoded)
	return nil
}

// ThreadRealtimeItemTranscriptDeltaNotification follows the upstream ThreadRealtimeItemTranscriptDeltaNotification schema.
type ThreadRealtimeItemTranscriptDeltaNotification struct {
	Delta    string `json:"delta"`
	ItemID   string `json:"itemId"`
	ThreadID string `json:"threadId"`
}

func (v *ThreadRealtimeItemTranscriptDeltaNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeItemTranscriptDeltaNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"delta", "itemId", "threadId"}, []string{"delta", "itemId", "threadId"}); err != nil {
		return err
	}
	*v = ThreadRealtimeItemTranscriptDeltaNotification(decoded)
	return nil
}
