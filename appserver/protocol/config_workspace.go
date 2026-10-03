package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// UnmarshalJSON retains both workspace restriction branches without changing
// the legacy single-ID field. Omitted fields retain reused receiver state.
// Invalid restrictions reject decoding. As with other custom JSON decoders,
// errors can stop traversal of an enclosing object before its later fields.
// A restriction error also stops later Config fields and may supersede an
// earlier saved stdlib type error. Do not consume a partially decoded Config.
func (c *Config) UnmarshalJSON(data []byte) error {
	// Keep the original type name for standard field error context.
	type Config configWithoutMethods
	wire := struct {
		*Config
		Workspace workspaceRestrictionDecoder `json:"forced_chatgpt_workspace_id"`
	}{Config: (*Config)(c), Workspace: workspaceRestrictionDecoder{single: &c.ForcedChatgptWorkspaceID, multiple: &c.ForcedChatgptWorkspaceIDs}}
	err := json.Unmarshal(data, &wire)
	// The shadow wire's anonymous embedding adds a synthetic Config path and
	// erases its name. Restore the direct Config field context.
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) && typeError.Field == "" && typeError.Type == reflect.TypeOf(wire) {
		typeError.Type = reflect.TypeOf(*c)
	}
	if errors.As(err, &typeError) && strings.HasPrefix(typeError.Field, "Config.") {
		typeError.Field = strings.TrimPrefix(typeError.Field, "Config.")
		if typeError.Struct == "" {
			typeError.Struct = "Config"
		}
	}
	if typeError != nil && typeError.Struct == "" && typeError.Field == "forced_chatgpt_workspace_id" {
		typeError.Struct = "Config"
	}
	return err
}

type configWithoutMethods Config

// MarshalJSON keeps the selected string or array branch. Conflicting constructed
// restrictions fail instead of silently narrowing the list to a single ID.
// Use Config as a named member in application envelopes: anonymous embedding
// promotes these JSON methods and requires the envelope's own implementation.
func (c Config) MarshalJSON() ([]byte, error) {
	if c.ForcedChatgptWorkspaceID != nil && c.ForcedChatgptWorkspaceIDs != nil {
		return nil, fmt.Errorf("forced_chatgpt_workspace_id: both single and multiple workspace restrictions are set")
	}
	var workspace any
	if c.ForcedChatgptWorkspaceIDs != nil {
		workspace = c.ForcedChatgptWorkspaceIDs
	} else if c.ForcedChatgptWorkspaceID != nil {
		workspace = c.ForcedChatgptWorkspaceID
	}
	return json.Marshal(struct {
		configWithoutMethods
		Workspace any `json:"forced_chatgpt_workspace_id,omitempty"`
	}{configWithoutMethods: configWithoutMethods(c), Workspace: workspace})
}

type workspaceRestrictionDecoder struct {
	single   **string
	multiple **[]string
}

func (w *workspaceRestrictionDecoder) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*w.single, *w.multiple = nil, nil
		return nil
	}
	if len(data) > 0 && data[0] == '[' {
		var values nonNullStringList
		if err := json.Unmarshal(data, &values); err != nil {
			return fmt.Errorf("forced_chatgpt_workspace_id: %w", err)
		}
		list := []string(values)
		*w.single, *w.multiple = nil, &list
		return nil
	}
	if err := json.Unmarshal(data, w.single); err != nil {
		return err
	}
	*w.multiple = nil
	return nil
}
