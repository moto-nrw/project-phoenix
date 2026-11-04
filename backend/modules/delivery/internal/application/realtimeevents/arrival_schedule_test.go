package realtimeevents

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

type arrivalSchedulePublisher struct{ calls int }

func (p *arrivalSchedulePublisher) BroadcastToTenant(_ int64, _ Event) error {
	p.calls++
	return nil
}

func TestQueueArrivalScheduleChangedWithoutTenantWarnsAndDoesNotPublish(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	publisher := &arrivalSchedulePublisher{}

	QueueArrivalScheduleChanged(context.Background(), publisher, logger)

	if publisher.calls != 0 {
		t.Fatalf("published %d events without a tenant", publisher.calls)
	}
	var entry struct {
		Level    string `json:"level"`
		Message  string `json:"msg"`
		TenantID *int64 `json:"tenant_id"`
	}
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("expected a structured warning, got %q: %v", logs.String(), err)
	}
	if entry.Level != "WARN" || entry.Message != "skipping arrival schedule change without valid tenant" {
		t.Fatalf("unexpected warning: %+v", entry)
	}
	if entry.TenantID == nil || *entry.TenantID != 0 {
		t.Fatalf("warning must identify the invalid tenant: %+v", entry)
	}
}
