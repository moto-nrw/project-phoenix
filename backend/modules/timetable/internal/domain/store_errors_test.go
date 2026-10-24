package domain

import (
	"errors"
	"fmt"
	"testing"
)

type foreignNotFound struct{}

func (foreignNotFound) Error() string       { return "not found elsewhere" }
func (foreignNotFound) RepositoryNotFound() {}

func TestStoreErrorKeepsTheRetainedShape(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("load: %w", &StoreError{Op: "find by id", Err: ErrRecordNotFound})
	if got, want := errors.Unwrap(err).Error(), "database error during find by id: repository: not found"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if got := (&StoreError{Op: "list"}).Error(); got != "database error during list" {
		t.Fatalf("message without cause = %q", got)
	}
	var failure interface{ StoreFailure() bool }
	if !errors.As(err, &failure) || !failure.StoreFailure() {
		t.Fatal("StoreError does not mark a store failure")
	}
	if !IsRecordNotFound(err) || !IsRecordNotFound(fmt.Errorf("x: %w", foreignNotFound{})) {
		t.Fatal("IsRecordNotFound misses a wrapped not-found sentinel")
	}
	if IsRecordNotFound(errors.New("repository: not found")) || IsRecordNotFound(nil) {
		t.Fatal("IsRecordNotFound matches an error without the marker")
	}
}

func TestUniqueIDsKeepFirstOccurrenceOrder(t *testing.T) {
	t.Parallel()

	if got := UniquePositiveIDs([]int64{3, 0, 1, 3, -2, 1, 2}); fmt.Sprint(got) != "[3 1 2]" {
		t.Fatalf("UniquePositiveIDs = %v, want [3 1 2]", got)
	}
	if got := UniquePositiveIDs(nil); got != nil {
		t.Fatalf("UniquePositiveIDs(nil) = %v, want nil", got)
	}
	if got := UniquePositiveIDs([]int64{0, -1}); got == nil || len(got) != 0 {
		t.Fatalf("UniquePositiveIDs without positive IDs = %#v, want an empty slice", got)
	}
	if got := UniqueInOrder([]string{"b", "a", "b"}); fmt.Sprint(got) != "[b a]" {
		t.Fatalf("UniqueInOrder = %v, want [b a]", got)
	}
}
