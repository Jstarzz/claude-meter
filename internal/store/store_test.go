package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jstarzz/claude-meter/internal/otel"
)

func TestInsertAndAggregate(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC()
	e := otel.Event{
		EventKey: "one", EventName: "api_request", EventTime: now, EventSequence: 1,
		PersonID: "developer-a", DeviceName: "workstation-a", AccountUUID: "acct1", AccountEmail: "dev-a@example.com",
		SessionID: "s1", Model: "claude-sonnet-5", InputTokens: 100, OutputTokens: 20, CacheReadTokens: 500, CostUSDMicros: 123456,
	}
	n, err := st.Insert(context.Background(), []otel.Event{e, e})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("dedupe: inserted %d, want 1", n)
	}
	totals, err := st.Totals(context.Background(), nil, "developer-a")
	if err != nil {
		t.Fatal(err)
	}
	if totals.Requests != 1 || totals.InputTokens != 100 || totals.CostUSDMicros != 123456 {
		t.Fatalf("bad totals: %+v", totals)
	}
	accounts, err := st.ByAccount(context.Background(), nil, "developer-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].AccountUUID != "acct1" {
		t.Fatalf("bad accounts: %+v", accounts)
	}
}
