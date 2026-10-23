package owners

import (
	"context"
	"strings"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

type staffListRecords interface {
	ListAllWithPerson(context.Context) ([]*peopleValues.Staff, error)
}

type staffNameRecords interface {
	FindWithPersonByIDs(context.Context, []int64) (map[int64]*peopleValues.Staff, error)
}

type monthCloseStaffQuery struct{ source staffListRecords }

// MonthCloseStaff lists the staff members a school-wide month close may
// cover; Workforce decides whom it closes.
func MonthCloseStaff(source staffListRecords) workforceCompose.MonthCloseStaffQuery {
	return monthCloseStaffQuery{source: source}
}

func (q monthCloseStaffQuery) ListMonthCloseStaff(ctx context.Context) ([]workforceModule.MonthCloseStaff, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	staff := make([]workforceModule.MonthCloseStaff, 0, len(rows))
	for _, row := range rows {
		staff = append(staff, workforceModule.MonthCloseStaff{ID: row.ID, ExternalCaregiver: row.IsExternalCaregiver})
	}
	return staff, nil
}

type overviewStaffQuery struct{ source staffListRecords }

// OverviewStaff lists the staff members the time-account overview shows.
func OverviewStaff(source staffListRecords) workforceCompose.OverviewStaffQuery {
	return overviewStaffQuery{source: source}
}

func (q overviewStaffQuery) ListOverviewStaff(ctx context.Context) ([]workforceCompose.OverviewStaff, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	staff := make([]workforceCompose.OverviewStaff, 0, len(rows))
	for _, row := range rows {
		member := workforceCompose.OverviewStaff{
			ID: row.ID, EmploymentType: row.EmploymentType, PersonnelNumber: row.PersonnelNumber,
			WorkTimeModelID: row.WorkTimeModelID, RotationAnchorDate: row.RotationAnchorDate, ExternalCaregiver: row.IsExternalCaregiver,
		}
		if row.Person != nil {
			member.FirstName = row.Person.FirstName
			member.LastName = row.Person.LastName
		}
		staff = append(staff, member)
	}
	return staff, nil
}

type timeExportStaffQuery struct{ source staffListRecords }

// TimeExportStaff lists the staff members the time exports cover.
func TimeExportStaff(source staffListRecords) workforceCompose.TimeExportStaffQuery {
	return timeExportStaffQuery{source: source}
}

func (q timeExportStaffQuery) ListExportStaff(ctx context.Context) ([]workforceCompose.TimeExportStaff, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	staff := make([]workforceCompose.TimeExportStaff, 0, len(rows))
	for _, row := range rows {
		member := workforceCompose.TimeExportStaff{ID: row.ID, ExternalCaregiver: row.IsExternalCaregiver}
		if row.Person != nil {
			member.FirstName = row.Person.FirstName
			member.LastName = row.Person.LastName
		}
		staff = append(staff, member)
	}
	return staff, nil
}

type staffDisplayNameQuery struct{ source staffNameRecords }

// StaffDisplayNames projects the tenant-scoped staff batch query for audit readers.
func StaffDisplayNames(source staffNameRecords) workforceCompose.StaffDisplayNameQuery {
	return staffDisplayNameQuery{source: source}
}

func (q staffDisplayNameQuery) StaffDisplayNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := q.source.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rows))
	for id, row := range rows {
		if row != nil && row.Person != nil {
			names[id] = strings.TrimSpace(row.Person.FirstName + " " + row.Person.LastName)
		}
	}
	return names, nil
}

type externalCaregiverStaff struct{ source staffNameRecords }

// externalCaregiverLookup answers whether a staff member is an external
// caregiver by School Membership's rule (#4573), which the person-hydrated
// staff rows apply: an active external caregiver record, whatever the account.
func externalCaregiverLookup(source staffNameRecords) externalCaregiverStaff {
	return externalCaregiverStaff{source: source}
}

func (q externalCaregiverStaff) IsExternalCaregiver(ctx context.Context, staffID int64) (bool, error) {
	rows, err := q.source.FindWithPersonByIDs(ctx, []int64{staffID})
	if err != nil {
		return false, err
	}
	row := rows[staffID]
	return row != nil && row.IsExternalCaregiver, nil
}

type workSessionStaffRecords interface {
	staffNameRecords
}

// staffScheduleBinder is School Membership's schedule binding command.
type staffScheduleBinder func(context.Context, schoolmembership.BindSchedule) (schoolmembership.Staff, error)

type workSessionStaff struct {
	source       workSessionStaffRecords
	schedules    workforceCompose.StaffEmploymentQuery
	bindSchedule staffScheduleBinder
}

// WorkSessionStaff reads names through the staff records and the binding
// from Workforce's employment query, and writes the schedule binding through
// School Membership's command.
func WorkSessionStaff(source workSessionStaffRecords, schedules workforceCompose.StaffEmploymentQuery, membership schoolmembership.StaffEmploymentCommands) workforceCompose.WorkSessionStaff {
	return workSessionStaff{source: source, schedules: schedules, bindSchedule: membership.BindSchedule}
}

func (q workSessionStaff) ScheduleAssignment(ctx context.Context, staffID int64) (*workforceCompose.StaffScheduleAssignment, error) {
	return workforceCompose.StaffScheduleAssignments(q.schedules).ScheduleAssignment(ctx, staffID)
}

func (q workSessionStaff) StaffNames(ctx context.Context, ids []int64) (map[int64]workforceCompose.WorkSessionStaffName, error) {
	rows, err := q.source.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]workforceCompose.WorkSessionStaffName, len(rows))
	for id, row := range rows {
		if row != nil && row.Person != nil {
			names[id] = workforceCompose.WorkSessionStaffName{FirstName: row.Person.FirstName, LastName: row.Person.LastName}
		}
	}
	return names, nil
}

// BindSchedule writes the schedule binding of a live staff member through
// School Membership; only the binding columns change.
func (q workSessionStaff) BindSchedule(ctx context.Context, binding workforceCompose.StaffScheduleBinding) error {
	_, err := q.bindSchedule(ctx, schoolmembership.BindSchedule{
		ID: binding.ID, WorkTimeModelID: binding.WorkTimeModelID, RotationAnchorDate: calendarDateString(binding.RotationAnchorDate),
	})
	return membershipError("update staff", err)
}
