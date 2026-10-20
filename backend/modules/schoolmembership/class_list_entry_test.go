package schoolmembership_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func TestClassListEntryErrorCodesAreStable(t *testing.T) {
	t.Parallel()
	if code := schoolmembership.ErrorCode(schoolmembership.ErrClassListEntryNotFound); code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
	wrapped := errors.Join(schoolmembership.ErrClassListEntryDuplicate, errors.New("driver detail"))
	if code := schoolmembership.ErrorCode(wrapped); code != "class_list_entry_conflict" {
		t.Fatalf("expected class_list_entry_conflict, got %q", code)
	}
}
