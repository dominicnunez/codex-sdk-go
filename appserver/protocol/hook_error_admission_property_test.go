package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type hookErrorStdlibItem struct {
	Message *string `json:"message"`
	Path    *string `json:"path"`
}

type hookErrorStdlibEntry struct {
	Errors []hookErrorStdlibItem `json:"errors"`
}

type hookErrorHistory struct {
	entryJSON  string
	stickyNull bool
}

// This bounded generator creates legal split-field histories plus histories
// with missing final fields and explicit nulls. The method-free reference lets
// encoding/json independently own duplicate-array merging and visible values;
// the generator separately records null occurrences because the protocol
// contract keeps those invalid even after later native repair or reset.
func TestHooksListHookErrorAdmissionMatchesGeneratedStdlibReference(t *testing.T) {
	rng := rand.New(rand.NewSource(127))
	accepted, rejected := 0, 0
	for caseIndex := 0; caseIndex < 128; caseIndex++ {
		history := generateHookErrorHistory(rng, caseIndex)
		var reference hookErrorStdlibEntry
		if err := json.Unmarshal([]byte(history.entryJSON), &reference); err != nil {
			t.Fatalf("reference decode for generated case %d failed: %v", caseIndex, err)
		}
		wantReject := history.stickyNull
		for _, item := range reference.Errors {
			if item.Message == nil || item.Path == nil {
				wantReject = true
			}
		}

		transport := NewMockTransport()
		client := codex.NewClient(transport)
		t.Cleanup(func() { _ = client.Close() })
		entryPayload := `{"data":[` + history.entryJSON + `]}`
		if err := transport.SetResponseData("hooks/list", json.RawMessage(entryPayload)); err != nil {
			t.Fatal(err)
		}
		response, err := client.Hooks.List(context.Background(), codex.HooksListParams{})
		if wantReject {
			rejected++
			if err == nil || len(response.Data) != 0 {
				t.Fatalf("generated case %d should reject; response=%+v err=%v input=%s", caseIndex, response, err, history.entryJSON)
			}
			assertRejectedHookErrorReceiverUnchanged(t, history.entryJSON, caseIndex)
			continue
		}

		accepted++
		if err != nil {
			t.Fatalf("generated case %d should pass: %v input=%s", caseIndex, err, history.entryJSON)
		}
		if len(response.Data) != 1 || len(response.Data[0].Errors) != len(reference.Errors) {
			t.Fatalf("generated case %d visible item count=%d; want %d", caseIndex, len(response.Data[0].Errors), len(reference.Errors))
		}
		for index, item := range reference.Errors {
			got := response.Data[0].Errors[index]
			if got.Message != *item.Message || got.Path != *item.Path {
				t.Fatalf("generated case %d item %d = %+v; want message=%q path=%q input=%s", caseIndex, index, got, *item.Message, *item.Path, history.entryJSON)
			}
		}
	}
	if accepted < 30 || rejected < 30 {
		t.Fatalf("generated coverage too one-sided: accepted=%d rejected=%d", accepted, rejected)
	}
}

func generateHookErrorHistory(rng *rand.Rand, caseIndex int) hookErrorHistory {
	switch caseIndex {
	case 0:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"me\u017Fsage":"split-message"}],"errors":[{"pa\u0074h":"split/path"}],"hooks":[],"warnings":[]}`}
	case 1:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"message":"first","path":"first"},{"message":"old","path":"retained"}],"errors":[{"message":"middle","path":"middle"}],"errors":[{"message":"final","path":"final"},{"message":"reused"}],"hooks":[],"warnings":[]}`}
	case 2:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"message":"old","path":"old"}],"errors":[],"errors":[{"message":"new"}],"hooks":[],"warnings":[]}`}
	case 3:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[null],"errors":[],"errors":[{"message":"repaired","path":"after-reset"}],"hooks":[],"warnings":[]}`, stickyNull: true}
	case 4:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"message":null,"path":"before-reset"}],"errors":[],"errors":[{"message":"repaired","path":"after-reset"}],"hooks":[],"warnings":[]}`, stickyNull: true}
	case 5:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"message":"first","path":"first"},{"message":"partial-hidden"}],"errors":[{"message":"visible","path":"visible"}],"hooks":[],"warnings":[]}`}
	case 6:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"message":"old","path":"old"}],"errors":[],"errors":[{}],"hooks":[],"warnings":[]}`}
	case 7:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"message":"old","path":null}],"errors":[],"errors":[{"message":"repaired","path":"after-reset"}],"hooks":[],"warnings":[]}`, stickyNull: true}
	case 8:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"message":"ok","path":null,"PATH":"repaired"}],"hooks":[],"warnings":[]}`, stickyNull: true}
	case 9:
		return hookErrorHistory{entryJSON: `{"cwd":"/tmp","errors":[{"message":null,"MESSAGE":"repaired","path":"opaque"}],"hooks":[],"warnings":[]}`, stickyNull: true}
	}

	operationCount := 2 + rng.Intn(3)
	forceNull := caseIndex%8 == 1
	stickyNull := false
	operations := make([]string, 0, operationCount)
	for operation := 0; operation < operationCount; operation++ {
		if forceNull && operation == 0 {
			if caseIndex%16 == 1 {
				operations = append(operations, `"errors":[null]`)
			} else {
				operations = append(operations, `"errors":[{"message":null,"path":"null-before-reset"}]`)
			}
			stickyNull = true
			continue
		}
		if forceNull && operation == 1 && caseIndex%16 == 9 {
			operations = append(operations, `"errors":[]`)
			continue
		}

		forceComplete := caseIndex%2 == 0 && operation == operationCount-1
		count := rng.Intn(4)
		if forceComplete {
			count = 1 + rng.Intn(3)
		}
		items := make([]string, 0, count)
		for index := 0; index < count; index++ {
			mask := rng.Intn(4)
			if forceComplete {
				mask = 3
			}
			item := generateHookErrorItem(rng, caseIndex, operation, index, mask)
			items = append(items, item)
		}
		operations = append(operations, `"errors":[`+strings.Join(items, ",")+`]`)
	}

	return hookErrorHistory{
		entryJSON:  `{"cwd":"/tmp",` + strings.Join(operations, ",") + `,"hooks":[],"warnings":[]}`,
		stickyNull: stickyNull,
	}
}

func generateHookErrorItem(rng *rand.Rand, caseIndex, operation, index, mask int) string {
	messageKey := []string{`message`, `MESSAGE`, `me\u0073sage`, `me\u017Fsage`}[rng.Intn(4)]
	pathKey := []string{`path`, `PATH`, `pa\u0074h`}[rng.Intn(3)]
	message := fmt.Sprintf("message-%d-%d-%d", caseIndex, operation, index)
	path := fmt.Sprintf("relative/%d/%d/%d", caseIndex, operation, index)
	messageJSON := `"` + message + `"`
	pathJSON := `"` + path + `"`
	fields := make([]string, 0, 2)
	if mask&1 != 0 {
		fields = append(fields, `"`+messageKey+`":`+messageJSON)
	}
	if mask&2 != 0 {
		fields = append(fields, `"`+pathKey+`":`+pathJSON)
	}
	return `{` + strings.Join(fields, ",") + `}`
}

func assertRejectedHookErrorReceiverUnchanged(t *testing.T, entryJSON string, caseIndex int) {
	t.Helper()
	entry := codex.HooksListEntry{
		Cwd:      "/before",
		Errors:   []codex.HookErrorInfo{{Message: "retained", Path: "opaque"}},
		Hooks:    []codex.HookMetadata{},
		Warnings: []string{"retained"},
	}
	want := codex.HooksListEntry{
		Cwd:      "/before",
		Errors:   []codex.HookErrorInfo{{Message: "retained", Path: "opaque"}},
		Hooks:    []codex.HookMetadata{},
		Warnings: []string{"retained"},
	}
	oldSlot := &entry.Errors[0]
	err := json.Unmarshal([]byte(entryJSON), &entry)
	if err == nil {
		t.Fatalf("generated rejected case %d decoded into receiver successfully", caseIndex)
	}
	if &entry.Errors[0] != oldSlot || !reflect.DeepEqual(entry, want) || entry.Errors[0] != (codex.HookErrorInfo{Message: "retained", Path: "opaque"}) {
		t.Fatalf("generated rejected case %d mutated previous receiver storage: %+v", caseIndex, entry)
	}
}
