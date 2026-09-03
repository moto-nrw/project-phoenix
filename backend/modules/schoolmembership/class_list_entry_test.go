package schoolmembership_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func TestClassListEntryReadsRequirePositiveIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	_, err := module.FindClassListEntry(ctx, 0)
	requireInvalid(t, engine, err, "class list entry ID is required")

	module, engine = newModule()
	_, err = module.FindClassListEntryForMutation(ctx, -3)
	requireInvalid(t, engine, err, "class list entry ID is required")
}

func TestFindClassListEntryForMutationAsksForTheRowLock(t *testing.T) {
	t.Parallel()
	module, engine := newModule()

	if _, err := module.FindClassListEntry(context.Background(), 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.entryLock != "" {
		t.Fatalf("a plain read must not lock, got %q", engine.entryLock)
	}
	if _, err := module.FindClassListEntryForMutation(context.Background(), 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.entryLock != "UPDATE" {
		t.Fatalf("expected the UPDATE lock, got %q", engine.entryLock)
	}
}

func TestClassListEntryWritesTrimAndValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	module, engine := newModule()
	_, err := module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "  ", LastName: "Aalders", SchoolClass: "1a",
	}})
	requireInvalid(t, engine, err, "first name is required")

	module, engine = newModule()
	_, err = module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "\t", SchoolClass: "1a",
	}})
	requireInvalid(t, engine, err, "last name is required")

	module, engine = newModule()
	_, err = module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders",
	}})
	requireInvalid(t, engine, err, "school class is required")

	module, engine = newModule()
	_, err = module.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a",
	}})
	requireInvalid(t, engine, err, "class list entry ID is required")

	module, engine = newModule()
	requireInvalid(t, engine, module.DeleteClassListEntry(ctx, 0), "class list entry ID is required")

	module, engine = newModule()
	zeroActor := int64(0)
	created, err := module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: " Zoe ", LastName: " Aalders ", SchoolClass: " 1a "},
		CreatedBy:            &zeroActor,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.FirstName != "Zoe" {
		t.Fatalf("the engine must receive trimmed fields, got %q", created.FirstName)
	}
	if engine.createdEntry.LastName != "Aalders" || engine.createdEntry.SchoolClass != "1a" {
		t.Fatalf("expected trimmed fields, got %+v", engine.createdEntry.ClassListEntryFields)
	}
	if engine.createdEntry.CreatedBy != nil {
		t.Fatalf("a zero actor means no creating account, got %v", *engine.createdEntry.CreatedBy)
	}

	module, engine = newModule()
	if _, err := module.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{ID: 9, ClassListEntryFields: schoolmembership.ClassListEntryFields{
		FirstName: "Zoe", LastName: "Aalders ", SchoolClass: " 1b",
	}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.updatedEntry.ID != 9 || engine.updatedEntry.SchoolClass != "1b" || engine.updatedEntry.LastName != "Aalders" {
		t.Fatalf("expected the trimmed update, got %+v", engine.updatedEntry)
	}
}

func TestListClassListEntriesTrimsFiltersAndDeduplicatesIDs(t *testing.T) {
	t.Parallel()
	module, engine := newModule()

	if _, err := module.ListClassListEntries(context.Background(), schoolmembership.ClassListEntryFilter{
		IDs: []int64{3, 0, 3, -1, 5}, FirstName: " Zoe ", LastName: " Aalders", SchoolClass: "1a ",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filter := engine.entryFilter
	if len(filter.IDs) != 2 || filter.IDs[0] != 3 || filter.IDs[1] != 5 {
		t.Fatalf("expected deduplicated positive IDs, got %v", filter.IDs)
	}
	if filter.FirstName != "Zoe" || filter.LastName != "Aalders" || filter.SchoolClass != "1a" {
		t.Fatalf("expected trimmed matches, got %+v", filter)
	}
}

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
