package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPermissionProfileAllowedRequired(t *testing.T) {
	for _, item := range []string{`{"id":"profile"}`, `{"id":"profile","allowed":null}`} {
		t.Run(item, func(t *testing.T) {
			var summary p.PermissionProfileSummary
			if err := json.Unmarshal([]byte(item), &summary); err == nil {
				t.Error("profile without required boolean admitted")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			mock.SetResponse("permissionProfile/list", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[` + item + `]}`)})
			response, err := client.PermissionProfile.List(context.Background(), p.PermissionProfileListParams{})
			if err == nil || !reflect.DeepEqual(response, p.PermissionProfileListResponse{}) {
				t.Errorf("malformed profile published: error=%v response=%+v", err, response)
			}
		})
	}
}

func TestPermissionProfileAllowedValues(t *testing.T) {
	for _, tc := range []struct {
		raw         string
		allowed     bool
		description *string
	}{
		{`{"id":"","allowed":false}`, false, nil},
		{`{"id":"profile","allowed":true,"description":null}`, true, nil},
		{`{"id":"profile","allowed":false,"description":""}`, false, p.Ptr("")},
		{`{"id":"profile","allowed":true,"description":"profile description"}`, true, p.Ptr("profile description")},
		{`{"id":"profile","\u0061llowed":true}`, true, nil},
		{`{"id":"profile","allowed":false,"allowed":true}`, true, nil},
		{`{"id":"profile","allowed":true,"allowed":false}`, false, nil},
		{`{"id":"profile","allowed":true,"ALLOWED":false}`, false, nil},
		{`{"id":"profile","allowed":true,"ALLOWED":null}`, true, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			var summary p.PermissionProfileSummary
			if err := json.Unmarshal([]byte(tc.raw), &summary); err != nil || summary.Allowed != tc.allowed || !reflect.DeepEqual(summary.Description, tc.description) {
				t.Fatalf("valid flag changed: error=%v summary=%+v", err, summary)
			}
			encoded, err := json.Marshal(summary)
			if err != nil {
				t.Fatal(err)
			}
			var members map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &members); err != nil {
				t.Fatal(err)
			}
			if string(members["allowed"]) != map[bool]string{true: "true", false: "false"}[tc.allowed] {
				t.Fatal("explicit boolean omitted on encoding")
			}
			if err := json.Unmarshal(encoded, &summary); err != nil || summary.Allowed != tc.allowed {
				t.Fatalf("round trip changed flag: %v", err)
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			mock.SetResponse("permissionProfile/list", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[` + tc.raw + `],"nextCursor":"next"}`)})
			response, err := client.PermissionProfile.List(context.Background(), p.PermissionProfileListParams{})
			if err != nil || len(response.Data) != 1 || response.Data[0].Allowed != tc.allowed || !reflect.DeepEqual(response.Data[0].Description, tc.description) || response.NextCursor == nil || *response.NextCursor != "next" {
				t.Fatalf("valid listing failed: %v response=%+v", err, response)
			}
		})
	}
}

func TestPermissionProfileAllowedFailures(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		sentinel error
		native   bool
	}{
		{`{"id":"profile"}`, p.ErrMissingResultField, false},
		{`{"id":"profile","ALLOWED":true}`, p.ErrMissingResultField, false},
		{`{"id":"profile","allowed":null}`, p.ErrNullResultField, false},
		{`{"id":"profile","allowed":null,"allowed":true}`, p.ErrNullResultField, false},
		{`{"id":"profile","allowed":false,"allowed":null}`, p.ErrNullResultField, false},
		{`{"id":"profile","\u0061llowed":null,"allowed":true}`, p.ErrNullResultField, false},
		{`{"id":"profile","allowed":1}`, nil, true},
		{`{"id":"profile","allowed":[]}`, nil, true},
		{`{"id":"profile","allowed":{}}`, nil, true},
		{`{"id":"profile","allowed":"false"}`, nil, true},
		{`{"id":"profile","allowed":1,"allowed":false}`, nil, true},
		{`{"id":17,"allowed":null}`, nil, true},
		{`{"id":"profile","description":17}`, nil, true},
		{`{"allowed":17}`, p.ErrMissingResultField, false},
		{`null`, p.ErrResultNotObject, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			checkError := func(err error) {
				t.Helper()
				if err == nil {
					t.Fatal("malformed profile accepted")
				}
				if tc.sentinel != nil && !errors.Is(err, tc.sentinel) {
					t.Fatalf("old/missing-field error precedence changed: %v", err)
				}
				if tc.native {
					var mismatch *json.UnmarshalTypeError
					if !errors.As(err, &mismatch) {
						t.Fatalf("native error precedence changed: %v", err)
					}
				}
			}
			seed := p.PermissionProfileSummary{ID: "old", Allowed: true, Description: p.Ptr("old-description")}
			before := seed
			err := json.Unmarshal([]byte(tc.raw), &seed)
			checkError(err)
			if !reflect.DeepEqual(seed, before) || seed.Description != before.Description {
				t.Fatal("failure replaced summary receiver/reference")
			}
			for _, validFirst := range []bool{false, true} {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				items := tc.raw
				if validFirst {
					items = `{"id":"valid-first","allowed":true},` + items
				}
				raw := `{"data":[` + items + `],"nextCursor":"unpublished"}`
				mock.SetResponse("permissionProfile/list", p.Response{JSONRPC: "2.0", Result: json.RawMessage(raw)})
				response, err := client.PermissionProfile.List(context.Background(), p.PermissionProfileListParams{})
				checkError(err)
				if !reflect.DeepEqual(response, p.PermissionProfileListResponse{}) {
					t.Fatal("failed list published partial entries or cursor")
				}
				prior := p.PermissionProfileListResponse{Data: []p.PermissionProfileSummary{before}, NextCursor: p.Ptr("old-cursor")}
				saved := prior
				err = json.Unmarshal([]byte(raw), &prior)
				checkError(err)
				if !reflect.DeepEqual(prior, saved) || &prior.Data[0] != &saved.Data[0] || prior.NextCursor != saved.NextCursor || prior.Data[0].Description != before.Description {
					t.Fatal("failed direct list replaced receiver/references")
				}
				mock.SetResponse("permissionProfile/list", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[{"id":"recovered","allowed":false}]}`)})
				response, err = client.PermissionProfile.List(context.Background(), p.PermissionProfileListParams{})
				if err != nil || len(response.Data) != 1 || response.Data[0].ID != "recovered" || response.Data[0].Allowed {
					t.Fatalf("recovery failed: %v response=%+v", err, response)
				}
			}
		})
	}
}

func TestPermissionProfileAllowedEnvelope(t *testing.T) {
	description := "old"
	var envelope struct {
		Before  int                        `json:"before"`
		Profile p.PermissionProfileSummary `json:"profile"`
		After   string                     `json:"after"`
	}
	envelope.Profile = p.PermissionProfileSummary{ID: "old", Allowed: true, Description: &description}
	before := envelope.Profile
	err := json.Unmarshal([]byte(`{"before":2,"profile":{"id":"new"},"after":"visited"}`), &envelope)
	if !errors.Is(err, p.ErrMissingResultField) || envelope.Before != 2 || envelope.After != "visited" || !reflect.DeepEqual(envelope.Profile, before) || envelope.Profile.Description != before.Description {
		t.Fatalf("enclosing traversal or receiver changed: %v envelope=%+v", err, envelope)
	}
	// Existing earlier native failures keep ownership even if the profile's new
	// required-field check also fails later in the envelope.
	var mismatch *json.UnmarshalTypeError
	err = json.Unmarshal([]byte(`{"before":"bad","profile":{"id":"new"},"after":"visited"}`), &envelope)
	if !errors.As(err, &mismatch) || mismatch.Field != "before" {
		t.Fatalf("earlier enclosing error lost precedence: %v", err)
	}
}
