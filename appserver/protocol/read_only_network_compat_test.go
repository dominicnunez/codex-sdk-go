package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"github.com/dominicnunez/codex-sdk-go/internal/deepcopy"
)

func TestReadOnlyNetworkPlainRecordCompatibility(t *testing.T) {
	// Independent method-free reference for the native Go record contract.
	// Schema admission belongs to SandboxPolicyWrapper, not this public record.
	type SandboxPolicyReadOnly struct {
		Access        *p.ReadOnlyAccessWrapper `json:"access,omitempty"`
		NetworkAccess *bool                    `json:"networkAccess,omitempty"`
	}
	actual := p.SandboxPolicyReadOnly{NetworkAccess: p.Ptr(true)}
	reference := SandboxPolicyReadOnly{NetworkAccess: p.Ptr(true)}
	for _, raw := range []string{`{}`, `null`, `{"NETWORKACCESS":false}`, `{"networkAccess":null}`, `{"networkAccess":true,"NETWORKACCESS":null}`, `{"networkAccess":42,"access":{"type":"fullAccess"}}`, `{"networkAccess":false,"networkAccess":"bad"}`, `{"access":42,"networkAccess":true}`} {
		got, want := json.Unmarshal([]byte(raw), &actual), json.Unmarshal([]byte(raw), &reference)
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(SandboxPolicyReadOnly(actual), reference) {
			t.Fatalf("native record contract changed for %s: actual=%+v/%v reference=%+v/%v", raw, actual, got, reference, want)
		}
	}
	if _, ok := any(&actual).(json.Unmarshaler); ok {
		t.Fatal("plain record acquired UnmarshalJSON")
	}
	if _, ok := any(actual).(json.Marshaler); ok {
		t.Fatal("plain record acquired MarshalJSON")
	}
	var anonymous struct {
		p.SandboxPolicyReadOnly
		After string `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"networkAccess":true,"after":"visited"}`), &anonymous); err != nil || anonymous.After != "visited" || anonymous.NetworkAccess == nil || !*anonymous.NetworkAccess {
		t.Fatal("anonymous record consumed enclosing application fields")
	}
	var named struct {
		Policy p.SandboxPolicyReadOnly `json:"policy"`
		After  string                  `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"policy":{"networkAccess":null},"after":"visited"}`), &named); err != nil || named.After != "visited" {
		t.Fatal("named record acquired strict carrier behavior")
	}
}

func TestReadOnlyNetworkWrapperReceiverAndComposition(t *testing.T) {
	flag := true
	roots := []string{"/retained"}
	access := &p.ReadOnlyAccessWrapper{Value: p.ReadOnlyAccessRestricted{ReadableRoots: roots}}
	prior := p.SandboxPolicyReadOnly{Access: access, NetworkAccess: &flag}
	for _, raw := range []string{`{"type":"readOnly","networkAccess":null}`, `{"type":"readOnly","NETWORKACCESS":null,"networkAccess":true}`, `{"type":"readOnly","networkAccess":false,"networ\u212aAccess":null}`} {
		wrapper := p.SandboxPolicyWrapper{Value: prior}
		if err := json.Unmarshal([]byte(raw), &wrapper); !errors.Is(err, p.ErrNullResultField) {
			t.Fatalf("new null restriction missing: %v", err)
		}
		retained, ok := wrapper.Value.(p.SandboxPolicyReadOnly)
		if !ok || retained.NetworkAccess != &flag || retained.Access != access || &retained.Access.Value.(p.ReadOnlyAccessRestricted).ReadableRoots[0] != &roots[0] || !flag {
			t.Fatal("new rejection replaced or mutated prior receiver references")
		}
	}
	var outer struct {
		Before int                    `json:"before"`
		Policy p.SandboxPolicyWrapper `json:"policy"`
		After  string                 `json:"after"`
	}
	err := json.Unmarshal([]byte(`{"before":1,"policy":{"type":"readOnly","networkAccess":null},"after":"visited"}`), &outer)
	if !errors.Is(err, p.ErrNullResultField) || outer.Before != 1 || outer.After != "visited" {
		t.Fatal("carrier failure changed enclosing traversal")
	}
	err = json.Unmarshal([]byte(`{"before":"bad","policy":{"type":"readOnly","networkAccess":null},"after":"again"}`), &outer)
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) || mismatch.Field != "before" || outer.After != "again" {
		t.Fatal("new restriction displaced earlier enclosing error")
	}
}

func TestReadOnlyNetworkSnapshotOwnership(t *testing.T) {
	flag := true
	policy := &p.SandboxPolicyReadOnly{NetworkAccess: &flag, Access: &p.ReadOnlyAccessWrapper{Value: p.ReadOnlyAccessRestricted{ReadableRoots: []string{"/original"}}}}
	cloned := deepcopy.Value(p.SandboxPolicyWrapper{Value: policy})
	snapshot := cloned.Value.(*p.SandboxPolicyReadOnly)
	flag = false
	policy.Access.Value.(p.ReadOnlyAccessRestricted).ReadableRoots[0] = "/changed"
	if snapshot.NetworkAccess == &flag || !*snapshot.NetworkAccess || snapshot.Access.Value.(p.ReadOnlyAccessRestricted).ReadableRoots[0] != "/original" {
		t.Fatal("snapshot shares policy flag or legacy references")
	}
	flag = true
	*snapshot.NetworkAccess = false
	if !flag {
		t.Fatal("snapshot mutation leaked back into source policy")
	}
}
