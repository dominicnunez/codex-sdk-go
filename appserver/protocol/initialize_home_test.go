package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestInitializeHomePathAdmission(t *testing.T) {
	for _, tc := range []struct {
		path  string
		valid bool
	}{
		{"/", true}, {"/tmp/audit-home", true}, {`C:\audit\home`, true}, {`\\server\share\home`, true},
		{"", false}, {"relative", false}, {"/tmp/../home", false}, {"/tmp/./home", false}, {"/tmp//home", false}, {"\x00", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			response := map[string]any{"codexHome": tc.path, "platformFamily": "test-family", "platformOs": "test-target", "userAgent": "audit"}
			if err := mock.SetResponseData("initialize", response); err != nil {
				t.Fatal(err)
			}
			got, err := c.Initialize(context.Background(), p.InitializeParams{ClientInfo: p.ClientInfo{Name: "audit", Version: "1"}})
			if tc.valid {
				expected := p.InitializeResponse{CodexHome: tc.path, PlatformFamily: "test-family", PlatformOS: "test-target", UserAgent: "audit"}
				if err != nil || !reflect.DeepEqual(got, expected) {
					t.Fatalf("valid path or metadata changed or rejected: got=%+v err=%v", got, err)
				}
				return
			}
			_, latched := c.InitializedParams()
			if err == nil || latched || !reflect.DeepEqual(got, p.InitializeResponse{}) {
				t.Fatalf("invalid server home admitted: path=%q cached=%v", tc.path, latched)
			}
			if !strings.Contains(err.Error(), "codexHome") {
				t.Fatalf("path field context lost: %v", err)
			}
			retry := p.InitializeParams{ClientInfo: p.ClientInfo{Name: "retry-client", Version: "2"}}
			expected := p.InitializeResponse{CodexHome: "/tmp/retry-home", PlatformFamily: "retry-family", PlatformOS: "retry-target", UserAgent: "retry-agent"}
			if err = mock.SetResponseData("initialize", expected); err != nil {
				t.Fatal(err)
			}
			recovered, err := c.Initialize(context.Background(), retry)
			if err != nil || !reflect.DeepEqual(recovered, expected) {
				t.Fatalf("retry failed or changed metadata: %+v %v", recovered, err)
			}
			params, ok := c.InitializedParams()
			if !ok || !reflect.DeepEqual(params, retry) || len(mock.SentRequests) != 2 {
				t.Fatal("failed attempt poisoned cached identity or prevented second request")
			}
			mock.SetResponse("initialize", p.Response{Result: json.RawMessage(`{}`)})
			cached, err := c.Initialize(context.Background(), retry)
			if err != nil || !reflect.DeepEqual(cached, expected) || len(mock.SentRequests) != 2 {
				t.Fatal("successful retry cache changed")
			}
		})
	}
}

func TestInitializeHomeSelectionAndPriorErrors(t *testing.T) {
	const metadata = `,"platformFamily":"fixture-family","platformOs":"fixture-target","userAgent":"fixture-agent"`
	for _, tc := range []struct{ name, body, selected, wantErr string }{
		{"native alias", `{"CODEXHOME":"/fixture/home"` + metadata + `}`, "/fixture/home", ""},
		{"escaped", `{"codex\u0048ome":"/fixture/home"` + metadata + `}`, "/fixture/home", ""},
		{"selected valid", `{"codexHome":"relative","codexHome":"/fixture/home"` + metadata + `}`, "/fixture/home", ""},
		{"selected invalid", `{"codexHome":"/fixture/home","CODEXHOME":"relative"` + metadata + `}`, "", "codexHome:"},
		{"required first", `{"codexHome":"relative","platformFamily":"fixture-family","platformOs":"fixture-target"}`, "", "missing userAgent"},
		{"earlier null first", `{"codexHome":null,"codexHome":"relative"` + metadata + `}`, "", "null"},
		{"sibling null first", `{"codexHome":"relative","userAgent":null` + metadata + `}`, "", "null"},
		{"native kind first", `{"codexHome":"relative","userAgent":42,"platformFamily":"fixture-family","platformOs":"fixture-target"}`, "", "unmarshal response result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			mock.SetResponse("initialize", p.Response{Result: json.RawMessage(tc.body)})
			got, err := c.Initialize(context.Background(), p.InitializeParams{ClientInfo: p.ClientInfo{Name: "fixture", Version: "1"}})
			_, cached := c.InitializedParams()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || cached || !reflect.DeepEqual(got, p.InitializeResponse{}) {
					t.Fatalf("old check priority/publication changed: %+v %v cached=%v", got, err, cached)
				}
				if tc.wantErr == "null" && !errors.Is(err, p.ErrNullResultField) {
					t.Fatalf("existing null occurrence cause lost: %v", err)
				}
			} else if err != nil || !cached || got.CodexHome != tc.selected {
				t.Fatalf("native selected path changed: %+v %v", got, err)
			}
		})
	}
}

func TestInitializeHomeBoundedDiagnosticAndPlainRecord(t *testing.T) {
	for _, path := range []string{"relative/" + strings.Repeat("peer-", 20000), "/tmp/" + strings.Repeat("peer-", 20000) + "/../home"} {
		mock := NewMockTransport()
		c := p.NewClient(mock)
		t.Cleanup(func() { _ = c.Close() })
		if err := mock.SetResponseData("initialize", p.InitializeResponse{CodexHome: path, PlatformFamily: "fixture-family", PlatformOS: "fixture-target", UserAgent: "fixture-agent"}); err != nil {
			t.Fatal(err)
		}
		got, err := c.Initialize(context.Background(), p.InitializeParams{ClientInfo: p.ClientInfo{Name: "fixture", Version: "1"}})
		if err == nil || len(err.Error()) > 2048 || !strings.Contains(err.Error(), "codexHome") || !reflect.DeepEqual(got, p.InitializeResponse{}) {
			t.Fatal("unbounded diagnostic or invalid publication")
		}
		if _, cached := c.InitializedParams(); cached {
			t.Fatal("invalid path cached")
		}
	}
	for _, v := range []any{p.InitializeResponse{}, &p.InitializeResponse{}} {
		if _, ok := v.(json.Unmarshaler); ok {
			t.Fatal("public response gained JSON decoder")
		}
		if _, ok := v.(json.Marshaler); ok {
			t.Fatal("public response gained JSON encoder")
		}
	}
	var anonymous struct {
		p.InitializeResponse
		Extra string
	}
	if err := json.Unmarshal([]byte(`{"codexHome":"relative","Extra":"kept"}`), &anonymous); err != nil || anonymous.CodexHome != "relative" || anonymous.Extra != "kept" {
		t.Fatal("ordinary anonymous JSON behavior changed")
	}
	var named struct {
		Response p.InitializeResponse
		Extra    string
	}
	if err := json.Unmarshal([]byte(`{"Response":{"codexHome":"relative"},"Extra":"kept"}`), &named); err != nil || named.Response.CodexHome != "relative" || named.Extra != "kept" {
		t.Fatal("ordinary named JSON behavior changed")
	}
}
