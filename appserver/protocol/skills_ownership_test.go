package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestSkillsListPreservesCallerParams(t *testing.T) {
	for _, failure := range []string{"", "cwd", "root"} {
		t.Run("failure="+failure, func(t *testing.T) {
			params := codex.SkillsListParams{
				Cwds:        []string{"relative/../selector"},
				ForceReload: ptr(false),
				PerCwdExtraUserRoots: []codex.SkillsListExtraRootsForCwd{
					{Cwd: "/tmp/../first", ExtraUserRoots: []string{"/tmp/../skills", "/other//skills"}},
					{Cwd: "/tmp/../second", ExtraUserRoots: []string{"/tmp/../valid", "/more//skills"}},
					{Cwd: "/nil", ExtraUserRoots: nil},
					{Cwd: "/empty", ExtraUserRoots: []string{}},
				},
			}
			wantError := ""
			switch failure {
			case "cwd":
				params.PerCwdExtraUserRoots[1].Cwd = "relative"
				wantError = "perCwdExtraUserRoots[1].cwd"
			case "root":
				params.PerCwdExtraUserRoots[1].ExtraUserRoots[1] = "relative"
				wantError = "perCwdExtraUserRoots[1].extraUserRoots[1]"
			}
			// Snapshot through JSON independently; both backing-array aliases and
			// nil/empty root distinctions must survive request preparation.
			before, err := json.Marshal(params)
			if err != nil {
				t.Fatal(err)
			}
			var want codex.SkillsListParams
			if err := json.Unmarshal(before, &want); err != nil {
				t.Fatal(err)
			}
			records := params.PerCwdExtraUserRoots
			roots := records[0].ExtraUserRoots
			transport := NewMockTransport()
			transport.SetResponse("skills/list", codex.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[]}`)})
			_, err = codex.NewClient(transport).Skills.List(context.Background(), params)
			if wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				if len(transport.SentRequests) != 1 {
					t.Fatalf("requests = %d, want 1", len(transport.SentRequests))
				}
				wantWire := `{"cwds":["relative/../selector"],"forceReload":false,"perCwdExtraUserRoots":[{"cwd":"/first","extraUserRoots":["/skills","/other/skills"]},{"cwd":"/second","extraUserRoots":["/valid","/more/skills"]},{"cwd":"/nil","extraUserRoots":[]},{"cwd":"/empty","extraUserRoots":[]}]}`
				if got := string(transport.GetSentRequest(0).Params); got != wantWire {
					t.Errorf("wire = %s, want %s", got, wantWire)
				}
			} else {
				if !errors.Is(err, codex.ErrInvalidParams) || !strings.Contains(err.Error(), wantError) {
					t.Errorf("error = %v, want invalid params at %s", err, wantError)
				}
				if len(transport.SentRequests) != 0 {
					t.Errorf("invalid request reached transport")
				}
			}
			if !reflect.DeepEqual(params, want) || !reflect.DeepEqual(records, want.PerCwdExtraUserRoots) || !reflect.DeepEqual(roots, want.PerCwdExtraUserRoots[0].ExtraUserRoots) {
				t.Errorf("caller data changed: got %#v, want %#v", params, want)
			}
		})
	}
}

func TestSkillsListConcurrentParamReuse(t *testing.T) {
	params := codex.SkillsListParams{PerCwdExtraUserRoots: []codex.SkillsListExtraRootsForCwd{
		{Cwd: "/tmp/../workspace", ExtraUserRoots: []string{"/tmp/../skills"}},
	}}
	transport := NewMockTransport()
	transport.SetResponse("skills/list", codex.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[]}`)})
	client := codex.NewClient(transport)
	const calls = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range calls {
		wg.Go(func() {
			<-start
			if _, err := client.Skills.List(context.Background(), params); err != nil {
				t.Errorf("List: %v", err)
			}
		})
	}
	close(start)
	wg.Wait()
	if len(transport.SentRequests) != calls {
		t.Fatalf("requests = %d, want %d", len(transport.SentRequests), calls)
	}
	for i := range calls {
		if got := string(transport.GetSentRequest(i).Params); got != `{"perCwdExtraUserRoots":[{"cwd":"/workspace","extraUserRoots":["/skills"]}]}` {
			t.Errorf("wire = %s", got)
		}
	}
	if params.PerCwdExtraUserRoots[0].Cwd != "/tmp/../workspace" || params.PerCwdExtraUserRoots[0].ExtraUserRoots[0] != "/tmp/../skills" {
		t.Errorf("caller data changed: %#v", params)
	}
}

func TestSkillsListPreservesEmptyLegacyRoots(t *testing.T) {
	for _, records := range [][]codex.SkillsListExtraRootsForCwd{nil, {}} {
		params := codex.SkillsListParams{PerCwdExtraUserRoots: records}
		transport := NewMockTransport()
		transport.SetResponse("skills/list", codex.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[]}`)})
		if _, err := codex.NewClient(transport).Skills.List(context.Background(), params); err != nil {
			t.Fatal(err)
		}
		if got := string(transport.GetSentRequest(0).Params); got != `{}` {
			t.Errorf("empty legacy records wire = %s, want {}", got)
		}
		if !reflect.DeepEqual(params.PerCwdExtraUserRoots, records) {
			t.Errorf("caller records changed: %#v", params.PerCwdExtraUserRoots)
		}
	}
}
