package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Staff-review sentinel errors, mapped to HTTP codes by the handler.
var (
	// ErrReviewNotFound means no change request matched the id in the tenant.
	ErrReviewNotFound = errors.New("users: change request not found")
	// ErrReviewNotPending means the request was already decided (lost race) or
	// is a Track A audit row that cannot be decided.
	ErrReviewNotPending = errors.New("users: change request is not pending")
	// ErrReviewInvalidTarget means the request's target/field is not one the
	// review service knows how to apply.
	ErrReviewInvalidTarget = errors.New("users: change request target is not applicable")
	// ErrReviewInvalidValue means the stored new_value could not be decoded when
	// applying an approval.
	ErrReviewInvalidValue = errors.New("users: change request value is invalid")
	// ErrReviewStaleValue means the live value no longer matches the request's
	// old_value baseline, so approval would overwrite a newer staff/import edit.
	ErrReviewStaleValue = errors.New("users: change request baseline changed")
	// ErrReviewForbidden means the caller may not decide this request because they
	// could not edit the child directly (not an admin and not the child's group
	// supervisor). Same per-child write gate as the direct student edit.
	ErrReviewForbidden = errors.New("users: change request forbidden")
)

// MasterDataReviewItem is one pending request enriched with the child's name for
// the staff queue.
type MasterDataReviewItem struct {
	Request   *userModels.StudentDataChangeRequest
	FirstName string
	LastName  string
}

// MasterDataReviewDecideInput carries a staff decision on one change request.
type MasterDataReviewDecideInput struct {
	RequestID  int64
	Approve    bool
	Reason     string
	ReviewedBy int64
}

// MasterDataReviewService is the staff-facing review queue for parent Track B
// Stammdaten change requests. It runs inside the tenant transaction established
// by the request middleware, so all repo calls are tenant-scoped via RLS.
type MasterDataReviewService interface {
	// ListPending returns every pending change request for the current tenant,
	// newest-first, enriched with the child's name.
	ListPending(ctx context.Context) ([]*MasterDataReviewItem, error)
	// Decide approves (and applies) or rejects one pending request and returns
	// the refreshed row enriched with the child's name.
	Decide(ctx context.Context, input MasterDataReviewDecideInput) (*MasterDataReviewItem, error)
}

type masterDataReviewService struct {
	changeRequestRepo userModels.StudentDataChangeRequestRepository
	studentRepo       userModels.StudentRepository
	personRepo        userModels.PersonRepository
	userCtx           authorize.StudentModifyUserContext
	broadcaster       realtime.Broadcaster
	emitter           *parentmessaging.Emitter
	studentAudit      StudentChangeRecorder
	logger            *slog.Logger
}

// NewMasterDataReviewService wires the staff review service. emitter posts the
// parent-visible decision pill into the child's chat thread (nil = no pills,
// e.g. in tests). userCtx resolves the caller's supervised groups for the
// per-child write gate on ListPending/Decide (nil is treated as "no groups", so
// only admins pass — keep it wired outside admin-only tests).
func NewMasterDataReviewService(
	changeRequestRepo userModels.StudentDataChangeRequestRepository,
	studentRepo userModels.StudentRepository,
	personRepo userModels.PersonRepository,
	userCtx authorize.StudentModifyUserContext,
	emitter *parentmessaging.Emitter,
	logger *slog.Logger,
	broadcasters ...realtime.Broadcaster,
) MasterDataReviewService {
	return newMasterDataReviewService(changeRequestRepo, studentRepo, personRepo, userCtx, emitter, nil, logger, broadcasters...)
}

// NewMasterDataReviewServiceWithAudit wires the staff review service and the
// per-child change recorder used for approved departure-plan requests.
func NewMasterDataReviewServiceWithAudit(
	changeRequestRepo userModels.StudentDataChangeRequestRepository,
	studentRepo userModels.StudentRepository,
	personRepo userModels.PersonRepository,
	userCtx authorize.StudentModifyUserContext,
	emitter *parentmessaging.Emitter,
	studentAudit StudentChangeRecorder,
	logger *slog.Logger,
	broadcasters ...realtime.Broadcaster,
) MasterDataReviewService {
	return newMasterDataReviewService(changeRequestRepo, studentRepo, personRepo, userCtx, emitter, studentAudit, logger, broadcasters...)
}

func newMasterDataReviewService(
	changeRequestRepo userModels.StudentDataChangeRequestRepository,
	studentRepo userModels.StudentRepository,
	personRepo userModels.PersonRepository,
	userCtx authorize.StudentModifyUserContext,
	emitter *parentmessaging.Emitter,
	studentAudit StudentChangeRecorder,
	logger *slog.Logger,
	broadcasters ...realtime.Broadcaster,
) MasterDataReviewService {
	if logger == nil {
		logger = slog.Default()
	}
	var broadcaster realtime.Broadcaster
	if len(broadcasters) > 0 {
		broadcaster = broadcasters[0]
	}
	return &masterDataReviewService{
		changeRequestRepo: changeRequestRepo,
		studentRepo:       studentRepo,
		personRepo:        personRepo,
		userCtx:           userCtx,
		broadcaster:       broadcaster,
		emitter:           emitter,
		studentAudit:      studentAudit,
		logger:            logger,
	}
}

func (s *masterDataReviewService) ListPending(ctx context.Context) ([]*MasterDataReviewItem, error) {
	rows, err := s.changeRequestRepo.ListPendingForTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("review: list pending: %w", err)
	}
	if len(rows) == 0 {
		return []*MasterDataReviewItem{}, nil
	}

	studentIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.StudentID]; !ok {
			seen[r.StudentID] = struct{}{}
			studentIDs = append(studentIDs, r.StudentID)
		}
	}
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("review: load students: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("review: load persons: %w", err)
	}

	// Scope the queue to children the caller may WRITE (admin, or the child's
	// group supervisor) — the same gate as Decide, so a staffer only ever sees
	// requests they can act on. Also scopes the sidebar badge (sums ListPending).
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.userCtx)

	items := make([]*MasterDataReviewItem, 0, len(rows))
	for _, r := range rows {
		if !writable(students[r.StudentID]) {
			continue
		}
		// A graduated child is soft-deleted: their pending requests survive the
		// graduation (the hard delete it replaced cascaded them away), so without
		// this they would sit in the staff queue and the sidebar badge and could
		// still be approved onto an alumnus. A revert brings them back (#405 review).
		if students[r.StudentID].IsAlumnus() {
			continue
		}
		item := &MasterDataReviewItem{Request: r}
		if st, ok := students[r.StudentID]; ok {
			if p, ok := persons[st.PersonID]; ok {
				item.FirstName = p.FirstName
				item.LastName = p.LastName
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *masterDataReviewService) Decide(ctx context.Context, input MasterDataReviewDecideInput) (*MasterDataReviewItem, error) {
	if input.RequestID <= 0 {
		return nil, ErrReviewNotFound
	}
	req, err := s.changeRequestRepo.FindPendingByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		if errors.Is(err, userModels.ErrChangeRequestNotFound) {
			return nil, ErrReviewNotFound
		}
		if errors.Is(err, userModels.ErrChangeRequestNotPending) {
			return nil, ErrReviewNotPending
		}
		return nil, fmt.Errorf("review: find pending request: %w", err)
	}

	// Per-child write authorization: the caller may decide this request only if
	// they could edit the child directly — admin, or the child's group supervisor
	// (auth/authorize.CanUpdateStudent, the same gate the direct student edit
	// uses). Both approve and reject are gated: a staffer who cannot edit the
	// child has no business deciding its request either way.
	//
	// Taken FOR UPDATE so the alumnus gate below decides on a state a concurrent
	// grade transition cannot change underneath it: the transition apply locks
	// exactly this row before flipping it to alumnus, so an unlocked read could
	// see "active", let the approve through, and have the graduation commit
	// before the person/student writes land — an alumnus' master data rewritten
	// past the gate that exists to refuse it. Under the lock the two serialize.
	// This is also the transaction's FIRST row lock: the apply below only ever
	// re-acquires this same student row or takes the child's person row after
	// it, so no student lock order is inverted and the person lock keeps its
	// single acquisition site (#405 review).
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return nil, fmt.Errorf("review: load student for decision: %w", err)
	}
	// The child graduated after filing this request. The lookup is unfiltered, so
	// without this gate an approve would still rewrite an alumnus' master data
	// (name, departure modes, companion links). Same 404 the rest of the child
	// surface returns for graduates (#405 review).
	if student.IsAlumnus() {
		return nil, ErrReviewNotFound
	}
	if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.userCtx); !ok {
		return nil, ErrReviewForbidden
	}

	var reason *string
	if trimmed := input.Reason; trimmed != "" {
		reason = &trimmed
	}

	if !input.Approve {
		if err := s.changeRequestRepo.Decide(ctx, req.ID, userModels.DataChangeStatusRejected, reason, input.ReviewedBy, false); err != nil {
			if errors.Is(err, userModels.ErrChangeRequestNotPending) {
				return nil, ErrReviewNotPending
			}
			return nil, fmt.Errorf("review: reject: %w", err)
		}
		s.logger.Info("staff rejected master data change",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.Int64("reviewed_by", input.ReviewedBy),
		)
		s.deferDecisionPill(ctx, req, input, false)
		row, findErr := s.changeRequestRepo.FindByID(ctx, req.ID)
		if findErr != nil {
			return nil, fmt.Errorf("review: reload rejected request: %w", findErr)
		}
		return s.enrichReviewItem(ctx, row)
	}

	// Run the apply in a recording scope so the companion announcement below can
	// be keyed off the WRITE instead of the request's target: a departure
	// approval that changes no weekday the links depend on leaves every link in
	// place (see userModels.CompanionChangeRecorder).
	applyCtx, companionChanges := userModels.ContextWithCompanionChangeRecorder(ctx)
	if err := s.applyApprovedChange(applyCtx, req, input.ReviewedBy); err != nil {
		return nil, err
	}
	if err := s.changeRequestRepo.Decide(ctx, req.ID, userModels.DataChangeStatusApproved, reason, input.ReviewedBy, true); err != nil {
		if errors.Is(err, userModels.ErrChangeRequestNotPending) {
			return nil, ErrReviewNotPending
		}
		return nil, fmt.Errorf("review: approve: %w", err)
	}
	s.logger.Info("staff approved master data change",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.String("target", req.Target),
		slog.String("field", req.FieldKey),
		slog.Int64("reviewed_by", input.ReviewedBy),
	)
	s.deferDecisionPill(ctx, req, input, true)
	s.deferStudentUpdated(ctx, req.StudentID)
	// Only when the write actually trimmed a "läuft mit" link — those links are
	// rows on ANOTHER child's card too. Everything else stays silent: the event
	// makes open companion forms drop or block a draft, which must not happen
	// for a rename, nor for a departure approval that left every link intact.
	if companionChanges.Changed() {
		s.deferStudentCompanionsChanged(ctx, req.StudentID)
	}
	row, findErr := s.changeRequestRepo.FindByID(ctx, req.ID)
	if findErr != nil {
		return nil, fmt.Errorf("review: reload approved request: %w", findErr)
	}
	return s.enrichReviewItem(ctx, row)
}

func (s *masterDataReviewService) enrichReviewItem(ctx context.Context, row *userModels.StudentDataChangeRequest) (*MasterDataReviewItem, error) {
	if row == nil {
		return nil, ErrReviewNotFound
	}
	item := &MasterDataReviewItem{Request: row}
	student, err := s.studentRepo.FindByID(ctx, row.StudentID)
	if err != nil {
		return nil, fmt.Errorf("review: load student: %w", err)
	}
	person, err := s.personRepo.FindByID(ctx, student.PersonID)
	if err != nil {
		return nil, fmt.Errorf("review: load person: %w", err)
	}
	item.FirstName = person.FirstName
	item.LastName = person.LastName
	return item, nil
}

// deferDecisionPill posts the parent-visible decision pill into the child's
// chat thread after the decision commits. Best-effort: the emitter self-gates
// on the messaging setting and opens its own detached tenant transaction, so
// a pill failure never affects the decision.
func (s *masterDataReviewService) deferDecisionPill(ctx context.Context, req *userModels.StudentDataChangeRequest, input MasterDataReviewDecideInput, approved bool) {
	if s.emitter == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	body := "Anfrage bestätigt, Stammdaten übernommen"
	status := userModels.ParentMessageRequestStatusDone
	reason := strings.TrimSpace(input.Reason)
	if !approved {
		status = userModels.ParentMessageRequestStatusRejected
		body = "Anfrage abgelehnt"
		if reason != "" {
			body = "Anfrage abgelehnt: " + reason
		}
	}
	refID := req.ID
	studentID := req.StudentID
	guardianAccountID := req.SubmittedBy
	actorAccountID := input.ReviewedBy
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitter.EmitChildEvent(tenantID, studentID, guardianAccountID, parentmessaging.ChildEvent{
			EventType:      "request_status",
			ActorKind:      userModels.ParentMessageSenderStaff,
			ActorAccountID: actorAccountID,
			Body:           body,
			RequestType:    userModels.ParentMessageRequestMasterData,
			RequestStatus:  status,
			DecisionReason: reason,
			RefTable:       "users.student_data_change_requests",
			RefID:          &refID,
		})
	})
}

func (s *masterDataReviewService) deferStudentUpdated(ctx context.Context, studentID int64) {
	if s.broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if tenantID <= 0 {
			s.logger.Warn("review: skipping student_updated broadcast without tenant",
				slog.Int64("student_id", studentID),
			)
			return
		}
		source := "master_data_review"
		event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
		if err := s.broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			s.logger.Warn("review: failed to broadcast student update",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// deferStudentCompanionsChanged announces, after commit, that the approved
// change may have trimmed the child's Laufgemeinschaft — the signal every
// mounted "läuft mit" view refetches on.
func (s *masterDataReviewService) deferStudentCompanionsChanged(ctx context.Context, studentID int64) {
	if s.broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if tenantID <= 0 {
			return
		}
		source := "master_data_review"
		event := realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{Source: &source})
		if err := s.broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			s.logger.Warn("review: failed to broadcast student companions change",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// applyApprovedChange writes the request's new_value to the live record.
func (s *masterDataReviewService) applyApprovedChange(ctx context.Context, req *userModels.StudentDataChangeRequest, reviewedBy int64) error {
	switch req.Target {
	case userModels.DataChangeTargetPerson:
		return s.applyPersonChange(ctx, req)
	case userModels.DataChangeTargetStudent:
		return s.applyStudentChange(ctx, req, reviewedBy)
	case userModels.DataChangeTargetDeparture:
		return s.applyDepartureChange(ctx, req, reviewedBy)
	default:
		return ErrReviewInvalidTarget
	}
}

func (s *masterDataReviewService) applyStudentChange(ctx context.Context, req *userModels.StudentDataChangeRequest, reviewedBy int64) error {
	if req.FieldKey != "school_class" {
		return ErrReviewInvalidTarget
	}
	var value string
	if err := json.Unmarshal(req.NewValue, &value); err != nil {
		return ErrReviewInvalidValue
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return ErrReviewInvalidValue
	}
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student: %w", err)
	}
	if !jsonRawEqual(jsonString(student.SchoolClass), req.OldValue) {
		return ErrReviewStaleValue
	}
	before := *student
	student.SchoolClass = value
	if err := s.studentRepo.Update(ctx, student); err != nil {
		return fmt.Errorf("review: update student class: %w", err)
	}
	if s.studentAudit != nil {
		if err := s.studentAudit.RecordChangesForActor(ctx, &before, student, reviewedBy); err != nil {
			return fmt.Errorf("review: audit student class: %w", err)
		}
	}
	return nil
}

func (s *masterDataReviewService) applyPersonChange(ctx context.Context, req *userModels.StudentDataChangeRequest) error {
	var value string
	if err := json.Unmarshal(req.NewValue, &value); err != nil {
		return ErrReviewInvalidValue
	}
	var parsedBirthday *timezone.Date
	if req.FieldKey == "birthday" {
		d, parseErr := timezone.ParseDate(value)
		if parseErr != nil {
			return ErrReviewInvalidValue
		}
		parsedBirthday = &d
	}

	student, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student: %w", err)
	}
	person, err := s.personRepo.FindByIDForUpdate(ctx, student.PersonID)
	if err != nil {
		return fmt.Errorf("review: load person: %w", err)
	}

	currentRaw, err := personFieldRaw(person, req.FieldKey)
	if err != nil {
		return err
	}
	if !jsonRawEqual(currentRaw, req.OldValue) {
		return ErrReviewStaleValue
	}

	switch req.FieldKey {
	case "first_name":
		person.FirstName = value
	case "last_name":
		person.LastName = value
	case "birthday":
		person.Birthday = parsedBirthday
	default:
		return ErrReviewInvalidTarget
	}
	if err := s.personRepo.Update(ctx, person); err != nil {
		return fmt.Errorf("review: update person: %w", err)
	}
	return nil
}

func (s *masterDataReviewService) applyDepartureChange(ctx context.Context, req *userModels.StudentDataChangeRequest, reviewedBy int64) error {
	if req.FieldKey != "allowed_departure_modes" {
		return ErrReviewInvalidTarget
	}
	modes, err := decodeDepartureModes(req.NewValue)
	if err != nil {
		return err
	}
	if modes.HasMode(userModels.DepartureAccompanied) {
		return ErrReviewInvalidValue
	}
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student: %w", err)
	}
	before := *student
	oldModes, err := decodeDepartureModes(req.OldValue)
	if err != nil {
		return err
	}
	if !departureModesEqual(student.AllowedDepartureModes.Normalize(), oldModes.Normalize()) {
		return ErrReviewStaleValue
	}
	// Setting AllowedDepartureModes makes StudentRepository.Update persist the
	// derived departure_days / pickup_days / bus_days columns too.
	student.AllowedDepartureModes = modes.Normalize()
	if err := s.studentRepo.Update(ctx, student); err != nil {
		return fmt.Errorf("review: update student departure: %w", err)
	}
	if s.studentAudit != nil {
		if err := s.studentAudit.RecordChangesForActor(ctx, &before, student, reviewedBy); err != nil {
			return fmt.Errorf("review: audit student departure: %w", err)
		}
	}
	return nil
}

func personFieldRaw(person *userModels.Person, field string) (json.RawMessage, error) {
	switch field {
	case "first_name":
		return jsonString(person.FirstName), nil
	case "last_name":
		return jsonString(person.LastName), nil
	case "birthday":
		if person.Birthday == nil {
			return json.RawMessage("null"), nil
		}
		return jsonString(person.Birthday.String()), nil
	default:
		return nil, ErrReviewInvalidTarget
	}
}

func decodeDepartureModes(raw json.RawMessage) (userModels.AllowedDepartureModes, error) {
	var modes userModels.AllowedDepartureModes
	if err := json.Unmarshal(raw, &modes); err != nil {
		return nil, ErrReviewInvalidValue
	}
	if modes == nil {
		modes = userModels.AllowedDepartureModes{}
	}
	if err := modes.Validate(); err != nil {
		return nil, ErrReviewInvalidValue
	}
	return modes, nil
}

func departureModesEqual(a, b userModels.AllowedDepartureModes) bool {
	for _, day := range userModels.PickupDayOrder {
		left := a[day]
		right := b[day]
		if len(left) != len(right) {
			return false
		}
		for i := range left {
			if left[i] != right[i] {
				return false
			}
		}
	}
	return true
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func jsonRawEqual(a, b json.RawMessage) bool {
	var left any
	var right any
	if err := json.Unmarshal(a, &left); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &right); err != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}
