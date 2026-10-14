package api

import (
	"context"
	"fmt"
	"strings"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// The time-tracking services read the staff members and their names from
// the retained People Directory staff rows; the adapters below serve their
// consumer-owned ports from those rows.

type staffListRecords interface {
	ListAllWithPerson(context.Context) ([]*users.Staff, error)
}

type staffNameRecords interface {
	FindWithPersonByIDs(context.Context, []int64) (map[int64]*users.Staff, error)
}

type monthCloseStaffQuery struct{ source staffListRecords }

// MonthCloseStaff preserves the staff selection used by school-wide month close.
func MonthCloseStaff(source staffListRecords) workforceCompose.MonthCloseStaffQuery {
	return monthCloseStaffQuery{source: source}
}

func (q monthCloseStaffQuery) ListStaffIDs(ctx context.Context) ([]int64, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		// External caregivers (#3823) record no working time to close.
		if !row.IsGuest {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
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
		// External caregivers (#3823) record no working time.
		if row.IsGuest {
			continue
		}
		member := workforceCompose.OverviewStaff{
			ID: row.ID, EmploymentType: row.EmploymentType, PersonnelNumber: row.PersonnelNumber,
			WorkTimeModelID: row.WorkTimeModelID, RotationAnchorDate: row.RotationAnchorDate,
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
		// External caregivers (#3823) record no working time.
		if row.IsGuest {
			continue
		}
		member := workforceCompose.TimeExportStaff{ID: row.ID}
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

type workSessionStaffRecords interface {
	staffNameRecords
	FindByID(context.Context, any) (*users.Staff, error)
	Update(context.Context, *users.Staff) error
}

type workSessionStaff struct {
	source    workSessionStaffRecords
	schedules workforceCompose.StaffEmploymentQuery
}

// WorkSessionStaff reads names and writes the schedule binding through the
// staff records, and reads the binding from Workforce's employment query.
func WorkSessionStaff(source workSessionStaffRecords, schedules workforceCompose.StaffEmploymentQuery) workforceCompose.WorkSessionStaff {
	return workSessionStaff{source: source, schedules: schedules}
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

// BindSchedule writes the schedule binding onto the staff row through the
// owner's update path: the row is re-read so the write carries the current
// master data and only the binding columns change.
func (q workSessionStaff) BindSchedule(ctx context.Context, binding workforceCompose.StaffScheduleBinding) error {
	staff, err := q.source.FindByID(ctx, binding.ID)
	if err != nil {
		return err
	}
	if staff == nil {
		return fmt.Errorf("bind schedule: staff %d not found", binding.ID)
	}
	staff.WorkTimeModelID = binding.WorkTimeModelID
	staff.RotationAnchorDate = binding.RotationAnchorDate
	return q.source.Update(ctx, staff)
}
