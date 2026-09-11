package repositories

import (
	"context"
	"fmt"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// staffGroupSupervisorRepository attaches the supervising staff member to
// active-group supervisions. The replaced LEFT JOIN carried no soft-delete
// filter, so an offboarded colleague keeps resolving here.
type staffGroupSupervisorRepository struct {
	activeModels.GroupSupervisorRepository
	membership staffLookup
}

func (r staffGroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*activeModels.GroupSupervisor, error) {
	rows, err := r.GroupSupervisorRepository.FindByActiveGroupID(ctx, activeGroupID, activeOnly)
	if err != nil {
		return nil, err
	}
	return rows, r.attachStaff(ctx, rows)
}

func (r staffGroupSupervisorRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*activeModels.GroupSupervisor, error) {
	rows, err := r.GroupSupervisorRepository.FindByActiveGroupIDs(ctx, activeGroupIDs, activeOnly)
	if err != nil {
		return nil, err
	}
	return rows, r.attachStaff(ctx, rows)
}

func (r staffGroupSupervisorRepository) attachStaff(ctx context.Context, rows []*activeModels.GroupSupervisor) error {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			ids = append(ids, row.StaffID)
		}
	}
	members, err := staffByID(ctx, r.membership, ids, true)
	if err != nil {
		return err
	}
	// A supervision whose staff row is gone keeps a nil Staff, which is what
	// the previous LEFT JOIN produced.
	for _, row := range rows {
		if row == nil {
			continue
		}
		if member, found := members[row.StaffID]; found {
			row.Staff = toLegacyStaff(member)
		}
	}
	return nil
}

// absenceRequestRowsQuery is the staff-ID-shaped listing the concrete
// repository exposes; subjectStaffIDs nil means "no subject filter".
type absenceRequestRowsQuery interface {
	ListRequestRows(ctx context.Context, filter activeModels.AbsenceRequestFilter, subjectStaffIDs []int64) ([]*activeModels.AbsenceRequestRow, error)
}

// staffAbsenceRepository translates between the person-shaped absence
// request contract and the staff IDs the rows actually carry: it maps
// filter.SubjectPersonIDs to staff IDs before the query and fills
// SubjectPersonID / DeciderPersonID afterwards. Soft-deleted staff are
// included so a request stays readable after its subject is offboarded —
// the replaced joins were LEFT joins without a deleted filter.
type staffAbsenceRepository struct {
	activeModels.StaffAbsenceRepository
	rows       absenceRequestRowsQuery
	membership staffLookup
}

func newStaffAbsenceRepository(inner activeModels.StaffAbsenceRepository, membership staffLookup) activeModels.StaffAbsenceRepository {
	rows, _ := inner.(absenceRequestRowsQuery)
	return staffAbsenceRepository{StaffAbsenceRepository: inner, rows: rows, membership: membership}
}

func (r staffAbsenceRepository) ListRequests(ctx context.Context, filter activeModels.AbsenceRequestFilter) ([]*activeModels.AbsenceRequestRow, error) {
	if r.rows == nil {
		return nil, fmt.Errorf("staff absence repository does not list request rows")
	}
	var subjectStaffIDs []int64
	if filter.SubjectPersonIDs != nil {
		// An explicitly empty person set matches nobody; never fall through
		// to the unfiltered listing.
		if len(filter.SubjectPersonIDs) == 0 {
			return []*activeModels.AbsenceRequestRow{}, nil
		}
		members, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{
			PersonIDs: filter.SubjectPersonIDs, IncludeDeleted: true,
		})
		if err != nil {
			return nil, fmt.Errorf("load staff for absence subjects: %w", err)
		}
		if len(members) == 0 {
			return []*activeModels.AbsenceRequestRow{}, nil
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
	members, err := staffByID(ctx, r.membership, ids, true)
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
