package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// The Stammdaten decision and the cross-kind parent-request commands are
// Care Plan's (#3354). The adapters below bind their consumer-owned ports to
// the People Directory rows and change history, the realtime hub, the parent
// chat and the identity-owned review scope; they decide nothing.

func NewMasterDataDecisions(wiring MasterDataDecisionWiring) (*carePlanCompose.MasterDataDecisions, error) {
	if wiring.Students == nil || wiring.Persons == nil || wiring.Fields == nil {
		return nil, errors.New("master data decisions: student and person repositories and the field review are required")
	}
	deps := carePlanCompose.MasterDataDecisionDependencies{
		CarePlan: wiring.CarePlan, People: masterDataFieldReviews{fields: wiring.Fields}, Scope: wiring.Scope, Shares: wiring.Shares,
		Records: masterDataRecords{
			students: wiring.Students, persons: wiring.Persons, details: wiring.Fields,
			writes: peopleCompose.NewAuditedStudentUpdates(wiring.Students, wiring.Audit),
		},
		Today: carePlanCompose.Today, Logger: wiring.Logger,
	}
	if wiring.Today != nil {
		deps.Today = func() careplan.Date { return careplan.Date(wiring.Today()) }
	}
	if wiring.Emitter != nil {
		deps.Messenger = NewExcusedRequestMessenger(wiring.Emitter)
	}
	if wiring.Broadcaster != nil {
		deps.Broadcaster = masterDataBroadcaster{broadcaster: wiring.Broadcaster}
	}
	if wiring.Events != nil {
		deps.Ledger = NewParentRequestLedger(wiring.Events)
	}
	return carePlanCompose.NewMasterDataDecisions(deps)
}

func NewParentRequestCoordinator(masterData *carePlanCompose.MasterDataDecisions, excused careplan.ExcusedAbsenceRequests, careRequests any, offering carePlanCompose.ParentRequestConflicts, events peopledirectory.ParentRequestEvents, permissions func(context.Context) []string) (parentrequests.Coordinator, error) {
	care, ok := careRequests.(carePlanCompose.ParentRequestConflicts)
	if !ok {
		return nil, errors.New("parent request coordinator: the care request service is not a conflict port")
	}
	deps := carePlanCompose.ParentRequestCoordinatorDependencies{
		Rights: parentRequestRights(permissions), MasterData: masterData, Excused: excused,
		Care: care, Offering: offering,
	}
	if events != nil {
		deps.Ledger = NewParentRequestLedger(events)
	}
	return carePlanCompose.NewParentRequestCoordinator(deps)
}

// parentRequestRights evaluates the request claims: users:update decides every
// queue, users:absence the sick and excused queue only.
func parentRequestRights(permissions func(context.Context) []string) func(context.Context) carePlanCompose.ParentRequestRights {
	return func(ctx context.Context) carePlanCompose.ParentRequestRights {
		writeQueues, absences := securityruntime.ParentRequestReviewRights(permissions(ctx))
		return carePlanCompose.ParentRequestRights{WriteQueues: writeQueues, Absences: absences}
	}
}

// ParentRequestWriteScope adapts the identity-owned review policy to the
// Stammdaten decision: the same reach the Stammdaten, Betreuungszeiten and
// Angebote queues are listed with, from the request claims.
func ParentRequestWriteScope(policy interface {
	Scope(ctx context.Context, permissions []string) (bool, []int64, error)
}, permissions func(context.Context) []string) carePlanCompose.ReviewScopeResolver {
	return func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := policy.Scope(ctx, permissions(ctx))
		if err != nil {
			return carePlanCompose.ReviewScope{}, err
		}
		return carePlanCompose.ReviewScope{SchoolWide: schoolWide, GroupIDs: groupIDs}, nil
	}
}

// masterDataFieldReviews binds the decision's field facts to People
// Directory's own review of a proposed change: the same facts the open queue
// shows, so the bulk approval agrees with the list it was offered from.
type masterDataFieldReviews struct {
	fields peopledirectory.StudentFieldReviewQuery
}

func (r masterDataFieldReviews) ReviewFields(ctx context.Context, changes []carePlanCompose.MasterDataFieldChange) (map[int64]carePlanCompose.MasterDataFieldFacts, error) {
	fields := make([]peopledirectory.StudentFieldChange, 0, len(changes))
	for _, change := range changes {
		fields = append(fields, peopledirectory.StudentFieldChange{
			RequestID: change.RequestID, StudentID: change.StudentID, Target: change.Target,
			Field: change.Field, OldValue: change.OldValue, NewValue: change.NewValue,
		})
	}
	reviews, err := r.fields.ReviewStudentFields(ctx, fields)
	if err != nil {
		return nil, err
	}
	facts := make(map[int64]carePlanCompose.MasterDataFieldFacts, len(reviews))
	for id, review := range reviews {
		fact := carePlanCompose.MasterDataFieldFacts{
			FirstName: review.FirstName, LastName: review.LastName,
			BulkEligible: review.BulkEligible, BulkIneligibleReason: review.BulkIneligibleReason,
			BulkIneligibleText: review.BulkIneligibleText, CurrentValueChanged: review.CurrentValueChanged,
		}
		if student := review.Student; student != nil {
			fact.Student = &carePlanCompose.ReviewStudent{
				ID: student.ID, PersonID: student.PersonID, GroupID: student.GroupID, Alumnus: student.IsAlumnus(),
				EnrolledUntil: careplan.Date(student.EnrolledUntil), SchoolClass: student.SchoolClass,
			}
		}
		facts[id] = fact
	}
	return facts, nil
}

// MasterDataPeople is the People Directory surface the decision binds: its
// review of a proposed field change and the person details write.
type MasterDataPeople interface {
	peopledirectory.StudentFieldReviewQuery
	peopledirectory.PersonDetailsCommand
}

// masterDataRecords binds the decision's People Directory port to the
// retained student and person reads, People Directory's audited student write
// over the same rows and its person details write.
type masterDataRecords struct {
	students masterDataStudentRows
	persons  masterDataPersonRows
	details  peopledirectory.PersonDetailsCommand
	writes   peopleCompose.AuditedStudentUpdates
}

// masterDataStudentRows and masterDataPersonRows are the retained student and
// person reads and writes the decision's records port takes.
type masterDataStudentRows interface {
	FindByID(ctx context.Context, id interface{}) (*peopledirectory.StudentState, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*peopledirectory.StudentState, error)
	Update(ctx context.Context, student *peopledirectory.StudentState) error
}

type masterDataPersonRows interface {
	FindByID(ctx context.Context, id interface{}) (*peopledirectory.DirectoryPersonSnapshot, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*peopledirectory.DirectoryPersonSnapshot, error)
}

var _ carePlanCompose.MasterDataRecords = masterDataRecords{}

func (r masterDataRecords) FindStudent(ctx context.Context, id int64) (carePlanCompose.MasterDataStudent, error) {
	student, err := r.students.FindByID(ctx, id)
	if err != nil {
		return carePlanCompose.MasterDataStudent{}, err
	}
	return masterDataStudent(student), nil
}

func (r masterDataRecords) LockStudent(ctx context.Context, id int64) (carePlanCompose.MasterDataStudent, error) {
	student, err := r.students.FindByIDForUpdate(ctx, id)
	if err != nil {
		return carePlanCompose.MasterDataStudent{}, err
	}
	return masterDataStudent(student), nil
}

func (r masterDataRecords) FindPerson(ctx context.Context, id int64) (carePlanCompose.MasterDataPerson, error) {
	person, err := r.persons.FindByID(ctx, id)
	if err != nil {
		return carePlanCompose.MasterDataPerson{}, err
	}
	return masterDataPerson(person), nil
}

// UpdateStudent reports whether the write trimmed a "läuft mit" link rather
// than guessing from the request's target (see UpdateStudentAudited).
func (r masterDataRecords) UpdateStudent(ctx context.Context, id, actorAccountID int64, change carePlanCompose.MasterDataStudentChange) (bool, error) {
	student, err := r.students.FindByIDForUpdate(ctx, id)
	if err != nil {
		return false, &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteLoad, Err: err}
	}
	write, err := change(masterDataStudent(student))
	if err != nil {
		return false, err
	}
	before := *student
	if write.SchoolClass != nil {
		student.SchoolClass = *write.SchoolClass
	}
	if write.DepartureModes != nil {
		// Setting AllowedDepartureModes makes StudentRepository.Update persist
		// the derived departure_days / pickup_days / bus_days columns too.
		student.AllowedDepartureModes = allowedDepartureModes(write.DepartureModes)
	}
	trimmed, err := r.writes.UpdateStudentAudited(ctx, &before, student, actorAccountID)
	var auditErr *peopledirectory.StudentAuditError
	switch {
	case errors.As(err, &auditErr):
		return false, &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteAudit, Err: auditErr.Err}
	case err != nil:
		return false, &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteUpdate, Err: err}
	}
	return trimmed, nil
}

func (r masterDataRecords) UpdatePerson(ctx context.Context, id int64, change carePlanCompose.MasterDataPersonChange) error {
	person, err := r.persons.FindByIDForUpdate(ctx, id)
	if err != nil {
		return &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteLoad, Err: err}
	}
	view := masterDataPerson(person)
	if err := change(&view); err != nil {
		return err
	}
	if err := r.details.SetPersonDetails(ctx, id, peopledirectory.PersonDetails{
		FirstName: view.FirstName, LastName: view.LastName, Birthday: view.Birthday,
	}); err != nil {
		return &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteUpdate, Err: err}
	}
	return nil
}

func masterDataStudent(student *peopledirectory.StudentState) carePlanCompose.MasterDataStudent {
	view := carePlanCompose.MasterDataStudent{
		ReviewStudent: carePlanCompose.ReviewStudent{
			ID: student.ID, PersonID: student.PersonID, SchoolClass: student.SchoolClass,
			GroupID: student.GroupID, Alumnus: student.IsAlumnus(),
		},
		DepartureModes: carePlanCompose.DepartureModes{},
	}
	if student.EnrolledUntil != nil {
		view.EnrolledUntil = careplan.Date(student.EnrolledUntil.String())
	}
	for day, modes := range student.AllowedDepartureModes {
		for _, mode := range modes {
			view.DepartureModes[day] = append(view.DepartureModes[day], string(mode))
		}
	}
	return view
}

func allowedDepartureModes(modes carePlanCompose.DepartureModes) peopledirectory.AllowedDepartureModes {
	allowed := make(peopledirectory.AllowedDepartureModes, len(modes))
	for day, values := range modes {
		for _, value := range values {
			allowed[day] = append(allowed[day], peopledirectory.DepartureMode(value))
		}
	}
	return allowed
}

func masterDataPerson(person *peopledirectory.DirectoryPersonSnapshot) carePlanCompose.MasterDataPerson {
	view := carePlanCompose.MasterDataPerson{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName}
	if person.Birthday != nil {
		view.Birthday = person.Birthday.String()
	}
	return view
}

// masterDataBroadcaster wakes staff tabs through the realtime hub.
type masterDataBroadcaster struct{ broadcaster realtime.Broadcaster }

const masterDataBroadcastSource = "master_data_review"

func (b masterDataBroadcaster) StudentUpdated(tenantID int64) error {
	source := masterDataBroadcastSource
	return b.broadcaster.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source}))
}

func (b masterDataBroadcaster) StudentCompanionsChanged(tenantID int64) error {
	source := masterDataBroadcastSource
	return b.broadcaster.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{Source: &source}))
}
