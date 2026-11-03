package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type marketplaceErrorReference struct {
	MarketplaceName *string `json:"marketplaceName"`
	Message         *string `json:"message"`
}

type marketplaceReference struct {
	Errors []marketplaceErrorReference `json:"errors"`
}

func marketplaceBody(fields string) []byte {
	return []byte(`{` + fields + `,"selectedMarketplaces":[],"upgradedRoots":[]}`)
}

func checkMarketplaceHistory(t *testing.T, raw []byte, stickyNull bool) {
	t.Helper()
	var reference marketplaceReference
	referenceErr := json.Unmarshal(raw, &reference)
	reject := stickyNull || referenceErr != nil
	for _, entry := range reference.Errors {
		if entry.MarketplaceName == nil || entry.Message == nil {
			reject = true
		}
	}
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	mock.SetResponse("marketplace/upgrade", p.Response{Result: raw})
	result, err := client.Marketplace.Upgrade(context.Background(), p.MarketplaceUpgradeParams{})
	if reject {
		if err == nil || !reflect.DeepEqual(result, p.MarketplaceUpgradeResponse{}) {
			t.Fatalf("invalid history admitted: %s", raw)
		}
		return
	}
	if err != nil || len(result.Errors) != len(reference.Errors) {
		t.Fatalf("valid native presence history rejected: %v input=%s", err, raw)
	}
	for i, entry := range reference.Errors {
		if result.Errors[i].MarketplaceName != *entry.MarketplaceName || result.Errors[i].Message != *entry.Message {
			t.Fatalf("native selected values changed: %s", raw)
		}
	}
}

func TestMarketplaceErrorHistories(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		sticky       bool
	}{
		{"split required fields", `"errors":[{"marketplaceName":"market"}],"errors":[{"message":"message"}]`, false},
		{"earlier incomplete replaced", `"errors":[{}],"errors":[{"marketplaceName":"","message":""}]`, false},
		{"hidden incomplete discarded", `"errors":[{"marketplaceName":"m","message":"x"},{}],"errors":[{}]`, false},
		{"shrink and regrow", `"errors":[{"marketplaceName":"a","message":"b"},{"marketplaceName":"hidden","message":"old"}],"errors":[{"marketplaceName":"new"}],"ERRORS":[{"message":"latest"},{"message":"regrown"}]`, false},
		{"empty resets presence", `"errors":[{"marketplaceName":"old","message":"old"}],"errors":[],"errors":[{"message":"new"}]`, false},
		{"missing name", `"errors":[{"message":""}]`, false},
		{"null element hidden", `"errors":[null],"errors":[],"errors":[{"marketplaceName":"","message":""}]`, true},
		{"null name hidden", `"errors":[{"marketplaceName":null,"marketplaceName":"","message":""}]`, true},
		{"null message alias hidden", `"errors":[{"marketplaceName":"","message":null,"MESSAGE":""}]`, true},
		{"escaped null", `"errors":[{"marketplaceNa\u006de":null,"message":""}]`, true},
		{"unicode alias null", `"errors":[{"marketplaceName":"","me\u017Fsage":null,"message":""}]`, true},
		{"unicode alias valid", `"errors":[{"MARKETPLACENAME":"","me\u017Fsage":""}]`, false},
		{"outer alias null", `"errors":[],"ERRORS":null`, true},
		{"outer unicode alias null", `"errors":[],"error\u017F":null`, true},
		{"open extras", `"errors":[{"marketplaceName":"","message":"","future":null}]`, false},
		{"native kind", `"errors":[{"marketplaceName":42,"message":""}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) { checkMarketplaceHistory(t, marketplaceBody(tc.fields), tc.sticky) })
	}
	// Force growth beyond the initial backing capacity after a nonempty shrink.
	for _, width := range []int{2, 5, 9} {
		t.Run("growth/"+strconv.Itoa(width), func(t *testing.T) {
			initial := make([]string, width)
			for i := range initial {
				initial[i] = `{"marketplaceName":"old","message":"kept"}`
			}
			grown := make([]string, width+7)
			for i := range grown {
				grown[i] = `{}`
				if i >= width {
					grown[i] = `{"marketplaceName":"new","message":"new"}`
				}
			}
			fields := `"errors":[` + strings.Join(initial, ",") + `],"errors":[{"marketplaceName":"shrink"}],"errors":[` + strings.Join(grown, ",") + `]`
			checkMarketplaceHistory(t, marketplaceBody(fields), false)
		})
	}
}

func TestMarketplaceErrorReceiverAndPriority(t *testing.T) {
	for _, fields := range []string{`"errors":[{}]`, `"errors":[null]`, `"errors":[{"marketplaceName":null,"message":""}]`, `"errors":[{"marketplaceName":"","message":null}]`} {
		before := p.MarketplaceUpgradeResponse{Errors: []p.MarketplaceUpgradeErrorInfo{{MarketplaceName: "prior", Message: "prior"}}, SelectedMarketplaces: []string{"prior"}, UpgradedRoots: []string{"/prior"}}
		actual := before
		if err := actual.UnmarshalJSON(marketplaceBody(fields)); err == nil || !reflect.DeepEqual(actual, before) || &actual.Errors[0] != &before.Errors[0] || &actual.UpgradedRoots[0] != &before.UpgradedRoots[0] {
			t.Fatal("failed record admission changed receiver or references")
		}
	}
	valid := `{"marketplaceName":"","message":""}`
	invalid := `{` + strings.Repeat(" ", len(valid)-2) + `}`
	for _, suffix := range []string{
		`,"selectedMarketplaces":[],"upgradedRoots":["relative"]}`,
		`,"selectedMarketplaces":[null],"upgradedRoots":[]}`,
		`,"selectedMarketplaces":42,"upgradedRoots":[]}`,
		`,"selectedMarketplaces":[]}`,
	} {
		t.Run(suffix, func(t *testing.T) {
			decode := func(item string) error {
				var r p.MarketplaceUpgradeResponse
				return r.UnmarshalJSON([]byte(`{"errors":[` + item + `]` + suffix))
			}
			expected, actual := decode(valid), decode(invalid)
			if expected == nil || actual == nil || !reflect.DeepEqual(expected, actual) {
				t.Fatalf("old error priority changed: %v vs %v", expected, actual)
			}
		})
	}
}

func TestMarketplaceErrorPlainRecord(t *testing.T) {
	for _, v := range []any{p.MarketplaceUpgradeErrorInfo{}, &p.MarketplaceUpgradeErrorInfo{}} {
		if _, ok := v.(json.Unmarshaler); ok {
			t.Fatal("plain error record gained decoder")
		}
		if _, ok := v.(json.Marshaler); ok {
			t.Fatal("plain error record gained encoder")
		}
	}
	var anonymous struct {
		p.MarketplaceUpgradeErrorInfo
		Extra string
	}
	if err := json.Unmarshal([]byte(`{"marketplaceName":"","message":"","Extra":"kept"}`), &anonymous); err != nil || anonymous.Extra != "kept" {
		t.Fatal("anonymous composition changed")
	}
	var named struct {
		Record p.MarketplaceUpgradeErrorInfo
		Extra  string
	}
	if err := json.Unmarshal([]byte(`{"Record":{},"Extra":"kept"}`), &named); err != nil || named.Extra != "kept" {
		t.Fatal("named plain record behavior changed")
	}
	raw := marketplaceBody(`"errors":[{"` + strings.Repeat("peer-", 20000) + `":null,"marketplaceName":null,"message":""}]`)
	var response p.MarketplaceUpgradeResponse
	if err := response.UnmarshalJSON(raw); err == nil || len(err.Error()) > 2048 || !strings.Contains(err.Error(), "errors[0].marketplaceName") {
		t.Fatal("unbounded or missing record diagnostic")
	}
}

func FuzzMarketplaceErrorHistory(f *testing.F) {
	for _, history := range [][]byte{{}, {13}, {5, 9}, {14, 5, 2}, {13, 0, 9}, {3}, {77}, {255}, {62, 5, 2}, {2, 13}, {125}} {
		f.Add(history)
	}
	f.Fuzz(func(t *testing.T, history []byte) {
		if len(history) > 12 {
			history = history[:12]
		}
		forceValid := len(history) > 0 && history[0]%2 == 0
		stickyNull := false
		fields := []string{`"errors":[]`}
		roots := []string{`"errors"`, `"ERRORS"`, `"err\u006frs"`, `"error\u017F"`}
		names := []string{`"marketplaceName"`, `"MARKETPLACENAME"`, `"marketplaceNa\u006de"`, `"mar\u212AetplaceName"`}
		messages := []string{`"message"`, `"MESSAGE"`, `"me\u0073sage"`, `"me\u017Fsage"`}
		for operation, b := range history {
			root := roots[int(b>>4)%len(roots)]
			if !forceValid && b == 255 {
				fields = append(fields, root+`:null`)
				stickyNull = true
				continue
			}
			count := int(b % 4)
			items := make([]string, 0, count)
			for index := 0; index < count; index++ {
				if !forceValid && b&128 != 0 && index == 0 {
					items = append(items, `null`)
					stickyNull = true
					continue
				}
				mask := (int(b>>2) + index) % 4
				if forceValid && operation == len(history)-1 {
					mask = 3
				}
				var members []string
				for fieldIndex := 0; fieldIndex < 2; fieldIndex++ {
					if mask&(1<<fieldIndex) == 0 {
						continue
					}
					value := ""
					if b%5 != 0 {
						value = fmt.Sprintf("value-%d-%d-%d", operation, index, fieldIndex)
					}
					encoded, _ := json.Marshal(value)
					raw := string(encoded)
					if !forceValid && b&64 != 0 && len(members) == 0 {
						raw = "null"
						stickyNull = true
					}
					key := names[(operation+index)%len(names)]
					if fieldIndex == 1 {
						key = messages[(operation+index)%len(messages)]
					}
					members = append(members, key+`:`+raw)
				}
				members = append(members, `"future":null`)
				items = append(items, `{`+strings.Join(members, ",")+`}`)
			}
			fields = append(fields, root+`:[`+strings.Join(items, ",")+`]`)
		}
		checkMarketplaceHistory(t, marketplaceBody(strings.Join(fields, ",")), stickyNull)
	})
}
