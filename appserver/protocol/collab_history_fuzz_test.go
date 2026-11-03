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

// Pointer presence in method-free native records independently models required
// members while preserving duplicate-object merging and nullable root resets.
type collabNativeMode struct {
	Mode     *string `json:"mode"`
	Settings *struct {
		Model *string `json:"model"`
	} `json:"settings"`
}

func FuzzCollabAdmissionHistory(f *testing.F) {
	for _, seed := range [][]byte{{}, {31}, {5, 26}, {31, 0, 26}, {159, 31}, {31, 4}, {31, 31, 0}} {
		f.Add(seed)
	}
	base, err := json.Marshal(validThreadLifecycleResponse(validProcessThreadPayload("collab-thread")))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, history []byte) {
		if len(history) > 16 {
			history = history[:16]
		}
		var members []string
		for _, op := range history {
			key := "collaborationMode"
			if op&128 != 0 {
				key = "COLLABORATIONMODE"
			}
			var value any
			if op%4 != 0 {
				object := map[string]any{}
				if op&4 != 0 {
					modeKey := "mode"
					if op&32 != 0 {
						modeKey = "MODE"
					}
					mode := "default"
					if op&1 != 0 {
						mode = "plan"
					}
					object[modeKey] = mode
				}
				if op&8 != 0 {
					settings := map[string]any{}
					if op&16 != 0 {
						modelKey := "model"
						if op&32 != 0 {
							modelKey = "MODEL"
						}
						model := "model"
						if op&2 != 0 {
							model = ""
						}
						settings[modelKey] = model
					}
					settingsKey := "settings"
					if op&64 != 0 {
						settingsKey = "ſettings"
					}
					object[settingsKey] = settings
				}
				value = object
			}
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			keyRaw, err := json.Marshal(key)
			if err != nil {
				t.Fatal(err)
			}
			members = append(members, string(keyRaw)+":"+string(raw))
		}
		raw := string(base)
		if len(members) > 0 {
			raw = raw[:len(raw)-1] + "," + strings.Join(members, ",") + "}"
		}
		var reference struct {
			Mode *collabNativeMode `json:"collaborationMode"`
		}
		if err := json.Unmarshal([]byte(raw), &reference); err != nil {
			t.Fatal(err)
		}
		valid := reference.Mode == nil || (reference.Mode.Mode != nil && reference.Mode.Settings != nil && reference.Mode.Settings.Model != nil)
		mock := NewMockTransport()
		client := p.NewClient(mock)
		defer client.Close()
		mock.SetResponse("thread/resume", p.Response{Result: json.RawMessage(raw)})
		result, err := client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"})
		if valid {
			if err != nil {
				t.Fatalf("valid native history rejected: %v input=%s", err, raw)
			}
			if reference.Mode == nil {
				if result.CollaborationMode != nil {
					t.Fatal("nullable root reset lost")
				}
			} else if result.CollaborationMode == nil || string(result.CollaborationMode.Mode) != *reference.Mode.Mode || result.CollaborationMode.Settings.Model != *reference.Mode.Settings.Model {
				t.Fatal("native merged values changed")
			}
		} else if !errors.Is(err, p.ErrMissingResultField) || !reflect.ValueOf(result).IsZero() {
			t.Fatalf("missing final presence published: %v", err)
		}
		// Explicit nested nulls are invalid even if a later allowed root null
		// discards the object, or a complete object supplies valid values.
		tail := `{"mode":"default","settings":{"model":"m"}}`
		if len(history) > 0 && history[0]&1 != 0 {
			tail = "null"
		}
		bad := `"collaborationMode":{"settings":{"MODEL":null}},"COLLABORATIONMODE":` + tail
		if err := resumeWithRawCollabMembers(t, bad); !errors.Is(err, p.ErrNullResultField) {
			t.Fatalf("later root erased forbidden nested null: %v", err)
		}
	})
}
