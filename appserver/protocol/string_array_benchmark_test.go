package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func BenchmarkStringArrayPublicAdmission(b *testing.B) {
	for _, size := range []int{1, 4096} {
		for _, method := range []string{"thread/loaded/list", "app/list", "model/verification"} {
			b.Run(fmt.Sprintf("%s/%d", method, size), func(b *testing.B) {
				item := `"value"`
				if method == "model/verification" {
					item = `"trustedAccessForCyber"`
				}
				array := "[" + strings.TrimSuffix(strings.Repeat(item+",", size), ",") + "]"
				body := `{"data":` + array + `}`
				switch method {
				case "app/list":
					body = `{"data":[{"id":"a","name":"A","appMetadata":{"categories":` + array + `}}]}`
				case "model/verification":
					body = `{"threadId":"t","turnId":"u","verifications":` + array + `}`
				}
				mock := NewMockTransport()
				client := protocol.NewClient(mock)
				defer client.Close()
				mock.SetResponse(method, protocol.Response{Result: json.RawMessage(body)})
				called := 0
				client.OnModelVerification(func(protocol.ModelVerificationNotification) { called++ })
				notification := protocol.Notification{Method: method, Params: json.RawMessage(body)}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					var err error
					switch method {
					case "thread/loaded/list":
						_, err = client.Thread.LoadedList(context.Background(), protocol.ThreadLoadedListParams{})
					case "app/list":
						_, err = client.Apps.List(context.Background(), protocol.AppsListParams{})
					case "model/verification":
						mock.InjectServerNotification(context.Background(), notification)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
				if method == "model/verification" && called != b.N {
					b.Fatalf("callbacks=%d operations=%d", called, b.N)
				}
			})
		}
	}
}

func FuzzStringArrayOccurrenceAdmission(f *testing.F) {
	f.Add(uint8(0), "", false)
	f.Add(uint8(13), "trustedAccessForCyber", true)
	f.Add(uint8(18), "escaped\nvalue", false)
	f.Fuzz(func(t *testing.T, owner uint8, value string, duplicate bool) {
		if len(value) > 256 {
			t.Skip()
		}
		cases := stringArrayContractCases()
		tc := cases[int(owner)%len(cases)]
		item := tc.validItem
		if item == `""` {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			item = string(encoded)
		}
		valid := "[" + item + "]"
		invalid := "[" + item + ",null]"
		if duplicate {
			invalid += `,"` + tc.field + `":` + valid
		}
		target := tc.newValue()
		if err := json.Unmarshal([]byte(strings.Replace(tc.body, "@VALUE@", valid, 1)), target); err != nil {
			t.Fatalf("schema-valid string rejected: %v", err)
		}
		before, err := json.Marshal(target)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(strings.Replace(tc.body, "@VALUE@", invalid, 1)), target); err == nil {
			t.Fatal("null item admitted")
		}
		after, err := json.Marshal(target)
		if err != nil || string(before) != string(after) {
			t.Fatal("rejection changed prior receiver")
		}
	})
}
