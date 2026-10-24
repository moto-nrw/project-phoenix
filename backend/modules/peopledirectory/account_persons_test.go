package peopledirectory_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// An account's person at a school is the one updated last; a tie keeps the
// first, and a person without an account belongs to no account.
func TestLatestPersonsByAccountSchool(t *testing.T) {
	t.Parallel()

	account, other := int64(41), int64(42)
	earlier := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	persons := []peopledirectory.Person{
		{ID: 10, TenantID: 5, AccountID: &account, UpdatedAt: earlier},
		{ID: 11, TenantID: 5, AccountID: &account, UpdatedAt: later},
		{ID: 12, TenantID: 6, AccountID: &account, UpdatedAt: earlier},
		{ID: 13, TenantID: 6, AccountID: &account, UpdatedAt: earlier},
		{ID: 14, TenantID: 5, AccountID: &other, UpdatedAt: later},
		{ID: 15, TenantID: 5, UpdatedAt: later},
	}

	got := peopledirectory.LatestPersonsByAccountSchool(persons)
	want := map[peopledirectory.AccountSchool]int64{
		{AccountID: account, TenantID: 5}: 11,
		{AccountID: account, TenantID: 6}: 12,
		{AccountID: other, TenantID: 5}:   14,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d account schools, want %d: %v", len(got), len(want), got)
	}
	for key, id := range want {
		if got[key].ID != id {
			t.Errorf("%+v: person %d, want %d", key, got[key].ID, id)
		}
	}
}
