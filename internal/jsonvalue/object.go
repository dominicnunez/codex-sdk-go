// Package jsonvalue snapshots JSON objects for internal protocol ownership.
package jsonvalue

import (
	"bytes"
	"encoding/json"
)

// CloneObject owns the serialized settings, including values represented by
// custom Go containers or marshalers. Numbers retain their exact JSON spelling.
func CloneObject(in map[string]interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	err = decoder.Decode(&out)
	return out, err
}
