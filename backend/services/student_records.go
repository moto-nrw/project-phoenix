package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

// presenceStudentSource is the slice of the student repository the presence
// adapter needs.
type presenceStudentSource interface {
	FindByID(context.Context, any) (*users.Student, error)
	FindByIDForUpdate(context.Context, int64) (*users.Student, error)
	FindByIDsForUpdate(context.Context, []int64) (map[int64]*users.Student, error)
}

type presenceStudents struct {
	source presenceStudentSource
	care   careplan.StudentProfileCommands
}

// PresenceStudents reads Directory records and writes only Care Plan's live flags.
func PresenceStudents(db *bun.DB, source presenceStudentSource) presenceservice.PresenceStudents {
	care, err := careCompose.NewStudentProfiles(db, func(careCompose.Observation) {})
	if err != nil {
		panic(err)
	}
	return presenceStudents{source: source, care: care}
}

func (p presenceStudents) FindByID(ctx context.Context, id int64) (*studentpresence.StudentRecord, error) {
	row, err := p.source.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return studentRecord(row), nil
}

func (p presenceStudents) FindByIDForUpdate(ctx context.Context, id int64) (*studentpresence.StudentRecord, error) {
	row, err := p.source.FindByIDForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	return studentRecord(row), nil
}

func (p presenceStudents) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*studentpresence.StudentRecord, error) {
	rows, err := p.source.FindByIDsForUpdate(ctx, ids)
	if err != nil {
		return nil, err
	}
	records := make(map[int64]*studentpresence.StudentRecord, len(rows))
	for id, row := range rows {
		records[id] = studentRecord(row)
	}
	return records, nil
}

func (p presenceStudents) UpdateLiveStatus(ctx context.Context, record *studentpresence.StudentRecord) error {
	if record == nil {
		return nil
	}
	affected, err := p.care.SetStudentLiveStatus(ctx, careplan.StudentLiveStatus{
		StudentID: record.ID, Sick: record.Sick != nil && *record.Sick, SickSince: record.SickSince,
		Excused: record.Excused != nil && *record.Excused, ExcusedSince: record.ExcusedSince,
	})
	if err != nil {
		return err
	}
	// The owner's full-row update failed loudly when the primary key matched
	// nothing, and the check-in rollback path depends on that: a student that
	// vanished or moved tenant mid-transaction must abort the visit rather
	// than leave the flags stale behind a silent no-op.
	if affected == 0 {
		return fmt.Errorf("update student live status: student %d not found", record.ID)
	}
	return nil
}

// studentLifecycle maps the owner's lifecycle status onto the three states the
// presence flows branch on. Every other status, today only "pending", is
// StudentLifecycleOther and is treated as not-yet-active, which is what the
// enrollment filter and the check-in guards already did for it.
func studentLifecycle(status users.StudentStatus) studentpresence.StudentLifecycle {
	switch status {
	case users.StudentStatusActive:
		return studentpresence.StudentLifecycleActive
	case users.StudentStatusInactive:
		return studentpresence.StudentLifecycleInactive
	case users.StudentStatusAlumnus:
		return studentpresence.StudentLifecycleAlumnus
	case users.StudentStatusPending:
		return studentpresence.StudentLifecycleOther
	default:
		return studentpresence.StudentLifecycleOther
	}
}

func studentRecord(row *users.Student) *studentpresence.StudentRecord {
	if row == nil {
		return nil
	}
	return &studentpresence.StudentRecord{
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

// statusDayOverviewSource is the slice of the person service the absence
// overview reads.
type statusDayOverviewSource interface {
	GetStudentsByGroupIDs(context.Context, []int64) ([]*users.Student, error)
	GetByIDs(context.Context, []int64) (map[int64]*users.Person, error)
}

type statusDayOverviewPeople struct{ source statusDayOverviewSource }

// StatusDayOverviewPeople serves the absence overview's people reads from the
// users service.
func StatusDayOverviewPeople(source statusDayOverviewSource) presenceservice.StatusDayOverviewPeople {
	return statusDayOverviewPeople{source: source}
}

func (p statusDayOverviewPeople) GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*studentpresence.StudentRecord, error) {
	rows, err := p.source.GetStudentsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	records := make([]*studentpresence.StudentRecord, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		records = append(records, studentRecord(row))
	}
	return records, nil
}

func (p statusDayOverviewPeople) GetByIDs(ctx context.Context, ids []int64) (map[int64]*studentpresence.PersonName, error) {
	rows, err := p.source.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]*studentpresence.PersonName, len(rows))
	for id, row := range rows {
		if row == nil {
			continue
		}
		names[id] = &studentpresence.PersonName{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName}
	}
	return names, nil
}
