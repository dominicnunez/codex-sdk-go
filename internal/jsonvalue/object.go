// Package jsonvalue snapshots JSON objects for internal protocol ownership.
package jsonvalue

import (
	"bytes"
	"encoding/json"
)

// CloneObject owns canonical raw settings. Object key order and whitespace do
// not affect identity; array order and exact number spelling remain significant.
func CloneObject(in map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var values map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&values); err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	if values != nil {
		out = make(map[string]json.RawMessage, len(values))
		for name, value := range values {
			data, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			out[name] = data
		}
	}
	return out, nil
}
