package codex

import (
	"reflect"
	"strings"
	"testing"
)

// TestNotificationListenerCoverage verifies that every typed public On* method
// on Client registers a method-scoped notification handler.
func TestNotificationListenerCoverage(t *testing.T) {
	client := NewClient(&mockInternalTransport{})

	methods := reflect.TypeOf(client)
	expectedListeners := 0

	for i := 0; i < methods.NumMethod(); i++ {
		method := methods.Method(i)
		if !isTypedNotificationMethod(method) {
			continue
		}

		handlerType := method.Type.In(1)
		handler := reflect.MakeFunc(handlerType, func(_ []reflect.Value) []reflect.Value {
			return nil
		})
		method.Func.Call([]reflect.Value{reflect.ValueOf(client), handler})
		expectedListeners++
	}

	if got := len(client.notificationListeners); got != expectedListeners {
		t.Fatalf("registered %d listeners, want %d typed On* handlers covered", got, expectedListeners)
	}
}

func isTypedNotificationMethod(method reflect.Method) bool {
	if !strings.HasPrefix(method.Name, "On") {
		return false
	}
	if method.Name == "OnNotification" || strings.HasPrefix(method.Name, "OnCollabToolCall") {
		return false
	}
	if method.Type.NumIn() != 2 || method.Type.NumOut() != 0 {
		return false
	}

	handlerType := method.Type.In(1)
	return handlerType.Kind() == reflect.Func && handlerType.NumIn() == 1 && handlerType.NumOut() == 0
}
