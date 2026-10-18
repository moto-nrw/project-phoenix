package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// NewStaffAbsenceRepository is the Workforce staff absence repository with
// the request rows translated between staff and person IDs through School
// Membership and the subject and decider names resolved through the People
// Directory (#2667, #2690).
func NewStaffAbsenceRepository(workTime workforceModule.Capability, membership schoolmembership.Query, persons peopledirectory.Query) workforceCompose.StaffAbsenceRepository {
	inner := workforceCompose.NewStaffAbsenceRepository(workTime)
	rows, _ := inner.(absenceRequestRowsQuery)
	return personStaffAbsenceRepository{
		StaffAbsenceRepository: staffAbsenceRepository{StaffAbsenceRepository: inner, rows: rows, membership: membership},
		persons:                persons,
	}
}

// absenceRequestRowsQuery is the staff-ID-shaped listing the concrete
// repository exposes; subjectStaffIDs nil means "no subject filter".
type absenceRequestRowsQuery interface {
	ListRequestRows(ctx context.Context, filter workforceCompose.AbsenceRequestFilter, subjectStaffIDs []int64) ([]*workforceCompose.AbsenceRequestRow, error)
}

// staffMemberQuery is the slice of the School Membership query the absence
// rows read: who a staff ID or a person is.
type staffMemberQuery interface {
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
}

// staffAbsenceRepository translates between the person-shaped absence
// request contract and the staff IDs the rows actually carry: it maps
// filter.SubjectPersonIDs to staff IDs before the query and fills
// SubjectPersonID / DeciderPersonID afterwards. Soft-deleted staff are
// included so a request stays readable after its subject is offboarded —
// the replaced joins were LEFT joins without a deleted filter.
type staffAbsenceRepository struct {
	workforceCompose.StaffAbsenceRepository
	rows       absenceRequestRowsQuery
	membership staffMemberQuery
}

func (r staffAbsenceRepository) ListRequests(ctx context.Context, filter workforceCompose.AbsenceRequestFilter) ([]*workforceCompose.AbsenceRequestRow, error) {
	if r.rows == nil {
		return nil, fmt.Errorf("staff absence repository does not list request rows")
	}
	subjectStaffIDs, matchesNobody, err := r.subjectStaffIDs(ctx, filter.SubjectPersonIDs)
	if err != nil {
		return nil, err
	}
	if matchesNobody {
		return []*workforceCompose.AbsenceRequestRow{}, nil
	}
	filter.SubjectPersonIDs = nil
	rows, err := r.rows.ListRequestRows(ctx, filter, subjectStaffIDs)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	if err := r.attachPersonIDs(ctx, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// subjectStaffIDs maps the subject persons to their staff IDs by the
// owner's subject rule.
func (r staffAbsenceRepository) subjectStaffIDs(ctx context.Context, personIDs []int64) ([]int64, bool, error) {
	return workforceModule.AbsenceSubjectStaffIDs(personIDs, func(personIDs []int64) ([]int64, error) {
		members, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs, IncludeDeleted: workforceModule.AbsenceRequestsIncludeOffboardedStaff})
		if err != nil {
			return nil, fmt.Errorf("load staff for absence subjects: %w", err)
		}
		ids := make([]int64, 0, len(members))
		for _, member := range members {
			ids = append(ids, member.ID)
		}
		return ids, nil
	})
}

// attachPersonIDs fills SubjectPersonID and DeciderPersonID from the staff
// members the rows name.
func (r staffAbsenceRepository) attachPersonIDs(ctx context.Context, rows []*workforceCompose.AbsenceRequestRow) error {
	ids := make([]int64, 0, 2*len(rows))
	for _, row := range rows {
		ids = append(ids, row.StaffID)
		if row.ApprovedBy != nil {
			ids = append(ids, *row.ApprovedBy)
		}
	}
	members, err := staffMembersByID(ctx, r.membership, ids)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if personID, found := memberPersonID(members, row.StaffID); found {
			row.SubjectPersonID = personID
		}
		if row.ApprovedBy == nil {
			continue
		}
		if personID, found := memberPersonID(members, *row.ApprovedBy); found {
			row.DeciderPersonID = personID
		}
	}
	return nil
}

// memberPersonID is the person behind a known staff member.
func memberPersonID(members map[int64]schoolmembership.Staff, staffID int64) (*int64, bool) {
	member, found := members[staffID]
	if !found {
		return nil, false
	}
	personID := member.PersonID
	return &personID, true
}

// personStaffAbsenceRepository resolves the free-text subject search and the
// subject and decider names of the request rows through the People Directory.
type personStaffAbsenceRepository struct {
	workforceCompose.StaffAbsenceRepository
	persons peopledirectory.Query
}

func (r personStaffAbsenceRepository) ListRequests(ctx context.Context, filter workforceCompose.AbsenceRequestFilter) ([]*workforceCompose.AbsenceRequestRow, error) {
	if search := workforceModule.AbsenceSubjectSearch(filter.Search, filter.SubjectPersonIDs); search != "" {
		matches, err := r.persons.SearchPersons(ctx, peopledirectory.PersonFilter{FullNameContains: search})
		if err != nil {
			return nil, err
		}
		filter.SubjectPersonIDs = make([]int64, 0, len(matches))
		for _, match := range matches {
			filter.SubjectPersonIDs = append(filter.SubjectPersonIDs, match.ID)
		}
	}
	rows, err := r.StaffAbsenceRepository.ListRequests(ctx, filter)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, 2*len(rows))
	for _, row := range rows {
		if row.SubjectPersonID != nil {
			ids = append(ids, *row.SubjectPersonID)
		}
		if row.DeciderPersonID != nil {
			ids = append(ids, *row.DeciderPersonID)
		}
	}
	persons, err := absencePersonsByID(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.StaffName = absenceFullName(persons, row.SubjectPersonID)
		row.DecidedByName = absenceFullName(persons, row.DeciderPersonID)
	}
	return rows, nil
}

// staffMembersByID resolves the staff members for ids, offboarded ones
// included, keyed by staff ID.
func staffMembersByID(ctx context.Context, query staffMemberQuery, ids []int64) (map[int64]schoolmembership.Staff, error) {
	unique := positiveUniqueIDs(ids)
	if len(unique) == 0 {
		return map[int64]schoolmembership.Staff{}, nil
	}
	values, err := query.ListStaff(ctx, schoolmembership.StaffFilter{IDs: unique, IncludeDeleted: workforceModule.AbsenceRequestsIncludeOffboardedStaff})
	if err != nil {
		return nil, fmt.Errorf("load staff: %w", err)
	}
	result := make(map[int64]schoolmembership.Staff, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}

// absencePersonsByID resolves the non-deleted persons for ids, keyed by
// person ID.
func absencePersonsByID(ctx context.Context, query peopledirectory.Query, ids []int64) (map[int64]peopledirectory.Person, error) {
	unique := positiveUniqueIDs(ids)
	if len(unique) == 0 {
		return map[int64]peopledirectory.Person{}, nil
	}
	values, err := query.ListPersonsByID(ctx, unique)
	if err != nil {
		return nil, fmt.Errorf("load persons: %w", err)
	}
	result := make(map[int64]peopledirectory.Person, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}

func absenceFullName(persons map[int64]peopledirectory.Person, id *int64) string {
	if id == nil {
		return ""
	}
	person, found := persons[*id]
	if !found {
		return ""
	}
	return person.FullName()
}

// positiveUniqueIDs drops non-positive IDs and duplicates, keeping order.
func positiveUniqueIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
