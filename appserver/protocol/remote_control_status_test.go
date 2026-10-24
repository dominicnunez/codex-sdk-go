package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRemoteControlStatusRejectsUnknownValue(t *testing.T) {
	var value p.RemoteControlStatusChangedNotification
	if err := json.Unmarshal([]byte(`{"installationId":"installation","serverName":"server","status":"sleeping"}`), &value); err == nil {
		t.Fatal("unknown remote-control status admitted")
	}
}

func remoteStatusJSON(status string) []byte {
	return []byte(`{"installationId":"installation","serverName":"server","environmentId":null,"status":` + status + `}`)
}

func TestRemoteControlStatusDeliveryAndRecovery(t *testing.T) {
	for _, status := range []string{"disabled", "connecting", "connected", "errored"} {
		t.Run(status, func(t *testing.T) {
			mock := NewMockTransport()
			var reported []error
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, err error) {
				if method != "remoteControl/status/changed" {
					t.Errorf("diagnostic method = %s", method)
				}
				reported = append(reported, err)
			}))
			var received []p.RemoteControlStatusChangedNotification
			handler := func(n p.RemoteControlStatusChangedNotification) { received = append(received, n) }
			client.OnRemoteControlStatusChanged(handler)
			remove := client.AddRemoteControlStatusChangedListener(handler)
			defer remove()
			inject := func(wire []byte) {
				mock.InjectServerNotification(context.Background(), p.Notification{Method: "remoteControl/status/changed", Params: wire})
			}
			inject(remoteStatusJSON(`"sleeping"`))
			if len(received) != 0 || len(reported) != 2 {
				t.Fatalf("invalid delivery: calls=%d reports=%d", len(received), len(reported))
			}
			inject(remoteStatusJSON(`"` + status + `"`))
			if len(received) != 2 || len(reported) != 2 {
				t.Fatalf("recovery delivery: calls=%d reports=%d", len(received), len(reported))
			}
			for _, n := range received {
				if string(n.Status) != status || n.InstallationID != "installation" || n.ServerName != "server" || n.EnvironmentID != nil {
					t.Fatalf("metadata lost: %#v", n)
				}
				wire, err := json.Marshal(n)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(wire, &fields); err != nil || string(fields["status"]) != `"`+status+`"` {
					t.Fatalf("status roundtrip=%s, %v", wire, err)
				}
			}
		})
	}
}

func TestRemoteControlStatusFailureAtomicity(t *testing.T) {
	for _, status := range []string{`"sleeping"`, `""`, `null`, `17`, `false`, `{}`, `[]`, `"sleeping","status":"connected"`, `"sleeping","STATUS":"connected"`, `"sleeping","st\u0061tus":"connected"`, `null,"status":"connected"`} {
		t.Run(status, func(t *testing.T) {
			value := p.RemoteControlStatusChangedNotification{Status: p.RemoteControlConnectionStatusConnected, InstallationID: "old", ServerName: "old", EnvironmentID: p.Ptr("old-env")}
			before := value
			if err := json.Unmarshal(remoteStatusJSON(status), &value); err == nil {
				t.Fatal("invalid state admitted")
			}
			if !reflect.DeepEqual(value, before) || value.EnvironmentID != before.EnvironmentID {
				t.Fatal("failed decode changed receiver or references")
			}
		})
	}
	for _, wire := range []string{`{"installationId":"","serverName":""}`, `{"installationId":"","serverName":"","status":null}`} {
		var value p.RemoteControlStatusChangedNotification
		err := json.Unmarshal([]byte(wire), &value)
		if err == nil || (!strings.Contains(err.Error(), "missing required field") && !strings.Contains(err.Error(), "must not be null")) {
			t.Fatalf("required status error=%v", err)
		}
	}
}

func TestRemoteControlStatusOptionalMetadata(t *testing.T) {
	value := p.RemoteControlStatusChangedNotification{}
	for _, tc := range []struct {
		member string
		want   *string
	}{{`,"environmentId":""`, p.Ptr("")}, {`,"environmentId":"environment"`, p.Ptr("environment")}, {`,"environmentId":null`, nil}, {"", nil}} {
		wire := []byte(`{"installationId":"","serverName":"","status":"disabled"` + tc.member + `}`)
		if err := json.Unmarshal(wire, &value); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(value.EnvironmentID, tc.want) {
			t.Fatalf("optional metadata = %#v; want %#v", value.EnvironmentID, tc.want)
		}
	}
}

func TestRemoteControlStatusKeepsPriorErrorsAndScalarEnvelope(t *testing.T) {
	var value p.RemoteControlStatusChangedNotification
	err := json.Unmarshal([]byte(`{"status":"sleeping","installationId":null,"serverName":"s"}`), &value)
	if err == nil || !strings.Contains(err.Error(), `required field "installationId" must not be null`) {
		t.Fatalf("prior required-field error masked: %v", err)
	}
	var envelope struct {
		p.RemoteControlConnectionStatus `json:"status"`
		Note                            string `json:"note"`
	}
	if err := json.Unmarshal([]byte(`{"status":"connected","note":"kept"}`), &envelope); err != nil || envelope.Note != "kept" || envelope.RemoteControlConnectionStatus != p.RemoteControlConnectionStatusConnected {
		t.Fatalf("scalar envelope changed: %#v, %v", envelope, err)
	}
}
