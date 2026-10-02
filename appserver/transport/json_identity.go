package transport

import (
	"encoding/json"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

func selectJSONObjectField(data []byte, name string) (json.RawMessage, bool, bool) {
	return jsonobject.SelectField(data, name)
}
func walkJSONObjectFields(data []byte, validated bool, visit func(key, value []byte)) bool {
	return jsonobject.WalkFields(data, validated, visit)
}
func scanJSONValueEnd(data []byte, start int) (int, bool) {
	return jsonobject.ValueEnd(data, start)
}
func scanJSONStringEnd(data []byte, start int) (int, bool) {
	return jsonobject.StringEnd(data, start)
}
func jsonFieldMatches(raw []byte, name string) bool { return jsonobject.FieldMatches(raw, name) }
func jsonFieldMatchesFolded(raw []byte, name string) bool {
	return jsonobject.FieldMatchesFolded(raw, name)
}
