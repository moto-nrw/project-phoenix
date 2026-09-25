package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// The Stammdaten decision and the cross-kind parent-request commands are
// Care Plan's (#3354). The adapters below bind their consumer-owned ports to
// the People Directory rows and change history, the realtime hub, the parent
// chat and the identity-owned review scope; they decide nothing.

// masterDataDecisionWiring is the root's view of the decision dependencies.
// Nil effects are skipped.
type masterDataDecisionWiring struct {
	carePlan    careplan.Capability
	fields      peopledirectory.StudentFieldReviewQuery
	students    usersModels.StudentRepository
	persons     usersModels.PersonRepository
	audit       users.StudentChangeRecorder
	scope       carePlanCompose.ReviewScopeResolver
	emitter     *parentmessaging.Emitter
	broadcaster realtime.Broadcaster
	events      usersModels.ParentRequestEventRepository
	shares      carePlanCompose.ShareVisibility
	logger      *slog.Logger
	today       func() timezone.Date
}

func newMasterDataDecisions(wiring masterDataDecisionWiring) (*carePlanCompose.MasterDataDecisions, error) {
	if wiring.students == nil || wiring.persons == nil || wiring.fields == nil {
		return nil, errors.New("master data decisions: student and person repositories and the field review are required")
	}
	deps := carePlanCompose.MasterDataDecisionDependencies{
		CarePlan: wiring.carePlan, People: masterDataFieldReviews{fields: wiring.fields}, Scope: wiring.scope, Shares: wiring.shares,
		Records: masterDataRecords{students: wiring.students, persons: wiring.persons, audit: wiring.audit},
		Today:   carePlanCompose.Today, Logger: wiring.logger,
	}
	if wiring.today != nil {
		deps.Today = func() careplan.Date { return careplan.Date(wiring.today()) }
	}
	if wiring.emitter != nil {
		deps.Messenger = excusedRequestMessenger{emitter: wiring.emitter}
	}
	if wiring.broadcaster != nil {
		deps.Broadcaster = masterDataBroadcaster{broadcaster: wiring.broadcaster}
	}
	if wiring.events != nil {
		deps.Ledger = newParentRequestLedger(wiring.events)
	}
	return carePlanCompose.NewMasterDataDecisions(deps)
}

// parentRequestCoordinatorWiring names the four request queues the
// cross-kind commands are composed over.
type parentRequestCoordinatorWiring struct {
	masterData *carePlanCompose.MasterDataDecisions
	excused    careplan.ExcusedAbsenceRequests
	care       any
	offering   carePlanCompose.ParentRequestConflicts
	events     usersModels.ParentRequestEventRepository
}

func newParentRequestCoordinator(wiring parentRequestCoordinatorWiring) (parentrequests.Coordinator, error) {
	care, ok := wiring.care.(carePlanCompose.ParentRequestConflicts)
	if !ok {
		return nil, errors.New("parent request coordinator: the care request service is not a conflict port")
	}
	deps := carePlanCompose.ParentRequestCoordinatorDependencies{
		Rights: parentRequestRights, MasterData: wiring.masterData, Excused: wiring.excused,
		Care: care, Offering: wiring.offering,
	}
	if wiring.events != nil {
		deps.Ledger = newParentRequestLedger(wiring.events)
	}
	return carePlanCompose.NewParentRequestCoordinator(deps)
}

// parentRequestRights evaluates the request claims: users:update decides every
// queue, users:absence the sick and excused queue only.
func parentRequestRights(ctx context.Context) carePlanCompose.ParentRequestRights {
	writeQueues, absences := securityruntime.ParentRequestReviewRights(authjwt.PermissionsFromCtx(ctx))
	return carePlanCompose.ParentRequestRights{WriteQueues: writeQueues, Absences: absences}
}

// parentRequestWriteScope adapts the identity-owned review policy to the
// Stammdaten decision: the same reach the Stammdaten, Betreuungszeiten and
// Angebote queues are listed with, from the request claims.
func parentRequestWriteScope(policy interface {
	Scope(ctx context.Context, permissions []string) (bool, []int64, error)
}) carePlanCompose.ReviewScopeResolver {
	return func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := policy.Scope(ctx, authjwt.PermissionsFromCtx(ctx))
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

// masterDataRecords binds the decision's People Directory port to the
// retained student and person repositories, which reach the owner's write
// path, and to the per-child change history. A nil audit recorder skips the
// history, as graphs without one always did.
type masterDataRecords struct {
	students usersModels.StudentRepository
	persons  usersModels.PersonRepository
	audit    users.StudentChangeRecorder
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

// UpdateStudent writes inside a companion recording scope, so the decision
// learns whether the write trimmed a "läuft mit" link rather than guessing
// from the request's target (see usersModels.CompanionChangeRecorder).
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
	writeCtx, companions := usersModels.ContextWithCompanionChangeRecorder(ctx)
	if err := r.students.Update(writeCtx, student); err != nil {
		return false, &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteUpdate, Err: err}
	}
	if r.audit != nil {
		if err := r.audit.RecordChangesForActor(writeCtx, &before, student, actorAccountID); err != nil {
			return false, &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteAudit, Err: err}
		}
	}
	return companions.Changed(), nil
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
	person.FirstName, person.LastName = view.FirstName, view.LastName
	person.Birthday = nil
	if view.Birthday != "" {
		birthday, err := timezone.ParseDate(view.Birthday)
		if err != nil {
			return &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteUpdate, Err: err}
		}
		person.Birthday = &birthday
	}
	if err := r.persons.Update(ctx, person); err != nil {
		return &carePlanCompose.MasterDataWriteError{Step: carePlanCompose.MasterDataWriteUpdate, Err: err}
	}
	return nil
}

func masterDataStudent(student *usersModels.Student) carePlanCompose.MasterDataStudent {
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

func allowedDepartureModes(modes carePlanCompose.DepartureModes) usersModels.AllowedDepartureModes {
	allowed := make(usersModels.AllowedDepartureModes, len(modes))
	for day, values := range modes {
		for _, value := range values {
			allowed[day] = append(allowed[day], usersModels.DepartureMode(value))
		}
	}
	return allowed
}

func masterDataPerson(person *usersModels.Person) carePlanCompose.MasterDataPerson {
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
