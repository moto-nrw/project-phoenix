package services

import (
	"context"
	"errors"
	"sort"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// The class-list entry HTTP adapter (#2668) reads through the School
// Membership capability but may not import the legacy services. The audited
// write flows, the student-match hint and the display order still live with
// users.ClassListEntryService; they are composed here with plain types so
// the HTTP root can bind the adapter without importing any service package.

// ClassListEntryInput carries the three fields an entry consists of.
type ClassListEntryInput struct {
	FirstName   string
	LastName    string
	SchoolClass string
}

// ClassListEntryRuntime is the legacy-service side of the class-list entry
// HTTP adapter. Every closure keeps the exact semantics of the handler code
// it replaced in api/classlistentries.
type ClassListEntryRuntime struct {
	Order              func([]schoolmembership.ClassListEntry)
	MatchingStudentIDs func(context.Context, ClassListEntryInput) ([]int64, error)
	Create             func(context.Context, ClassListEntryInput, int64) (schoolmembership.ClassListEntry, error)
	Update             func(context.Context, int64, ClassListEntryInput, int64) (schoolmembership.ClassListEntry, error)
	Delete             func(context.Context, int64, int64) error
	Assign             func(ctx context.Context, entryID, studentID, actorID int64) error
}

// NewClassListEntryRuntime composes the closures over the service factory.
func (f *Factory) NewClassListEntryRuntime() ClassListEntryRuntime {
	service := f.ClassListEntries
	return ClassListEntryRuntime{
		Order: SortClassListEntries,
		MatchingStudentIDs: func(ctx context.Context, input ClassListEntryInput) ([]int64, error) {
			return service.MatchingStudentIDs(ctx, classListEntryInput(input))
		},
		Create: func(ctx context.Context, input ClassListEntryInput, actorID int64) (schoolmembership.ClassListEntry, error) {
			entry, err := service.Create(ctx, classListEntryInput(input), actorID)
			return classListEntryToPublic(entry), err
		},
		Update: func(ctx context.Context, id int64, input ClassListEntryInput, actorID int64) (schoolmembership.ClassListEntry, error) {
			entry, err := service.Update(ctx, id, classListEntryInput(input), actorID)
			return classListEntryToPublic(entry), err
		},
		Delete: service.Delete,
		Assign: service.Assign,
	}
}

// SortClassListEntries orders a listing the way the class list reads it:
// class (grade-aware), then last and first name with German collation.
func SortClassListEntries(entries []schoolmembership.ClassListEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return usersSvc.CompareClassListEntryOrder(
			entries[i].SchoolClass, entries[i].LastName, entries[i].FirstName, entries[i].ID,
			entries[j].SchoolClass, entries[j].LastName, entries[j].FirstName, entries[j].ID,
		) < 0
	})
}

// ClassifyClassListEntryFailure maps the service sentinels onto the HTTP
// outcome the legacy handlers produced: unknown entry -> 404, the four
// German conflict messages -> 400, everything else -> 500.
func ClassifyClassListEntryFailure(err error) (StaffFailureKind, error) {
	switch {
	case errors.Is(err, usersSvc.ErrClassListEntryNotFound):
		return StaffFailureNotFound, err
	case errors.Is(err, usersSvc.ErrClassListEntryDuplicate),
		errors.Is(err, usersSvc.ErrClassListEntryStudentExists),
		errors.Is(err, usersSvc.ErrClassListEntryStudentNotFound),
		errors.Is(err, usersSvc.ErrClassListEntryAssignMismatch):
		return StaffFailureInvalidRequest, err
	default:
		return StaffFailureInternal, err
	}
}

func classListEntryInput(input ClassListEntryInput) usersSvc.ClassListEntryInput {
	return usersSvc.ClassListEntryInput{FirstName: input.FirstName, LastName: input.LastName, SchoolClass: input.SchoolClass}
}

func classListEntryToPublic(entry *userModels.ClassListEntry) schoolmembership.ClassListEntry {
	if entry == nil {
		return schoolmembership.ClassListEntry{}
	}
	return schoolmembership.ClassListEntry{
		ID: entry.ID, TenantID: entry.GetTenantID(), CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
		FirstName: entry.FirstName, LastName: entry.LastName, SchoolClass: entry.SchoolClass, CreatedBy: entry.CreatedBy,
	}
}
