package deepcopy

import (
	"reflect"
	"testing"
)

func TestSharedBackingSlices(t *testing.T) {
	data := []int{1, 2, 3}
	for _, slices := range [][][]int{
		{data[:1], data}, {data, data[:1]}, {data[:0], data},
		{nil, data[:0], data}, {data, data},
	} {
		got := Value(slices)
		if !reflect.DeepEqual(got, slices) {
			t.Fatalf("copy = %v, want %v", got, slices)
		}
		for _, slice := range got {
			if len(slice) > 0 {
				slice[0] = 99
			}
		}
		if data[0] != 1 {
			t.Fatal("copy aliases original backing array")
		}
	}
}

func TestCyclesSurviveCopy(t *testing.T) {
	s := make([]any, 2)
	s[0] = s[:1]
	s[1] = s
	got := Value(s)
	if !reflect.DeepEqual(got, s) {
		t.Fatal("slice cycle changed")
	}
	shorter := got[0].([]any)
	if len(shorter) != 1 {
		t.Fatal("slice cycle has wrong length")
	}
	shorter[0] = "changed"
	if _, ok := s[0].([]any); !ok {
		t.Fatal("slice cycle aliases original")
	}
	m := map[string]any{}
	m["self"] = m
	cloned := Value(m)
	cloned["self"].(map[string]any)["new"] = true
	if _, ok := cloned["new"]; !ok {
		t.Fatal("map cycle not preserved")
	}
	if _, ok := m["new"]; ok {
		t.Fatal("map cycle aliases original")
	}
}
