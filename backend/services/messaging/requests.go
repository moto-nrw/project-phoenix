package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// maxMasterDataFieldLen bounds any single free-text master-data field value (in
// runes) submitted in a change request, so a direct API client cannot persist an
// unbounded payload. Matches the message-body limit.
const maxMasterDataFieldLen = 2000

// studentMasterDataField describes one parent-editable child master-data field.
// This ordered table is the SINGLE source of truth for the field set: the
// create-time allowlist, the confirm-time apply switch (guarded by its default
// case), and the staff/parent diff order+labels all derive from it, so adding
// or renaming a field is a one-line change here instead of four parallel edits.
//
// first_name/last_name/birthday live on the Person, the rest on the Student.
// Medical/health (health_info), consent and internal staff-only fields
// (supervisor_notes) plus operational fields (group, class, tag, photo consent)
// are deliberately excluded — health_info is Art. 9 GDPR data and the operational
// fields are not the parent's to change.
type studentMasterDataField struct {
	Key   string
	Label string // German diff label
}

var studentMasterDataFields = []studentMasterDataField{
	{Key: "first_name", Label: "Vorname"},
	{Key: "last_name", Label: "Nachname"},
	{Key: "birthday", Label: "Geburtsdatum"},
	{Key: "guardian_email", Label: "E-Mail"},
	{Key: "guardian_phone", Label: "Telefon"},
	{Key: "extra_info", Label: "Hinweise"},
}

func isAllowedStudentMasterDataField(field string) bool {
	for _, f := range studentMasterDataFields {
		if f.Key == field {
			return true
		}
	}
	return false
}

// requestHandler is the SINGLE per-request-type behavior table. Every aspect
// of a request type lives here — the immutable guardian-facing body, the
// confirm system-event body, payload validation, the staff/parent diff, the
// apply, and the audit summary — so adding a new request type is one registry
// entry instead of touching five parallel switch statements that would silently
// no-op the ones you forget. TestRequestRegistryComplete asserts no field is
// nil/empty, turning a forgotten aspect into a failing test.
type requestHandler struct {
	// requestBody is the immutable guardian-facing message body for this type.
	requestBody string
	// confirmBody is the system-event body appended when staff confirm.
	confirmBody string
	apply       func(context.Context, *service, *usersModels.ParentMessage, int64) error
	validate    func(payload map[string]any) error
	// canonicalize validates AND returns the sanitized payload that is safe to
	// persist: unknown keys dropped, duplicate/over-large structures collapsed to
	// one canonical entry per aspect. The create path stores THIS, never the raw
	// guardian-supplied map, so a direct API client cannot persist ignored junk
	// that every later thread load would echo back. validate is the same logic
	// with the result discarded, so the two cannot drift.
	canonicalize func(payload map[string]any) (map[string]any, error)
	// diff is a method expression on *service, so the receiver is the first arg.
	diff         func(*service, context.Context, *diffSource, map[string]any) ([]RequestDiffEntry, error)
	auditSummary func(*usersModels.ParentMessage) string
}

var parentMessageRequestRegistry = map[string]requestHandler{
	usersModels.ParentMessageRequestCareSchedule: {
		requestBody:  "Anfrage: Dauerhafte Betreuungszeiten ändern",
		confirmBody:  "Anfrage bestätigt, Betreuungszeiten übernommen",
		apply:        applyCareScheduleRequest,
		validate:     validateCareSchedulePayload,
		canonicalize: canonicalizeCareSchedulePayload,
		diff:         (*service).careScheduleDiffFrom,
		auditSummary: careScheduleAuditSummary,
	},
	usersModels.ParentMessageRequestStudentMasterData: {
		requestBody:  "Anfrage: Stammdaten ändern",
		confirmBody:  "Anfrage bestätigt, Stammdaten übernommen",
		apply:        applyStudentMasterDataRequest,
		validate:     validateMasterDataPayload,
		canonicalize: canonicalizeMasterDataPayload,
		diff:         (*service).masterDataDiffFrom,
		auditSummary: masterDataAuditSummary,
	},
}

// IsKnownRequestType reports whether requestType has a registered handler. The
// parent create path uses it so its allowlist derives from the same registry.
func IsKnownRequestType(requestType string) bool {
	_, ok := parentMessageRequestRegistry[requestType]
	return ok
}

// RequestBody returns the immutable guardian-facing message body for a request
// type, falling back to a generic label for an unknown type.
func RequestBody(requestType string) string {
	if h, ok := parentMessageRequestRegistry[requestType]; ok && h.requestBody != "" {
		return h.requestBody
	}
	return "Anfrage"
}

// careWeekdayPayload is one weekday (1=Mon..5=Fri) of the parent's desired
// weekly plan. Every field is optional: an empty Mode/Arrival/Pickup means
// "leave that aspect of this weekday unchanged", so the apply merges the
// requested changes onto the child's current plan instead of replacing it.
type careWeekdayPayload struct {
	Weekday int    `json:"weekday"`
	Mode    string `json:"mode,omitempty"`    // alone|bus|pickup, "" = unchanged
	Arrival string `json:"arrival,omitempty"` // HH:MM, "" = unchanged
	Pickup  string `json:"pickup,omitempty"`  // HH:MM, "" = unchanged
}

type careSchedulePayload struct {
	Weekdays []careWeekdayPayload `json:"weekdays"`
}

// careScheduleChanges is the validated, bucketed form of a care-schedule
// payload: per-weekday departure modes (keyed by abbreviation), arrival times
// and pickup times (keyed by weekday number, both "HH:MM"). Produced by
// buildCareScheduleChanges and consumed by both the create-time validation and
// the confirm-time apply.
type careScheduleChanges struct {
	modes    map[string]usersModels.DepartureMode
	arrivals map[int]string
	pickups  map[int]string
}

func (c careScheduleChanges) isEmpty() bool {
	return len(c.modes) == 0 && len(c.arrivals) == 0 && len(c.pickups) == 0
}

// toCanonicalPayload rebuilds the persistable payload from the bucketed changes:
// exactly one entry per weekday (1=Mon..5=Fri) that has a change, in fixed
// weekday order, carrying only the merged mode/arrival/pickup. This is what
// collapses a direct API client's duplicate or out-of-order weekday entries
// into the same form the apply path already derives.
func (c careScheduleChanges) toCanonicalPayload() careSchedulePayload {
	weekdays := make([]careWeekdayPayload, 0, len(usersModels.PickupDayOrder))
	for i, abbrev := range usersModels.PickupDayOrder {
		wd := i + 1
		entry := careWeekdayPayload{Weekday: wd}
		present := false
		if mode, ok := c.modes[abbrev]; ok {
			entry.Mode = string(mode)
			present = true
		}
		if arrival, ok := c.arrivals[wd]; ok {
			entry.Arrival = arrival
			present = true
		}
		if pickup, ok := c.pickups[wd]; ok {
			entry.Pickup = pickup
			present = true
		}
		if present {
			weekdays = append(weekdays, entry)
		}
	}
	return careSchedulePayload{Weekdays: weekdays}
}

type studentMasterDataPayload struct {
	Fields map[string]*string `json:"fields"`
}

func decodePayload[T any](payload map[string]any) (T, error) {
	var out T
	raw, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (s *service) ConfirmRequest(ctx context.Context, requestID int64) ([]*usersModels.ParentMessage, error) {
	request, thread, err := s.loadAuthorizedRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}

	handler, ok := parentMessageRequestRegistry[request.RequestType]
	if !ok {
		return nil, ErrInvalidRequestType
	}

	// Confirming applies the requested change AND writes a parent-visible
	// "bestätigt" event + broadcast, so the guardian must still be linked with
	// parent_portal.access. A staffer who wants to dispose of a revoked
	// guardian's request rejects it instead (see loadAuthorizedRequest).
	if err := s.requireLinkedGuardian(ctx, thread); err != nil {
		return nil, err
	}

	accountID := accountIDFromCtx(ctx)
	if err := s.requireStudentWrite(ctx, request.StudentID); err != nil {
		return nil, err
	}

	// Lock the request row for the rest of the request transaction and re-read
	// its status under the lock. Two staff confirming the same request now
	// serialize: the second blocks here until the first commits, then sees the
	// decided status and returns ErrInvalidRequestStatus instead of applying the
	// schedule/master-data change a second time.
	request, err = s.lockOpenRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	// apply, the status update and the system event all run in the ambient
	// request transaction (TenantTxMiddleware), which only rolls back on a 5xx.
	// A mid-apply failure must therefore propagate as a plain error (→ 500) so
	// the WHOLE transaction rolls back — masking it as ErrRequestNoLongerPossible
	// (409) would COMMIT a half-applied change and leave the child inconsistent.
	if err := handler.apply(ctx, s, request, accountID); err != nil {
		return nil, fmt.Errorf("messaging: apply request %d: %w", requestID, err)
	}

	now := time.Now()
	request.RequestStatus = usersModels.ParentMessageRequestStatusDone
	request.AppliedAt = &now
	request.AppliedBy = &accountID
	if err := s.messageRepo.Update(ctx, request); err != nil {
		return nil, fmt.Errorf("messaging: update request: %w", err)
	}
	if err := s.appendSystemEvent(ctx, thread, accountID, "request_status", confirmEventBody(request.RequestType)); err != nil {
		return nil, err
	}
	// Emit the "applied" audit line only AFTER the tenant transaction actually
	// commits. The ListByThread below (and any later statement) can still fail —
	// TenantTxMiddleware then rolls the whole apply back on the 5xx — and an audit
	// record for a change that never persisted would be a phantom. Registering it
	// post-commit (like broadcastAfterCommit) ties the log to the durable outcome.
	tenant.RegisterAfterCommit(ctx, func() {
		s.recordApplyAudit(request, accountID)
	})
	s.broadcastAfterCommit(ctx, thread)
	return s.messageRepo.ListByThread(ctx, thread.ID, 0)
}

func (s *service) RejectRequest(ctx context.Context, requestID int64, reason string) ([]*usersModels.ParentMessage, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrRejectReasonRequired
	}
	// Bound the reason like every other free-text field: staff/API clients reach
	// this directly, so an unbounded reason would be persisted as decision_reason
	// AND duplicated into the system-event body, making every later thread load
	// carry the payload. Count runes so German umlauts are not penalized.
	if utf8.RuneCountInString(reason) > maxMasterDataFieldLen {
		return nil, ErrRejectReasonTooLong
	}
	// The request row is re-read under the lock below; the initial load is used
	// for authorization (thread read access + the student write check).
	authRequest, thread, err := s.loadAuthorizedRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	// NOTE: deliberately no requireEnabled() gate here — and, for the same
	// reason, no requireLinkedGuardian() gate. Rejecting (and the parent-side
	// withdraw) must stay available after an admin turns
	// operations.parent_notes_enabled OFF *or* after the guardian is unlinked /
	// loses parent_portal.access, so outstanding requests can be wound down
	// instead of frozen forever in the open-request queue. Confirm remains gated
	// on both because it applies a change (and notifies a recipient) a disabled
	// feature / revoked guardian should not receive.
	// Rejecting writes a terminal, parent-visible "Anfrage abgelehnt" decision,
	// so it requires write access to the child — symmetric with ConfirmRequest
	// (the accept side). Without this, under gdpr.student_data_scope='all_staff'
	// any read-only staffer could terminally reject another supervisor's request.
	if err := s.requireStudentWrite(ctx, authRequest.StudentID); err != nil {
		return nil, err
	}
	// Lock + re-read under the lock so a reject racing a concurrent confirm
	// cannot flip an already-applied request to "rejected" (which would leave
	// the schedule/master-data change applied but the request marked rejected).
	request, err := s.lockOpenRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	request.RequestStatus = usersModels.ParentMessageRequestStatusRejected
	request.DecisionReason = reason
	if err := s.messageRepo.Update(ctx, request); err != nil {
		return nil, fmt.Errorf("messaging: update request: %w", err)
	}
	if err := s.appendSystemEvent(ctx, thread, accountIDFromCtx(ctx), "request_status", "Anfrage abgelehnt: "+reason); err != nil {
		return nil, err
	}
	// Reject deliberately skips requireLinkedGuardian (so staff can wind down a
	// revoked guardian's stuck request — see loadAuthorizedRequest), but the SSE
	// wake-up must NOT leak this child's thread/student ids to a guardian who has
	// been unlinked / lost parent_portal.access: that is the exact leak
	// requireLinkedGuardian closes on the confirm/post paths. So only broadcast
	// when the guardian is still linked. A transient link-check failure fails
	// closed (no broadcast). The acting staffer's own view already updates from
	// this call's return value, so suppressing the fan-out on this rare cleanup
	// reject only costs OTHER open tabs an auto-refresh.
	if s.requireLinkedGuardian(ctx, thread) == nil {
		s.broadcastAfterCommit(ctx, thread)
	}
	return s.messageRepo.ListByThread(ctx, thread.ID, 0)
}

func (s *service) loadAuthorizedRequest(ctx context.Context, requestID int64) (*usersModels.ParentMessage, *usersModels.ParentMessageThread, error) {
	request, err := s.messageRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, nil, fmt.Errorf("messaging: find request: %w", err)
	}
	if request == nil || request.Kind != usersModels.ParentMessageKindRequest {
		return nil, nil, ErrRequestNotFound
	}
	thread, err := s.loadAuthorizedThread(ctx, request.ThreadID)
	if err != nil {
		if errors.Is(err, ErrThreadNotFound) {
			return nil, nil, ErrRequestNotFound
		}
		return nil, nil, err
	}
	// NOTE: the guardian-link gate (requireLinkedGuardian) is deliberately NOT
	// applied here. It belongs only on the confirm path, which writes a
	// parent-visible event / applies a change for a recipient the parent APIs now
	// hide. RejectRequest must stay
	// available regardless, so staff can wind down an outstanding request after
	// the guardian is unlinked or loses parent_portal.access; gating reject too
	// would leave the request permanently stuck in the open-request queue with no
	// way to close it (the parent withdraw path is hidden as well). This mirrors
	// the requireEnabled() gating: applied on advance, skipped on reject.
	return request, thread, nil
}

// lockOpenRequest re-reads the request row FOR UPDATE and re-validates, under
// the lock, that it is still a decidable ('offen') request — so the two decision
// paths (confirm / reject) share ONE race-safe guard instead of copies that
// could drift when the lifecycle gains a state. Two staff acting on the same
// request serialize here: the second blocks until the first commits, then sees
// the decided status and gets ErrInvalidRequestStatus rather than
// applying/flipping it twice.
func (s *service) lockOpenRequest(ctx context.Context, requestID int64) (*usersModels.ParentMessage, error) {
	locked, err := s.messageRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("messaging: lock request: %w", err)
	}
	if locked == nil || locked.Kind != usersModels.ParentMessageKindRequest {
		return nil, ErrRequestNotFound
	}
	if locked.RequestStatus != usersModels.ParentMessageRequestStatusOpen {
		return nil, ErrInvalidRequestStatus
	}
	return locked, nil
}

func (s *service) requireStudentWrite(ctx context.Context, studentID int64) error {
	student, err := s.persons.GetStudentByID(ctx, studentID)
	if err != nil {
		// Transient lookup failure → server error (500, retryable), not a
		// permanent 403. Only a missing student is an authorization failure.
		return fmt.Errorf("messaging: load student for write check: %w", err)
	}
	if student == nil {
		return ErrForbidden
	}
	ok, err := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.userContext)
	if err != nil {
		return fmt.Errorf("messaging: check student write access: %w", err)
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *service) appendSystemEvent(ctx context.Context, thread *usersModels.ParentMessageThread, actorAccountID int64, eventType, body string) error {
	msg := &usersModels.ParentMessage{
		ThreadID:        thread.ID,
		StudentID:       thread.StudentID,
		SenderAccountID: actorAccountID,
		SenderKind:      usersModels.ParentMessageSenderSystem,
		SenderName:      "System",
		Body:            body,
		Kind:            usersModels.ParentMessageKindEvent,
		EventType:       eventType,
		// Staff-triggered decision (confirm / reject). Attribute the
		// event to the staff side explicitly so the guardian's unread badge fires,
		// even when this staffer is ALSO the thread's guardian (dual-role account).
		EventActorKind: usersModels.ParentMessageSenderStaff,
	}
	msg.SetTenantID(thread.TenantID)
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return fmt.Errorf("messaging: create system event: %w", err)
	}
	// Touch the thread with the event row's own DB-stamped created_at, not a Go
	// time.Now(): messages.created_at is the Postgres clock, so mixing in the app
	// clock here would let the monotonic preview guard compare values from two
	// clocks and pick the wrong "latest" under host skew (see appendStaffMessage).
	// All staff-side system events (confirmed / rejected) are a
	// staff response: flag the thread as last-touched by staff so the inbox
	// stops listing a decided request as "awaiting OGS reply".
	// Guarded, monotonic update so a concurrent newer guardian message keeps
	// owning the inbox preview/order (see TouchLastMessage): a decided request
	// that committed later but with an older instant must not reorder the thread.
	if err := s.threadRepo.TouchLastMessage(ctx, thread.ID, msg.CreatedAt, msg.ID, usersModels.ParentMessageSenderStaff, body); err != nil {
		return fmt.Errorf("messaging: update thread: %w", err)
	}
	return nil
}

// parseWallClock turns an "HH:MM" string into the normalized wall-clock
// time.Time the schedule TIME columns store. Routing through timezone.WallClock
// is the mandated normalization for TIME columns (CLAUDE.md rule 11) and
// replaces the previous hand-rolled 2000-01-01 anchoring. Preserved rows
// carried through a merge must be re-normalized the same way before re-insert,
// since TIME columns scan back with a driver-chosen year that Postgres rejects.
func parseWallClock(hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	return timezone.WallClock(t), nil
}

// applyCareScheduleRequest sets the child's permanent weekly plan from the
// parent's request: per weekday a departure mode and/or arrival/pickup time.
// It MERGES onto the current plan — only the weekdays/aspects the parent filled
// in are changed, the rest are preserved — so a request that touches Friday
// never wipes Monday. The acting staff member (the confirmer) is stamped as
// CreatedBy on any schedule row it writes.
func applyCareScheduleRequest(ctx context.Context, s *service, request *usersModels.ParentMessage, _ int64) error {
	if s.students == nil || s.arrival == nil || s.pickup == nil || s.userContext == nil {
		return errors.New("schedule services not configured")
	}
	// Re-validate and bucket the payload through the SAME parser the create path
	// uses, so the two paths cannot drift on weekday range, mode enum, or time
	// format.
	changes, err := buildCareScheduleChanges(request.Payload)
	if err != nil {
		return err
	}
	if changes.isEmpty() {
		return errors.New("no weekdays to apply")
	}

	staff, err := s.userContext.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return fmt.Errorf("resolve acting staff: %w", err)
	}
	staffID := staff.ID
	studentID := request.StudentID

	// Lock the student row up front (held until the request tx commits) so two
	// staff confirming two DIFFERENT requests for the same child serialize: the
	// arrival/pickup applies below are delete-all-then-reinsert read-modify-
	// writes that the per-request row lock does not cover, so without this lock
	// the second confirm would read the same pre-change snapshot and overwrite
	// the first. The locked student is reused for the departure-mode merge.
	student, err := s.students.GetByIDForUpdate(ctx, studentID)
	if err != nil {
		return err
	}

	if err := s.applyDepartureModeChanges(ctx, student, changes.modes); err != nil {
		return err
	}
	if err := s.applyArrivalChanges(ctx, studentID, staffID, changes.arrivals); err != nil {
		return err
	}
	return s.applyPickupChanges(ctx, studentID, staffID, changes.pickups)
}

// applyDepartureModeChanges merges the requested per-weekday departure modes
// onto the (already row-locked) student's AllowedDepartureModes, changing ONLY
// the touched weekdays. The parent request carries a single mode per day, so a
// touched day becomes exactly that one allowed mode; untouched days keep every
// allowed mode they had. This is the fix for the collapse bug: writing through
// DepartureDays (the exclusive projection) and clearing AllowedDepartureModes
// made the repository re-derive single-mode allowed sets for ALL days, silently
// dropping staff-configured multi-mode days (e.g. bus+pickup) on any confirm.
// The derived exclusive/legacy fields are cleared so the repository re-derives
// them from the merged allowed modes (the single source of truth).
func (s *service) applyDepartureModeChanges(ctx context.Context, student *usersModels.Student, changes map[string]usersModels.DepartureMode) error {
	if len(changes) == 0 {
		return nil
	}
	merged := usersModels.AllowedDepartureModes{}
	for day, modes := range student.AllowedDepartureModes {
		merged[day] = append([]usersModels.DepartureMode(nil), modes...)
	}
	for day, mode := range changes {
		merged[day] = []usersModels.DepartureMode{mode}
	}
	student.AllowedDepartureModes = merged
	student.DepartureDays = nil
	student.BusDays = nil
	student.PickupDays = nil
	if err := student.Validate(); err != nil {
		return err
	}
	return s.students.Update(ctx, student)
}

func (s *service) applyArrivalChanges(ctx context.Context, studentID, staffID int64, changes map[int]string) error {
	if len(changes) == 0 {
		return nil
	}
	current, err := s.arrival.GetStudentArrivalSchedules(ctx, studentID)
	if err != nil {
		return err
	}
	byDay := map[int]*scheduleModels.StudentArrivalSchedule{}
	for _, sc := range current {
		byDay[sc.Weekday] = sc
	}
	for weekday, hhmm := range changes {
		// Do NOT discard the parse error: buildCareScheduleChanges validates the
		// same string on the create path, but if any future caller reaches apply
		// without that pre-validation, an unparseable time would otherwise become
		// the zero time.Time and silently overwrite the real arrival with 00:00.
		t, err := parseWallClock(hhmm)
		if err != nil {
			return fmt.Errorf("apply arrival weekday %d: %w", weekday, err)
		}
		byDay[weekday] = &scheduleModels.StudentArrivalSchedule{
			StudentID:       studentID,
			Weekday:         weekday,
			ExpectedArrival: t,
			CreatedBy:       staffID,
		}
	}
	merged := make([]*scheduleModels.StudentArrivalSchedule, 0, len(byDay))
	for _, sc := range byDay {
		sc.ExpectedArrival = timezone.WallClock(sc.ExpectedArrival)
		merged = append(merged, sc)
	}
	return s.arrival.UpsertBulkStudentArrivalSchedules(ctx, studentID, merged)
}

func (s *service) applyPickupChanges(ctx context.Context, studentID, staffID int64, changes map[int]string) error {
	if len(changes) == 0 {
		return nil
	}
	current, err := s.pickup.GetStudentPickupSchedules(ctx, studentID)
	if err != nil {
		return err
	}
	byDay := map[int]*scheduleModels.StudentPickupSchedule{}
	for _, sc := range current {
		byDay[sc.Weekday] = sc
	}
	for weekday, hhmm := range changes {
		// See applyArrivalChanges: propagate the parse error rather than letting an
		// unparseable time silently become 00:00 on the child's pickup schedule.
		t, err := parseWallClock(hhmm)
		if err != nil {
			return fmt.Errorf("apply pickup weekday %d: %w", weekday, err)
		}
		byDay[weekday] = &scheduleModels.StudentPickupSchedule{
			StudentID:  studentID,
			Weekday:    weekday,
			PickupTime: t,
			CreatedBy:  staffID,
		}
	}
	merged := make([]*scheduleModels.StudentPickupSchedule, 0, len(byDay))
	for _, sc := range byDay {
		sc.PickupTime = timezone.WallClock(sc.PickupTime)
		merged = append(merged, sc)
	}
	return s.pickup.UpsertBulkStudentPickupSchedules(ctx, studentID, merged)
}

func applyStudentMasterDataRequest(ctx context.Context, s *service, request *usersModels.ParentMessage, _ int64) error {
	if s.students == nil || s.persons == nil {
		return errors.New("student/person service not configured")
	}
	payload, err := decodePayload[studentMasterDataPayload](request.Payload)
	if err != nil {
		return err
	}
	if len(payload.Fields) == 0 {
		return errors.New("no fields to apply")
	}
	student, err := s.students.GetByIDForUpdate(ctx, request.StudentID)
	if err != nil {
		return err
	}

	// Person fields (name, birthday) are loaded lazily — only when the request
	// actually touches one — to avoid an unnecessary read for contact-only
	// changes.
	var person *usersModels.Person
	loadPerson := func() error {
		if person != nil {
			return nil
		}
		p, err := s.persons.Get(ctx, student.PersonID)
		if err != nil {
			return err
		}
		if p == nil {
			return errors.New("person not found")
		}
		person = p
		return nil
	}

	personChanged, studentChanged := false, false
	for field, value := range payload.Fields {
		if !isAllowedStudentMasterDataField(field) {
			return fmt.Errorf("field %q is not allowed", field)
		}
		switch field {
		case "first_name", "last_name":
			name := ""
			if value != nil {
				name = strings.TrimSpace(*value)
			}
			if name == "" {
				return fmt.Errorf("field %q must not be empty", field)
			}
			if err := loadPerson(); err != nil {
				return err
			}
			if field == "first_name" {
				person.FirstName = name
			} else {
				person.LastName = name
			}
			personChanged = true
		case "birthday":
			if err := loadPerson(); err != nil {
				return err
			}
			if value == nil || strings.TrimSpace(*value) == "" {
				person.Birthday = nil
			} else {
				d, err := timezone.ParseDate(strings.TrimSpace(*value))
				if err != nil {
					return fmt.Errorf("invalid birthday: %w", err)
				}
				person.Birthday = &d
			}
			personChanged = true
		case "guardian_email":
			student.GuardianEmail = normalizedStringPtr(value)
			studentChanged = true
		case "guardian_phone":
			student.GuardianPhone = normalizedStringPtr(value)
			studentChanged = true
		case "extra_info":
			student.ExtraInfo = normalizedStringPtr(value)
			studentChanged = true
		default:
			// The field passed the allowlist (studentMasterDataFields) but has no
			// apply branch here — fail loudly rather than silently no-op, so a
			// newly-added field can never validate-but-never-apply.
			return fmt.Errorf("field %q has no apply handler", field)
		}
	}

	if studentChanged {
		if err := student.Validate(); err != nil {
			return err
		}
		if err := s.students.Update(ctx, student); err != nil {
			return err
		}
	}
	if personChanged {
		if err := person.Validate(); err != nil {
			return err
		}
		if err := s.persons.Update(ctx, person); err != nil {
			return err
		}
	}
	return nil
}

func normalizedStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func confirmEventBody(requestType string) string {
	if h, ok := parentMessageRequestRegistry[requestType]; ok && h.confirmBody != "" {
		return h.confirmBody
	}
	return "Anfrage bestätigt"
}

// recordApplyAudit writes the audit trail for an applied request. A durable
// per-child change-audit table does not exist yet (#1455 owns that); until then
// this is a structured, GDPR-safe slog record: it carries the actor, child,
// tenant and WHICH fields/weekdays changed — never the values, since names and
// contact details must not land in logs. (Previously this misused the GDPR
// data-ACCESS log to record a mutation, which would have polluted access
// reports.)
func (s *service) recordApplyAudit(request *usersModels.ParentMessage, accountID int64) {
	s.logger.Info("parent request applied",
		slog.Int64("request_id", request.ID),
		slog.Int64("student_id", request.StudentID),
		slog.Int64("tenant_id", request.TenantID),
		slog.Int64("account_id", accountID),
		slog.String("request_type", request.RequestType),
		slog.String("changed", auditChangeSummary(request)),
	)
}

// auditChangeSummary lists the changed field keys (master data) or weekday
// numbers (care schedule) — keys only, no values — for the audit log, via the
// per-type summary in the registry.
func auditChangeSummary(request *usersModels.ParentMessage) string {
	if h, ok := parentMessageRequestRegistry[request.RequestType]; ok && h.auditSummary != nil {
		return h.auditSummary(request)
	}
	return ""
}

func masterDataAuditSummary(request *usersModels.ParentMessage) string {
	p, err := decodePayload[studentMasterDataPayload](request.Payload)
	if err != nil {
		return ""
	}
	keys := make([]string, 0, len(p.Fields))
	for k := range p.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func careScheduleAuditSummary(request *usersModels.ParentMessage) string {
	p, err := decodePayload[careSchedulePayload](request.Payload)
	if err != nil {
		return ""
	}
	days := make([]string, 0, len(p.Weekdays))
	for _, wd := range p.Weekdays {
		days = append(days, strconv.Itoa(wd.Weekday))
	}
	return "weekdays:" + strings.Join(days, ",")
}

// BuildRequestDiffs computes, for every still-open request in the thread, a
// human-readable "current → requested" comparison so the reader can see what
// the change would do without re-deriving it. Used by both the staff review
// card and the parent's own request card (the guardian sees what they asked
// to change, not just a status). Closed/applied requests get no diff (the
// change already happened). A diff that can't be built is logged and skipped
// rather than failing the whole thread load.
//
// All open requests in one call share a single diffSource, so the underlying
// student / person / schedule records are read at most once even when several
// requests are open. The cache is deliberately per-call: the "current" side of
// a diff must reflect live data because staff confirm acts on it, so caching
// across thread loads would be unsafe.
func (s *service) BuildRequestDiffs(ctx context.Context, studentID int64, messages []*usersModels.ParentMessage) map[int64][]RequestDiffEntry {
	out := map[int64][]RequestDiffEntry{}
	src := &diffSource{s: s, studentID: studentID}
	for _, m := range messages {
		if m.Kind != usersModels.ParentMessageKindRequest {
			continue
		}
		if m.RequestStatus != usersModels.ParentMessageRequestStatusOpen {
			continue
		}
		handler, ok := parentMessageRequestRegistry[m.RequestType]
		if !ok || handler.diff == nil {
			continue
		}
		entries, err := handler.diff(s, ctx, src, m.Payload)
		if err != nil {
			s.logger.Warn("messaging: build request diff failed",
				slog.Int64("request_id", m.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		if len(entries) > 0 {
			out[m.ID] = entries
		}
	}
	return out
}

// diffSource lazily loads and memoizes the student records a request diff
// compares against. Each getter loads its record at most once, so building the
// diffs for several open requests in one thread (e.g. a care-schedule and a
// master-data request) reads the student a single time. Not safe to reuse
// across BuildRequestDiffs calls — the values must be live per load.
type diffSource struct {
	s         *service
	studentID int64

	student     *usersModels.Student
	studentErr  error
	studentDone bool

	person     *usersModels.Person
	personErr  error
	personDone bool

	arrivalMap  map[int]string
	arrivalDone bool

	pickupMap  map[int]string
	pickupDone bool
}

func (d *diffSource) getStudent(ctx context.Context) (*usersModels.Student, error) {
	if !d.studentDone {
		d.studentDone = true
		student, err := d.s.persons.GetStudentByID(ctx, d.studentID)
		if err != nil || student == nil {
			d.studentErr = fmt.Errorf("messaging: load student for diff: %w", err)
		} else {
			d.student = student
		}
	}
	return d.student, d.studentErr
}

func (d *diffSource) getPerson(ctx context.Context) (*usersModels.Person, error) {
	if !d.personDone {
		d.personDone = true
		student, err := d.getStudent(ctx)
		if err != nil {
			d.personErr = err
		} else if person, err := d.s.persons.Get(ctx, student.PersonID); err != nil || person == nil {
			d.personErr = fmt.Errorf("messaging: load person for diff: %w", err)
		} else {
			d.person = person
		}
	}
	return d.person, d.personErr
}

// getArrival / getPickup are best-effort: a load failure yields an empty map so
// the diff shows "—" as the current value rather than failing the whole diff
// (matches the prior behavior).
func (d *diffSource) getArrival(ctx context.Context) map[int]string {
	if !d.arrivalDone {
		d.arrivalDone = true
		d.arrivalMap = map[int]string{}
		if cur, err := d.s.arrival.GetStudentArrivalSchedules(ctx, d.studentID); err == nil {
			for _, a := range cur {
				d.arrivalMap[a.Weekday] = a.ExpectedArrival.Format("15:04")
			}
		}
	}
	return d.arrivalMap
}

func (d *diffSource) getPickup(ctx context.Context) map[int]string {
	if !d.pickupDone {
		d.pickupDone = true
		d.pickupMap = map[int]string{}
		if cur, err := d.s.pickup.GetStudentPickupSchedules(ctx, d.studentID); err == nil {
			for _, pc := range cur {
				d.pickupMap[pc.Weekday] = pc.PickupTime.Format("15:04")
			}
		}
	}
	return d.pickupMap
}

func (s *service) masterDataDiffFrom(ctx context.Context, src *diffSource, payload map[string]any) ([]RequestDiffEntry, error) {
	p, err := decodePayload[studentMasterDataPayload](payload)
	if err != nil {
		return nil, err
	}
	student, err := src.getStudent(ctx)
	if err != nil {
		return nil, err
	}
	person, err := src.getPerson(ctx)
	if err != nil {
		return nil, err
	}

	var entries []RequestDiffEntry
	for _, f := range studentMasterDataFields {
		val, ok := p.Fields[f.Key]
		if !ok {
			continue
		}
		var oldVal string
		switch f.Key {
		case "first_name":
			oldVal = person.FirstName
		case "last_name":
			oldVal = person.LastName
		case "birthday":
			if person.Birthday != nil {
				oldVal = person.Birthday.String()
			}
		case "guardian_email":
			oldVal = derefString(student.GuardianEmail)
		case "guardian_phone":
			oldVal = derefString(student.GuardianPhone)
		case "extra_info":
			oldVal = derefString(student.ExtraInfo)
		}
		entries = append(entries, RequestDiffEntry{
			Label: f.Label,
			Old:   dashIfEmpty(oldVal),
			New:   dashIfEmpty(strings.TrimSpace(derefString(val))),
		})
	}
	return entries, nil
}

func (s *service) careScheduleDiffFrom(ctx context.Context, src *diffSource, payload map[string]any) ([]RequestDiffEntry, error) {
	p, err := decodePayload[careSchedulePayload](payload)
	if err != nil {
		return nil, err
	}
	student, err := src.getStudent(ctx)
	if err != nil {
		return nil, err
	}

	arrivalMap := src.getArrival(ctx)
	pickupMap := src.getPickup(ctx)

	weekdays := append([]careWeekdayPayload(nil), p.Weekdays...)
	sort.Slice(weekdays, func(i, j int) bool { return weekdays[i].Weekday < weekdays[j].Weekday })

	var entries []RequestDiffEntry
	for _, wd := range weekdays {
		if wd.Weekday < 1 || wd.Weekday > 5 {
			continue
		}
		name := scheduleModels.WeekdayNames[wd.Weekday]
		abbrev := usersModels.PickupDayOrder[wd.Weekday-1]
		if wd.Mode != "" {
			entries = append(entries, RequestDiffEntry{
				Label: name + " · Abholart",
				// "Current" must read AllowedDepartureModes — the non-exclusive field
				// the apply path actually mutates — not the lossy single-mode
				// DepartureDays projection. Otherwise a day allowing {bus, pickup}
				// renders as one mode, a parent re-requesting that mode looks like a
				// no-op, and confirming silently drops the other allowed mode.
				Old: germanAllowedDepartureModes(student.AllowedDepartureModes[abbrev]),
				New: usersModels.DepartureMode(wd.Mode).GermanLabel(),
			})
		}
		if wd.Arrival != "" {
			entries = append(entries, RequestDiffEntry{
				Label: name + " · Bringzeit",
				Old:   dashIfEmpty(arrivalMap[wd.Weekday]),
				New:   wd.Arrival,
			})
		}
		if wd.Pickup != "" {
			entries = append(entries, RequestDiffEntry{
				Label: name + " · Abholzeit",
				Old:   dashIfEmpty(pickupMap[wd.Weekday]),
				New:   wd.Pickup,
			})
		}
	}
	return entries, nil
}

// ValidateRequestPayload checks a parent-submitted request payload for shape
// and policy (allowed fields, valid weekdays/modes/times) WITHOUT touching the
// database, so the parent create path can reject bad input immediately instead
// of letting it sit until a staff member tries to confirm it. The apply path
// still re-validates against live data (capacity/staleness). It is the single
// source of truth for the field allowlist shared with the apply path.
func ValidateRequestPayload(requestType string, payload map[string]any) error {
	handler, ok := parentMessageRequestRegistry[requestType]
	if !ok || handler.validate == nil {
		return ErrInvalidRequestType
	}
	return handler.validate(payload)
}

// CanonicalizeRequestPayload validates the payload AND returns the sanitized
// canonical form that is safe to persist. It is the create path's single entry
// point: storing the raw guardian-supplied map would let a direct API client
// persist unknown keys or duplicate/over-large structures that validation
// otherwise ignores, and every later thread load would echo them back. The
// apply path re-derives the same canonical shape, so storing it changes nothing
// for staff confirmation.
func CanonicalizeRequestPayload(requestType string, payload map[string]any) (map[string]any, error) {
	handler, ok := parentMessageRequestRegistry[requestType]
	if !ok || handler.canonicalize == nil {
		return nil, ErrInvalidRequestType
	}
	return handler.canonicalize(payload)
}

// structToMap round-trips a typed payload struct through JSON into a generic
// map for JSONB storage, dropping any keys not declared on the struct.
func structToMap(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// buildCareScheduleChanges validates a care-schedule payload (weekday range,
// departure-mode enum, HH:MM format) and buckets it into the per-aspect change
// maps. It is the SINGLE parser shared by the create-time validation and the
// confirm-time apply, so the two paths cannot drift. Validation failures wrap
// ErrInvalidRequestPayload; the apply path treats any non-nil error as a 500.
func buildCareScheduleChanges(payload map[string]any) (careScheduleChanges, error) {
	p, err := decodePayload[careSchedulePayload](payload)
	if err != nil {
		return careScheduleChanges{}, ErrInvalidRequestPayload
	}
	out := careScheduleChanges{
		modes:    map[string]usersModels.DepartureMode{},
		arrivals: map[int]string{},
		pickups:  map[int]string{},
	}
	for _, wd := range p.Weekdays {
		if wd.Weekday < 1 || wd.Weekday > 5 {
			return careScheduleChanges{}, fmt.Errorf("%w: weekday %d", ErrInvalidRequestPayload, wd.Weekday)
		}
		abbrev := usersModels.PickupDayOrder[wd.Weekday-1]
		if wd.Mode != "" {
			mode := usersModels.DepartureMode(wd.Mode)
			switch mode {
			case usersModels.DepartureAlone, usersModels.DepartureBus, usersModels.DeparturePickup:
				out.modes[abbrev] = mode
			default:
				return careScheduleChanges{}, fmt.Errorf("%w: mode %q", ErrInvalidRequestPayload, wd.Mode)
			}
		}
		if wd.Arrival != "" {
			if _, err := parseWallClock(wd.Arrival); err != nil {
				return careScheduleChanges{}, fmt.Errorf("%w: arrival %q", ErrInvalidRequestPayload, wd.Arrival)
			}
			out.arrivals[wd.Weekday] = wd.Arrival
		}
		if wd.Pickup != "" {
			if _, err := parseWallClock(wd.Pickup); err != nil {
				return careScheduleChanges{}, fmt.Errorf("%w: pickup %q", ErrInvalidRequestPayload, wd.Pickup)
			}
			out.pickups[wd.Weekday] = wd.Pickup
		}
	}
	return out, nil
}

func validateCareSchedulePayload(payload map[string]any) error {
	_, err := canonicalizeCareSchedulePayload(payload)
	return err
}

// canonicalizeCareSchedulePayload validates and returns the sanitized payload to
// persist. Buckets via the shared parser (which rejects bad weekdays/modes/times),
// then rebuilds one entry per changed weekday so unknown keys and duplicate
// weekday rows from a direct API client never reach storage.
func canonicalizeCareSchedulePayload(payload map[string]any) (map[string]any, error) {
	changes, err := buildCareScheduleChanges(payload)
	if err != nil {
		return nil, err
	}
	if changes.isEmpty() {
		return nil, fmt.Errorf("%w: no changes", ErrInvalidRequestPayload)
	}
	return structToMap(changes.toCanonicalPayload())
}

func validateMasterDataPayload(payload map[string]any) error {
	_, err := canonicalizeMasterDataPayload(payload)
	return err
}

// canonicalizeMasterDataPayload validates and returns the sanitized payload to
// persist. After validation it re-marshals the typed struct, so unknown sibling
// keys a direct API client appended alongside "fields" are dropped and only the
// canonical {"fields": {...}} shape reaches storage.
func canonicalizeMasterDataPayload(payload map[string]any) (map[string]any, error) {
	p, err := decodePayload[studentMasterDataPayload](payload)
	if err != nil {
		return nil, ErrInvalidRequestPayload
	}
	if len(p.Fields) == 0 {
		return nil, fmt.Errorf("%w: no fields", ErrInvalidRequestPayload)
	}
	for field, value := range p.Fields {
		if !isAllowedStudentMasterDataField(field) {
			return nil, fmt.Errorf("%w: field %q not allowed", ErrInvalidRequestPayload, field)
		}
		// Bound every free-text value so a direct API client cannot persist a
		// multi-megabyte payload (notably extra_info, which neither Student.Validate
		// nor any other layer length-caps). Matches maxMessageLen / the old
		// maxParentNoteLen note limit; count runes, not bytes, so German umlauts are
		// not penalized.
		if value != nil && utf8.RuneCountInString(*value) > maxMasterDataFieldLen {
			return nil, fmt.Errorf("%w: %q too long", ErrInvalidRequestPayload, field)
		}
		switch field {
		case "first_name", "last_name":
			if value == nil || strings.TrimSpace(*value) == "" {
				return nil, fmt.Errorf("%w: %q must not be empty", ErrInvalidRequestPayload, field)
			}
		case "birthday":
			if value != nil && strings.TrimSpace(*value) != "" {
				if _, err := timezone.ParseDate(strings.TrimSpace(*value)); err != nil {
					return nil, fmt.Errorf("%w: birthday", ErrInvalidRequestPayload)
				}
			}
		case "guardian_email":
			// Validate at create time with the SAME canonical rule Student.Validate
			// applies on apply, so an invalid value is rejected up front (4xx) instead
			// of being accepted, then 500ing every staff ConfirmRequest and leaving the
			// request stuck open and unconfirmable. Empty is allowed (clears the field).
			if value != nil {
				if err := usersModels.ValidateOptionalEmail(*value); err != nil {
					return nil, fmt.Errorf("%w: guardian_email", ErrInvalidRequestPayload)
				}
			}
		case "guardian_phone":
			if value != nil {
				if err := usersModels.ValidateOptionalPhone(*value); err != nil {
					return nil, fmt.Errorf("%w: guardian_phone", ErrInvalidRequestPayload)
				}
			}
		}
	}
	return structToMap(p)
}

// germanAllowedDepartureModes renders a day's allowed departure modes (the
// non-exclusive source of truth the apply path writes) as a combined German
// label, e.g. "Fährt Bus / Wird abgeholt". An empty set means the child goes
// home alone.
func germanAllowedDepartureModes(modes []usersModels.DepartureMode) string {
	if len(modes) == 0 {
		return usersModels.DepartureAlone.GermanLabel()
	}
	parts := make([]string, 0, len(modes))
	for _, m := range modes {
		parts = append(parts, m.GermanLabel())
	}
	return strings.Join(parts, " / ")
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
