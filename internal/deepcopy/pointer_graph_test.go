package deepcopy

import (
	"reflect"
	"testing"
)

type graphPointer *pointerGraph
type pointerGraph struct {
	Value int
	Next  graphPointer
	Refs  []any
}

func TestDefinedPointerGraph(t *testing.T) {
	source := "original"
	pointer := definedStringPointer(&source)
	input := map[definedStringPointer]any{pointer: pointer}
	cloned := Value(input)
	for key, value := range cloned {
		if key == pointer || value.(definedStringPointer) != key {
			t.Fatal("map key/value lost pointer type, shared reference or ownership")
		}
		*key = "copy"
		if source != "original" {
			t.Fatal("map copy aliases input")
		}
	}

	type interfacePointer *any
	var boxed any
	var self interfacePointer = &boxed
	boxed = self
	copied := Value(self)
	if copied == self || (*copied).(interfacePointer) != copied {
		t.Fatal("defined pointer/interface cycle lost identity or ownership")
	}

	type innerPointer *int
	type outerPointer *innerPointer
	number := 7
	var inner innerPointer = &number
	var outer outerPointer = &inner
	chain := Value(outer)
	if chain == outer || *chain == inner || **chain != 7 {
		t.Fatal("nested defined pointers changed")
	}
	**chain = 9
	if number != 7 {
		t.Fatal("nested pointer copy aliases input")
	}

	var nilPointer definedStringPointer
	var typed any = nilPointer
	if got := Value(typed); reflect.TypeOf(got) != reflect.TypeOf(typed) || !reflect.ValueOf(got).IsNil() {
		t.Fatal("interface-held typed nil changed")
	}
}

func FuzzDefinedPointerGraph(f *testing.F) {
	f.Add([]byte{0})
	f.Add([]byte{1, 2, 3, 4, 255})
	f.Add([]byte{7, 0, 0, 2, 2, 1, 6, 8})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 || len(data) > 256 {
			return
		}
		count := 1 + int(data[0])%16
		nodes := make([]pointerGraph, count)
		pointers := make([]graphPointer, count)
		for i := range nodes {
			pointers[i] = &nodes[i]
		}
		for i := range nodes {
			value := data[i%len(data)]
			edge := pointers[int(value)%count]
			nodes[i] = pointerGraph{Value: int(value), Next: edge, Refs: []any{pointers[i], pointers[int(data[(i+1)%len(data)])%count], graphPointer(nil)}}
			if value&128 != 0 {
				nodes[i].Next = nil
			}
		}
		input := pointers[0]
		copied := Value(input)
		// Compare graph relationships against the source, independently of the
		// reflection copier. Distinct source nodes must remain distinct; every
		// repeated typed reference must reach the same owned copied node.
		type pair struct{ source, cloned graphPointer }
		queue := []pair{{input, copied}}
		forward := make(map[graphPointer]graphPointer)
		reverse := make(map[graphPointer]graphPointer)
		values := make(map[graphPointer]int)
		for len(queue) > 0 {
			item := queue[0]
			queue = queue[1:]
			if item.source == nil {
				if item.cloned != nil {
					t.Fatal("nil graph edge changed")
				}
				continue
			}
			if prior, ok := forward[item.source]; ok {
				if prior != item.cloned {
					t.Fatal("shared graph reference split")
				}
				continue
			}
			if item.cloned == nil || item.cloned.Value != item.source.Value {
				t.Fatal("graph node value changed")
			}
			for _, original := range pointers {
				if item.cloned == original {
					t.Fatal("copied graph retains an input node")
				}
			}
			if prior, ok := reverse[item.cloned]; ok && prior != item.source {
				t.Fatal("distinct graph nodes merged")
			}
			forward[item.source] = item.cloned
			reverse[item.cloned] = item.source
			values[item.source] = item.source.Value
			if len(item.cloned.Refs) != len(item.source.Refs) {
				t.Fatal("reference list changed")
			}
			queue = append(queue, pair{item.source.Next, item.cloned.Next})
			for i, ref := range item.source.Refs {
				actual, ok := item.cloned.Refs[i].(graphPointer)
				if !ok {
					t.Fatalf("interface edge lost defined pointer type: %T", item.cloned.Refs[i])
				}
				queue = append(queue, pair{ref.(graphPointer), actual})
			}
		}
		for original, cloned := range forward {
			cloned.Value = -1
			if original.Value != values[original] {
				t.Fatal("copy mutation changed source")
			}
		}
		for original, cloned := range forward {
			original.Value = -2
			if cloned.Value != -1 {
				t.Fatal("source mutation changed copy")
			}
		}
	})
}
