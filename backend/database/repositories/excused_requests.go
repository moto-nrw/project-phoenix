package repositories

import (
	"context"
	"errors"
	"log/slog"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
)

// ExcusedRequestWiring is the legacy graph's contribution to the Care Plan
// excused-absence workflow: the People Directory rows through the repository
// contracts, plus the effect ports the root supplies. Today defaults to the
// Berlin calendar day.
type ExcusedRequestWiring struct {
	CarePlan    careplan.Capability
	Students    usersModels.StudentRepository
	Persons     usersModels.PersonRepository
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
		today = carePlanLegacy.TodayDate
	}
	return carePlanCompose.NewExcusedAbsenceRequests(carePlanCompose.ExcusedRequestDependencies{
		CarePlan: wiring.CarePlan,
		Students: excusedRequestStudentDirectory{students: wiring.Students, persons: wiring.Persons},
		Scope:    wiring.Scope,
		Today:    today,
		Observe:  wiring.Observe, Logger: wiring.Logger,
		Messenger: wiring.Messenger, Broadcaster: wiring.Broadcaster, Notifier: wiring.Notifier,
		Ledger: wiring.Ledger, Shares: wiring.Shares,
	})
}

// excusedRequestStudentDirectory answers the workflow's People Directory port
// from the legacy repositories, keeping the query shape of the legacy
// service: one student lookup and one person lookup per queue page.
type excusedRequestStudentDirectory struct {
	students usersModels.StudentRepository
	persons  usersModels.PersonRepository
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
	return personNames(rows), nil
}

func (d excusedRequestStudentDirectory) ReviewerNames(ctx context.Context, accountIDs []int64) (map[int64]carePlanCompose.PersonName, error) {
	rows, err := d.persons.FindByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	return personNames(rows), nil
}

// SetLiveAbsenceFlags mirrors the direct parent path: an approved sick day
// that covers today raises the live sick flag, an approved excused day clears
// it. The row is re-read under lock so a concurrent flag change is not lost.
func (d excusedRequestStudentDirectory) SetLiveAbsenceFlags(ctx context.Context, studentID int64, status string, at time.Time) error {
	fresh, err := d.students.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return err
	}
	if status == careplan.StudentStatusDaySick {
		trueVal, falseVal := true, false
		fresh.Sick = &trueVal
		fresh.SickSince = &at
		fresh.Excused = &falseVal
		fresh.ExcusedSince = nil
	} else {
		falseVal := false
		fresh.Sick = &falseVal
		fresh.SickSince = nil
	}
	return d.students.Update(ctx, fresh)
}

func reviewStudent(row *usersModels.Student) carePlanCompose.ReviewStudent {
	student := carePlanCompose.ReviewStudent{ID: row.ID, PersonID: row.PersonID, GroupID: row.GroupID, Alumnus: row.IsAlumnus()}
	if row.EnrolledUntil != nil {
		student.EnrolledUntil = careplan.Date(*row.EnrolledUntil)
	}
	return student
}

func personNames(rows map[int64]*usersModels.Person) map[int64]carePlanCompose.PersonName {
	result := make(map[int64]carePlanCompose.PersonName, len(rows))
	for id, row := range rows {
		if row == nil {
			continue
		}
		result[id] = carePlanCompose.PersonName{FirstName: row.FirstName, LastName: row.LastName}
	}
	return result
}
