package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

// ExcusedRequestWiring is the service graph's contribution to the Care Plan
// excused-absence workflow: the People Directory rows through the repository
// contracts, plus the effect ports the root supplies. Today defaults to the
// Berlin calendar day.
type ExcusedRequestWiring struct {
	CarePlan    careplan.Capability
	Students    ExcusedRequestStudentRows
	Persons     ExcusedRequestPersonRows
	Scope       carePlanCompose.ReviewScopeResolver
	Today       func() careplan.Date
	Logger      *slog.Logger
	Observe     func(carePlanCompose.Observation)
	Messenger   carePlanCompose.RequestMessenger
	Broadcaster carePlanCompose.StaffBroadcaster
	Notifier    carePlanCompose.AbsenceNotifier
	Ledger      carePlanCompose.RequestLedger
	Shares      carePlanCompose.ShareVisibility
}

// NewExcusedAbsenceRequests composes the Care Plan excused-absence workflow
// over the legacy student and person repositories.
func NewExcusedAbsenceRequests(wiring ExcusedRequestWiring) (*carePlanCompose.ExcusedAbsenceRequests, error) {
	if wiring.Students == nil || wiring.Persons == nil {
		return nil, errors.New("compose excused requests: student and person repositories are required")
	}
	today := wiring.Today
	if today == nil {
		today = carePlanCompose.Today
	}
	return carePlanCompose.NewExcusedAbsenceRequests(carePlanCompose.ExcusedRequestDependencies{
		CarePlan: wiring.CarePlan,
		Students: excusedRequestStudentDirectory{
			students: wiring.Students, persons: wiring.Persons, writes: peopleCompose.NewAuditedStudentUpdates(wiring.Students, nil),
		},
		Scope:   wiring.Scope,
		Today:   today,
		Observe: wiring.Observe, Logger: wiring.Logger,
		Messenger: wiring.Messenger, Broadcaster: wiring.Broadcaster, Notifier: wiring.Notifier,
		Ledger: wiring.Ledger, Shares: wiring.Shares,
	})
}

// ExcusedRequestStudentRows are the People Directory child rows the
// excused-absence workflow reads, locks and flags.
type ExcusedRequestStudentRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.StudentState, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*peopledirectory.StudentState, error)
	Update(ctx context.Context, student *peopledirectory.StudentState) error
}

// ExcusedRequestPersonRows names the children and the reviewers.
type ExcusedRequestPersonRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.DirectoryPersonSnapshot, error)
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*peopledirectory.DirectoryPersonSnapshot, error)
}

// excusedRequestStudentDirectory answers the workflow's People Directory port
// from the legacy repositories, keeping the query shape of the legacy
// service: one student lookup and one person lookup per queue page.
type excusedRequestStudentDirectory struct {
	students ExcusedRequestStudentRows
	persons  ExcusedRequestPersonRows
	writes   peopleCompose.AuditedStudentUpdates
}

func (d excusedRequestStudentDirectory) FindStudents(ctx context.Context, ids []int64) (map[int64]carePlanCompose.ReviewStudent, error) {
	rows, err := d.students.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]carePlanCompose.ReviewStudent, len(rows))
	for id, row := range rows {
		if row == nil {
			continue
		}
		result[id] = reviewStudent(row)
	}
	return result, nil
}

func (d excusedRequestStudentDirectory) LockStudent(ctx context.Context, id int64) (carePlanCompose.ReviewStudent, error) {
	row, err := d.students.FindByIDForUpdate(ctx, id)
	if err != nil {
		return carePlanCompose.ReviewStudent{}, err
	}
	if row == nil {
		return carePlanCompose.ReviewStudent{}, errors.New("student not found")
	}
	return reviewStudent(row), nil
}

func (d excusedRequestStudentDirectory) PersonNames(ctx context.Context, personIDs []int64) (map[int64]carePlanCompose.PersonName, error) {
	rows, err := d.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	return carePlanPersonNames(rows), nil
}

func (d excusedRequestStudentDirectory) ReviewerNames(ctx context.Context, accountIDs []int64) (map[int64]carePlanCompose.PersonName, error) {
	rows, err := d.persons.FindByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	return carePlanPersonNames(rows), nil
}

// SetLiveAbsenceFlags writes the live flags Care Plan decides for the
// approved absence through People Directory's locked SetLiveAbsenceFlags.
func (d excusedRequestStudentDirectory) SetLiveAbsenceFlags(ctx context.Context, studentID int64, status string, at time.Time) error {
	flags := careplan.LiveFlagsForApprovedAbsence(status)
	return d.writes.SetLiveAbsenceFlags(ctx, studentID, peopledirectory.LiveAbsenceFlags{Sick: flags.Sick, ClearExcused: flags.ClearExcused}, at)
}

func reviewStudent(row *peopledirectory.StudentState) carePlanCompose.ReviewStudent {
	student := carePlanCompose.ReviewStudent{ID: row.ID, PersonID: row.PersonID, GroupID: row.GroupID, Alumnus: row.IsAlumnus()}
	if row.EnrolledUntil != nil {
		student.EnrolledUntil = careplan.Date(*row.EnrolledUntil)
	}
	return student
}

func carePlanPersonNames(rows map[int64]*peopledirectory.DirectoryPersonSnapshot) map[int64]carePlanCompose.PersonName {
	result := make(map[int64]carePlanCompose.PersonName, len(rows))
	for id, row := range rows {
		if row == nil {
			continue
		}
		result[id] = carePlanCompose.PersonName{FirstName: row.FirstName, LastName: row.LastName}
	}
	return result
}
