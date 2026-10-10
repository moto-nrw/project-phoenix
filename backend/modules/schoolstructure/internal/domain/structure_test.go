package domain

import (
	"errors"
	"testing"
)

func TestGroupValidateRequiresAndTrimsTheName(t *testing.T) {
	t.Parallel()

	roomID := int64(41)
	for _, tt := range []struct {
		name    string
		group   Group
		wantErr bool
	}{
		{"valid group", Group{Name: "Class 1A"}, false},
		{"valid group with room", Group{Name: "Class 2B", RoomID: &roomID}, false},
		{"empty name", Group{Name: ""}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			group := tt.group
			if err := group.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	group := Group{Name: "  Class 1A  "}
	if err := group.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error = %v", err)
	}
	if group.Name != "Class 1A" {
		t.Fatalf("Name = %q, want Class 1A", group.Name)
	}
}

func TestIsRecordNotFoundRecognisesTheMarker(t *testing.T) {
	t.Parallel()

	if !IsRecordNotFound(&StoreError{Op: "find", Err: errors.Join(RecordNotFound, errors.New("no rows"))}) {
		t.Fatal("a wrapped RecordNotFound must classify as not found")
	}
	if IsRecordNotFound(&StoreError{Op: "find"}) {
		t.Fatal("a store error without a missing row is not a not-found")
	}
	if !(&StoreError{Op: "find"}).StoreFailure() {
		t.Fatal("a store error must mark itself as the store's failure")
	}
}
