package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func TestClassListEntryNamesAreComposed(t *testing.T) {
	t.Parallel()

	fields := schoolmembership.ClassListEntryFields{FirstName: " Zoe\u0308 ", LastName: "Go\u0301mez", SchoolClass: " 2a "}
	if err := validateClassListEntryFields(&fields); err != nil {
		t.Fatalf("validateClassListEntryFields: %v", err)
	}
	if fields.FirstName != "Zoë" || fields.LastName != "Gómez" || fields.SchoolClass != "2a" {
		t.Fatalf("fields = %+v", fields)
	}
}
