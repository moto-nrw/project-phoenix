package services

import (
	"context"
	"fmt"

	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"

	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// staffGroupSupervisorRepository attaches the supervising staff member to
// active-group supervisions. The replaced LEFT JOIN carried no soft-delete
// filter, so an offboarded colleague keeps resolving here.
func sessionStaff(member schoolmembership.Staff) *presenceCompose.SessionStaff {
	return &presenceCompose.SessionStaff{
		ID: member.ID, TenantID: member.TenantID, CreatedAt: member.CreatedAt, UpdatedAt: member.UpdatedAt,
		PersonID: member.PersonID, StaffNotes: member.StaffNotes, EmploymentType: member.EmploymentType,
		WorkTimeModelID: member.WorkTimeModelID, RotationAnchorDate: peopleCompose.ParseRetainedCalendarDate(member.RotationAnchorDate),
		BirthdayDisplayOptOut: member.BirthdayDisplayOptOut,
	}
}

// StaffAbsenceRow and AbsenceRequestFilter name the Workforce rows the staff
// absence decorators below read and write, for the suites that test them.
// They go when the decorators move into the root (#4008, #4007).
type (
	StaffAbsenceRow      = workforceCompose.StaffAbsence
	AbsenceRequestFilter = workforceCompose.AbsenceRequestFilter
)

// absenceRequestRowsQuery is the staff-ID-shaped listing the concrete
// repository exposes; subjectStaffIDs nil means "no subject filter".
type absenceRequestRowsQuery interface {
	ListRequestRows(ctx context.Context, filter workforceCompose.AbsenceRequestFilter, subjectStaffIDs []int64) ([]*workforceCompose.AbsenceRequestRow, error)
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
	membership schoolMembershipCompose.StaffLookup
}

// NewStaffAbsenceRepository is the Workforce staff absence repository with
// the request rows translated between staff and person IDs through School
// Membership and the subject and decider names resolved through the People
// Directory (#2667, #2690).
func NewStaffAbsenceRepository(workTime workforce.Capability, membership schoolmembership.Query, persons peopledirectory.Query) workforceCompose.StaffAbsenceRepository {
	return personStaffAbsenceRepository{
		StaffAbsenceRepository: newStaffAbsenceRepository(workforceCompose.NewStaffAbsenceRepository(workTime), membership),
		persons:                persons,
	}
}

func newStaffAbsenceRepository(inner workforceCompose.StaffAbsenceRepository, membership schoolMembershipCompose.StaffLookup) workforceCompose.StaffAbsenceRepository {
	rows, _ := inner.(absenceRequestRowsQuery)
	return staffAbsenceRepository{StaffAbsenceRepository: inner, rows: rows, membership: membership}
}

func (r staffAbsenceRepository) ListRequests(ctx context.Context, filter workforceCompose.AbsenceRequestFilter) ([]*workforceCompose.AbsenceRequestRow, error) {
	if r.rows == nil {
		return nil, fmt.Errorf("staff absence repository does not list request rows")
	}
	var subjectStaffIDs []int64
	if filter.SubjectPersonIDs != nil {
		// An explicitly empty person set matches nobody; never fall through
		// to the unfiltered listing.
		if len(filter.SubjectPersonIDs) == 0 {
			return []*workforceCompose.AbsenceRequestRow{}, nil
		}
		members, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{
			PersonIDs: filter.SubjectPersonIDs, IncludeDeleted: true,
		})
		if err != nil {
			return nil, fmt.Errorf("load staff for absence subjects: %w", err)
		}
		if len(members) == 0 {
			return []*workforceCompose.AbsenceRequestRow{}, nil
		}
		subjectStaffIDs = make([]int64, 0, len(members))
		for _, member := range members {
			subjectStaffIDs = append(subjectStaffIDs, member.ID)
		}
	}
	filter.SubjectPersonIDs = nil
	rows, err := r.rows.ListRequestRows(ctx, filter, subjectStaffIDs)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, 2*len(rows))
	for _, row := range rows {
		ids = append(ids, row.StaffID)
		if row.ApprovedBy != nil {
			ids = append(ids, *row.ApprovedBy)
		}
	}
	members, err := schoolMembershipCompose.StaffByID(ctx, r.membership, ids, true)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if member, found := members[row.StaffID]; found {
			personID := member.PersonID
			row.SubjectPersonID = &personID
		}
		if row.ApprovedBy == nil {
			continue
		}
		if member, found := members[*row.ApprovedBy]; found {
			personID := member.PersonID
			row.DeciderPersonID = &personID
		}
	}
	return rows, nil
}

type personStaffAbsenceRepository struct {
	workforceCompose.StaffAbsenceRepository
	persons peopledirectory.Query
}

func (r personStaffAbsenceRepository) ListRequests(ctx context.Context, filter workforceCompose.AbsenceRequestFilter) ([]*workforceCompose.AbsenceRequestRow, error) {
	if search := strings.TrimSpace(filter.Search); search != "" && filter.SubjectPersonIDs == nil {
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
	persons, err := peopleCompose.PersonsByID(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.StaffName = fullNameOf(persons, row.SubjectPersonID)
		row.DecidedByName = fullNameOf(persons, row.DeciderPersonID)
	}
	return rows, nil
}

func fullNameOf(persons map[int64]peopledirectory.Person, id *int64) string {
	if id == nil {
		return ""
	}
	person, found := persons[*id]
	if !found {
		return ""
	}
	return person.FullName()
}
