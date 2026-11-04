package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

type failedPushReader struct{ err error }

func (r failedPushReader) Read([]byte) (int, error) { return 0, r.err }

func TestPushTipProtocol(t *testing.T) {
	oid, zero := strings.Repeat("a", 40), strings.Repeat("0", 40)
	valid := "refs/heads/café\u00a0branch " + oid + " refs/heads/remote " + zero + "\n"
	for _, line := range []string{"\n", "invalid\n", valid + "invalid\n", strings.ReplaceAll(valid, " ", "\t"), strings.Replace(valid, oid, "bad", 1), strings.Replace(valid, zero, strings.Repeat("0", 64), 1)} {
		if tips, err := pushTips(strings.NewReader(line)); err == nil || tips != nil {
			t.Fatalf("malformed protocol accepted: %q: %v, %v", line, tips, err)
		}
	}
	for _, ending := range []string{"", "\n", "\r\n"} {
		tips, err := pushTips(strings.NewReader(strings.TrimSuffix(valid, "\n") + ending))
		if err != nil || !reflect.DeepEqual(tips, []outgoingTip{{"refs/heads/café\u00a0branch", oid}}) {
			t.Fatalf("exact ref identity lost for ending %q: %v, %v", ending, tips, err)
		}
	}
	wantErr := errors.New("ref producer failed")
	tips, err := pushTips(io.MultiReader(strings.NewReader(valid), failedPushReader{wantErr}))
	if !errors.Is(err, wantErr) || tips != nil {
		t.Fatalf("failed producer returned partial selection: %v, %v", tips, err)
	}
}

func FuzzPushTipProtocol(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4})
	f.Add([]byte{255, 7, 0, 3})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64 {
			data = data[:64]
		}
		var input bytes.Buffer
		var want []outgoingTip
		for n, choice := range data {
			width := 20
			if choice&1 != 0 {
				width = 32
			}
			oid := hex.EncodeToString(bytes.Repeat([]byte{choice}, width))
			ref := fmt.Sprintf("refs/heads/café\u00a0branch-%02x-%d", choice, n)
			ending := "\n"
			if choice&2 != 0 {
				ending = "\r\n"
			}
			fmt.Fprintf(&input, "%s %s refs/heads/remote-%d %s%s", ref, oid, n, strings.Repeat("0", width*2), ending)
			if choice != 0 {
				want = append(want, outgoingTip{ref, oid})
			}
		}
		got, err := pushTips(bytes.NewReader(input.Bytes()))
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("ordered nondeletion tips=%v, want=%v, err=%v", got, want, err)
		}
		input.WriteString("malformed trailing record\n")
		if got, err := pushTips(bytes.NewReader(input.Bytes())); err == nil || got != nil {
			t.Fatalf("malformed tail exposed partial selection: %v, %v", got, err)
		}
	})
}
