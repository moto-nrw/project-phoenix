package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// The audited class-list administration (#2382). The two capabilities it
// depends on — the People Directory student lookup and the Audit append
// command — are handed in by the composition root after this module exists,
// so they are adapted onto the internal ports here rather than in New.

func (e engine) ListClassListEntriesInDisplayOrder(ctx context.Context, filter schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, error) {
	values, err := e.service.ListClassListEntriesInDisplayOrder(ctx, classListEntryFilterToDomain(filter))
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolmembership.ClassListEntry, 0, len(values))
	for _, value := range values {
		result = append(result, classListEntryToPublic(value))
	}
	return result, nil
}

func (e engine) MatchingStudentIDs(ctx context.Context, fields schoolmembership.ClassListEntryFields) ([]int64, error) {
	ids, err := e.service.MatchingStudentIDs(ctx, classListEntryFieldsToDomain(fields))
	return ids, mapError(err)
}

func (e engine) AddClassListEntry(ctx context.Context, input schoolmembership.AddClassListEntry) (schoolmembership.ClassListEntry, error) {
	value, err := e.service.AddClassListEntry(ctx, classListEntryFieldsToDomain(input.ClassListEntryFields), input.ChangedBy)
	return classListEntryToPublic(value), mapError(err)
}

func (e engine) ReviseClassListEntry(ctx context.Context, input schoolmembership.ReviseClassListEntry) (schoolmembership.ClassListEntry, error) {
	value, err := e.service.ReviseClassListEntry(ctx, input.ID, classListEntryFieldsToDomain(input.ClassListEntryFields), input.ChangedBy)
	return classListEntryToPublic(value), mapError(err)
}

func (e engine) RemoveClassListEntry(ctx context.Context, input schoolmembership.RemoveClassListEntry) error {
	return mapError(e.service.RemoveClassListEntry(ctx, input.ID, input.ChangedBy))
}

func (e engine) ResolveClassListEntry(ctx context.Context, input schoolmembership.ResolveClassListEntry) error {
	return mapError(e.service.ResolveClassListEntry(ctx, input.ID, input.StudentID, input.ChangedBy))
}

func (e engine) BindClassListEntryAdministration(students schoolmembership.ClassListEntryStudents, trail schoolmembership.ClassListEntryTrail) {
	e.service.BindClassListEntryAdministration(classListStudents{students}, classListTrail{trail})
}

// classListStudents passes the public student port straight through: it
// already speaks plain IDs and names, so nothing has to be mapped.
type classListStudents struct {
	schoolmembership.ClassListEntryStudents
}

// classListTrail maps the module's internal change onto the public one the
// composition root's Audit command accepts.
type classListTrail struct {
	trail schoolmembership.ClassListEntryTrail
}

func (t classListTrail) AppendClassListEntryChange(ctx context.Context, change domain.ClassListEntryChange) error {
	return t.trail.AppendClassListEntryChange(ctx, schoolmembership.ClassListEntryChange{
		EntryID: change.EntryID, Action: change.Action, OldValue: change.OldValue,
		NewValue: change.NewValue, MatchedStudentID: change.MatchedStudentID, ChangedBy: change.ChangedBy,
	})
}

func classListEntryFilterToDomain(filter schoolmembership.ClassListEntryFilter) domain.ClassListEntryFilter {
	return domain.ClassListEntryFilter{
		IDs: filter.IDs, FirstName: filter.FirstName, LastName: filter.LastName, SchoolClass: filter.SchoolClass,
	}
}
