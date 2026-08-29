package users

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
	Request              *userModels.StudentDataChangeRequest
	FirstName            string
	LastName             string
	BulkEligible         bool
	BulkIneligibleReason string
}

// HistoryCursor points at the last DB row of a change-request page (shared by
// all five request queues, open list and history alike). The next page
// continues strictly before (UpdatedAt, ID) — the field name says updated_at
// because the history keys on it; the open list keys on created_at, which is
// the same shape. It must be built from the last row the repository returned —
// not the last VISIBLE item — because the per-child scope filters after the DB
// limit; a cursor built from a filtered page would skip rows.
type HistoryCursor struct {
	UpdatedAt time.Time
	ID        int64
}

// NextCursor turns a repository page into the cursor for the page after it.
// Callers ask the repository for limit+1 rows so one extra row proves an older
// page exists without a second count query; this trims that probe row off and
// returns the position to resume from, or nil on the last page.
func NextCursor[T any](rows []T, limit int, key func(T) (time.Time, int64)) ([]T, *HistoryCursor) {
	if limit <= 0 || len(rows) <= limit {
		return rows, nil
	}
	rows = rows[:limit]
	instant, id := key(rows[len(rows)-1])
	return rows, &HistoryCursor{UpdatedAt: instant, ID: id}
}

// MasterDataHistoryItem is one decided request enriched with the child's name
// and the reviewer's display name for the staff history.
type MasterDataHistoryItem struct {
	Request      *userModels.StudentDataChangeRequest
	FirstName    string
	LastName     string
	ReviewerName string // "" when the row carries no reviewer (auto-applied)
}

// MasterDataReviewDecideInput carries a staff decision on one change request.
type MasterDataReviewDecideInput struct {
	RequestID       int64
	Approve         bool
	Reason          string
	ReviewedBy      int64
	ExpectedVersion string
}

// MasterDataReviewService is the staff-facing review queue for parent Track B
// Stammdaten change requests. It runs inside the tenant transaction established
// by the request middleware, so all repo calls are tenant-scoped via RLS.
type MasterDataReviewService interface {
	// ListPending returns pending change requests for the current tenant,
	// newest submission first, enriched with the child's name. The filters
	// narrow and page the query in SQL; their zero value returns the whole
	// queue, which is what the pending-count badge needs.
	ListPending(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*MasterDataReviewItem, next *HistoryCursor, err error)
	// ListHistory returns decided requests newest-decision-first, keyset
	// paginated on (updated_at, id). A zero BeforeInstant returns the first
	// page; next is nil when no older rows exist beyond this page.
	ListHistory(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*MasterDataHistoryItem, next *HistoryCursor, err error)
	// Decide approves (and applies) or rejects one pending request and returns
	// the refreshed row enriched with the child's name.
	Decide(ctx context.Context, input MasterDataReviewDecideInput) (*MasterDataReviewItem, error)
}

type masterDataReviewService struct {
	changeRequestRepo userModels.StudentDataChangeRequestRepository
	studentRepo       userModels.StudentRepository
	personRepo        userModels.PersonRepository
	userCtx           authorize.StudentAccessUserContext
	broadcaster       realtime.Broadcaster
	emitter           *parentmessaging.Emitter
	studentAudit      StudentChangeRecorder
	logger            *slog.Logger
	reviewPolicy      RequestReviewPolicy
}

type RequestReviewPolicy interface {
	StudentFilter(context.Context, []string) (func(*userModels.Student) bool, error)
	Allows(context.Context, []string, *userModels.Student) (bool, error)
}

// NewMasterDataReviewServiceWithAuditAndPolicy requires the production review
// policy at construction, so missing wiring cannot widen reviewer access.
func NewMasterDataReviewServiceWithAuditAndPolicy(
	changeRequestRepo userModels.StudentDataChangeRequestRepository,
	studentRepo userModels.StudentRepository,
	personRepo userModels.PersonRepository,
	userCtx authorize.StudentAccessUserContext,
	emitter *parentmessaging.Emitter,
	studentAudit StudentChangeRecorder,
	reviewPolicy RequestReviewPolicy,
	logger *slog.Logger,
	broadcasters ...realtime.Broadcaster,
) MasterDataReviewService {
	if reviewPolicy == nil {
		panic("master data review policy is required")
	}
	return newMasterDataReviewService(changeRequestRepo, studentRepo, personRepo, userCtx, emitter, studentAudit, reviewPolicy, logger, broadcasters...)
}

func newMasterDataReviewService(
	changeRequestRepo userModels.StudentDataChangeRequestRepository,
	studentRepo userModels.StudentRepository,
	personRepo userModels.PersonRepository,
	userCtx authorize.StudentAccessUserContext,
	emitter *parentmessaging.Emitter,
	studentAudit StudentChangeRecorder,
	reviewPolicy RequestReviewPolicy,
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
		reviewPolicy:      reviewPolicy,
		logger:            logger,
	}
}

// reviewStudentScope bundles the loaded child rows, their persons, and the
// caller's per-child write gate — shared between ListPending and ListHistory so
// both surfaces show exactly the children the caller may act on.
type reviewStudentScope struct {
	students map[int64]*userModels.Student
	persons  map[int64]*userModels.Person
	writable func(*userModels.Student) bool
}

// includes reports whether the caller may see rows of this child. Besides the
// write gate it skips alumni: a graduated child is soft-deleted and their
// request rows survive the graduation, so without this they would sit in the
// staff queue / sidebar badge and could still be approved onto an alumnus. A
// revert brings them back (#405 review). Same gate for the history — every
// other child surface 404s for alumni.
func (sc *reviewStudentScope) includes(studentID int64) bool {
	st := sc.students[studentID]
	return sc.writable(st) && !st.IsAlumnus()
}

// includesPending narrows the queue further than the history: a child whose
// care has ended must not be offered for a decision (#2487). Their closed
// requests deliberately stay readable in the history — the acceptance criteria
// require the trail to survive the departure.
func (sc *reviewStudentScope) includesPending(studentID int64) bool {
	if !sc.includes(studentID) {
		return false
	}
	return !sc.students[studentID].CareEndedOn(timezone.TodayDate())
}

func (sc *reviewStudentScope) name(studentID int64) (string, string) {
	if st, ok := sc.students[studentID]; ok {
		if p, ok := sc.persons[st.PersonID]; ok {
			return p.FirstName, p.LastName
		}
	}
	return "", ""
}

// loadStudentScope loads the children behind the given request rows and builds
// the caller's write gate (admin, or the child's group supervisor — the same
// gate as Decide).
func (s *masterDataReviewService) loadStudentScope(ctx context.Context, studentIDs []int64) (*reviewStudentScope, error) {
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
	writable, err := s.reviewableFilter(ctx)
	if err != nil {
		return nil, err
	}
	return &reviewStudentScope{
		students: students,
		persons:  persons,
		writable: writable,
	}, nil
}

func (s *masterDataReviewService) reviewableFilter(ctx context.Context) (func(*userModels.Student) bool, error) {
	if s.reviewPolicy == nil {
		return authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.userCtx), nil
	}
	filter, err := s.reviewPolicy.StudentFilter(ctx, jwt.PermissionsFromCtx(ctx))
	if err != nil {
		return nil, fmt.Errorf("review: resolve reviewer scope: %w", err)
	}
	return filter, nil
}

func uniqueStudentIDs(rows []*userModels.StudentDataChangeRequest) []int64 {
	ids := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.StudentID]; !ok {
			seen[r.StudentID] = struct{}{}
			ids = append(ids, r.StudentID)
		}
	}
	return ids
}

func (s *masterDataReviewService) ListPending(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*MasterDataReviewItem, *HistoryCursor, error) {
	// limit+1 probes for an older page without a second count query.
	rows, err := s.changeRequestRepo.ListPendingForTenant(ctx, probeLimit(filters))
	if err != nil {
		return nil, nil, fmt.Errorf("review: list pending: %w", err)
	}
	rows, next := NextCursor(rows, filters.Limit, func(r *userModels.StudentDataChangeRequest) (time.Time, int64) {
		return r.CreatedAt, r.ID
	})
	if len(rows) == 0 {
		return []*MasterDataReviewItem{}, next, nil
	}

	// Scope the queue to children the caller may WRITE (admin, or the child's
	// group supervisor) — the same gate as Decide, so a staffer only ever sees
	// requests they can act on. Also scopes the sidebar badge (sums ListPending).
	scope, err := s.loadStudentScope(ctx, uniqueStudentIDs(rows))
	if err != nil {
		return nil, nil, err
	}

	items := make([]*MasterDataReviewItem, 0, len(rows))
	for _, r := range rows {
		if !scope.includesPending(r.StudentID) {
			continue
		}
		item := &MasterDataReviewItem{Request: r}
		item.FirstName, item.LastName = scope.name(r.StudentID)
		item.BulkEligible, item.BulkIneligibleReason = masterDataBulkEligibility(r, scope)
		items = append(items, item)
	}
	return items, next, nil
}

func (s *masterDataReviewService) GetBulkCandidate(ctx context.Context, requestID int64) (*MasterDataReviewItem, error) {
	req, err := s.changeRequestRepo.FindByID(ctx, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrReviewNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("review: load bulk candidate: %w", err)
	}
	if req == nil || req.Status != userModels.DataChangeStatusPending {
		return nil, ErrReviewNotFound
	}
	scope, err := s.loadStudentScope(ctx, []int64{req.StudentID})
	if err != nil {
		return nil, err
	}
	if !scope.includesPending(req.StudentID) {
		return nil, ErrReviewNotFound
	}
	item := &MasterDataReviewItem{Request: req}
	item.FirstName, item.LastName = scope.name(req.StudentID)
	item.BulkEligible, item.BulkIneligibleReason = masterDataBulkEligibility(req, scope)
	return item, nil
}

func (s *masterDataReviewService) LockBulkRequest(ctx context.Context, requestID int64) error {
	_, err := s.changeRequestRepo.FindPendingByIDForUpdate(ctx, requestID)
	return err
}

// LockBulkStudents establishes one canonical student-lock frontier after all
// request rows have been locked in request-kind/id order. Single decisions use
// the same request-before-student order.
func (s *masterDataReviewService) LockBulkStudents(ctx context.Context, studentIDs []int64) error {
	for _, studentID := range studentIDs {
		if _, err := s.studentRepo.FindByIDForUpdate(ctx, studentID); err != nil {
			return err
		}
	}
	return nil
}

func masterDataBulkEligibility(req *userModels.StudentDataChangeRequest, scope *reviewStudentScope) (bool, string) {
	student := scope.students[req.StudentID]
	if student == nil {
		return false, "Das Kind ist nicht mehr verfügbar."
	}
	switch req.Target {
	case userModels.DataChangeTargetPerson:
		return personRequestBulkEligibility(req, scope.persons[student.PersonID])
	case userModels.DataChangeTargetStudent:
		return studentRequestBulkEligibility(req, student)
	case userModels.DataChangeTargetDeparture:
		return departureRequestBulkEligibility(req, student)
	default:
		return false, "Diese Anfrage kann nur einzeln freigegeben werden."
	}
}

func personRequestBulkEligibility(req *userModels.StudentDataChangeRequest, person *userModels.Person) (bool, string) {
	if person == nil {
		return false, "Die Stammdaten sind nicht mehr verfügbar."
	}
	current, err := personFieldRaw(person, req.FieldKey)
	if err != nil || !validPersonRequestValue(req) {
		return false, "Diese Anfrage kann nur einzeln freigegeben werden."
	}
	if !jsonRawEqual(current, req.OldValue) {
		return false, "Der aktuelle Wert wurde nach der Anfrage geändert."
	}
	return true, ""
}

func studentRequestBulkEligibility(req *userModels.StudentDataChangeRequest, student *userModels.Student) (bool, string) {
	if req.FieldKey != "school_class" || !validRequiredString(req.NewValue) {
		return false, "Diese Anfrage kann nur einzeln freigegeben werden."
	}
	if !jsonRawEqual(jsonString(student.SchoolClass), req.OldValue) {
		return false, "Der aktuelle Wert wurde nach der Anfrage geändert."
	}
	return true, ""
}

func departureRequestBulkEligibility(req *userModels.StudentDataChangeRequest, student *userModels.Student) (bool, string) {
	requested, err := decodeDepartureModes(req.NewValue)
	previous, oldErr := decodeDepartureModes(req.OldValue)
	if req.FieldKey != "allowed_departure_modes" || err != nil || oldErr != nil || requested.HasMode(userModels.DepartureAccompanied) {
		return false, "Diese Anfrage kann nur einzeln freigegeben werden."
	}
	if !departureModesEqual(student.AllowedDepartureModes.Normalize(), previous.Normalize()) {
		return false, "Der aktuelle Wert wurde nach der Anfrage geändert."
	}
	return true, ""
}

func validRequiredString(raw json.RawMessage) bool {
	var value string
	return json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != ""
}

func validPersonRequestValue(req *userModels.StudentDataChangeRequest) bool {
	if !validRequiredString(req.NewValue) {
		return false
	}
	if req.FieldKey != "birthday" {
		return req.FieldKey == "first_name" || req.FieldKey == "last_name"
	}
	var value string
	_ = json.Unmarshal(req.NewValue, &value)
	_, err := timezone.ParseDate(value)
	return err == nil
}

// probeLimit asks the repository for one row more than the caller wants, so a
// present extra row proves an older page exists. An unbounded page stays
// unbounded.
func probeLimit(filters modelBase.RequestQueueFilters) modelBase.RequestQueueFilters {
	if filters.Limit > 0 {
		filters.Limit++
	}
	return filters
}

func (s *masterDataReviewService) ListHistory(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*MasterDataHistoryItem, *HistoryCursor, error) {
	// limit+1 probes for an older page without a second count query.
	rows, err := s.changeRequestRepo.ListDecidedForTenant(ctx, probeLimit(filters))
	if err != nil {
		return nil, nil, fmt.Errorf("review: list decided: %w", err)
	}
	rows, next := NextCursor(rows, filters.Limit, func(r *userModels.StudentDataChangeRequest) (time.Time, int64) {
		return r.UpdatedAt, r.ID
	})
	if len(rows) == 0 {
		return []*MasterDataHistoryItem{}, nil, nil
	}

	scope, err := s.loadStudentScope(ctx, uniqueStudentIDs(rows))
	if err != nil {
		return nil, nil, err
	}

	reviewerIDs := make([]int64, 0, len(rows))
	seenReviewers := make(map[int64]struct{}, len(rows))
	for _, r := range rows {
		if r.ReviewedBy == nil || *r.ReviewedBy <= 0 {
			continue
		}
		if _, ok := seenReviewers[*r.ReviewedBy]; !ok {
			seenReviewers[*r.ReviewedBy] = struct{}{}
			reviewerIDs = append(reviewerIDs, *r.ReviewedBy)
		}
	}
	reviewers, err := s.personRepo.FindByAccountIDs(ctx, reviewerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("review: load reviewers: %w", err)
	}

	items := make([]*MasterDataHistoryItem, 0, len(rows))
	for _, r := range rows {
		if !scope.includes(r.StudentID) {
			continue
		}
		item := &MasterDataHistoryItem{Request: r}
		item.FirstName, item.LastName = scope.name(r.StudentID)
		item.ReviewerName = ReviewerDisplayName(reviewers, r.ReviewedBy)
		items = append(items, item)
	}
	return items, next, nil
}

// ReviewerDisplayName resolves a nullable reviewer account id to a person's
// display name (shared by all four change-request history services). A decided
// row whose reviewer account was deleted (or never had a person) still shows up
// — as "Unbekannt" — instead of losing the fact that somebody decided it.
func ReviewerDisplayName(reviewers map[int64]*userModels.Person, reviewedBy *int64) string {
	if reviewedBy == nil || *reviewedBy <= 0 {
		return ""
	}
	if p, ok := reviewers[*reviewedBy]; ok {
		return strings.TrimSpace(p.FirstName + " " + p.LastName)
	}
	return "Unbekannt"
}

func (s *masterDataReviewService) Decide(ctx context.Context, input MasterDataReviewDecideInput) (*MasterDataReviewItem, error) {
	req, err := s.loadPendingDecisionRequest(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeMasterDataDecision(ctx, req.StudentID); err != nil {
		return nil, err
	}
	var reason *string
	if input.Reason != "" {
		reason = &input.Reason
	}
	if !input.Approve {
		return s.rejectMasterDataRequest(ctx, req, input, reason)
	}
	return s.approveMasterDataRequest(ctx, req, input, reason)
}

func (s *masterDataReviewService) loadPendingDecisionRequest(ctx context.Context, input MasterDataReviewDecideInput) (*userModels.StudentDataChangeRequest, error) {
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
	if input.ExpectedVersion != "" && ParentRequestVersion(req.UpdatedAt) != input.ExpectedVersion {
		return nil, ErrParentRequestStale
	}
	return req, nil
}

func (s *masterDataReviewService) authorizeMasterDataDecision(ctx context.Context, studentID int64) error {
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
	student, err := s.studentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return fmt.Errorf("review: load student for decision: %w", err)
	}
	// The child graduated after filing this request. The lookup is unfiltered, so
	// without this gate an approve would still rewrite an alumnus' master data
	// (name, departure modes, companion links). Same 404 the rest of the child
	// surface returns for graduates (#405 review).
	if student.IsAlumnus() {
		return ErrReviewNotFound
	}
	// The child left the OGS after filing this request; approving it would
	// rewrite the master data of a child the school no longer cares for (#2487).
	if student.CareEndedOn(timezone.TodayDate()) {
		return ErrReviewNotFound
	}
	allowed, authErr := s.canReviewStudent(ctx, student)
	if authErr != nil {
		return authErr
	}
	if !allowed {
		return ErrReviewForbidden
	}
	return nil
}

func (s *masterDataReviewService) rejectMasterDataRequest(
	ctx context.Context, req *userModels.StudentDataChangeRequest, input MasterDataReviewDecideInput, reason *string,
) (*MasterDataReviewItem, error) {
	if err := s.changeRequestRepo.Decide(ctx, req.ID, userModels.DataChangeStatusRejected, reason, input.ReviewedBy, false); err != nil {
		if errors.Is(err, userModels.ErrChangeRequestNotPending) {
			return nil, ErrReviewNotPending
		}
		return nil, fmt.Errorf("review: reject: %w", err)
	}
	s.logger.Info(
		"staff rejected master data change",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.Int64("reviewed_by", input.ReviewedBy),
	)
	s.deferDecisionPill(ctx, req, input, false)
	return s.reloadMasterDataReviewItem(ctx, req.ID, "rejected")
}

func (s *masterDataReviewService) approveMasterDataRequest(
	ctx context.Context, req *userModels.StudentDataChangeRequest, input MasterDataReviewDecideInput, reason *string,
) (*MasterDataReviewItem, error) {
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
	return s.reloadMasterDataReviewItem(ctx, req.ID, "approved")
}

func (s *masterDataReviewService) reloadMasterDataReviewItem(ctx context.Context, requestID int64, status string) (*MasterDataReviewItem, error) {
	row, findErr := s.changeRequestRepo.FindByID(ctx, requestID)
	if findErr != nil {
		return nil, fmt.Errorf("review: reload %s request: %w", status, findErr)
	}
	return s.enrichReviewItem(ctx, row)
}

func (s *masterDataReviewService) canReviewStudent(ctx context.Context, student *userModels.Student) (bool, error) {
	if s.reviewPolicy == nil {
		ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.userCtx)
		return ok, nil
	}
	ok, err := s.reviewPolicy.Allows(ctx, jwt.PermissionsFromCtx(ctx), student)
	if err != nil {
		return false, fmt.Errorf("review: resolve reviewer scope: %w", err)
	}
	return ok, nil
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
