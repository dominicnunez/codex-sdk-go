package protocol

// BedrockProviderID identifies a built-in Bedrock provider.
type BedrockProviderID string

const (
	BedrockProviderIDAmazonBedrock        BedrockProviderID = "amazon-bedrock"
	BedrockProviderIDAmazonBedrockRuntime BedrockProviderID = "amazon-bedrock-runtime"
)

var validBedrockProviderIDs = map[BedrockProviderID]struct{}{BedrockProviderIDAmazonBedrock: {}, BedrockProviderIDAmazonBedrockRuntime: {}}

func (v BedrockProviderID) MarshalJSON() ([]byte, error) {
	return marshalEnumString("BedrockProviderID", v, validBedrockProviderIDs)
}
func (v *BedrockProviderID) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "BedrockProviderID", validBedrockProviderIDs, v)
}

// GcpProviderID identifies the built-in GCP provider.
type GcpProviderID string

const GcpProviderIDGcp GcpProviderID = "gcp"

var validGcpProviderIDs = map[GcpProviderID]struct{}{GcpProviderIDGcp: {}}

func (v GcpProviderID) MarshalJSON() ([]byte, error) {
	return marshalEnumString("GcpProviderID", v, validGcpProviderIDs)
}
func (v *GcpProviderID) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "GcpProviderID", validGcpProviderIDs, v)
}
