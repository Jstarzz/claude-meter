package otel

import (
	"testing"
	"time"
)

func TestDecodeAPIRequest(t *testing.T) {
	payload := []byte(`{
      "resourceLogs":[{
        "resource":{"attributes":[
          {"key":"meter.person","value":{"stringValue":"brandon"}},
          {"key":"meter.device","value":{"stringValue":"brandons-macbook-pro"}},
          {"key":"user.account_uuid","value":{"stringValue":"acct-uuid-2"}},
          {"key":"user.email","value":{"stringValue":"brandon@example.com"}},
          {"key":"session.id","value":{"stringValue":"sess-1"}}
        ]},
        "scopeLogs":[{"logRecords":[{
          "timeUnixNano":"1787774400000000000",
          "attributes":[
            {"key":"event.name","value":{"stringValue":"api_request"}},
            {"key":"event.sequence","value":{"intValue":"9"}},
            {"key":"model","value":{"stringValue":"claude-sonnet-5"}},
            {"key":"request_id","value":{"stringValue":"req-123"}},
            {"key":"input_tokens","value":{"intValue":"12345"}},
            {"key":"output_tokens","value":{"intValue":"678"}},
            {"key":"cache_read_tokens","value":{"intValue":"90000"}},
            {"key":"cache_creation_tokens","value":{"intValue":"321"}},
            {"key":"cost_usd_micros","value":{"intValue":"481200"}},
            {"key":"duration_ms","value":{"intValue":"2400"}}
          ]
        }]}]
      }]
    }`)

	events, err := Decode(payload, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	e := events[0]
	if e.PersonID != "brandon" || e.DeviceName != "brandons-macbook-pro" {
		t.Fatalf("bad identity: %+v", e)
	}
	if e.AccountUUID != "acct-uuid-2" || e.AccountEmail != "brandon@example.com" {
		t.Fatalf("bad account: %+v", e)
	}
	if e.InputTokens != 12345 || e.OutputTokens != 678 || e.CacheReadTokens != 90000 {
		t.Fatalf("bad tokens: %+v", e)
	}
	if e.CostUSDMicros != 481200 {
		t.Fatalf("bad cost: %d", e.CostUSDMicros)
	}
	if e.EventKey == "" {
		t.Fatal("event key empty")
	}
}
