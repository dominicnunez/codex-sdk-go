package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sync"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// Each synced wire type has one fixed schema owner. Reuse its immutable field
// plan and pool only the scratch admission state. The decoding and error rules
// are the same as decodeObjectWithValidation; existing decoders are unchanged.
type syncedObjectPlan struct {
	required      []string
	nonNullFields []string
	fields        map[string]inboundObjectField
	nonNull       map[string]struct{}
	states        sync.Pool
}

var syncedObjectPlans sync.Map

func unmarshalSyncedInbound(data []byte, dest any, required, nonNull []string) error {
	return unmarshalSyncedObject(data, dest, required, nonNull, inboundObjectValidationErrors())
}

func unmarshalSyncedResponse(data []byte, dest any, required, nonNull []string) error {
	return unmarshalSyncedObject(data, dest, required, nonNull, responseObjectValidationErrors())
}

func unmarshalSyncedObject(data []byte, dest any, required, nonNull []string, validation objectValidationErrors) error {
	value, fields, handled, err := resolveInboundObjectDestination(data, dest)
	if err != nil || handled {
		return err
	}
	if !value.IsValid() || !json.Valid(data) {
		return decodeObjectWithValidation(data, dest, required, nonNull, validation)
	}
	if bytes.TrimSpace(data)[0] != '{' {
		return validation.notObject(fmt.Errorf("expected JSON object"))
	}
	plan := syncedPlan(value.Type(), fields, required, nonNull)
	// Keep this helper safe if a future owner supplies a different contract for
	// the same wire type. Such calls use the uncached reference implementation.
	if !slices.Equal(plan.required, required) || !slices.Equal(plan.nonNullFields, nonNull) {
		return decodeObjectWithValidation(data, dest, required, nonNull, validation)
	}
	state, ok := plan.states.Get().(*inboundObjectDecodeState)
	if !ok {
		return decodeObjectWithValidation(data, dest, required, nonNull, validation)
	}
	state.dest = value
	state.validation = validation
	defer func() {
		state.dest = reflect.Value{}
		state.validation = objectValidationErrors{}
		state.err = nil
		clear(state.selectedStrings)
		for name := range state.required.seen {
			state.required.seen[name] = false
		}
		plan.states.Put(state)
	}()
	jsonobject.WalkFields(data, true, state.visit)
	state.flushStrings()
	if state.err != nil {
		return state.err
	}
	return validateRequiredInboundObjectFields(state.required, validation)
}

func syncedPlan(typ reflect.Type, fields map[string]inboundObjectField, required, nonNull []string) *syncedObjectPlan {
	if cached, ok := syncedObjectPlans.Load(typ); ok {
		if plan, ok := cached.(*syncedObjectPlan); ok {
			return plan
		}
	}
	plan := &syncedObjectPlan{required: slices.Clone(required), nonNullFields: slices.Clone(nonNull), fields: fields, nonNull: make(map[string]struct{}, len(nonNull))}
	for _, name := range nonNull {
		plan.nonNull[name] = struct{}{}
	}
	plan.states.New = func() any {
		seen := make(map[string]bool, len(plan.required))
		for _, name := range plan.required {
			seen[name] = false
		}
		return &inboundObjectDecodeState{fields: plan.fields, required: inboundRequiredFields{order: plan.required, seen: seen}, nonNull: plan.nonNull}
	}
	actual, _ := syncedObjectPlans.LoadOrStore(typ, plan)
	if loaded, ok := actual.(*syncedObjectPlan); ok {
		return loaded
	}
	return plan
}
