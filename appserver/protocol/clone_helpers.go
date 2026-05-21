package protocol

import "github.com/dominicnunez/codex-sdk-go/internal/deepcopy"

func cloneThreadState(thread Thread) Thread {
	return cloneArbitraryValue(thread)
}

func cloneThreadStatusWrapper(w ThreadStatusWrapper) ThreadStatusWrapper {
	return cloneArbitraryValue(w)
}

func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

func cloneArbitraryValue[T any](in T) T {
	return deepcopy.Value(in)
}
