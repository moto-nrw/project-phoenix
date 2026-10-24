package users

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPersonRFIDCardJSON(t *testing.T) {
	t.Parallel()
	stamp := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	card := PersonRFIDCard{ID: "ABCD1234", CreatedAt: stamp, UpdatedAt: stamp, TenantID: 42, Active: false}
	data, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	const expected = `{"id":"ABCD1234","created_at":"2025-01-02T03:04:05Z","updated_at":"2025-01-02T03:04:05Z","tenant_id":42,"active":false}`
	if string(data) != expected {
		t.Fatalf("card metadata JSON changed: got %s, want %s", data, expected)
	}
}
