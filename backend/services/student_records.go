package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/active"
)

// presenceStudentSource is the slice of the student repository the presence
// adapter needs.
type presenceStudentSource interface {
	FindByID(context.Context, any) (*users.Student, error)
	FindByIDForUpdate(context.Context, int64) (*users.Student, error)
	FindByIDsForUpdate(context.Context, []int64) (map[int64]*users.Student, error)
	UpdateColumns(context.Context, *users.Student, ...string) (int64, error)
}

type presenceStudents struct{ source presenceStudentSource }

// PresenceStudents serves active.PresenceStudents from the users repository.
// The live-flag write touches only the four flag columns, so the departure
// plan and master data of the row stay untouched.
func PresenceStudents(source presenceStudentSource) active.PresenceStudents {
	return presenceStudents{source: source}
}

func (p presenceStudents) FindByID(ctx context.Context, id int64) (*active.StudentRecord, error) {
	row, err := p.source.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return studentRecord(row), nil
}

func (p presenceStudents) FindByIDForUpdate(ctx context.Context, id int64) (*active.StudentRecord, error) {
	row, err := p.source.FindByIDForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	return studentRecord(row), nil
}

func (p presenceStudents) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*active.StudentRecord, error) {
	rows, err := p.source.FindByIDsForUpdate(ctx, ids)
	if err != nil {
		return nil, err
	}
	records := make(map[int64]*active.StudentRecord, len(rows))
	for id, row := range rows {
		records[id] = studentRecord(row)
	}
	return records, nil
}

func (p presenceStudents) UpdateLiveStatus(ctx context.Context, record *active.StudentRecord) error {
	if record == nil {
		return nil
	}
	row := &users.Student{Sick: record.Sick, SickSince: record.SickSince, Excused: record.Excused, ExcusedSince: record.ExcusedSince}
	row.ID = record.ID
	row.TenantID = record.TenantID
	_, err := p.source.UpdateColumns(ctx, row, "sick", "sick_since", "excused", "excused_since")
	return err
}

// StudentLiveStatusUpdate copies the presence flags onto the owner's row so a
// caller holding the full row can persist them through the owner's write path.
func StudentLiveStatusUpdate(row *users.Student, record *active.StudentRecord) {
	if row == nil || record == nil {
		return
	}
	row.Sick = record.Sick
	row.SickSince = record.SickSince
	row.Excused = record.Excused
	row.ExcusedSince = record.ExcusedSince
}

// studentLifecycle maps the owner's lifecycle status onto the three states the
// presence flows branch on. Every other status, today only "pending", is
// StudentLifecycleOther and is treated as not-yet-active, which is what the
// enrollment filter and the check-in guards already did for it.
func studentLifecycle(status users.StudentStatus) active.StudentLifecycle {
	switch status {
	case users.StudentStatusActive:
		return active.StudentLifecycleActive
	case users.StudentStatusInactive:
		return active.StudentLifecycleInactive
	case users.StudentStatusAlumnus:
		return active.StudentLifecycleAlumnus
	case users.StudentStatusPending:
		return active.StudentLifecycleOther
	default:
		return active.StudentLifecycleOther
	}
}

func studentRecord(row *users.Student) *active.StudentRecord {
	if row == nil {
		return nil
	}
	return &active.StudentRecord{
		ID:            row.ID,
		TenantID:      row.TenantID,
		PersonID:      row.PersonID,
		GroupID:       row.GroupID,
		SchoolClass:   row.SchoolClass,
		Lifecycle:     studentLifecycle(row.Status),
		EnrolledFrom:  row.EnrolledFrom,
		EnrolledUntil: row.EnrolledUntil,
		Sick:          row.Sick,
		SickSince:     row.SickSince,
		Excused:       row.Excused,
		ExcusedSince:  row.ExcusedSince,
	}
}

// StudentRecord projects an owner row into the presence view. Inbound
// adapters that already hold the locked row use it for the status-day port.
func StudentRecord(row *users.Student) *active.StudentRecord {
	return studentRecord(row)
}

// statusDayOverviewSource is the slice of the person service the absence
// overview reads.
type statusDayOverviewSource interface {
	GetStudentsByGroupIDs(context.Context, []int64) ([]*users.Student, error)
	GetByIDs(context.Context, []int64) (map[int64]*users.Person, error)
}

type statusDayOverviewPeople struct{ source statusDayOverviewSource }

// StatusDayOverviewPeople serves the absence overview's people reads from the
// users service.
func StatusDayOverviewPeople(source statusDayOverviewSource) active.StatusDayOverviewPeople {
	return statusDayOverviewPeople{source: source}
}

func (p statusDayOverviewPeople) GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*active.StudentRecord, error) {
	rows, err := p.source.GetStudentsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	records := make([]*active.StudentRecord, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		records = append(records, studentRecord(row))
	}
	return records, nil
}

func (p statusDayOverviewPeople) GetByIDs(ctx context.Context, ids []int64) (map[int64]*active.PersonName, error) {
	rows, err := p.source.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]*active.PersonName, len(rows))
	for id, row := range rows {
		if row == nil {
			continue
		}
		names[id] = &active.PersonName{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName}
	}
	return names, nil
}
