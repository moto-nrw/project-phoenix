package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Care Plan's offering-change review (#3561) reads and writes its own
// requests, Enrollment's requests, children, phases and bookings, People
// Directory students, the settings, the Timetable planning and the
// Communication side through its own ports. The adapters below bind them over
// the owners' capabilities with the queries and error shapes the retained
// repositories had; they decide nothing.

// offeringChangeInputs are the owners the offering-change review is composed
// over.
type offeringChangeInputs struct {
	CarePlan careplan.Capability
	// Catalog reads the care offerings; nil reads them from CarePlan.
	Catalog     carePlanCompose.OfferingChangeCatalog
	Enrollment  offeringChangeEnrollmentReads
	Students    usersModels.StudentRepository
	Withdrawals usersModels.CareWithdrawalCompletionRepository
	Settings    offeringChangeSettingsReads
	Planning    carePlanCompose.OfferingChangePlanning
	Bookings    careplan.BookingMaterializationCapability
	Reviews     offeringChangeReviewPolicy
	Emitter     *parentmessaging.Emitter
	Events      users.ParentRequestEventRecorder
	Shares      carePlanCompose.ShareVisibility
	Today       func() calendar.Date
	Logger      *slog.Logger
}

func newOfferingChanges(inputs offeringChangeInputs) (careplan.OfferingChangeCapability, error) {
	if inputs.CarePlan == nil || inputs.Enrollment == nil || inputs.Students == nil || inputs.Reviews == nil {
		return nil, errors.New("offering changes: care plan, enrollment, students and review policy are required")
	}
	var messenger carePlanCompose.RequestMessenger
	if inputs.Emitter != nil {
		messenger = excusedRequestMessenger{emitter: inputs.Emitter}
	}
	var ledger carePlanCompose.RequestLedger
	if inputs.Events != nil {
		ledger = excusedRequestLedger{events: inputs.Events}
	}
	var catalog carePlanCompose.OfferingChangeCatalog = inputs.CarePlan
	if inputs.Catalog != nil {
		catalog = inputs.Catalog
	}
	changes, err := carePlanCompose.NewOfferingChanges(carePlanCompose.OfferingChangeDependencies{
		Rows:       offeringChangeRows{carePlan: inputs.CarePlan},
		Catalog:    catalog,
		Enrollment: offeringChangeEnrollment{owner: inputs.Enrollment},
		Students:   offeringChangeStudents{students: inputs.Students, withdrawals: inputs.Withdrawals},
		Settings:   offeringChangeSettings{bookingSettings{settings: inputs.Settings}, inputs.Settings},
		Planning:   inputs.Planning,
		Bookings:   inputs.Bookings,
		Scope:      offeringChangeReviewScope(inputs.Reviews),
		Messenger:  messenger,
		Ledger:     ledger,
		Shares:     inputs.Shares,
		Today:      inputs.Today,
		Logger:     inputs.Logger,
	})
	if err != nil {
		return nil, err
	}
	return offeringChangeLifecycle{OfferingChangeCapability: changes}, nil
}

// offeringChangeReviewPolicy is the parent-request review policy's scope of
// the caller.
type offeringChangeReviewPolicy interface {
	Scope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
}

// offeringChangeReviewScope applies the identity-owned review policy with
// the permissions of the request claims.
func offeringChangeReviewScope(policy offeringChangeReviewPolicy) carePlanCompose.ReviewScopeResolver {
	return func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := policy.Scope(ctx, authjwt.PermissionsFromCtx(ctx))
		if err != nil {
			return carePlanCompose.ReviewScope{}, err
		}
		return carePlanCompose.ReviewScope{SchoolWide: schoolWide, GroupIDs: groupIDs}, nil
	}
}

// offeringChangeLifecycle presents the review with the shared parent-request
// lifecycle sentinels the staff routes, the parents portal and the conflict
// coordinator match on.
type offeringChangeLifecycle struct {
	careplan.OfferingChangeCapability
}

func (l offeringChangeLifecycle) Decide(ctx context.Context, input careplan.OfferingChangeDecisionInput) error {
	return mapOfferingLifecycleError(l.OfferingChangeCapability.Decide(ctx, input))
}

func (l offeringChangeLifecycle) Edit(ctx context.Context, requestID int64, input careplan.CreateOfferingChangeInput, expectedVersion string) (*careplan.OfferingChangeRequest, error) {
	row, err := l.OfferingChangeCapability.Edit(ctx, requestID, input, expectedVersion)
	return row, mapOfferingLifecycleError(err)
}

func (l offeringChangeLifecycle) MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error {
	return mapOfferingLifecycleError(l.OfferingChangeCapability.MarkDone(ctx, requestID, expectedVersion, reason, reviewedBy))
}

func (l offeringChangeLifecycle) DecideConflictRequest(ctx context.Context, decision careplan.OfferingConflictDecision) error {
	return mapOfferingLifecycleError(l.OfferingChangeCapability.DecideConflictRequest(ctx, decision))
}

// mapOfferingLifecycleError translates the Care Plan lifecycle sentinels
// into the shared parent-request ones. Domain sentinels pass through.
func mapOfferingLifecycleError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, careplan.ErrParentRequestStale):
		return users.ErrParentRequestStale
	case errors.Is(err, careplan.ErrParentRequestReasonRequired):
		return users.ErrParentRequestReasonRequired
	case errors.Is(err, careplan.ErrParentRequestPast):
		return users.ErrParentRequestPast
	case errors.Is(err, careplan.ErrParentRequestNotPast):
		return users.ErrParentRequestNotPast
	default:
		return err
	}
}

// offeringChangeConflictPort presents the review to the cross-kind conflict
// coordinator in the coordinator's own vocabulary.
type offeringChangeConflictPort struct {
	changes careplan.OfferingChangeConflicts
}

var _ users.ParentRequestConflictPort = offeringChangeConflictPort{}

func (p offeringChangeConflictPort) ConflictCandidate(ctx context.Context, requestID int64) (*users.ParentRequestConflictCandidate, error) {
	candidate, err := p.changes.ConflictCandidate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return &users.ParentRequestConflictCandidate{StudentID: candidate.StudentID, UpdatedAt: candidate.UpdatedAt}, nil
}

func (p offeringChangeConflictPort) LockConflictRequest(ctx context.Context, requestID int64) error {
	return p.changes.LockConflictRequest(ctx, requestID)
}

func (p offeringChangeConflictPort) DecideConflictRequest(ctx context.Context, decision users.ParentRequestConflictDecision) error {
	return p.changes.DecideConflictRequest(ctx, careplan.OfferingConflictDecision{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewerID: decision.ReviewerID, ActorRole: decision.ActorRole, ExpectedVersion: decision.ExpectedVersion,
	})
}

func (p offeringChangeConflictPort) WriteStaffValue(ctx context.Context, write users.ParentRequestStaffValueWrite) error {
	effectiveFrom, selections, err := parseStaffOfferingValue(write.Value)
	if err != nil {
		return err
	}
	return p.changes.WriteStaffValue(ctx, careplan.OfferingStaffValueWrite{
		StudentID: write.StudentID, EffectiveFrom: effectiveFrom, Selections: selections, Reason: write.Reason,
		ReviewerID: write.ReviewerID, ActorRole: write.ActorRole,
	})
}

// offeringChangeRows reads and writes enrollment.offering_change_requests
// through Care Plan's own capability, with the not-found and database error
// shapes of the retained request repository.
type offeringChangeRows struct{ carePlan careplan.Capability }

func (r offeringChangeRows) Create(ctx context.Context, row careplan.OfferingChangeRequest) (careplan.OfferingChangeRequest, error) {
	created, err := r.carePlan.CreateOfferingChange(ctx, row)
	if err != nil {
		return careplan.OfferingChangeRequest{}, offeringChangeDatabaseError("create", err)
	}
	return created, nil
}

func (r offeringChangeRows) Find(ctx context.Context, id int64) (careplan.OfferingChangeRequest, error) {
	return r.find(ctx, id, false, "find by id")
}

func (r offeringChangeRows) FindForUpdate(ctx context.Context, id int64) (careplan.OfferingChangeRequest, error) {
	row, err := r.find(ctx, id, true, "find offering change request for update")
	if errors.Is(err, careplan.ErrOfferingChangeNotFound) || errors.Is(err, sql.ErrNoRows) {
		return careplan.OfferingChangeRequest{}, careplan.ErrOfferingChangeNotFound
	}
	return row, err
}

func (r offeringChangeRows) find(ctx context.Context, id int64, lock bool, op string) (careplan.OfferingChangeRequest, error) {
	row, err := r.carePlan.FindOfferingChange(ctx, id, lock)
	if errors.Is(err, careplan.ErrOfferingChangeNotFound) {
		return careplan.OfferingChangeRequest{}, offeringChangeDatabaseError(op, sql.ErrNoRows)
	}
	if err != nil {
		return careplan.OfferingChangeRequest{}, offeringChangeDatabaseError(op, err)
	}
	return row, nil
}

func (r offeringChangeRows) PendingForStudent(ctx context.Context, studentID int64) (*careplan.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, careplan.OfferingChangeFilter{StudentID: studentID, Statuses: []string{careplan.OfferingChangePending}, Order: careplan.ChangeOrderCreated})
	if err != nil {
		return nil, offeringChangeDatabaseError("get pending offering change request", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (r offeringChangeRows) ListByStudent(ctx context.Context, studentID int64) ([]careplan.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, careplan.OfferingChangeFilter{StudentID: studentID, Order: careplan.ChangeOrderReviewed})
	if err != nil {
		return nil, offeringChangeDatabaseError("list offering change requests by student", err)
	}
	return rows, nil
}

func (r offeringChangeRows) ListPending(ctx context.Context) ([]careplan.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, careplan.OfferingChangeFilter{Statuses: []string{careplan.OfferingChangePending}, Order: careplan.ChangeOrderCreated})
	if err != nil {
		return nil, offeringChangeDatabaseError("list pending offering change requests", err)
	}
	return rows, nil
}

// list reads the rows and checks that their stored JSON decodes, as the
// retained repository did when it mapped every row.
func (r offeringChangeRows) list(ctx context.Context, filter careplan.OfferingChangeFilter) ([]careplan.OfferingChangeRequest, error) {
	rows, err := r.carePlan.ListOfferingChanges(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := decodableOfferingChange(row); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func decodableOfferingChange(row careplan.OfferingChangeRequest) error {
	var payload map[string]any
	if err := unmarshalOptionalOfferingJSON(row.Payload, &payload); err != nil {
		return fmt.Errorf("decode offering change payload: %w", err)
	}
	var snapshot map[string]any
	if err := unmarshalOptionalOfferingJSON(row.DecisionSnapshot, &snapshot); err != nil {
		return fmt.Errorf("decode offering change decision snapshot: %w", err)
	}
	return nil
}

func unmarshalOptionalOfferingJSON(data json.RawMessage, target any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, target)
}

func (r offeringChangeRows) UpdateEffectiveFrom(ctx context.Context, id int64, effectiveFrom calendar.Date) error {
	return offeringPendingError("update offering change effective date", r.carePlan.UpdateOfferingChangeEffectiveFrom(ctx, id, effectiveFrom.String()))
}

func (r offeringChangeRows) UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error {
	return offeringPendingError("update approved complete-withdrawal result", r.carePlan.UpdateApprovedCompleteWithdrawal(ctx, id, complete))
}

func (r offeringChangeRows) UpdatePending(ctx context.Context, id int64, payload json.RawMessage, effectiveFrom calendar.Date, note *string) error {
	err := r.carePlan.UpdatePendingOfferingChange(ctx, careplan.UpdatePendingOfferingChange{
		ID: id, Payload: payload, EffectiveFrom: effectiveFrom.String(), ParentNote: note,
	})
	return offeringPendingError("update pending offering change request", err)
}

func (r offeringChangeRows) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error {
	err := r.carePlan.DecideOfferingChange(ctx, careplan.DecideOfferingChange{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied})
	return offeringPendingError("decide offering change request", err)
}

func (r offeringChangeRows) UpdateDecisionSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	if err := r.carePlan.UpdateOfferingChangeSnapshot(ctx, id, snapshot); err != nil {
		return offeringChangeDatabaseError("update offering change decision snapshot", err)
	}
	return nil
}

func offeringPendingError(op string, err error) error {
	if errors.Is(err, careplan.ErrOfferingChangeNotPending) {
		return careplan.ErrOfferingChangeNotPending
	}
	if err != nil {
		return offeringChangeDatabaseError(op, err)
	}
	return nil
}

// offeringChangeEnrollmentReads is the part of the Enrollment owner an
// offering change reads.
type offeringChangeEnrollmentReads interface {
	enrollment.StudentCarePeriodReader
	RequestByID(context.Context, int64, bool) (*enrollmentOwner.Request, error)
	ChildByID(context.Context, int64) (*enrollmentOwner.RequestChild, error)
	ChildrenByID(context.Context, []int64) ([]*enrollmentOwner.RequestChild, error)
	Phase(context.Context, int64) (*enrollmentOwner.Phase, error)
	RequestChildOfferingsAtDate(context.Context, int64, enrollmentOwner.Date) ([]*enrollmentOwner.RequestChildOffering, error)
	EffectiveOfferingSelectionsAtDates(context.Context, map[int64]enrollmentOwner.Date) ([]*enrollmentOwner.RequestChildOffering, error)
	OfferingCatalogState(context.Context, int64, int64, enrollmentOwner.Date, enrollmentOwner.Date) (*enrollmentOwner.OfferingCatalogState, error)
	OfferingCapacityPeak(context.Context, int64, []int64, enrollmentOwner.Date, enrollmentOwner.Date) (int, error)
}

// offeringChangeEnrollment reads Enrollment's requests, children, phases and
// booked selections for the review.
type offeringChangeEnrollment struct{ owner offeringChangeEnrollmentReads }

func (e offeringChangeEnrollment) StudentCarePeriods(ctx context.Context, studentID int64) ([]carePlanCompose.OfferingCarePeriod, error) {
	periods, err := enrollment.ReadStudentCarePeriods(ctx, e.owner, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.OfferingCarePeriod, 0, len(periods))
	for _, period := range periods {
		result = append(result, carePlanCompose.OfferingCarePeriod{
			RequestChildID: period.RequestChildID, RequestID: period.RequestID, PhaseID: period.PhaseID,
			PhaseName: period.PhaseName, ServiceStart: period.ServiceStartDate, ServiceEnd: period.ServiceEndDate,
			TargetGradeLevel: period.TargetGradeLevel, TargetSchoolClass: period.TargetSchoolClass,
		})
	}
	return result, nil
}

func (e offeringChangeEnrollment) Child(ctx context.Context, id int64) (*carePlanCompose.OfferingChangeChild, error) {
	child, err := e.owner.ChildByID(ctx, id)
	if err != nil || child == nil {
		return nil, err
	}
	value := offeringChangeChild(child)
	return &value, nil
}

func (e offeringChangeEnrollment) Children(ctx context.Context, ids []int64) ([]carePlanCompose.OfferingChangeChild, error) {
	children, err := e.owner.ChildrenByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.OfferingChangeChild, 0, len(children))
	for _, child := range children {
		if child != nil {
			result = append(result, offeringChangeChild(child))
		}
	}
	return result, nil
}

func offeringChangeChild(child *enrollmentOwner.RequestChild) carePlanCompose.OfferingChangeChild {
	return carePlanCompose.OfferingChangeChild{
		ID: child.ID, RequestID: child.RequestID, TargetGradeLevel: child.TargetGradeLevel, TargetSchoolClass: child.TargetSchoolClass,
	}
}

func (e offeringChangeEnrollment) Request(ctx context.Context, id int64) (*carePlanCompose.BookingRequest, error) {
	request, err := e.owner.RequestByID(ctx, id, false)
	if err != nil || request == nil {
		return nil, err
	}
	return &carePlanCompose.BookingRequest{ID: request.ID, PhaseID: request.PhaseID}, nil
}

func (e offeringChangeEnrollment) Phase(ctx context.Context, id int64) (*carePlanCompose.BookingPhase, error) {
	phase, err := e.owner.Phase(ctx, id)
	if err != nil || phase == nil {
		return nil, err
	}
	return &carePlanCompose.BookingPhase{OfferingPhase: bookingOfferingPhase(phase), CareOfferingSelectionMode: phase.CareOfferingSelectionMode}, nil
}

func (e offeringChangeEnrollment) SelectionsAt(ctx context.Context, childID int64, on calendar.Date) ([]careplan.BookedOffering, error) {
	values, err := e.owner.RequestChildOfferingsAtDate(ctx, childID, enrollmentOwner.Date(on))
	return bookedOfferings(values), err
}

func (e offeringChangeEnrollment) EffectiveSelectionsAt(ctx context.Context, dates map[int64]calendar.Date) ([]careplan.BookedOffering, error) {
	ownerDates := make(map[int64]enrollmentOwner.Date, len(dates))
	for childID, date := range dates {
		ownerDates[childID] = enrollmentOwner.Date(date)
	}
	values, err := e.owner.EffectiveOfferingSelectionsAtDates(ctx, ownerDates)
	return bookedOfferings(values), err
}

func (e offeringChangeEnrollment) CatalogState(ctx context.Context, phaseID, childID int64, on, until calendar.Date) (carePlanCompose.OfferingCatalogState, error) {
	state, err := e.owner.OfferingCatalogState(ctx, phaseID, childID, enrollmentOwner.Date(on), enrollmentOwner.Date(until))
	if err != nil || state == nil {
		return carePlanCompose.OfferingCatalogState{}, err
	}
	return carePlanCompose.OfferingCatalogState{Current: bookedOfferings(state.Current), CapacityPeaks: state.CapacityPeaks}, nil
}

func (e offeringChangeEnrollment) CapacityPeak(ctx context.Context, offeringID int64, excludeChildIDs []int64, from, until calendar.Date) (int, error) {
	return e.owner.OfferingCapacityPeak(ctx, offeringID, excludeChildIDs, enrollmentOwner.Date(from), enrollmentOwner.Date(until))
}

// offeringChangeStudents reads the People Directory students a decision
// authorizes against and their open complete-withdrawal follow-up.
type offeringChangeStudents struct {
	students    usersModels.StudentRepository
	withdrawals usersModels.CareWithdrawalCompletionRepository
}

func (s offeringChangeStudents) FindStudent(ctx context.Context, id int64) (*carePlanCompose.ReviewStudent, error) {
	student, err := s.students.FindByID(ctx, id)
	return offeringReviewStudent(student), err
}

func (s offeringChangeStudents) LockStudent(ctx context.Context, id int64) (*carePlanCompose.ReviewStudent, error) {
	student, err := s.students.FindByIDForUpdate(ctx, id)
	return offeringReviewStudent(student), err
}

func (s offeringChangeStudents) HasPendingWithdrawalCompletion(ctx context.Context, studentID int64) (bool, error) {
	if s.withdrawals == nil {
		return false, nil
	}
	_, total, err := s.withdrawals.ListPending(ctx, usersModels.CareWithdrawalCompletionFilter{StudentID: studentID, Page: 1, PageSize: 1})
	if err != nil {
		return false, err
	}
	return total > 0, nil
}

func offeringReviewStudent(row *usersModels.Student) *carePlanCompose.ReviewStudent {
	if row == nil {
		return nil
	}
	student := &carePlanCompose.ReviewStudent{
		ID: row.ID, PersonID: row.PersonID, SchoolClass: row.SchoolClass, GroupID: row.GroupID, Alumnus: row.IsAlumnus(),
	}
	if row.EnrolledUntil != nil {
		student.EnrolledUntil = careplan.Date(*row.EnrolledUntil)
	}
	return student
}

type offeringChangeSettingsReads interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// offeringChangeSettings resolves the tenant settings offering changes and
// pickup adjustments obey.
type offeringChangeSettings struct {
	bookingSettings
	reads offeringChangeSettingsReads
}

func (s offeringChangeSettings) OfferingChangesEnabled(ctx context.Context) (bool, error) {
	return s.reads.ResolveBool(ctx, configModels.KeyEnrollmentOfferingChangesEnabled)
}

func (s offeringChangeSettings) CourseRequestsEnabled(ctx context.Context) (bool, error) {
	return s.reads.ResolveBool(ctx, configModels.KeyEnrollmentParentCourseRequestsEnabled)
}

func (s offeringChangeSettings) OfferingChangeLeadDays(ctx context.Context) (string, error) {
	return s.reads.ResolveString(ctx, configModels.KeyEnrollmentOfferingChangesLeadDays)
}

func (s offeringChangeSettings) PickupOfferingReviewRequired(ctx context.Context) (bool, error) {
	return s.reads.ResolveBool(ctx, configModels.KeyRequirePickupOfferingReview)
}

// offeringChangeDatabaseError keeps the error text of the retained request
// repository.
func offeringChangeDatabaseError(op string, err error) error {
	return fmt.Errorf("database error during %s: %w", op, err)
}
