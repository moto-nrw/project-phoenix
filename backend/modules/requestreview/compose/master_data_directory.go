package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carecompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// ReviewPeople contains the identity facts shared by Care Plan review queues.
// Every read is scoped by the ambient tenant transaction.
type ReviewPeople interface {
	Students
	ListPersonsByID(context.Context, []int64) ([]peopledirectory.Person, error)
	ListPersonsByAccount(context.Context, []int64) ([]peopledirectory.Person, error)
}

type MasterDataPeople interface {
	ReviewPeople
	peopledirectory.StudentFieldReviewQuery
}

type reviewDirectory struct{ people ReviewPeople }

type masterDataDirectory struct {
	reviewDirectory
	fields peopledirectory.StudentFieldReviewQuery
}

func (d masterDataDirectory) ReviewFields(ctx context.Context, changes []carecompose.MasterDataFieldChange) (map[int64]carecompose.MasterDataFieldFacts, error) {
	fields := make([]peopledirectory.StudentFieldChange, 0, len(changes))
	for _, change := range changes {
		fields = append(fields, peopledirectory.StudentFieldChange{
			RequestID: change.RequestID, StudentID: change.StudentID, Target: change.Target,
			Field: change.Field, OldValue: change.OldValue, NewValue: change.NewValue,
		})
	}
	reviews, err := d.fields.ReviewStudentFields(ctx, fields)
	if err != nil {
		return nil, err
	}
	facts := make(map[int64]carecompose.MasterDataFieldFacts, len(reviews))
	for id, review := range reviews {
		fact := carecompose.MasterDataFieldFacts{
			FirstName: review.FirstName, LastName: review.LastName,
			BulkEligible: review.BulkEligible, BulkIneligibleReason: review.BulkIneligibleReason,
			BulkIneligibleText: review.BulkIneligibleText, CurrentValueChanged: review.CurrentValueChanged,
		}
		if review.Student != nil {
			student := reviewStudent(*review.Student)
			fact.Student = &student
		}
		facts[id] = fact
	}
	return facts, nil
}

func reviewStudent(student peopledirectory.Student) carecompose.ReviewStudent {
	return carecompose.ReviewStudent{
		ID: student.ID, PersonID: student.PersonID, GroupID: student.GroupID,
		Alumnus: student.IsAlumnus(), EnrolledUntil: careplan.Date(student.EnrolledUntil), SchoolClass: student.SchoolClass,
	}
}

func (d reviewDirectory) FindStudents(ctx context.Context, ids []int64) (map[int64]carecompose.ReviewStudent, error) {
	students, err := d.people.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]carecompose.ReviewStudent, len(students))
	for _, student := range students {
		result[student.ID] = reviewStudent(student)
	}
	return result, nil
}

func (d reviewDirectory) PersonNames(ctx context.Context, ids []int64) (map[int64]carecompose.PersonName, error) {
	persons, err := d.people.ListPersonsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]carecompose.PersonName, len(persons))
	for _, person := range persons {
		names[person.ID] = carecompose.PersonName{FirstName: person.FirstName, LastName: person.LastName}
	}
	return names, nil
}

func (d reviewDirectory) ReviewerNames(ctx context.Context, ids []int64) (map[int64]carecompose.PersonName, error) {
	persons, err := d.people.ListPersonsByAccount(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]carecompose.PersonName, len(persons))
	for _, person := range persons {
		if person.AccountID != nil {
			names[*person.AccountID] = carecompose.PersonName{FirstName: person.FirstName, LastName: person.LastName}
		}
	}
	return names, nil
}
