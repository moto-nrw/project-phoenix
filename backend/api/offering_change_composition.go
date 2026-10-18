package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Care Plan's offering-change review (#3561) reads and writes its own
// requests, Enrollment's requests, children, phases and bookings, People
// Directory students, the settings, the Timetable planning and the
// Communication side through its own ports. The adapters below bind them over
// the owners' capabilities with the queries and error shapes the retained
// repositories had; they decide nothing.

// offeringChangeOwners binds the offering-change review's ports over its
// owners. A nil catalog reads the care offerings from carePlan. The caller
// completes the dependencies with the planning, the booking materialization,
// the share visibility, the clock and the logger, and composes them through
// newOfferingChanges.
func offeringChangeOwners(
	carePlan careplan.Capability, catalog carePlanCompose.OfferingChangeCatalog, enrollment OfferingChangeEnrollmentReads,
	students offeringChangeStudentRows, withdrawals offeringChangeWithdrawalRows, settings OfferingChangeSettingsReads,
	reviews OfferingChangeReviewPolicy, permissions func(context.Context) []string,
	emitter *communicationCompose.ParentEventEmitter, events peopledirectory.ParentRequestEvents,
) (carePlanCompose.OfferingChangeDependencies, error) {
	if carePlan == nil || enrollment == nil || students == nil || reviews == nil || permissions == nil {
		return carePlanCompose.OfferingChangeDependencies{}, errors.New("offering changes: care plan, enrollment, students, review policy and request permissions are required")
	}
	var messenger carePlanCompose.RequestMessenger
	if emitter != nil {
		messenger = excusedRequestMessenger{emitter: emitter}
	}
	var ledger carePlanCompose.RequestLedger
	if events != nil {
		ledger = NewParentRequestLedger(events)
	}
	if catalog == nil {
		catalog = carePlan
	}
	return carePlanCompose.OfferingChangeDependencies{
		Rows:       offeringChangeRows{carePlan: carePlan},
		Catalog:    catalog,
		Enrollment: offeringChangeEnrollment{owner: enrollment},
		Students:   offeringChangeStudents{students: students, withdrawals: withdrawals},
		Settings:   NewOfferingChangeSettings(settings),
		Scope:      offeringChangeReviewScope(reviews, permissions),
		Messenger:  messenger,
		Ledger:     ledger,
	}, nil
}

// newOfferingChanges composes the offering-change review over deps from
// offeringChangeOwners.
func newOfferingChanges(deps carePlanCompose.OfferingChangeDependencies) (careplan.OfferingChangeCapability, error) {
	changes, err := carePlanCompose.NewOfferingChanges(deps)
	if err != nil {
		return nil, err
	}
	return offeringChangeLifecycle{OfferingChangeCapability: changes}, nil
}

// offeringChangeReviewScope applies the identity-owned review policy with
// the permissions of the request claims.
func offeringChangeReviewScope(policy OfferingChangeReviewPolicy, permissions func(context.Context) []string) carePlanCompose.ReviewScopeResolver {
	return func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := policy.Scope(ctx, permissions(ctx))
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
		return parentrequests.ErrStale
	case errors.Is(err, careplan.ErrParentRequestPast):
		return parentrequests.ErrPast
	case errors.Is(err, careplan.ErrParentRequestNotPast):
		return parentrequests.ErrNotPast
	default:
		return err
	}
}

// newOfferingChangeConflicts presents the review to the cross-kind conflict
// coordinator in the coordinator's own vocabulary.
func newOfferingChangeConflicts(changes careplan.OfferingChangeConflicts) carePlanCompose.ParentRequestConflicts {
	return offeringChangeConflictPort{changes: changes}
}

type offeringChangeConflictPort struct {
	changes careplan.OfferingChangeConflicts
}

func (p offeringChangeConflictPort) ConflictCandidate(ctx context.Context, requestID int64) (*carePlanCompose.ConflictCandidate, error) {
	candidate, err := p.changes.ConflictCandidate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return &carePlanCompose.ConflictCandidate{StudentID: candidate.StudentID, UpdatedAt: candidate.UpdatedAt}, nil
}

func (p offeringChangeConflictPort) LockConflictRequest(ctx context.Context, requestID int64) error {
	return p.changes.LockConflictRequest(ctx, requestID)
}

func (p offeringChangeConflictPort) DecideConflictRequest(ctx context.Context, decision carePlanCompose.ConflictDecision) error {
	return p.changes.DecideConflictRequest(ctx, careplan.OfferingConflictDecision{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewerID: decision.ReviewerID, ActorRole: decision.ActorRole, ExpectedVersion: decision.ExpectedVersion,
	})
}

func (p offeringChangeConflictPort) WriteStaffValue(ctx context.Context, write carePlanCompose.StaffValueWrite) error {
	effectiveFrom, selections, err := careplan.ParseOfferingStaffValue(write.Value["value"])
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
	if errors.Is(err, careplan.ErrOfferingChangeNotFound) || errors.Is(err, transactionruntime.ErrNoRows) {
		return careplan.OfferingChangeRequest{}, careplan.ErrOfferingChangeNotFound
	}
	return row, err
}

func (r offeringChangeRows) find(ctx context.Context, id int64, lock bool, op string) (careplan.OfferingChangeRequest, error) {
	row, err := r.carePlan.FindOfferingChange(ctx, id, lock)
	if errors.Is(err, careplan.ErrOfferingChangeNotFound) {
		return careplan.OfferingChangeRequest{}, offeringChangeDatabaseError(op, transactionruntime.ErrNoRows)
	}
	if err != nil {
		return careplan.OfferingChangeRequest{}, offeringChangeDatabaseError(op, err)
	}
	return row, nil
}

func (r offeringChangeRows) PendingForStudent(ctx context.Context, studentID int64) (*careplan.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, careplan.PendingOfferingChanges(studentID))
	if err != nil {
		return nil, offeringChangeDatabaseError("get pending offering change request", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (r offeringChangeRows) ListByStudent(ctx context.Context, studentID int64) ([]careplan.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, careplan.StudentOfferingChangeHistory(studentID))
	if err != nil {
		return nil, offeringChangeDatabaseError("list offering change requests by student", err)
	}
	return rows, nil
}

func (r offeringChangeRows) ListPending(ctx context.Context) ([]careplan.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, careplan.PendingOfferingChanges(0))
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
		if err := row.CheckStoredJSON(); err != nil {
			return nil, err
		}
	}
	return rows, nil
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

// offeringChangeEnrollment reads Enrollment's requests, children, phases and
// booked selections for the review.
type offeringChangeEnrollment struct {
	owner OfferingChangeEnrollmentReads
}

func (e offeringChangeEnrollment) StudentCarePeriods(ctx context.Context, studentID int64) ([]carePlanCompose.OfferingCarePeriod, error) {
	periods, err := enrollmentOwner.StudentCarePeriodRecords(ctx, e.owner, studentID)
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
	return &carePlanCompose.BookingPhase{OfferingPhase: BookingOfferingPhase(phase), CareOfferingSelectionMode: phase.CareOfferingSelectionMode}, nil
}

func (e offeringChangeEnrollment) SelectionsAt(ctx context.Context, childID int64, on calendar.Date) ([]careplan.BookedOffering, error) {
	values, err := e.owner.RequestChildOfferingsAtDate(ctx, childID, enrollmentOwner.Date(on))
	return BookedOfferings(values), err
}

func (e offeringChangeEnrollment) EffectiveSelectionsAt(ctx context.Context, dates map[int64]calendar.Date) ([]careplan.BookedOffering, error) {
	ownerDates := make(map[int64]enrollmentOwner.Date, len(dates))
	for childID, date := range dates {
		ownerDates[childID] = enrollmentOwner.Date(date)
	}
	values, err := e.owner.EffectiveOfferingSelectionsAtDates(ctx, ownerDates)
	return BookedOfferings(values), err
}

func (e offeringChangeEnrollment) CatalogState(ctx context.Context, phaseID, childID int64, on, until calendar.Date) (carePlanCompose.OfferingCatalogState, error) {
	state, err := e.owner.OfferingCatalogState(ctx, phaseID, childID, enrollmentOwner.Date(on), enrollmentOwner.Date(until))
	if err != nil || state == nil {
		return carePlanCompose.OfferingCatalogState{}, err
	}
	return carePlanCompose.OfferingCatalogState{Current: BookedOfferings(state.Current), CapacityPeaks: state.CapacityPeaks}, nil
}

func (e offeringChangeEnrollment) CapacityPeak(ctx context.Context, offeringID int64, excludeChildIDs []int64, from, until calendar.Date) (int, error) {
	return e.owner.OfferingCapacityPeak(ctx, offeringID, excludeChildIDs, enrollmentOwner.Date(from), enrollmentOwner.Date(until))
}

// offeringChangeStudents reads the People Directory students a decision
// authorizes against and their open complete-withdrawal follow-up.
type offeringChangeStudents struct {
	students    offeringChangeStudentRows
	withdrawals offeringChangeWithdrawalRows
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
	_, total, err := s.withdrawals.ListPending(ctx, careplan.WithdrawalListFilter{StudentID: studentID, Page: 1, PageSize: 1})
	if err != nil {
		return false, err
	}
	return total > 0, nil
}

func offeringReviewStudent(row *peopledirectory.StudentState) *carePlanCompose.ReviewStudent {
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

// offeringChangeDatabaseError keeps the error text of the retained request
// repository.
func offeringChangeDatabaseError(op string, err error) error {
	return fmt.Errorf("database error during %s: %w", op, err)
}

// ParentRequestLedger is the one append-only history of every parent request
// (#2267), shared by all four queues so a request's history survives the
// edits, decisions and corrections that overwrite the request rows. People
// Directory owns users.parent_request_events; the queues record through
// their consumer-owned ledger ports, inside the ambient transaction of the
// change an entry describes, so an event can never survive a rolled-back
// write and a committed write can never miss its event.
//
// A nil ledger records nothing, which is what graphs without a ledger need.
type ParentRequestLedger struct {
	repo peopledirectory.ParentRequestEvents
}

var _ carePlanCompose.RequestLedger = (*ParentRequestLedger)(nil)

// Record appends one entry. The entry's version is the request row's
// updated_at AFTER the event, so a reader can tell which version was acted
// on.
func (l *ParentRequestLedger) Record(ctx context.Context, entry carePlanCompose.RequestLedgerEntry) error {
	if l == nil {
		return nil
	}
	if err := peopledirectory.ValidateParentRequestEvent(entry.StudentID, entry.RequestType, entry.RequestID, entry.EventType); err != nil {
		return err
	}
	event := &peopledirectory.ParentRequestEventSnapshot{
		StudentID:   entry.StudentID,
		RequestType: entry.RequestType,
		RequestID:   entry.RequestID,
		EventType:   entry.EventType,
		Version:     careplan.ParentRequestVersion(entry.UpdatedAt),
	}
	if entry.Payload != nil {
		payload, err := json.Marshal(entry.Payload)
		if err != nil {
			return repositoryError("create parent request event", err)
		}
		event.Payload = payload
	}
	if entry.ActorAccountID > 0 {
		actor := entry.ActorAccountID
		event.ActorAccountID = &actor
	}
	return l.repo.AppendParentRequestEvent(ctx, event)
}

// ListForRequest returns one request's history oldest first.
func (l *ParentRequestLedger) ListForRequest(ctx context.Context, requestType string, requestID int64) ([]*peopledirectory.ParentRequestEventSnapshot, error) {
	if l == nil {
		return nil, nil
	}
	return l.repo.ParentRequestEventsForRequest(ctx, requestType, requestID)
}

// NewParentRequestLedger is the request ledger over repo; a nil repo
// records nothing.
func NewParentRequestLedger(repo peopledirectory.ParentRequestEvents) *ParentRequestLedger {
	if repo == nil {
		return nil
	}
	return &ParentRequestLedger{repo: repo}
}

// excusedRequestMessenger maps the workflow's request events onto the
// Communication parent-event emitter.
type excusedRequestMessenger struct {
	emitter *communicationCompose.ParentEventEmitter
}

func (m excusedRequestMessenger) EnqueueRequestDecision(ctx context.Context, tenantID, studentID, guardianAccountID int64, event carePlanCompose.RequestEvent) error {
	return m.emitter.EnqueueRequestDecision(ctx, tenantID, studentID, guardianAccountID, childEvent(event))
}

func (m excusedRequestMessenger) EmitChildEvent(ctx context.Context, tenantID, studentID, guardianAccountID int64, event carePlanCompose.RequestEvent) error {
	return m.emitter.EmitChildEvent(ctx, tenantID, studentID, guardianAccountID, childEvent(event))
}

func (m excusedRequestMessenger) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	m.emitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
}

func (m excusedRequestMessenger) GuardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error) {
	return m.emitter.GuardianHasChildAccess(ctx, studentID, guardianAccountID)
}

func (m excusedRequestMessenger) ResolveDecisionAudience(ctx context.Context, studentID, submitterAccountID int64, sharedAccountIDs []int64) (carePlanCompose.DecisionAudience, error) {
	audience, err := m.emitter.ResolveDecisionAudience(ctx, studentID, submitterAccountID, sharedAccountIDs)
	return carePlanCompose.DecisionAudience{Full: audience.Full, Neutral: audience.Neutral}, err
}

func (m excusedRequestMessenger) EmitDecisionAudience(ctx context.Context, tenantID, studentID int64, audience carePlanCompose.DecisionAudience, full, neutral carePlanCompose.RequestEvent) error {
	return m.emitter.EmitDecisionAudience(ctx, tenantID, studentID, communicationCompose.ParentDecisionAudience{Full: audience.Full, Neutral: audience.Neutral}, childEvent(full), childEvent(neutral))
}

func childEvent(event carePlanCompose.RequestEvent) communicationCompose.ParentChildEvent {
	result := communicationCompose.ParentChildEvent{
		EventType: event.EventType, ActorKind: event.ActorKind, ActorAccountID: event.ActorAccountID, Body: event.Body,
		RequestType: event.RequestType, RequestStatus: event.RequestStatus, DecisionReason: event.DecisionReason,
		RefTable: event.RefTable, Payload: event.Payload,
	}
	if event.RefID > 0 {
		refID := event.RefID
		result.RefID = &refID
	}
	return result
}

// NewExcusedRequestMessenger maps Care Plan's request events onto the
// Communication parent-event emitter, for the root's Stammdaten decision.
func NewExcusedRequestMessenger(emitter *communicationCompose.ParentEventEmitter) carePlanCompose.RequestMessenger {
	return excusedRequestMessenger{emitter: emitter}
}

type BookingSettingsReads interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}
