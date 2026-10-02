package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func selectionMetadata(prefix string, count int, escaped bool) []byte {
	var data strings.Builder
	data.WriteString(prefix)
	for i := range count {
		if escaped {
			fmt.Fprintf(&data, `,"\u006detadata%d":0`, i)
		} else {
			fmt.Fprintf(&data, `,"metadata%d":0`, i)
		}
	}
	data.WriteByte('}')
	return []byte(data.String())
}

func TestObjectSelectionAllocations(t *testing.T) {
	for _, escaped := range []bool{false, true} {
		unknown := selectionMetadata(`{"type":"future"`, 50000, escaped)
		noDiscriminator := selectionMetadata(`{"ignored":true`, 50000, escaped)
		for name, check := range map[string]func() error{
			"response shape":        func() error { return validateObjectResponseResult(unknown) },
			"account type":          func() error { var value AccountWrapper; return value.UnmarshalJSON(unknown) },
			"login type":            func() error { _, err := UnmarshalLoginAccountResponse(unknown); return err },
			"config type":           func() error { var value ConfigLayerSourceWrapper; return value.UnmarshalJSON(unknown) },
			"session discriminator": func() error { var value SessionSourceWrapper; return value.UnmarshalJSON(unknown) },
			"sub-agent discriminator": func() error {
				_, err := unmarshalSubAgentSource(noDiscriminator)
				if err == nil {
					t.Fatal("missing discriminator accepted")
				}
				return nil
			},
			"approval discriminator": func() error {
				var value AskForApprovalWrapper
				if err := value.UnmarshalJSON(noDiscriminator); err == nil {
					t.Fatal("missing discriminator accepted")
				}
				return nil
			},
			"command decision discriminator": func() error { var value CommandExecutionApprovalDecisionWrapper; return value.UnmarshalJSON(unknown) },
			"review decision discriminator":  func() error { var value ReviewDecisionWrapper; return value.UnmarshalJSON(unknown) },
			"thread item type":               func() error { var value ThreadItemWrapper; return value.UnmarshalJSON(unknown) },
			"patch type":                     func() error { var value PatchChangeKindWrapper; return value.UnmarshalJSON(unknown) },
			"web search type":                func() error { var value WebSearchActionWrapper; return value.UnmarshalJSON(unknown) },
			"review target type":             func() error { var value ReviewTargetWrapper; return value.UnmarshalJSON(unknown) },
		} {
			t.Run(fmt.Sprintf("%s/escaped=%v", name, escaped), func(t *testing.T) {
				allocations := testing.AllocsPerRun(3, func() {
					if err := check(); err != nil {
						t.Fatal(err)
					}
				})
				if allocations > 100 {
					t.Fatalf("fixed-field selection allocated %.0f times for ignored properties", allocations)
				}
			})
		}
	}
}

func TestTypedOwnerDuplicateAllocations(t *testing.T) {
	data := []byte(`{` + strings.Repeat(`"threadId":"prior",`, 50000) + `"threadId":"final","threadName":null}`)
	allocations := testing.AllocsPerRun(3, func() {
		var value ThreadNameUpdatedNotification
		if err := json.Unmarshal(data, &value); err != nil || value.ThreadID != "final" {
			t.Fatal(value, err)
		}
	})
	if allocations > 100 {
		t.Fatalf("typed owner allocated %.0f times for overwritten identities", allocations)
	}
}
