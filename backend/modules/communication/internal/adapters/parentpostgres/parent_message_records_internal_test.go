package parentpostgres

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func TestNativeMessageStoreSnapshot(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)
	const id int64 = 9007199254740993
	ref, actor := id+1, id+2
	message := &communication.PortalParentMessage{
		ID: id, TenantID: id + 3, CreatedAt: at, UpdatedAt: at.Add(time.Second), ThreadID: id + 4, StudentID: id + 5,
		SenderAccountID: id + 6, SenderKind: "staff", SenderName: "Test", StaffNameVisible: true, Body: "Antwort",
		Kind: "request", EventType: "request_status", EventActorKind: "guardian", RequestType: "pickup_change", RequestStatus: "erledigt",
		Payload:  map[string]json.RawMessage{"missing": json.RawMessage(`null`), "value": json.RawMessage(`{"enabled":true,"items":["a",2]}`)},
		RefTable: "schedule.care_schedule_change_requests", RefID: &ref, AppliedAt: &at, AppliedBy: &actor, DecisionReason: "begründet",
	}
	stored, err := messageValue(message)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := messageModel(stored)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(message, restored) {
		t.Fatalf("message snapshot changed: got %#v, want %#v", restored, message)
	}
	for _, payload := range []map[string]json.RawMessage{nil, {}, {"value": json.RawMessage(`1.00`)}} {
		message.Payload = payload
		stored, err = messageValue(message)
		if err != nil {
			t.Fatal(err)
		}
		restored, err = messageModel(stored)
		if err != nil {
			t.Fatal(err)
		}
		if (restored.Payload == nil) != (payload == nil) || len(restored.Payload) != len(payload) {
			t.Fatalf("payload presence changed: %#v -> %#v", payload, restored.Payload)
		}
		if len(payload) > 0 && string(restored.Payload["value"]) != "1.00" {
			t.Fatalf("numeric payload spelling changed through owner codec: %s", restored.Payload["value"])
		}
	}
	message.Payload = map[string]json.RawMessage{"broken": json.RawMessage(`{`)}
	if _, err := messageValue(message); err == nil {
		t.Fatal("invalid payload accepted at store boundary")
	}
	if got, err := messageModel(nil); got != nil || err != nil {
		t.Fatalf("nil snapshot changed: %v, %v", got, err)
	}
}
