package enrollment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	importsvc "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// guardianRoleName is the auth.roles.name value the guardian invitation
// flow uses on accept. Mirrored here so an approval that finds an
// existing account can attach the role for the new tenant directly.
const guardianRoleName = "guardian"

// DecisionService sentinel errors. Mapped to HTTP status codes by the
// admin handlers.
var (
	ErrDecisionRequestNotFound   = errors.New("enrollment request not found")
	ErrDecisionChildNotFound     = errors.New("request child not found")
	ErrDecisionStudentNotFound   = errors.New("student not found")
	ErrDecisionInvalidStatus     = errors.New("invalid decision status")
	ErrDecisionAlreadyTerminal   = errors.New("child is already in a terminal status")
	ErrOfferingAdjustmentInvalid = errors.New("offering adjustment is invalid")
	// ErrDecisionInvalidData marks an approval that failed because the
	// parent-supplied request data (e.g. guardian phone) doesn't pass the
	// student/person validators. Mapped to 400, not 500 — submit/edit now
	// validate up front, so this is defense-in-depth for legacy rows.
	ErrDecisionInvalidData = errors.New("enrollment request data is invalid")
	// ErrExportTooLarge guards the phase export against assembling an
	// unbounded payload in memory. At OGS scale a phase holds hundreds of
	// requests (a few MB); this cap only trips on a pathological phase,
	// turning "theoretically unbounded" into an explicit, mapped 400 (the
	// handler treats an over-cap phase as a client-side limit, not a
	// server fault).
	ErrExportTooLarge = errors.New("phase has too many registrations for a single export")
)

// maxExportRequests is the upper bound on requests assembled into one
// phase export. The whole document is built in memory (the PDF format
// cannot stream — its xref table needs every object's byte offset), so
// this ceiling caps the request count rather than the byte size: a
// single request with many children and large custom_data still grows
// per row, so this is a coarse runaway guard, not a hard memory bound.
// Far above any real OGS phase; it exists to fail loudly rather than
// OOM on a pathological phase.
const maxExportRequests = 5000

// DecisionStatus enumerates the per-child decisions an admin can apply.
// Mirrors the request_children.status CHECK constraint subset that
// admins are allowed to write (parent-initiated 'withdrawn' goes
// through a different path).
type DecisionStatus string

const (
	DecisionApproved    DecisionStatus = enrollmentModels.ChildStatusApproved
	DecisionWaitlisted  DecisionStatus = enrollmentModels.ChildStatusWaitlisted
	DecisionRejected    DecisionStatus = enrollmentModels.ChildStatusRejected
	DecisionUnderReview DecisionStatus = enrollmentModels.ChildStatusUnderReview
)

var validDecisionStatuses = map[DecisionStatus]bool{
	DecisionApproved:    true,
	DecisionWaitlisted:  true,
	DecisionRejected:    true,
	DecisionUnderReview: true,
}

// DecideInput carries the per-child decision the admin makes.
type DecideInput struct {
	RequestID  int64
	ChildID    int64
	Status     DecisionStatus
	Reason     string // optional; surfaced to parent only when phase.show_status_reason_to_parent
	ReviewedBy int64  // admin's auth account id
}

type OfferingAdjustmentSelection struct {
	OfferingID   int64
	SelectedDays []string
}

type UpdateChildOfferingsInput struct {
	RequestID      int64
	ChildID        int64
	Offerings      []OfferingAdjustmentSelection
	Reason         string
	ActorAccountID int64
	ActorRole      string
}

type SyncApprovedChildDataInput struct {
	RequestID                int64
	ChildID                  int64
	ActorAccountID           int64
	ReplaceTargetedData      bool
	PreviousSnapshot         map[string]any
	PreviousRequestGuardians []*enrollmentModels.RequestGuardian
}

// DecideOutcome is what the admin handler gets back from Decide. It
// carries the refreshed RequestChild plus an optional follow-up
// instruction asking the handler to issue a guardian invitation
// post-commit (after the tenant tx the handler owns completes).
//
// We surface the invitation as a side-effect rather than firing it from
// inside the service so:
//   - the invitation flow's own DB writes happen only if the approval
//     tx committed cleanly
//   - the handler can apply best-effort error handling without rolling
//     back the approval
type DecideOutcome struct {
	Child         *enrollmentModels.RequestChild
	PendingInvite *PendingGuardianInvite
}

// PendingGuardianInvite is the post-commit hook for fresh approvals
// where the guardian doesn't yet have a portal account. The handler is
// expected to call services/auth.GuardianInvitationService.Create with
// these values once the tenant tx commits.
type PendingGuardianInvite struct {
	GuardianProfileID int64
	CreatedBy         int64 // admin auth account id (for audit)
}

// RequestSummary is the admin-list shape: one row per request with
// per-child counts so the admin can scan the queue without expanding
// every detail page.
type RequestSummary struct {
	Request   *enrollmentModels.Request
	Phase     *enrollmentModels.Phase
	Children  []*enrollmentModels.RequestChild
	Guardians []*enrollmentModels.RequestGuardian
}

// RequestFilters narrows the admin list. Zero-value fields are
// ignored.
type RequestFilters struct {
	PhaseID     int64
	ChildStatus string // matches when ANY child carries this status
}

// DecisionService backs the admin review UI. Slice 2 wires the full
// approval pipeline: status mutation + downstream record creation
// (users.persons / users.students / users.guardian_profiles /
// users.students_guardians / activities.student_enrollments) + outbox
// enqueue for parent decision emails. The guardian invitation is
// surfaced via DecideOutcome.PendingInvite so the handler can fire it
// post-commit.
type DecisionService interface {
	List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error)
	ListByStudent(ctx context.Context, studentID int64) ([]*RequestSummary, error)
	Get(ctx context.Context, requestID int64) (*RequestSummary, error)
	Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error)
	UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error)
	ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*auditModels.EnrollmentOfferingAdjustment, error)

	// ListChildOfferings returns the request_child_offerings rows for
	// every child under requestID, joined to the offering's name +
	// description so the admin detail page can render labels without
	// a second per-offering fetch. Map key is request_child_id.
	ListChildOfferings(ctx context.Context, requestID int64) (map[int64][]ChildOfferingRow, error)

	// ExportPhase loads every request of a phase with its fully-resolved
	// children + offerings + form schema(s) in a fixed handful of
	// queries (N+1-free) AND records the GDPR access-log row in the same
	// call, so no caller can disclose the phase's PII without leaving an
	// audit trail. Read-only on enrollment data (no status changes, no
	// downstream record creation); the only write is the append-only
	// audit row. If the audit write fails the whole call fails (no trail,
	// no disclosure). Must run inside the request's tenant transaction so
	// both the reads and the audit row land under the correct tenant (RLS).
	//
	// childStatusFilter, when non-empty, keeps only children whose own
	// status equals it (requests with no matching child are dropped) —
	// mirroring the admin list's per-child status dropdown. Empty means
	// "all". The audit row's counts reflect the filtered (disclosed) set.
	ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*PhaseExport, error)
	ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*StudentEnrollmentExport, error)

	// RecordPhaseExportAudit appends one append-only row to
	// audit.data_access_log recording that an admin exported the full
	// PII of a phase. The caller MUST refuse the export if this returns
	// an error (no trail, no disclosure). range_start/range_end carry
	// the phase's service window — the temporal span of the disclosed
	// data — and metadata carries phase_id, format, status_filter and the
	// row counts. Must run inside the request's tenant transaction so the
	// row lands under the correct tenant (RLS).
	RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *enrollmentModels.Phase, format, statusFilter string, requestCount, childCount int) error
}

// PhaseExport is the fully-assembled payload for the compact phase
// export. Rows preserve ListAdmin order (newest submission first).
// Schemas is keyed by schema_id so a renderer can resolve a custom
// field's German label + select-option labels for any request,
// regardless of which form-schema version it was pinned to.
type PhaseExport struct {
	Phase   *enrollmentModels.Phase
	Schemas map[int64]*enrollmentModels.FormSchema
	Rows    []ExportRequestRow
}

// Counts returns the number of requests (rows) and the total number of
// children across them. Pure derivation from Rows — the single source
// of truth for both the audit row's metadata (ExportPhase) and the
// rendered document subtitle (the export handler).
func (e *PhaseExport) Counts() (requests, children int) {
	for _, row := range e.Rows {
		children += len(row.Children)
	}
	return len(e.Rows), children
}

// StudentEnrollmentExport is the fully-assembled payload for exports from
// one student's kartei tab. Rows contain only the matching request_child row,
// even when the original parent submission included siblings.
type StudentEnrollmentExport struct {
	StudentID int64
	Schemas   map[int64]*enrollmentModels.FormSchema
	Phases    map[int64]*enrollmentModels.Phase
	Rows      []ExportRequestRow
}

func (e *StudentEnrollmentExport) Counts() (requests, children int) {
	for _, row := range e.Rows {
		children += len(row.Children)
	}
	return len(e.Rows), children
}

// ExportRequestRow is one parent submission with its resolved children
// and the additional guardians (co-guardians) the parent submitted
// alongside the primary contact. Guardians is nil when the submission had
// none.
type ExportRequestRow struct {
	Request   *enrollmentModels.Request
	Children  []ExportChildRow
	Guardians []*enrollmentModels.RequestGuardian
}

// ExportChildRow is one child plus its care-offering selections,
// resolved to offering names/days via the phase's offering catalog.
type ExportChildRow struct {
	Child     *enrollmentModels.RequestChild
	Offerings []ChildOfferingRow
}

// ChildOfferingRow is one care-offering selection for a child, as
// surfaced by ListChildOfferings. SelectedDays mirrors the DB column
// — nil when the offering runs in admin-fixed mode, non-nil only when
// the parent picked specific days.
type ChildOfferingRow struct {
	OfferingID            int64
	OfferingName          string
	DaysOfWeekMode        string
	SelectedDays          []string
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
	AvailableDays         []string
}

// DecisionServiceConfig is the dep-injection bundle. The auth-side
// repos (Account/AccountTenant/AccountRole/Role) are the slice-2-fix
// addition: they let an approval that touches a known email attach
// the new tenant directly to the parent's existing portal account
// instead of mailing an invite that would overwrite their password
// on accept.
// DecisionSettingsResolver is the narrow contract the decision service
// needs from the platform settings service. Only the activation-mode
// lookup is required today, so the interface stays minimal.
type DecisionSettingsResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

type DecisionServiceConfig struct {
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestGuardianRepo      enrollmentModels.RequestGuardianRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	FormSchemaRepo           enrollmentModels.FormSchemaRepository // needed to look up FormField.Target for each submitted answer
	DataAccessLogRepo        auditModels.DataAccessLogRepository   // append-only GDPR audit row written on phase export
	OfferingAdjustmentRepo   auditModels.EnrollmentOfferingAdjustmentRepository
	SchoolRepo               platformModels.SchoolRepository
	PersonRepo               users.PersonRepository
	StaffRepo                users.StaffRepository
	StudentRepo              users.StudentRepository
	StudentGuardianRepo      users.StudentGuardianRepository
	GuardianProfileRepo      users.GuardianProfileRepository
	GuardianPhoneRepo        users.GuardianPhoneNumberRepository             // target: guardian.phone_numbers / contact.phone_numbers
	PickupScheduleRepo       scheduleModels.StudentPickupScheduleRepository  // target: schedule.pickup
	ArrivalScheduleRepo      scheduleModels.StudentArrivalScheduleRepository // target: schedule.arrival
	StudentEnrollmentRepo    activities.StudentEnrollmentRepository
	ActivityGroupRepo        activities.GroupRepository
	ActivityScheduleRepo     activities.ScheduleRepository
	CalendarPeriodRepo       scheduleModels.CalendarPeriodRepository
	AccountRepo              authModels.AccountRepository
	AccountTenantRepo        authModels.AccountTenantRepository
	AccountRoleRepo          authModels.AccountRoleRepository
	RoleRepo                 authModels.RoleRepository
	OutboxEnqueuer           OutboxEnqueuer
	FrontendURL              string                   // not used by parent-facing emails today; kept for future admin links
	ParentsURL               string                   // status link in approved/waitlisted/rejected emails. Falls back to FrontendURL when empty.
	Settings                 DecisionSettingsResolver // resolves enrollment.default_activation_mode on approval; nil-safe (defaults to scheduled)
	Logger                   *slog.Logger
}

type decisionService struct {
	requestRepo              enrollmentModels.RequestRepository
	requestChildRepo         enrollmentModels.RequestChildRepository
	requestGuardianRepo      enrollmentModels.RequestGuardianRepository
	requestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	careOfferingRepo         enrollmentModels.CareOfferingRepository
	phaseRepo                enrollmentModels.PhaseRepository
	formSchemaRepo           enrollmentModels.FormSchemaRepository
	dataAccessLogRepo        auditModels.DataAccessLogRepository
	offeringAdjustmentRepo   auditModels.EnrollmentOfferingAdjustmentRepository
	schoolRepo               platformModels.SchoolRepository
	personRepo               users.PersonRepository
	staffRepo                users.StaffRepository
	studentRepo              users.StudentRepository
	studentGuardianRepo      users.StudentGuardianRepository
	guardianProfileRepo      users.GuardianProfileRepository
	guardianPhoneRepo        users.GuardianPhoneNumberRepository
	pickupScheduleRepo       scheduleModels.StudentPickupScheduleRepository
	arrivalScheduleRepo      scheduleModels.StudentArrivalScheduleRepository
	studentEnrollmentRepo    activities.StudentEnrollmentRepository
	activityGroupRepo        activities.GroupRepository
	activityScheduleRepo     activities.ScheduleRepository
	calendarPeriodRepo       scheduleModels.CalendarPeriodRepository
	accountRepo              authModels.AccountRepository
	accountTenantRepo        authModels.AccountTenantRepository
	accountRoleRepo          authModels.AccountRoleRepository
	roleRepo                 authModels.RoleRepository
	outboxEnqueuer           OutboxEnqueuer
	frontendURL              string
	parentsURL               string
	settings                 DecisionSettingsResolver
	logger                   *slog.Logger
}

func NewDecisionService(cfg DecisionServiceConfig) DecisionService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &decisionService{
		requestRepo:              cfg.RequestRepo,
		requestChildRepo:         cfg.RequestChildRepo,
		requestGuardianRepo:      cfg.RequestGuardianRepo,
		requestChildOfferingRepo: cfg.RequestChildOfferingRepo,
		careOfferingRepo:         cfg.CareOfferingRepo,
		phaseRepo:                cfg.PhaseRepo,
		formSchemaRepo:           cfg.FormSchemaRepo,
		dataAccessLogRepo:        cfg.DataAccessLogRepo,
		offeringAdjustmentRepo:   cfg.OfferingAdjustmentRepo,
		schoolRepo:               cfg.SchoolRepo,
		personRepo:               cfg.PersonRepo,
		staffRepo:                cfg.StaffRepo,
		studentRepo:              cfg.StudentRepo,
		studentGuardianRepo:      cfg.StudentGuardianRepo,
		guardianProfileRepo:      cfg.GuardianProfileRepo,
		guardianPhoneRepo:        cfg.GuardianPhoneRepo,
		pickupScheduleRepo:       cfg.PickupScheduleRepo,
		arrivalScheduleRepo:      cfg.ArrivalScheduleRepo,
		studentEnrollmentRepo:    cfg.StudentEnrollmentRepo,
		activityGroupRepo:        cfg.ActivityGroupRepo,
		activityScheduleRepo:     cfg.ActivityScheduleRepo,
		calendarPeriodRepo:       cfg.CalendarPeriodRepo,
		accountRepo:              cfg.AccountRepo,
		accountTenantRepo:        cfg.AccountTenantRepo,
		accountRoleRepo:          cfg.AccountRoleRepo,
		roleRepo:                 cfg.RoleRepo,
		outboxEnqueuer:           cfg.OutboxEnqueuer,
		frontendURL:              cfg.FrontendURL,
		parentsURL: func() string {
			parents := strings.TrimRight(strings.TrimSpace(cfg.ParentsURL), "/")
			if parents != "" {
				return parents
			}
			return strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")
		}(),
		settings: cfg.Settings,
		logger:   logger,
	}
}

func (s *decisionService) List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error) {
	requests, err := s.requestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{
		PhaseID:          filters.PhaseID,
		ChildStatus:      filters.ChildStatus,
		CreatedStudentID: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("decision: list requests: %w", err)
	}

	out := make([]*RequestSummary, 0, len(requests))
	for _, req := range requests {
		summary, err := s.assemble(ctx, req)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *decisionService) ListByStudent(ctx context.Context, studentID int64) ([]*RequestSummary, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("decision: student_id required")
	}
	requests, err := s.requestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{
		CreatedStudentID: studentID,
	})
	if err != nil {
		return nil, fmt.Errorf("decision: list requests by student: %w", err)
	}

	out := make([]*RequestSummary, 0, len(requests))
	for _, req := range requests {
		summary, err := s.assemble(ctx, req)
		if err != nil {
			return nil, err
		}
		filterRequestSummaryChildren(summary, studentID)
		if len(summary.Children) > 0 {
			out = append(out, summary)
		}
	}
	return out, nil
}

func filterRequestSummaryChildren(summary *RequestSummary, studentID int64) {
	if summary == nil || studentID <= 0 {
		return
	}
	children := summary.Children[:0]
	for _, child := range summary.Children {
		if child != nil && child.CreatedStudentID != nil && *child.CreatedStudentID == studentID {
			children = append(children, child)
		}
	}
	summary.Children = children
}

func (s *decisionService) Get(ctx context.Context, requestID int64) (*RequestSummary, error) {
	req, err := s.requestRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	return s.assemble(ctx, req)
}

func (s *decisionService) assemble(ctx context.Context, req *enrollmentModels.Request) (*RequestSummary, error) {
	phase, err := s.phaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil {
		// Phase may have been deleted under us - surface as "phase
		// missing" but don't drop the row from the list.
		s.logger.Warn("decision: phase lookup failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("phase_id", req.PhaseID),
			slog.String("error", err.Error()))
		phase = nil
	}
	children, err := s.requestChildRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("decision: list children for request %d: %w", req.ID, err)
	}
	var guardians []*enrollmentModels.RequestGuardian
	if s.requestGuardianRepo != nil {
		guardians, err = s.requestGuardianRepo.ListByRequestID(ctx, req.ID)
		if err != nil {
			return nil, fmt.Errorf("decision: list guardians for request %d: %w", req.ID, err)
		}
	}
	return &RequestSummary{Request: req, Phase: phase, Children: children, Guardians: guardians}, nil
}

// ListChildOfferings returns the offerings each child in this request
// picked. Per-child rows are keyed by request_child_id; offerings
// missing a parent_choice day picker land with SelectedDays == nil.
// Used by the admin detail endpoint to render the Betreuungsangebote
// next to each child for the decision UI.
func (s *decisionService) ListChildOfferings(ctx context.Context, requestID int64) (map[int64][]ChildOfferingRow, error) {
	if requestID <= 0 {
		return nil, fmt.Errorf("decision: request_id required")
	}
	children, err := s.requestChildRepo.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("decision: list children for offerings: %w", err)
	}
	out := make(map[int64][]ChildOfferingRow, len(children))
	for _, child := range children {
		links, lerr := s.requestChildOfferingRepo.ListByRequestChildID(ctx, child.ID)
		if lerr != nil {
			return nil, fmt.Errorf("decision: list offerings for child %d: %w", child.ID, lerr)
		}
		rows := make([]ChildOfferingRow, 0, len(links))
		for _, link := range links {
			row := ChildOfferingRow{
				OfferingID:            link.CareOfferingID,
				SelectedDays:          link.SelectedDays,
				ManualSelectedDays:    link.ManualSelectedDays,
				AutomaticSelectedDays: link.AutomaticSelectedDays,
			}
			if s.careOfferingRepo != nil {
				if off, err := s.careOfferingRepo.FindByID(ctx, link.CareOfferingID); err == nil && off != nil {
					row.OfferingName = off.Name
					row.DaysOfWeekMode = off.DaysOfWeekMode
					row.AvailableDays = off.AvailableDays
				}
			}
			rows = append(rows, row)
		}
		out[child.ID] = rows
	}
	return out, nil
}

// ExportPhase loads the export payload (exportData) and records the
// GDPR access-log row before returning, so the two are inseparable: a
// caller cannot obtain the PII without the audit, and a failed audit
// write fails the whole call. Both halves run on the caller's tenant tx.
func (s *decisionService) ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*PhaseExport, error) {
	data, err := s.exportData(ctx, phaseID, childStatusFilter)
	if err != nil {
		return nil, err
	}
	requestCount, childCount := data.Counts()
	if err := s.RecordPhaseExportAudit(ctx, actorAccountID, actorRole, data.Phase, format, childStatusFilter, requestCount, childCount); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *decisionService) ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*StudentEnrollmentExport, error) {
	data, err := s.exportStudentData(ctx, studentID)
	if err != nil {
		return nil, err
	}
	requestCount, childCount := data.Counts()
	if err := s.recordStudentExportAudit(ctx, actorAccountID, actorRole, data, format, requestCount, childCount); err != nil {
		return nil, err
	}
	return data, nil
}

// exportData assembles the whole phase in a fixed number of queries:
//  1. all requests of the phase            (requestRepo.ListAdmin)
//  2. the phase row                         (phaseRepo.FindByID)
//  3. all children of those requests        (requestChildRepo.ListByRequestIDs)
//  4. all offering links of those children  (requestChildOfferingRepo.ListByRequestChildIDs)
//  5. the phase's care-offering catalog     (careOfferingRepo.ListByPhase)
//  6. each distinct form-schema version     (formSchemaRepo.FindByID, ~1×)
//
// Everything else is in-memory grouping — no query runs inside a loop.
// Unexported: the only way to obtain export data is via ExportPhase,
// which always records the GDPR audit row.
func (s *decisionService) exportData(ctx context.Context, phaseID int64, childStatusFilter string) (*PhaseExport, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("decision: export: phase_id required")
	}

	requests, err := s.requestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{PhaseID: phaseID})
	if err != nil {
		return nil, fmt.Errorf("decision: export list requests: %w", err)
	}
	// Bound the in-memory payload: the renderers assemble the whole
	// document at once, so reject a pathologically large phase up front
	// rather than allocating an unbounded file. The cap is well above any
	// real OGS phase.
	if len(requests) > maxExportRequests {
		return nil, fmt.Errorf("decision: export phase %d has %d requests (max %d): %w",
			phaseID, len(requests), maxExportRequests, ErrExportTooLarge)
	}
	phase, err := s.phaseRepo.FindByID(ctx, phaseID)
	if err != nil {
		// Map a missing/unreachable phase to the not-found sentinel so the
		// handler can answer 404 rather than 500. Mirrors phaseService.GetByID,
		// which collapses every FindByID error to ErrPhaseNotFound.
		return nil, fmt.Errorf("decision: export load phase %d: %w", phaseID, ErrPhaseNotFound)
	}

	reqIDs := make([]int64, 0, len(requests))
	for _, req := range requests {
		reqIDs = append(reqIDs, req.ID)
	}

	children, err := s.requestChildRepo.ListByRequestIDs(ctx, reqIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: export load children: %w", err)
	}
	childIDs := make([]int64, 0, len(children))
	for _, c := range children {
		childIDs = append(childIDs, c.ID)
	}

	links, err := s.requestChildOfferingRepo.ListByRequestChildIDs(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: export load offerings: %w", err)
	}

	offerings, err := s.careOfferingRepo.ListByPhase(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: export load care offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, off := range offerings {
		offeringByID[off.ID] = off
	}

	// Group offering links per child, resolving each to its catalog name/days.
	offeringsByChild := make(map[int64][]ChildOfferingRow, len(childIDs))
	for _, link := range links {
		row := ChildOfferingRow{
			OfferingID:            link.CareOfferingID,
			SelectedDays:          link.SelectedDays,
			ManualSelectedDays:    link.ManualSelectedDays,
			AutomaticSelectedDays: link.AutomaticSelectedDays,
		}
		if off := offeringByID[link.CareOfferingID]; off != nil {
			row.OfferingName = off.Name
			row.DaysOfWeekMode = off.DaysOfWeekMode
			row.AvailableDays = off.AvailableDays
		}
		offeringsByChild[link.RequestChildID] = append(offeringsByChild[link.RequestChildID], row)
	}

	// Group children per request.
	childrenByRequest := make(map[int64][]*enrollmentModels.RequestChild, len(reqIDs))
	for _, c := range children {
		childrenByRequest[c.RequestID] = append(childrenByRequest[c.RequestID], c)
	}

	// Load + group the additional guardians (co-guardians) so the export
	// carries every submitted contact, matching the admin detail and the
	// public status page. Defensive against an unwired repo.
	guardiansByRequest := make(map[int64][]*enrollmentModels.RequestGuardian)
	if s.requestGuardianRepo != nil {
		guardians, gerr := s.requestGuardianRepo.ListByRequestIDs(ctx, reqIDs)
		if gerr != nil {
			return nil, fmt.Errorf("decision: export load co-guardians: %w", gerr)
		}
		for _, g := range guardians {
			guardiansByRequest[g.RequestID] = append(guardiansByRequest[g.RequestID], g)
		}
	}

	// Load each distinct pinned schema version once for label resolution.
	schemas := make(map[int64]*enrollmentModels.FormSchema)
	for _, req := range requests {
		if req.SchemaID == nil {
			continue
		}
		if _, ok := schemas[*req.SchemaID]; ok {
			continue
		}
		fs, ferr := s.formSchemaRepo.FindByID(ctx, *req.SchemaID)
		if ferr != nil {
			// Fail closed. The renderer only emits custom answers for fields
			// found in the loaded schemas, so a missing schema would silently
			// drop this request's custom_data from the file while the audit
			// row still records a "complete" disclosure. That is worse than a
			// hard failure for a GDPR export. There is also no legitimate way
			// to reach this branch: DeleteSchema refuses to drop any schema
			// version a request still references (ErrFormSchemaHasRequests),
			// so a pinned schema behind an existing request cannot have been
			// deleted. A FindByID error here is therefore a transient read
			// error (a retry succeeds) or data corruption (must be loud) —
			// never an intentionally-removed schema. Abort before any audit
			// row is written so no incomplete disclosure is recorded.
			s.logger.Error("decision: export schema lookup failed, aborting export",
				slog.Int64("schema_id", *req.SchemaID),
				slog.String("error", ferr.Error()))
			return nil, fmt.Errorf("decision: export load schema %d: %w", *req.SchemaID, ferr)
		}
		schemas[*req.SchemaID] = fs
	}

	rows := make([]ExportRequestRow, 0, len(requests))
	for _, req := range requests {
		kids := childrenByRequest[req.ID]
		childRows := make([]ExportChildRow, 0, len(kids))
		for _, c := range kids {
			// Per-child status filter, mirroring the admin list's status
			// dropdown (exact match on the child's own status).
			if childStatusFilter != "" && c.Status != childStatusFilter {
				continue
			}
			childRows = append(childRows, ExportChildRow{Child: c, Offerings: offeringsByChild[c.ID]})
		}
		// Under an active filter, a request with no matching child is omitted
		// entirely — the list shows children, not registrations, so a request
		// with zero matching children is invisible there too.
		if childStatusFilter != "" && len(childRows) == 0 {
			continue
		}
		rows = append(rows, ExportRequestRow{Request: req, Children: childRows, Guardians: guardiansByRequest[req.ID]})
	}

	return &PhaseExport{Phase: phase, Schemas: schemas, Rows: rows}, nil
}

func (s *decisionService) exportStudentData(ctx context.Context, studentID int64) (*StudentEnrollmentExport, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("decision: export student: student_id required")
	}
	if s.studentRepo == nil {
		return nil, fmt.Errorf("decision: export student: student repo not configured")
	}
	if _, err := s.studentRepo.FindByID(ctx, studentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("decision: export student load student %d: %w", studentID, ErrDecisionStudentNotFound)
		}
		return nil, fmt.Errorf("decision: export student load student %d: %w", studentID, err)
	}

	requests, err := s.requestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{CreatedStudentID: studentID})
	if err != nil {
		return nil, fmt.Errorf("decision: export student list requests: %w", err)
	}
	if len(requests) > maxExportRequests {
		return nil, fmt.Errorf("decision: export student %d has %d requests (max %d): %w",
			studentID, len(requests), maxExportRequests, ErrExportTooLarge)
	}

	reqIDs := make([]int64, 0, len(requests))
	phaseIDs := make(map[int64]struct{}, len(requests))
	for _, req := range requests {
		reqIDs = append(reqIDs, req.ID)
		phaseIDs[req.PhaseID] = struct{}{}
	}

	children, err := s.requestChildRepo.ListByRequestIDs(ctx, reqIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: export student load children: %w", err)
	}
	filteredChildren := make([]*enrollmentModels.RequestChild, 0, len(children))
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		if child.CreatedStudentID == nil || *child.CreatedStudentID != studentID {
			continue
		}
		filteredChildren = append(filteredChildren, child)
		childIDs = append(childIDs, child.ID)
	}

	links, err := s.requestChildOfferingRepo.ListByRequestChildIDs(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: export student load offerings: %w", err)
	}

	offeringByID := make(map[int64]*enrollmentModels.CareOffering)
	for phaseID := range phaseIDs {
		offerings, err := s.careOfferingRepo.ListByPhase(ctx, phaseID)
		if err != nil {
			return nil, fmt.Errorf("decision: export student load care offerings: %w", err)
		}
		for _, off := range offerings {
			offeringByID[off.ID] = off
		}
	}

	offeringsByChild := make(map[int64][]ChildOfferingRow, len(childIDs))
	for _, link := range links {
		row := ChildOfferingRow{
			OfferingID:            link.CareOfferingID,
			SelectedDays:          link.SelectedDays,
			ManualSelectedDays:    link.ManualSelectedDays,
			AutomaticSelectedDays: link.AutomaticSelectedDays,
		}
		if off := offeringByID[link.CareOfferingID]; off != nil {
			row.OfferingName = off.Name
			row.DaysOfWeekMode = off.DaysOfWeekMode
			row.AvailableDays = off.AvailableDays
		}
		offeringsByChild[link.RequestChildID] = append(offeringsByChild[link.RequestChildID], row)
	}

	childrenByRequest := make(map[int64][]*enrollmentModels.RequestChild, len(reqIDs))
	for _, child := range filteredChildren {
		childrenByRequest[child.RequestID] = append(childrenByRequest[child.RequestID], child)
	}

	phases := make(map[int64]*enrollmentModels.Phase, len(phaseIDs))
	for phaseID := range phaseIDs {
		phase, err := s.phaseRepo.FindByID(ctx, phaseID)
		if err != nil {
			return nil, fmt.Errorf("decision: export student load phase %d: %w", phaseID, err)
		}
		phases[phaseID] = phase
	}

	schemas := make(map[int64]*enrollmentModels.FormSchema)
	for _, req := range requests {
		if req.SchemaID == nil {
			continue
		}
		if _, ok := schemas[*req.SchemaID]; ok {
			continue
		}
		if s.formSchemaRepo == nil {
			return nil, fmt.Errorf("decision: export student schema repo not configured")
		}
		schema, err := s.formSchemaRepo.FindByID(ctx, *req.SchemaID)
		if err != nil {
			return nil, fmt.Errorf("decision: export student load schema %d: %w", *req.SchemaID, err)
		}
		schemas[*req.SchemaID] = schema
	}

	rows := make([]ExportRequestRow, 0, len(requests))
	for _, req := range requests {
		childRows := make([]ExportChildRow, 0, len(childrenByRequest[req.ID]))
		for _, child := range childrenByRequest[req.ID] {
			childRows = append(childRows, ExportChildRow{
				Child:     child,
				Offerings: offeringsByChild[child.ID],
			})
		}
		if len(childRows) == 0 {
			continue
		}
		rows = append(rows, ExportRequestRow{Request: req, Children: childRows})
	}

	return &StudentEnrollmentExport{
		StudentID: studentID,
		Schemas:   schemas,
		Phases:    phases,
		Rows:      rows,
	}, nil
}

// RecordPhaseExportAudit writes the GDPR access-log row for a phase
// export. Synchronous and blocking: the caller refuses to serve the
// file when this errors. The DataAccessLog repo populates tenant_id
// from the context's tenant transaction, so this must be called inside
// one.
func (s *decisionService) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *enrollmentModels.Phase, format, statusFilter string, requestCount, childCount int) error {
	if s.dataAccessLogRepo == nil {
		return fmt.Errorf("decision: export audit: data access log repo not configured")
	}
	if phase == nil {
		return fmt.Errorf("decision: export audit: phase required")
	}
	if actorAccountID <= 0 {
		return fmt.Errorf("decision: export audit: actor account id required")
	}
	// actor_role is NOT NULL; the column never carries an empty string.
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}
	// An empty filter means the export covered every child — record it as
	// "all" so the audit trail is explicit about the disclosed scope.
	statusFilterLabel := statusFilter
	if statusFilterLabel == "" {
		statusFilterLabel = "all"
	}

	entry := &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		ResourceType:   auditModels.ResourceTypeEnrollmentPhaseExport,
		RangeStart:     phase.ServiceStartDate.BerlinMidnight(),
		RangeEnd:       phase.ServiceEndDate.EndOfDay(),
		AccessedAt:     time.Now(),
	}
	entry.SetMetadata("phase_id", phase.ID)
	entry.SetMetadata("format", format)
	entry.SetMetadata("status_filter", statusFilterLabel)
	entry.SetMetadata("request_count", requestCount)
	entry.SetMetadata("child_count", childCount)

	if err := s.dataAccessLogRepo.Create(ctx, entry); err != nil {
		return fmt.Errorf("decision: export audit write: %w", err)
	}
	return nil
}

func (s *decisionService) recordStudentExportAudit(ctx context.Context, actorAccountID int64, actorRole string, data *StudentEnrollmentExport, format string, requestCount, childCount int) error {
	if s.dataAccessLogRepo == nil {
		return fmt.Errorf("decision: student export audit: data access log repo not configured")
	}
	if data == nil || data.StudentID <= 0 {
		return fmt.Errorf("decision: student export audit: student required")
	}
	if actorAccountID <= 0 {
		return fmt.Errorf("decision: student export audit: actor account id required")
	}
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}

	now := time.Now()
	rangeStart := now
	rangeEnd := now
	for _, phase := range data.Phases {
		if phase == nil {
			continue
		}
		start := phase.ServiceStartDate.BerlinMidnight()
		end := phase.ServiceEndDate.EndOfDay()
		if rangeStart.Equal(now) || start.Before(rangeStart) {
			rangeStart = start
		}
		if rangeEnd.Equal(now) || end.After(rangeEnd) {
			rangeEnd = end
		}
	}

	entry := &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		ResourceType:   auditModels.ResourceTypeEnrollmentStudentExport,
		StudentID:      &data.StudentID,
		RangeStart:     rangeStart,
		RangeEnd:       rangeEnd,
		AccessedAt:     now,
	}
	entry.SetMetadata("format", format)
	entry.SetMetadata("request_count", requestCount)
	entry.SetMetadata("child_count", childCount)

	if err := s.dataAccessLogRepo.Create(ctx, entry); err != nil {
		return fmt.Errorf("decision: student export audit write: %w", err)
	}
	return nil
}

// Decide updates a single child's status. When status==approved the
// service also creates the downstream records (Person + Student +
// GuardianProfile + StudentGuardian + StudentEnrollment[s]) inside the
// same tenant tx the handler provides - failure of any one rolls the
// whole approval back. Parent decision emails are enqueued via the
// outbox in the same tx; guardian invitation creation is returned as a
// PendingGuardianInvite for the handler to fire post-commit.
//
// Idempotency: applying the same status twice is a no-op. Re-applying
// any new status to an already-terminal child (approved/rejected/
// withdrawn) returns ErrDecisionAlreadyTerminal - admins must use
// dedicated revoke/promote flows for those (deferred).
func (s *decisionService) Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error) {
	if input.RequestID <= 0 {
		return nil, fmt.Errorf("%w: request_id required", ErrDecisionInvalidStatus)
	}
	if input.ChildID <= 0 {
		return nil, fmt.Errorf("%w: child_id required", ErrDecisionInvalidStatus)
	}
	if !validDecisionStatuses[input.Status] {
		return nil, fmt.Errorf("%w: %s", ErrDecisionInvalidStatus, input.Status)
	}

	request, err := s.requestRepo.FindByID(ctx, input.RequestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	phase, err := s.phaseRepo.FindByID(ctx, request.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: load phase: %w", err)
	}

	children, err := s.requestChildRepo.ListByRequestID(ctx, input.RequestID)
	if err != nil {
		return nil, fmt.Errorf("decision: load children: %w", err)
	}

	var target *enrollmentModels.RequestChild
	for _, c := range children {
		if c.ID == input.ChildID {
			target = c
			break
		}
	}
	if target == nil {
		return nil, ErrDecisionChildNotFound
	}

	// No-op: same status. Don't bump reviewed_at when nothing changes.
	if target.Status == string(input.Status) {
		return &DecideOutcome{Child: target}, nil
	}

	// Block transitions out of a terminal status. Promotion flows
	// (waitlisted → approved, etc.) come in slice 2; for slice 1 the
	// admin can only move out of submitted / under_review / waitlisted.
	if target.IsTerminal() {
		return nil, ErrDecisionAlreadyTerminal
	}

	reason := strings.TrimSpace(input.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	outcome := &DecideOutcome{}

	// Approval is the heavy path: create downstream records first so
	// any failure rolls back BEFORE we flip the status. The status
	// update closes the loop after the records exist; if it fails the
	// records are still rolled back via the surrounding tenant tx.
	if input.Status == DecisionApproved {
		invite, err := s.applyApproval(ctx, request, target, phase, input.ReviewedBy)
		if err != nil {
			return nil, err
		}
		outcome.PendingInvite = invite
	}

	if err := s.requestChildRepo.UpdateStatus(ctx, target.ID, string(input.Status), reasonPtr, input.ReviewedBy); err != nil {
		return nil, fmt.Errorf("decision: update child status: %w", err)
	}

	s.logger.Info("enrollment decision applied",
		slog.Int64("request_id", input.RequestID),
		slog.Int64("child_id", input.ChildID),
		slog.String("status", string(input.Status)),
		slog.Int64("reviewed_by", input.ReviewedBy),
		slog.Bool("created_records", input.Status == DecisionApproved),
	)

	// Enqueue parent decision email. Best-effort: log on error but
	// don't roll back the approval. (Outbox writes share the outer tx,
	// so a hard failure WILL roll back - log+swallow keeps the
	// behaviour aligned with submit's "delivery is downstream of the
	// decision".)
	s.enqueueDecisionEmail(ctx, request, target, phase, input.Status, reasonPtr)

	// Refetch to surface DB-managed fields (reviewed_at, updated_at).
	refreshed, err := s.findChildByID(ctx, input.RequestID, input.ChildID)
	if err != nil {
		// Fall back to the in-memory copy with the new status applied.
		target.Status = string(input.Status)
		target.StatusReason = reasonPtr
		outcome.Child = target
		return outcome, nil
	}
	outcome.Child = refreshed
	return outcome, nil
}

// applyApproval creates the downstream records that an approval
// implies. Runs inside the outer tenant tx the handler provides - every
// repo call shares that tx via base.GetDB(ctx, db).
//
// Returns a PendingGuardianInvite when the guardian needs an invitation
// (no existing portal account) so the handler can fire it post-commit.
// resolveActivationMode reads enrollment.default_activation_mode for the
// tenant in context. The setting was registered from the start with a
// registry default of "scheduled", so a plain ResolveString (tenant
// override → registry default) is correct — there is no env-var fallback.
//
// Any resolve error or empty/unknown value falls back to the safe
// "scheduled" default rather than failing the approval: an unconfigured
// or momentarily-unreadable setting must never block a school from
// approving a child.
func (s *decisionService) resolveActivationMode(ctx context.Context) string {
	if s.settings == nil {
		return configModel.EnrollmentActivationModeScheduled
	}
	mode, err := s.settings.ResolveString(ctx, configModel.KeyEnrollmentDefaultActivationMode)
	if err != nil {
		s.logger.Warn("decision: resolve activation mode failed, defaulting to scheduled",
			slog.String("key", configModel.KeyEnrollmentDefaultActivationMode),
			slog.String("error", err.Error()),
		)
		return configModel.EnrollmentActivationModeScheduled
	}
	if mode == configModel.EnrollmentActivationModeImmediate {
		return configModel.EnrollmentActivationModeImmediate
	}
	// Empty (unconfigured) or any unknown value normalizes to scheduled.
	return configModel.EnrollmentActivationModeScheduled
}

type approvalActivationPlan struct {
	Mode          string
	ActivateOn    *timezone.Date
	StudentStatus users.StudentStatus
}

func (s *decisionService) approvalActivationPlan(ctx context.Context, phase *enrollmentModels.Phase) approvalActivationPlan {
	mode := s.resolveActivationMode(ctx)
	if mode == configModel.EnrollmentActivationModeImmediate {
		return approvalActivationPlan{
			Mode:          enrollmentModels.ChildActivationImmediate,
			StudentStatus: users.StudentStatusActive,
		}
	}

	activateOn := phase.ServiceStartDate
	status := users.StudentStatusPending
	if !activateOn.After(timezone.TodayDate()) {
		status = users.StudentStatusActive
	}
	return approvalActivationPlan{
		Mode:          enrollmentModels.ChildActivationScheduled,
		ActivateOn:    &activateOn,
		StudentStatus: status,
	}
}

func (s *decisionService) stampActivationPlan(ctx context.Context, requestChildID int64, plan approvalActivationPlan) error {
	if err := s.requestChildRepo.UpdateActivationPlan(ctx, requestChildID, plan.Mode, plan.ActivateOn); err != nil {
		return fmt.Errorf("decision: stamp activation plan: %w", err)
	}
	return nil
}

func (s *decisionService) applyApproval(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
	reviewedBy int64,
) (*PendingGuardianInvite, error) {
	if s.personRepo == nil || s.studentRepo == nil || s.guardianProfileRepo == nil ||
		s.studentGuardianRepo == nil {
		return nil, fmt.Errorf("decision: approval requires user repos (person/student/guardian)")
	}

	// Rollover branch (migration 1.15.62): when this request_child was
	// carried forward from a previous year's approved enrollment, we
	// already have a Person + Student row for this human. Update the
	// existing student (new school year, possibly bumped class) and
	// skip Person/Student creation entirely. Materialize the new
	// year's care offerings and link the new request_child to the
	// same student so the admin UI still navigates correctly.
	if child.RolloverSourceChildID != nil {
		return s.applyApprovalRollover(ctx, request, child, phase)
	}

	// 1. Resolve or create the guardian profile (per-tenant).
	guardian, profileWasNew, err := s.resolveGuardianProfile(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("decision: resolve guardian: %w", err)
	}

	// 1b. Cross-tenant account check. If the email already has a global
	// auth.accounts row (from another school's enrollment, an admin
	// invitation, etc.), attach the new tenant + this profile to it
	// directly. This bypasses the invitation flow entirely - the
	// invitation accept path overwrites the password hash, which is
	// the wrong UX when the parent already has a working password from
	// another school.
	//
	// PR 11/4: when the request carries guardian_account_id (parent
	// submitted while logged in), prefer the by-ID lookup over the
	// email lookup. A parent who edits their email in the form would
	// otherwise miss the attach step and trigger an invitation that
	// overwrites their existing password. The by-ID path is also
	// strictly cheaper - no platform-wide email index hit.
	if guardian.AccountID == nil {
		var (
			linked bool
			err    error
		)
		switch {
		case request.GuardianAccountID != nil && *request.GuardianAccountID > 0:
			linked, err = s.attachExistingAccountByID(ctx, guardian, *request.GuardianAccountID)
		case guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "":
			linked, err = s.attachExistingAccountIfPresent(ctx, guardian)
		}
		if err != nil {
			return nil, fmt.Errorf("decision: attach existing account: %w", err)
		}
		if linked {
			s.logger.Info("decision: linked approval to existing global account",
				slog.Int64("guardian_profile_id", guardian.ID),
				slog.Int64("tenant_id", tenant.FromContext(ctx)),
				slog.Bool("profile_was_new", profileWasNew),
				slog.Bool("via_request_account_id", request.GuardianAccountID != nil),
			)
		}
	}

	// 2. Person row for the child. DateOfBirth is required so a copy
	// is fine.
	dob := child.DateOfBirth
	person := &users.Person{
		FirstName: child.FirstName,
		LastName:  child.LastName,
		Birthday:  &dob,
	}
	if err := person.Validate(); err != nil {
		return nil, fmt.Errorf("decision: validate person: %w: %w", ErrDecisionInvalidData, err)
	}
	if err := s.personRepo.Create(ctx, person); err != nil {
		return nil, fmt.Errorf("decision: create person: %w", err)
	}

	// 3. Student row pinned to the phase's service window. The
	// enrollment.default_activation_mode setting decides the initial
	// status:
	//   - "scheduled" (default): status 'pending' lets the
	//     activate-students scheduler flip to 'active' once enrolled_from
	//     (ServiceStartDate) arrives.
	//   - "immediate": status 'active' right away, so the child appears
	//     in lists/attendance/check-in immediately.
	//
	// enrolled_from stays the phase's ServiceStartDate in BOTH modes — it
	// is the official, consistent start date (not the arbitrary approval
	// day) and is no longer read once a student is active; only
	// enrolled_until drives later deactivation.
	schoolClass := s.gradeToClass(child.TargetGradeLevel)
	enrolledFrom := phase.ServiceStartDate
	enrolledUntil := phase.ServiceEndDate
	guardianEmail := request.GuardianEmail
	guardianPhone := request.GuardianPhone
	activationPlan := s.approvalActivationPlan(ctx, phase)

	student := &users.Student{
		PersonID:      person.ID,
		SchoolClass:   schoolClass,
		Status:        activationPlan.StudentStatus,
		EnrolledFrom:  &enrolledFrom,
		EnrolledUntil: &enrolledUntil,
		GuardianEmail: &guardianEmail,
		GuardianPhone: guardianPhone,
	}
	if err := student.Validate(); err != nil {
		return nil, fmt.Errorf("decision: validate student: %w: %w", ErrDecisionInvalidData, err)
	}
	if err := s.studentRepo.Create(ctx, student); err != nil {
		return nil, fmt.Errorf("decision: create student: %w", err)
	}

	// 4. Link student ↔ guardian as the primary relationship.
	rel := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  guardian.ID,
		RelationshipType:   "guardian",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
	}
	authorize.ApplyStudentGuardianRole(rel, authorize.GuardianRolePrimaryGuardian)
	if err := rel.Validate(); err != nil {
		return nil, fmt.Errorf("decision: validate student_guardian: %w", err)
	}
	if err := s.studentGuardianRepo.Create(ctx, rel); err != nil {
		return nil, fmt.Errorf("decision: create student_guardian: %w", err)
	}

	// 4a. Link any additional guardians (co-guardians the parent added
	// beyond the primary) to the same student. Mapped identically to the
	// primary except IsPrimary=false; contact-only, so no account
	// attach/invitation. Resolved once per request and reused across the
	// request's children.
	if err := s.linkAdditionalGuardians(ctx, request, student.ID); err != nil {
		return nil, fmt.Errorf("decision: link additional guardians: %w", err)
	}

	// 4b. Dispatch every targeted form field onto the right downstream
	// record. Scalar targets (health_info, extra_info, photo_
	// consent, pickup_status) update the Student row in place;
	// structured targets (bus weekday flags, phone_list,
	// weekday_schedule, contact_list)
	// create association rows. Failures inside one field don't abort
	// the approval — the targeted-field path is best-effort, the same
	// philosophy the invitation-email enqueue uses elsewhere in this
	// service.
	if err := s.applyTargetedFields(ctx, request, child, student, guardian, reviewedBy, targetedFieldSyncOptions{}); err != nil {
		s.logger.Warn("decision: targeted-field dispatch had errors",
			slog.Int64("request_id", request.ID),
			slog.Int64("child_id", child.ID),
			slog.String("error", err.Error()),
		)
	}
	if err := s.linkAdditionalGuardians(ctx, request, student.ID); err != nil {
		return nil, fmt.Errorf("decision: relink additional guardians after targeted fields: %w", err)
	}

	// 5. Materialize per-care-offering enrollments. Every offering the
	// parent picked that is bound to an activity_group becomes a row
	// in activities.student_enrollments. Offerings without an activity
	// group (pure schedule-only offerings) are skipped.
	if err := s.materializeEnrollments(ctx, child.ID, student.ID, phase); err != nil {
		return nil, err
	}

	if err := s.stampActivationPlan(ctx, child.ID, activationPlan); err != nil {
		return nil, err
	}

	// 6. Stamp the request_children row with the resulting student id
	// so the admin UI can link to the new student record. Failure is
	// fatal - if we can't link them, future revoke flows can't reverse
	// the approval cleanly.
	if err := s.linkCreatedStudent(ctx, child.ID, student.ID); err != nil {
		return nil, fmt.Errorf("decision: link created student: %w", err)
	}

	// 7. Decide whether to schedule a guardian invitation. Skip when
	// the guardian already has a portal account (per the design Q
	// answer: "when they already have an account we do not need to
	// create a new one").
	if !guardian.HasAccount && guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "" {
		s.logger.Debug("decision: scheduling guardian invitation",
			slog.Int64("guardian_profile_id", guardian.ID),
			slog.Bool("profile_was_new", profileWasNew),
		)
		return &PendingGuardianInvite{
			GuardianProfileID: guardian.ID,
			CreatedBy:         reviewedBy,
		}, nil
	}
	return nil, nil
}

// applyApprovalRollover is the abbreviated approval path for
// rolled-over enrollments. The student row already exists from last
// year's approval — we update its school_class + enrollment window,
// materialize the new year's care offerings, and link the new
// request_child to that same student.
//
// Falls back to the full applyApproval path when the source row
// doesn't have a created_student_id (defensive — the migration's
// unique index already prevents source-row reuse so this is rare).
func (s *decisionService) applyApprovalRollover(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
) (*PendingGuardianInvite, error) {
	source, err := s.requestChildRepo.FindByID(ctx, *child.RolloverSourceChildID)
	if err != nil || source == nil || source.CreatedStudentID == nil {
		s.logger.Warn("decision: rollover source has no created_student, falling back to fresh approval",
			slog.Int64("request_child_id", child.ID),
			slog.Any("source_id", child.RolloverSourceChildID),
		)
		// Falling back means we'd re-enter applyApproval, which would
		// loop back here because child.RolloverSourceChildID is still
		// set. To break the loop, clear it in-memory for this call
		// only — the DB row is unchanged, so the audit trail still
		// shows the row was a rollover.
		clone := *child
		clone.RolloverSourceChildID = nil
		// reviewedBy isn't tracked on this code path; falling back to
		// 0 keeps the audit row consistent (UpdateStatus already
		// handles 0 by skipping the column).
		return s.applyApproval(ctx, request, &clone, phase, 0)
	}

	studentID := *source.CreatedStudentID
	existing, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("decision: rollover load existing student %d: %w", studentID, err)
	}

	activationPlan := s.approvalActivationPlan(ctx, phase)

	// Update school_class / enrollment window. Already-active children
	// stay active even for a future rollover phase, so current attendance
	// workflows are not interrupted. Inactive/pending children follow the
	// approval-time activation plan.
	existing.SchoolClass = s.gradeToClass(child.TargetGradeLevel)
	enrolledFrom := phase.ServiceStartDate
	enrolledUntil := phase.ServiceEndDate
	existing.EnrolledFrom = &enrolledFrom
	existing.EnrolledUntil = &enrolledUntil
	if existing.Status != users.StudentStatusActive {
		existing.Status = activationPlan.StudentStatus
	}
	if err := s.studentRepo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("decision: rollover update student: %w", err)
	}

	// Materialize the new year's care offerings under this student.
	if err := s.materializeEnrollments(ctx, child.ID, studentID, phase); err != nil {
		return nil, err
	}

	if err := s.stampActivationPlan(ctx, child.ID, activationPlan); err != nil {
		return nil, err
	}

	// Link the new request_child to the same student so the admin UI
	// can navigate from either year's submission to one student row.
	if err := s.linkCreatedStudent(ctx, child.ID, studentID); err != nil {
		return nil, fmt.Errorf("decision: rollover link student: %w", err)
	}

	s.logger.Info("decision: rollover approval — updated existing student",
		slog.Int64("request_child_id", child.ID),
		slog.Int64("student_id", studentID),
	)

	// Skip guardian invitation logic — by definition a rolled-over
	// child's parent already had an enrollment last year, so they
	// either already have a portal account or they were already
	// offered one last year. No new invite here.
	return nil, nil
}

// resolveGuardianProfile finds an existing tenant-scoped guardian by
// email or creates a new one. Phone numbers from the submission are
// NOT migrated into guardian_phone_numbers in slice 2 - that's a
// separate hop the admin guardian editor already supports if they want
// to enrich the profile later.
func (s *decisionService) resolveGuardianProfile(
	ctx context.Context,
	request *enrollmentModels.Request,
) (*users.GuardianProfile, bool, error) {
	email := strings.TrimSpace(strings.ToLower(request.GuardianEmail))

	if email != "" {
		existing, err := s.guardianProfileRepo.FindByEmail(ctx, email)
		if err == nil && existing != nil {
			if err := s.applyStandaloneGuardianNameCorrection(ctx, existing, request); err != nil {
				return nil, false, err
			}
			return existing, false, nil
		}
		// errors.Is(sql.ErrNoRows) and "not found" both flow through;
		// we don't distinguish - if the lookup fails we still create.
	}

	// Build a fresh profile.
	first := strings.TrimSpace(request.GuardianFirstName)
	last := strings.TrimSpace(request.GuardianLastName)

	profile := &users.GuardianProfile{
		FirstName:              first,
		LastName:               last,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	if email != "" {
		emailCopy := email
		profile.Email = &emailCopy
	}
	if err := profile.Validate(); err != nil {
		return nil, false, fmt.Errorf("decision: validate guardian profile: %w", err)
	}
	if err := s.guardianProfileRepo.Create(ctx, profile); err != nil {
		return nil, false, fmt.Errorf("decision: create guardian profile: %w", err)
	}
	return profile, true, nil
}

func (s *decisionService) applyStandaloneGuardianNameCorrection(ctx context.Context, profile *users.GuardianProfile, request *enrollmentModels.Request) error {
	if request == nil {
		return nil
	}
	return s.applyStandaloneGuardianProfileNameCorrection(ctx, profile, request.GuardianFirstName, request.GuardianLastName)
}

func (s *decisionService) applyStandaloneGuardianProfileNameCorrection(ctx context.Context, profile *users.GuardianProfile, firstName, lastName string) error {
	if profile == nil || profile.AccountID != nil || profile.HasAccount {
		return nil
	}
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
	if profile.FirstName == first && profile.LastName == last {
		return nil
	}
	profile.FirstName = first
	profile.LastName = last
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("decision: validate guardian profile name correction: %w", err)
	}
	if err := s.guardianProfileRepo.Update(ctx, profile); err != nil {
		return fmt.Errorf("decision: update guardian profile name correction: %w", err)
	}
	return nil
}

// linkAdditionalGuardians materializes the co-guardians stored on the
// request (enrollment.request_guardians) as additional students_guardians
// links for the just-created student. Co-guardians are mapped identically
// to the primary guardian except IsPrimary=false; they are contact-only,
// so there is deliberately NO account attach and NO invitation here.
//
// Each co-guardian resolves to a users.guardian_profiles row exactly once
// per request: the first child approval stamps guardian_profile_id back on
// the request_guardians row and later child approvals reuse it. Children
// are approved one at a time, and an email-less co-guardian cannot be
// deduped by email — the stamp is what prevents duplicate profiles.
func (s *decisionService) linkAdditionalGuardians(
	ctx context.Context,
	request *enrollmentModels.Request,
	studentID int64,
) error {
	if s.requestGuardianRepo == nil {
		return nil
	}
	extras, err := s.requestGuardianRepo.ListByRequestID(ctx, request.ID)
	if err != nil {
		return fmt.Errorf("list additional guardians: %w", err)
	}
	for _, extra := range extras {
		profileID, err := s.resolveAdditionalGuardianProfile(ctx, extra)
		if err != nil {
			return err
		}
		rel := &users.StudentGuardian{
			StudentID:          studentID,
			GuardianProfileID:  profileID,
			RelationshipType:   "guardian",
			IsPrimary:          false,
			IsEmergencyContact: true,
			CanPickup:          true,
		}
		authorize.ApplyStudentGuardianRole(rel, authorize.GuardianRoleEmergency)
		if err := rel.Validate(); err != nil {
			return fmt.Errorf("validate co-guardian student_guardian: %w", err)
		}
		if err := s.upsertContactStudentGuardianLink(ctx, rel); err != nil {
			return fmt.Errorf("link co-guardian student_guardian: %w", err)
		}
		// Persist the co-guardian's phone number, mirroring the primary
		// guardian. A co-guardian can be a phone-only contact (no email),
		// so this is the ONLY reachable detail downstream contact views
		// have — dropping it would leave an approved emergency contact /
		// pickup-authorized person unreachable. Non-fatal: a phone write
		// failure must not roll back an otherwise-valid approval, same as
		// the primary path.
		if extra.Phone != nil {
			if err := s.createGuardianPhoneNumber(ctx, profileID, *extra.Phone); err != nil {
				s.logger.Warn("decision: persist co-guardian phone failed",
					slog.Int64("guardian_profile_id", profileID),
					slog.String("error", err.Error()))
			}
		}
	}
	return nil
}

func (s *decisionService) reconcilePrimaryGuardianLink(
	ctx context.Context,
	request *enrollmentModels.Request,
	studentID int64,
) (*users.GuardianProfile, error) {
	guardian, _, err := s.resolveGuardianProfile(ctx, request)
	if err != nil {
		return nil, err
	}
	if s.studentGuardianRepo == nil {
		return guardian, nil
	}
	links, err := s.studentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("decision: list approved child guardians: %w", err)
	}
	var primaryLink *users.StudentGuardian
	var currentLink *users.StudentGuardian
	for _, link := range links {
		if link == nil {
			continue
		}
		if link.IsPrimary {
			primaryLink = link
		}
		if link.GuardianProfileID == guardian.ID {
			currentLink = link
		}
	}

	if currentLink != nil {
		currentLink.RelationshipType = "guardian"
		currentLink.IsPrimary = true
		currentLink.IsEmergencyContact = true
		currentLink.CanPickup = true
		authorize.ApplyStudentGuardianRole(currentLink, authorize.GuardianRolePrimaryGuardian)
		if err := currentLink.Validate(); err != nil {
			return nil, fmt.Errorf("decision: validate current primary guardian link: %w", err)
		}
		if err := s.studentGuardianRepo.Update(ctx, currentLink); err != nil {
			return nil, fmt.Errorf("decision: update current primary guardian link: %w", err)
		}
		if primaryLink != nil && primaryLink.ID != currentLink.ID {
			if err := s.studentGuardianRepo.Delete(ctx, primaryLink.ID); err != nil {
				return nil, fmt.Errorf("decision: remove stale primary guardian link: %w", err)
			}
		}
		return guardian, nil
	}

	if primaryLink != nil {
		primaryLink.GuardianProfileID = guardian.ID
		primaryLink.RelationshipType = "guardian"
		primaryLink.IsPrimary = true
		primaryLink.IsEmergencyContact = true
		primaryLink.CanPickup = true
		authorize.ApplyStudentGuardianRole(primaryLink, authorize.GuardianRolePrimaryGuardian)
		if err := primaryLink.Validate(); err != nil {
			return nil, fmt.Errorf("decision: validate primary guardian link: %w", err)
		}
		if err := s.studentGuardianRepo.Update(ctx, primaryLink); err != nil {
			return nil, fmt.Errorf("decision: update primary guardian link: %w", err)
		}
		return guardian, nil
	}

	rel := &users.StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardian.ID,
		RelationshipType:   "guardian",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
	}
	authorize.ApplyStudentGuardianRole(rel, authorize.GuardianRolePrimaryGuardian)
	if err := rel.Validate(); err != nil {
		return nil, fmt.Errorf("decision: validate missing primary guardian link: %w", err)
	}
	if err := s.studentGuardianRepo.Create(ctx, rel); err != nil {
		return nil, fmt.Errorf("decision: create missing primary guardian link: %w", err)
	}
	return guardian, nil
}

func (s *decisionService) reconcileApprovedChildGuardians(
	ctx context.Context,
	request *enrollmentModels.Request,
	studentID int64,
	previousGuardians []*enrollmentModels.RequestGuardian,
) (map[int64]bool, error) {
	currentProfileIDs := map[int64]bool{}
	if s.requestGuardianRepo == nil || s.studentGuardianRepo == nil {
		return currentProfileIDs, nil
	}
	if err := s.linkAdditionalGuardians(ctx, request, studentID); err != nil {
		return currentProfileIDs, fmt.Errorf("decision: relink additional guardians: %w", err)
	}
	current, err := s.requestGuardianRepo.ListByRequestID(ctx, request.ID)
	if err != nil {
		return currentProfileIDs, fmt.Errorf("decision: list current additional guardians: %w", err)
	}
	for _, row := range current {
		if row == nil {
			continue
		}
		profileID, err := s.resolveAdditionalGuardianProfile(ctx, row)
		if err != nil {
			return currentProfileIDs, err
		}
		if profileID > 0 {
			currentProfileIDs[profileID] = true
		}
	}
	previousProfileIDs := map[int64]bool{}
	for _, row := range previousGuardians {
		if row != nil && row.GuardianProfileID != nil && *row.GuardianProfileID > 0 {
			previousProfileIDs[*row.GuardianProfileID] = true
		}
	}
	if err := s.deleteRemovedStudentGuardianLinks(ctx, studentID, previousProfileIDs, currentProfileIDs); err != nil {
		return currentProfileIDs, fmt.Errorf("decision: unlink removed additional guardians: %w", err)
	}
	return currentProfileIDs, nil
}

func (s *decisionService) deleteRemovedStudentGuardianLinks(ctx context.Context, studentID int64, previous, keep map[int64]bool) error {
	if len(previous) == 0 || s.studentGuardianRepo == nil {
		return nil
	}
	links, err := s.studentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link == nil || link.IsPrimary || authorize.IsFullGuardianRole(link.GuardianRole) {
			continue
		}
		if previous[link.GuardianProfileID] && !keep[link.GuardianProfileID] {
			if err := s.studentGuardianRepo.Delete(ctx, link.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeGuardianProfileKeepSets(sets ...map[int64]bool) map[int64]bool {
	out := map[int64]bool{}
	for _, set := range sets {
		for id := range set {
			if id > 0 {
				out[id] = true
			}
		}
	}
	return out
}

func (s *decisionService) contactProfileIDsFromPreviousSnapshot(
	ctx context.Context,
	snapshot map[string]any,
	child *enrollmentModels.RequestChild,
	studentID int64,
	fieldKey string,
) (map[int64]bool, error) {
	out := map[int64]bool{}
	if snapshot == nil || child == nil || s.guardianProfileRepo == nil {
		return out, nil
	}
	childRow := snapshotChildByID(snapshot, child.ID)
	if childRow == nil {
		return out, nil
	}
	custom := mapFromAny(childRow["custom_data"])
	raw := custom[fieldKey]
	if raw == nil {
		return out, nil
	}
	var entries []enrollmentModels.ContactEntry
	if err := decodeStructured(raw, &entries); err != nil {
		return out, nil
	}
	for _, entry := range entries {
		email := strings.TrimSpace(strings.ToLower(entry.Email))
		if email == "" {
			profiles, err := s.phoneOnlyContactProfilesForStudent(ctx, studentID, entry)
			if err != nil {
				return out, err
			}
			for _, profile := range profiles {
				if profile != nil && profile.ID > 0 {
					out[profile.ID] = true
				}
			}
			continue
		}
		profile, err := s.guardianProfileRepo.FindByEmail(ctx, email)
		if err == nil && profile != nil && profile.ID > 0 {
			out[profile.ID] = true
			continue
		}
		if err != nil && !errors.Is(err, users.ErrGuardianProfileNotFound) {
			return out, err
		}
	}
	return out, nil
}

// createGuardianPhoneNumber inserts phone as the guardian's primary mobile
// number on users.guardian_phone_numbers. A blank phone or an unwired repo
// is a no-op. A unique-violation (the guardian already has this number on
// file from a previous enrollment, or an earlier child of the same request
// already wrote it) is benign and returns nil; any other error is returned
// for the caller to decide whether to surface or swallow.
func (s *decisionService) createGuardianPhoneNumber(ctx context.Context, profileID int64, phone string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" || s.guardianPhoneRepo == nil {
		return nil
	}
	row := &users.GuardianPhoneNumber{
		GuardianProfileID: profileID,
		PhoneNumber:       phone,
		PhoneType:         users.PhoneType("mobile"),
		IsPrimary:         true,
	}
	if err := s.guardianPhoneRepo.Create(ctx, row); err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil
		}
		return err
	}
	return nil
}

// resolveAdditionalGuardianProfile returns the guardian_profiles id for a
// co-guardian, creating the profile on first use and stamping the id back
// on the request_guardians row so the request's other children reuse it.
// A stamped id always wins (idempotent across the request's children).
// Otherwise an existing profile is preferred by email when the co-guardian
// gave one; absent that, a fresh contact-only profile is created.
func (s *decisionService) resolveAdditionalGuardianProfile(
	ctx context.Context,
	extra *enrollmentModels.RequestGuardian,
) (int64, error) {
	if extra.GuardianProfileID != nil && *extra.GuardianProfileID > 0 {
		return *extra.GuardianProfileID, nil
	}

	email := ""
	if extra.Email != nil {
		email = strings.TrimSpace(strings.ToLower(*extra.Email))
	}

	var profileID int64
	if email != "" {
		if existing, err := s.guardianProfileRepo.FindByEmail(ctx, email); err == nil && existing != nil {
			if err := s.applyStandaloneGuardianProfileNameCorrection(ctx, existing, extra.FirstName, extra.LastName); err != nil {
				return 0, err
			}
			profileID = existing.ID
		}
	}
	if profileID == 0 {
		contactMethod := "email"
		if email == "" {
			contactMethod = "phone"
		}
		profile := &users.GuardianProfile{
			FirstName:              strings.TrimSpace(extra.FirstName),
			LastName:               strings.TrimSpace(extra.LastName),
			PreferredContactMethod: contactMethod,
			LanguagePreference:     "de",
		}
		if email != "" {
			e := email
			profile.Email = &e
		}
		if err := profile.Validate(); err != nil {
			return 0, fmt.Errorf("validate co-guardian profile: %w", err)
		}
		if err := s.guardianProfileRepo.Create(ctx, profile); err != nil {
			return 0, fmt.Errorf("create co-guardian profile: %w", err)
		}
		profileID = profile.ID
	}

	// Stamp the resolved profile back so the request's other children
	// reuse it. Non-fatal on failure: the link is created regardless;
	// worst case a later child creates a duplicate email-less profile.
	if err := s.requestGuardianRepo.StampResolvedProfile(ctx, extra.ID, profileID); err != nil {
		s.logger.Warn("decision: stamp co-guardian profile failed",
			slog.Int64("request_guardian_id", extra.ID),
			slog.Int64("guardian_profile_id", profileID),
			slog.String("error", err.Error()),
		)
	}
	return profileID, nil
}

// gradeToClass renders the optional grade level into the student's
// school_class field. The student schema uses free-form text; we land
// "1", "2", … when the grade is provided and "" otherwise. Admins can
// rename via the student profile UI later.
func (s *decisionService) gradeToClass(grade *int16) string {
	if grade == nil || *grade == 0 {
		return ""
	}
	return strconv.Itoa(int(*grade))
}

// materializeEnrollments writes one activities.student_enrollments row
// per RequestChildOffering whose CareOffering points at an activity
// group. Offerings without an activity_group_id are skipped (e.g.
// schedule-only offerings have no group to enroll into).
func (s *decisionService) materializeEnrollments(
	ctx context.Context,
	requestChildID, studentID int64,
	phase *enrollmentModels.Phase,
) error {
	if s.requestChildOfferingRepo == nil || s.careOfferingRepo == nil ||
		s.studentEnrollmentRepo == nil || s.activityGroupRepo == nil {
		// Wired without the offering repos: skip silently. Approvals
		// will still create the student record; the admin can attach
		// activity groups later via the activity admin UI.
		s.logger.Warn("decision: enrollment repos missing; skipping activity materialization",
			slog.Int64("request_child_id", requestChildID),
			slog.Int64("student_id", studentID))
		return nil
	}

	links, err := s.requestChildOfferingRepo.ListByRequestChildID(ctx, requestChildID)
	if err != nil {
		return fmt.Errorf("decision: list child offerings: %w", err)
	}
	if len(links) == 0 {
		return nil
	}
	offeringIDs := make([]int64, 0, len(links))
	seenOfferingIDs := make(map[int64]bool, len(links))
	for _, link := range links {
		if link.CareOfferingID <= 0 || seenOfferingIDs[link.CareOfferingID] {
			continue
		}
		seenOfferingIDs[link.CareOfferingID] = true
		offeringIDs = append(offeringIDs, link.CareOfferingID)
	}
	offerings, err := s.careOfferingRepo.ListByIDs(ctx, offeringIDs)
	if err != nil {
		return fmt.Errorf("decision: list linked care offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		offeringByID[offering.ID] = offering
	}

	validFrom := phase.ServiceStartDate
	validUntil := phase.ServiceEndDate
	type enrollmentDraft struct {
		activityGroupID  int64
		calendarPeriodID *int64
		selectedWeekday  map[int]bool
		allWeekdays      bool
	}
	drafts := make(map[int64]*enrollmentDraft)

	for _, link := range links {
		offering := offeringByID[link.CareOfferingID]
		if offering == nil {
			s.logger.Warn("decision: care offering missing for child link",
				slog.Int64("request_child_id", requestChildID),
				slog.Int64("care_offering_id", link.CareOfferingID))
			continue
		}
		if offering.ActivityGroupID == nil || *offering.ActivityGroupID == 0 {
			// Schedule-only offering - no activity group, nothing to enroll into.
			continue
		}
		group, period, err := resolveCareOfferingLinkedGroupPeriod(ctx, careOfferingTemplateDeps{
			activityGroupRepo:    s.activityGroupRepo,
			activityScheduleRepo: s.activityScheduleRepo,
			calendarPeriodRepo:   s.calendarPeriodRepo,
		}, *offering.ActivityGroupID)
		if err != nil {
			return fmt.Errorf("decision: validate linked activity group for care offering %d: %w", link.CareOfferingID, err)
		}
		if group.IsTemplate && len(offering.AvailableDays) == 0 && len(link.SelectedDays) == 0 {
			return fmt.Errorf("decision: care offering %d links to a timetable template but has no selected or available days", link.CareOfferingID)
		}
		if period != nil {
			if err := validatePhaseWithinTemplatePeriod(phase, period); err != nil {
				return fmt.Errorf("decision: validate linked timetable template period for care offering %d: %w", link.CareOfferingID, err)
			}
		}
		var periodID *int64
		if period != nil {
			periodID = &period.ID
		}
		if draft := drafts[*offering.ActivityGroupID]; draft != nil && !sameOptionalInt64(draft.calendarPeriodID, periodID) {
			return fmt.Errorf("decision: care offering %d resolves to conflicting calendar_period_id", link.CareOfferingID)
		}
		draft := drafts[*offering.ActivityGroupID]
		if draft == nil {
			draft = &enrollmentDraft{
				activityGroupID:  *offering.ActivityGroupID,
				calendarPeriodID: periodID,
				selectedWeekday:  make(map[int]bool),
			}
			drafts[*offering.ActivityGroupID] = draft
		}
		if !group.IsTemplate {
			draft.allWeekdays = true
			continue
		}
		days, err := effectiveOfferingDaysForEnrollment(offering, link)
		if err != nil {
			return fmt.Errorf("decision: resolve selected days for care offering %d: %w", link.CareOfferingID, err)
		}
		if len(days) == 0 {
			draft.allWeekdays = true
			continue
		}
		if draft.allWeekdays {
			continue
		}
		for _, day := range days {
			weekday, ok := enrollmentDayToISOWeekday(day)
			if !ok {
				return fmt.Errorf("decision: invalid selected day %q for care offering %d", day, link.CareOfferingID)
			}
			draft.selectedWeekday[weekday] = true
		}
	}

	for _, draft := range drafts {
		row := &activities.StudentEnrollment{
			StudentID:                studentID,
			ActivityGroupID:          draft.activityGroupID,
			ValidFrom:                validFrom,
			ValidUntil:               &validUntil,
			CalendarPeriodID:         draft.calendarPeriodID,
			EnrollmentRequestChildID: &requestChildID,
		}
		if !draft.allWeekdays && len(draft.selectedWeekday) > 0 {
			row.SelectedWeekdays = sortedWeekdaySet(draft.selectedWeekday)
		}
		if err := row.Validate(); err != nil {
			return fmt.Errorf("decision: validate enrollment: %w", err)
		}
		if err := s.studentEnrollmentRepo.Create(ctx, row); err != nil {
			return fmt.Errorf("decision: create enrollment: %w", err)
		}
	}
	return nil
}

func sameOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func effectiveOfferingDaysForEnrollment(
	offering *enrollmentModels.CareOffering,
	link *enrollmentModels.RequestChildOffering,
) ([]string, error) {
	if len(link.SelectedDays) > 0 {
		return link.SelectedDays, nil
	}
	switch offering.DaysOfWeekMode {
	case enrollmentModels.DaysOfWeekModeFixed:
		return offering.AvailableDays, nil
	case enrollmentModels.DaysOfWeekModeParentChoice:
		return nil, fmt.Errorf("parent-choice offering has no selected_days")
	default:
		return nil, fmt.Errorf("unknown days_of_week_mode %q", offering.DaysOfWeekMode)
	}
}

func enrollmentDayToISOWeekday(day string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(day)) {
	case "mon":
		return 1, true
	case "tue":
		return 2, true
	case "wed":
		return 3, true
	case "thu":
		return 4, true
	case "fri":
		return 5, true
	case "sat":
		return 6, true
	case "sun":
		return 7, true
	default:
		return 0, false
	}
}

func sortedWeekdaySet(days map[int]bool) []int {
	out := make([]int, 0, len(days))
	for day := 1; day <= 7; day++ {
		if days[day] {
			out = append(out, day)
		}
	}
	return out
}

// linkCreatedStudent stamps request_children.created_student_id so the
// admin UI can link from a historical request back to the new student.
func (s *decisionService) linkCreatedStudent(ctx context.Context, requestChildID, studentID int64) error {
	return s.requestChildRepo.LinkCreatedStudent(ctx, requestChildID, studentID)
}

// enqueueDecisionEmail enqueues a parent decision email matching the
// new status. Only approved/waitlisted/rejected get emails; transitions
// to under_review are admin-internal.
func (s *decisionService) enqueueDecisionEmail(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
	status DecisionStatus,
	reason *string,
) {
	if s.outboxEnqueuer == nil {
		return
	}

	var kind string
	switch status {
	case DecisionApproved:
		kind = platformModels.EmailKindEnrollmentApproved
	case DecisionWaitlisted:
		kind = platformModels.EmailKindEnrollmentWaitlisted
	case DecisionRejected:
		kind = platformModels.EmailKindEnrollmentRejected
	default:
		// under_review (and any future intermediate status) is
		// admin-internal - parent stays on the existing status email.
		return
	}

	schoolName, logoURL := emailBrandForSchool(ctx, s.schoolRepo, request.TenantID, s.parentsURL)
	footerLogoURL := motoLogoURL(s.parentsURL)
	statusURL := fmt.Sprintf("%s/enroll/status/%s", s.parentsURL, request.StatusToken)
	phaseName := ""
	if phase != nil {
		phaseName = phase.Name
	}

	payload := map[string]any{
		EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadStatusURL:         statusURL,
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadMotoLogoURL:       footerLogoURL,
		EnrollmentPayloadChildNames:        []string{child.FirstName + " " + child.LastName},
		EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
		"phase_name":                       phaseName,
	}
	if phase != nil && phase.ShowStatusReasonToParent && reason != nil && *reason != "" {
		payload["status_reason"] = *reason
	}

	if err := s.outboxEnqueuer.Enqueue(ctx, OutboxEnqueueRequest{
		Kind:              kind,
		Payload:           payload,
		RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
		RelatedEntityID:   request.ID,
	}); err != nil {
		s.logger.Error("decision: enqueue parent decision email failed",
			slog.Int64("request_id", request.ID),
			slog.Int64("child_id", child.ID),
			slog.String("kind", kind),
			slog.String("error", err.Error()))
	}
}

func (s *decisionService) findChildByID(ctx context.Context, requestID, childID int64) (*enrollmentModels.RequestChild, error) {
	children, err := s.requestChildRepo.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	for _, c := range children {
		if c.ID == childID {
			return c, nil
		}
	}
	return nil, ErrDecisionChildNotFound
}

// attachExistingAccountIfPresent looks up the parent email in the
// global auth.accounts table (no tenant_id - emails are unique
// platform-wide). If a row exists, it ensures the new tenant is
// represented in account_tenants + account_roles, and links the
// per-tenant guardian profile to that account_id. Returns true when
// the attachment happened so the caller can skip enqueueing an
// invitation.
//
// Why this exists (slice-2 follow-up): without this step, an admin
// approving the same parent at a second school would queue another
// guardian-invitation email, and the accept flow's
// createOrFindAccount overwrites the existing password hash. This
// surfaces as "I just got accepted at school B and now my school A
// password no longer works." Linking directly here keeps the parent
// silent and on the same credentials.
func (s *decisionService) attachExistingAccountIfPresent(
	ctx context.Context,
	guardian *users.GuardianProfile,
) (bool, error) {
	if s.accountRepo == nil || s.accountTenantRepo == nil ||
		s.accountRoleRepo == nil || s.roleRepo == nil {
		// Auth repos not wired - fall back to the original invitation
		// flow. Test factories that don't bring up the auth side will
		// hit this path.
		return false, nil
	}
	if guardian.Email == nil || strings.TrimSpace(*guardian.Email) == "" {
		return false, nil
	}

	email := strings.TrimSpace(strings.ToLower(*guardian.Email))
	account, err := s.accountRepo.FindByEmail(ctx, email)
	if err != nil {
		// Not-found is the common case (parent has no portal account
		// yet) - treat it as "nothing to attach", let the invitation
		// flow run. We don't import the auth package's notfound
		// detection here; instead we rely on the FindByEmail wrapper
		// returning a typed DatabaseError on real failures. Logging
		// at debug level covers both branches.
		s.logger.Debug("decision: account lookup result",
			slog.String("email", email),
			slog.String("error", err.Error()),
		)
		return false, nil
	}
	if account == nil {
		return false, nil
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return false, fmt.Errorf("attach: tenant not in context")
	}

	// 1. account_tenants mapping. Create is idempotent (ON CONFLICT
	// DO NOTHING on (account_id, tenant_id)).
	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   account.ID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.accountTenantRepo.Create(ctx, mapping); err != nil {
		return false, fmt.Errorf("attach: account_tenants: %w", err)
	}

	// 2. Guardian role for this tenant. AccountRoleRepo.Create has no
	// ON CONFLICT, so check first via FindByAccountAndRole (which
	// honours tenant scope from context) and only create when missing.
	if err := s.ensureGuardianRoleForTenant(ctx, account.ID); err != nil {
		return false, err
	}

	// 3. Link the per-tenant guardian profile row to the global
	// account. LinkAccount also flips has_account=true so future
	// approvals for the same profile see the linked state.
	if err := s.guardianProfileRepo.LinkAccount(ctx, guardian.ID, account.ID); err != nil {
		return false, fmt.Errorf("attach: link profile: %w", err)
	}
	guardian.AccountID = &account.ID
	guardian.HasAccount = true

	return true, nil
}

// attachExistingAccountByID links the guardian profile to the account
// identified by accountID directly, bypassing the email lookup that
// attachExistingAccountIfPresent uses. Called when the enrollment
// request was submitted by an authenticated parent (PR 11) - the
// JWT-derived account_id is more authoritative than the email field
// (which the parent could have typed differently in the form).
//
// Same downstream steps as the email-based path: account_tenants
// mapping + guardian role for the new tenant + LinkAccount on the
// per-tenant profile. Returns true on success so the caller skips the
// invitation enqueue.
func (s *decisionService) attachExistingAccountByID(
	ctx context.Context,
	guardian *users.GuardianProfile,
	accountID int64,
) (bool, error) {
	if s.accountRepo == nil || s.accountTenantRepo == nil ||
		s.accountRoleRepo == nil || s.roleRepo == nil {
		return false, nil
	}
	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil || account == nil {
		// Account was deleted between submission and decision - fall
		// back to email lookup so the approval still goes through.
		s.logger.Warn("decision: request guardian_account_id no longer resolvable, falling back to email",
			slog.Int64("guardian_account_id", accountID),
		)
		if guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "" {
			return s.attachExistingAccountIfPresent(ctx, guardian)
		}
		return false, nil
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return false, fmt.Errorf("attach by id: tenant not in context")
	}

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   account.ID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.accountTenantRepo.Create(ctx, mapping); err != nil {
		return false, fmt.Errorf("attach by id: account_tenants: %w", err)
	}
	if err := s.ensureGuardianRoleForTenant(ctx, account.ID); err != nil {
		return false, err
	}
	if err := s.guardianProfileRepo.LinkAccount(ctx, guardian.ID, account.ID); err != nil {
		return false, fmt.Errorf("attach by id: link profile: %w", err)
	}
	guardian.AccountID = &account.ID
	guardian.HasAccount = true
	return true, nil
}

// ensureGuardianRoleForTenant assigns the guardian base role for the
// current tenant, idempotently. Mirrors the linkProfileToAccount step
// in services/auth.guardianInvitationService so a parent linked here
// gets the same role footprint as one who came in via the invite
// accept flow.
func (s *decisionService) ensureGuardianRoleForTenant(ctx context.Context, accountID int64) error {
	role, err := s.roleRepo.FindByName(ctx, guardianRoleName)
	if err != nil {
		return fmt.Errorf("attach: guardian role lookup: %w", err)
	}
	if role == nil {
		return fmt.Errorf("attach: guardian role not found")
	}

	existing, err := s.accountRoleRepo.FindByAccountAndRole(ctx, accountID, role.ID)
	if err == nil && existing != nil {
		// Already assigned for this tenant (FindByAccountAndRole
		// honours tenant scope) - nothing to do.
		return nil
	}

	assignment := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    role.ID,
	}
	if err := s.accountRoleRepo.Create(ctx, assignment); err != nil {
		return fmt.Errorf("attach: create account_role: %w", err)
	}
	return nil
}

// applyTargetedFields walks the request's pinned schema and dispatches
// every field carrying a non-empty Target onto the appropriate
// downstream record. The student row may be mutated in place for
// scalar targets and persisted at the end via studentRepo.Update.
//
// Best-effort overall: per-field errors are collected and returned in
// one combined error string but never abort the approval. The student
// + per-child records have already been written by the caller.
type targetedFieldSyncOptions struct {
	Replace                bool
	PreviousSnapshot       map[string]any
	KeepGuardianProfileIDs map[int64]bool
}

func (s *decisionService) applyTargetedFields(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	student *users.Student,
	guardian *users.GuardianProfile,
	reviewedBy int64,
	options targetedFieldSyncOptions,
) error {
	if s.formSchemaRepo == nil || request.SchemaID == nil {
		return nil
	}
	schema, err := s.formSchemaRepo.FindByID(ctx, *request.SchemaID)
	if err != nil || schema == nil {
		return nil
	}

	var errs []string
	studentDirty := false
	// A unified departure target wins by replacing both legacy maps after the
	// loop; the legacy Buskind/Abholregelung targets mutate their own map
	// directly, so a form carrying both still combines correctly (#1610).
	var explicitDeparture *users.DepartureDays
	var explicitAllowedDeparture *users.AllowedDepartureModes

	fieldRaws := make([]any, len(schema.Fields))
	targetHasMeaningfulValue := make(map[string]bool, len(schema.Fields))
	for i := range schema.Fields {
		field := schema.Fields[i]
		if field.Target == "" {
			continue
		}
		raw := s.readFieldValue(request, child, &field)
		fieldRaws[i] = raw
		if targetedFieldHasMeaningfulValue(field.Target, raw) {
			targetHasMeaningfulValue[field.Target] = true
		}
	}

	pickupScheduleDeleted := false
	arrivalScheduleDeleted := false

	for i := range schema.Fields {
		field := schema.Fields[i]
		if field.Target == "" {
			continue
		}
		raw := fieldRaws[i]
		if raw == nil && !options.Replace {
			continue
		}
		if options.Replace && !targetedFieldHasMeaningfulValue(field.Target, raw) && targetHasMeaningfulValue[field.Target] {
			continue
		}

		switch field.Target {
		case enrollmentModels.TargetStudentHealthInfo:
			if str := stringValue(raw); str != "" {
				student.HealthInfo = &str
				studentDirty = true
			} else if options.Replace {
				student.HealthInfo = nil
				studentDirty = true
			}
		case enrollmentModels.TargetStudentExtraInfo:
			if str := stringValue(raw); str != "" {
				student.ExtraInfo = &str
				studentDirty = true
			} else if options.Replace {
				student.ExtraInfo = nil
				studentDirty = true
			}
		case enrollmentModels.TargetStudentDeparture:
			if raw == nil {
				student.AllowedDepartureModes = users.AllowedDepartureModes{}
				student.DepartureDays = users.DepartureDays{}
				student.BusDays = users.BusDays{}
				student.PickupDays = users.PickupDays{}
				student.DepartureCompanionNote = nil
				studentDirty = true
			} else if days, err := decodeDepartureDays(raw); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			} else {
				// Union with any earlier same-target field rather than overwriting:
				// last-field-wins would let a later bus/pickup field silently drop an
				// accompanied day an earlier field carried, which validation and
				// sanitization (which use any-field semantics) already accepted (#1694).
				if explicitDeparture != nil {
					days = explicitDeparture.Merge(days)
				}
				explicitDeparture = &days
				studentDirty = true
			}
		case enrollmentModels.TargetStudentAllowedDepartureModes:
			if raw == nil {
				student.AllowedDepartureModes = users.AllowedDepartureModes{}
				student.DepartureDays = users.DepartureDays{}
				student.BusDays = users.BusDays{}
				student.PickupDays = users.PickupDays{}
				student.DepartureCompanionNote = nil
				studentDirty = true
			} else if modes, err := decodeAllowedDepartureModes(raw); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			} else {
				if explicitAllowedDeparture != nil {
					modes = explicitAllowedDeparture.Merge(modes)
				}
				explicitAllowedDeparture = &modes
				studentDirty = true
			}
		case enrollmentModels.TargetStudentBusDays, enrollmentModels.TargetStudentBus:
			if raw == nil {
				student.BusDays = users.BusDays{}
				studentDirty = true
			} else if days, err := decodeBusDays(raw); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			} else {
				student.BusDays = days
				studentDirty = true
			}
		case enrollmentModels.TargetStudentPickupStatus:
			if raw == nil {
				student.PickupDays = users.PickupDays{}
				studentDirty = true
			} else if days, err := decodePickupDays(raw); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			} else {
				student.PickupDays = days
				studentDirty = true
			}
		case enrollmentModels.TargetSchedulePickup:
			if options.Replace && s.pickupScheduleRepo != nil && !pickupScheduleDeleted {
				pickupScheduleDeleted = true
				if err := s.pickupScheduleRepo.DeleteByStudentID(ctx, student.ID); err != nil {
					errs = append(errs, fmt.Sprintf("%s: delete existing: %v", field.Target, err))
					continue
				}
			}
			if raw == nil {
				continue
			}
			if err := s.dispatchWeekdaySchedule(ctx, raw, student.ID, reviewedBy, true); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			}
		case enrollmentModels.TargetScheduleArrival:
			if options.Replace && s.arrivalScheduleRepo != nil && !arrivalScheduleDeleted {
				arrivalScheduleDeleted = true
				if err := s.arrivalScheduleRepo.DeleteByStudentID(ctx, student.ID); err != nil {
					errs = append(errs, fmt.Sprintf("%s: delete existing: %v", field.Target, err))
					continue
				}
			}
			if raw == nil {
				continue
			}
			if err := s.dispatchWeekdaySchedule(ctx, raw, student.ID, reviewedBy, false); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
			}
		case enrollmentModels.TargetStudentContacts:
			oldContactIDs := map[int64]bool{}
			if options.Replace {
				var err error
				oldContactIDs, err = s.contactProfileIDsFromPreviousSnapshot(ctx, options.PreviousSnapshot, child, student.ID, field.Key)
				if err != nil {
					errs = append(errs, fmt.Sprintf("%s: resolve previous contacts: %v", field.Target, err))
					continue
				}
			}
			newContactIDs := map[int64]bool{}
			if raw != nil {
				ids, err := s.dispatchContactList(ctx, raw, student.ID)
				if err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", field.Target, err))
					continue
				}
				newContactIDs = ids
			}
			if options.Replace {
				if err := s.deleteRemovedStudentGuardianLinks(ctx, student.ID, oldContactIDs, mergeGuardianProfileKeepSets(newContactIDs, options.KeepGuardianProfileIDs)); err != nil {
					errs = append(errs, fmt.Sprintf("%s: remove stale links: %v", field.Target, err))
				}
			}
		}
	}

	// Auto-flow from core base-form fields. These don't appear in the
	// Stammdaten-target picker (photo consent + guardian_phone are
	// already collected by the public base form via consent_flags.photo
	// and the dedicated guardian_phone input), but their values still
	// need to land in the right downstream rows on approval.
	//
	// All present consent flags get copied onto the student row so staff
	// looking at a single child see the consent state without joining back
	// to enrollment.requests. The public form now submits only configured
	// legal blocks, so absent flags simply leave the matching timestamp null.
	// Each stamp records the approval moment, not the parent submission
	// moment — the parent submission timestamp lives on enrollment.requests
	// if a more precise audit is ever needed.
	if request.ConsentFlags != nil {
		now := time.Now()
		if photo, ok := request.ConsentFlags[enrollmentModels.ConsentKeyPhoto].(bool); ok && photo {
			student.PhotoConsentGivenAt = &now
			if reviewedBy > 0 {
				rb := reviewedBy
				student.PhotoConsentGivenBy = &rb
			}
			studentDirty = true
		}
		if agb, ok := request.ConsentFlags[enrollmentModels.ConsentKeyAGB].(bool); ok && agb {
			student.AGBAcceptedAt = &now
			studentDirty = true
		}
		if dp, ok := request.ConsentFlags[enrollmentModels.ConsentKeyDataProcessing].(bool); ok && dp {
			student.DataProcessingAcceptedAt = &now
			studentDirty = true
		}
		if email, ok := request.ConsentFlags[enrollmentModels.ConsentKeyEmailContact].(bool); ok && email {
			student.EmailContactAcceptedAt = &now
			studentDirty = true
		}
		if options.Replace {
			if photo, _ := request.ConsentFlags[enrollmentModels.ConsentKeyPhoto].(bool); !photo {
				student.PhotoConsentGivenAt = nil
				student.PhotoConsentGivenBy = nil
				studentDirty = true
			}
			if agb, _ := request.ConsentFlags[enrollmentModels.ConsentKeyAGB].(bool); !agb {
				student.AGBAcceptedAt = nil
				studentDirty = true
			}
			if dp, _ := request.ConsentFlags[enrollmentModels.ConsentKeyDataProcessing].(bool); !dp {
				student.DataProcessingAcceptedAt = nil
				studentDirty = true
			}
			if email, _ := request.ConsentFlags[enrollmentModels.ConsentKeyEmailContact].(bool); !email {
				student.EmailContactAcceptedAt = nil
				studentDirty = true
			}
		}
	}
	if request.GuardianPhone != nil {
		if err := s.createGuardianPhoneNumber(ctx, guardian.ID, *request.GuardianPhone); err != nil {
			errs = append(errs, fmt.Sprintf("auto guardian_phone: %v", err))
		}
	}

	if explicitAllowedDeparture != nil {
		student.AllowedDepartureModes = explicitAllowedDeparture.Normalize()
		student.DepartureDays = student.AllowedDepartureModes.DepartureDays()
		student.BusDays = student.AllowedDepartureModes.BusDays()
		student.PickupDays = student.AllowedDepartureModes.PickupDays()
	}

	// A unified departure field wins by replacing both legacy maps; otherwise
	// the legacy bus/pickup targets already set their map (each preserving the
	// other). The student repository folds bus_days + pickup_days into
	// departure_days, the single source of truth, on Update (#1610).
	if explicitAllowedDeparture == nil && explicitDeparture != nil {
		student.AllowedDepartureModes = users.AllowedDepartureModesFromDeparture(*explicitDeparture)
		student.BusDays = explicitDeparture.BusDays()
		student.PickupDays = explicitDeparture.PickupDays()
	}

	// Coupled "mit wem" note (#1694): the enrollment form carries it on a
	// reserved per-child custom-data key alongside allowed_departure_modes
	// (not a separate admin-added field). Apply it ONLY when the child's final
	// departure plan actually allows the accompanied mode, so a note from a
	// client that toggled accompanied off — or a crafted submit — never lands on
	// a child with no "Mit anderem Kind" day. Capped server-side (the column is
	// unbounded TEXT).
	if child != nil && child.CustomData != nil &&
		student.AllowedDepartureModes.HasMode(users.DepartureAccompanied) {
		if note := strings.TrimSpace(stringValue(child.CustomData[enrollmentModels.TargetStudentDepartureCompanionNote])); note != "" {
			note = truncateRunes(note, users.MaxDepartureCompanionNoteLen)
			student.DepartureCompanionNote = &note
			studentDirty = true
		}
	}

	if studentDirty {
		if err := s.studentRepo.Update(ctx, student); err != nil {
			errs = append(errs, fmt.Sprintf("update student: %v", err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func targetedFieldHasMeaningfulValue(target string, raw any) bool {
	if raw == nil {
		return false
	}
	switch target {
	case enrollmentModels.TargetStudentHealthInfo, enrollmentModels.TargetStudentExtraInfo:
		return strings.TrimSpace(stringValue(raw)) != ""
	default:
		return structuredTargetValueHasEntries(raw)
	}
}

func structuredTargetValueHasEntries(raw any) bool {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value) != ""
	case []any:
		return len(value) > 0
	case []map[string]any:
		return len(value) > 0
	case []string:
		return len(value) > 0
	case map[string]any:
		return len(value) > 0
	case map[string]string:
		return len(value) > 0
	case map[string][]string:
		return len(value) > 0
	case map[string][]any:
		return len(value) > 0
	default:
		return true
	}
}

// truncateRunes caps s to at most max runes (not bytes), preserving valid
// UTF-8. Used to bound parent free-text that bypasses the client length limit.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// decodeDepartureDays decodes a FormFieldWeekdayMode submission (mon..fri →
// alone/bus/pickup) into the unified per-weekday departure model.
func decodeDepartureDays(raw any) (users.DepartureDays, error) {
	var modes enrollmentModels.WeekdayMode
	if err := decodeStructured(raw, &modes); err != nil {
		return nil, fmt.Errorf("decode weekday_mode: %w", err)
	}
	if err := modes.Validate(); err != nil {
		return nil, err
	}
	out := users.DepartureDays{}
	for day, mode := range modes {
		switch mode {
		case enrollmentModels.WeekdayModeBus:
			out[day] = users.DepartureBus
		case enrollmentModels.WeekdayModePickup:
			out[day] = users.DeparturePickup
		case enrollmentModels.WeekdayModeAccompanied:
			out[day] = users.DepartureAccompanied
		}
	}
	return out.Normalize(), nil
}

func decodeAllowedDepartureModes(raw any) (users.AllowedDepartureModes, error) {
	var modes enrollmentModels.WeekdayMultiMode
	if err := decodeStructured(raw, &modes); err != nil {
		return nil, fmt.Errorf("decode weekday_multi_mode: %w", err)
	}
	if err := modes.Validate(); err != nil {
		return nil, err
	}
	out := users.AllowedDepartureModes{}
	for day, rawModes := range modes {
		for _, mode := range rawModes {
			switch mode {
			case enrollmentModels.WeekdayModeAlone:
				out[day] = append(out[day], users.DepartureAlone)
			case enrollmentModels.WeekdayModeBus:
				out[day] = append(out[day], users.DepartureBus)
			case enrollmentModels.WeekdayModePickup:
				out[day] = append(out[day], users.DeparturePickup)
			case enrollmentModels.WeekdayModeAccompanied:
				out[day] = append(out[day], users.DepartureAccompanied)
			}
		}
	}
	return out.Normalize(), nil
}

func decodeBusDays(raw any) (users.BusDays, error) {
	if enabled, ok := raw.(bool); ok {
		return users.BusDaysFromLegacyFlag(enabled), nil
	}
	var days enrollmentModels.WeekdayBoolean
	if err := decodeStructured(raw, &days); err != nil {
		return nil, fmt.Errorf("decode weekday_boolean: %w", err)
	}
	if err := days.Validate(); err != nil {
		return nil, err
	}
	out := users.BusDays{}
	for _, day := range users.BusDayOrder {
		if days[day] {
			out[day] = true
		}
	}
	return out, nil
}

// decodePickupDays accepts either a legacy pickup answer (string) or the
// weekday_boolean map the reserved pickup_status target now uses. Pending
// submissions created before the migration carry a string; new submissions
// carry the map.
func decodePickupDays(raw any) (users.PickupDays, error) {
	if str, ok := raw.(string); ok {
		return pickupDaysFromLegacyPickupAnswer(str), nil
	}
	var days enrollmentModels.WeekdayBoolean
	if err := decodeStructured(raw, &days); err != nil {
		return nil, fmt.Errorf("decode weekday_boolean: %w", err)
	}
	if err := days.Validate(); err != nil {
		return nil, err
	}
	out := users.PickupDays{}
	for _, day := range users.PickupDayOrder {
		if days[day] {
			out[day] = true
		}
	}
	return out, nil
}

// pickupDaysFromLegacyPickupAnswer maps a pending pre-migration submission's
// pickup_status answer to the per-weekday model. Such submissions stored the
// frontend select option *value* ("picked_up" / "alone"), not the German
// label — so map "picked_up" to "all five weekdays" explicitly. A stored
// German label ("Wird abgeholt") is handled by the model helper for safety.
// Anything else — "alone", "Geht alleine nach Hause", the empty string — means
// no pickup days ("Geht alleine nach Hause").
func pickupDaysFromLegacyPickupAnswer(answer string) users.PickupDays {
	if strings.TrimSpace(answer) == "picked_up" {
		return users.PickupDaysFromLegacyStatus(users.PickupStatusPickedUp)
	}
	return users.PickupDaysFromLegacyStatus(answer)
}

// readFieldValue pulls the submission value for a field. Guardian-level
// fields live on request.CustomData; per-child fields on
// request_children.custom_data.
func (s *decisionService) readFieldValue(
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	field *enrollmentModels.FormField,
) any {
	if field.AppliesToCh {
		if child == nil || child.CustomData == nil {
			return nil
		}
		return child.CustomData[field.Key]
	}
	if request.CustomData == nil {
		return nil
	}
	return request.CustomData[field.Key]
}

// stringValue extracts a trimmed string from a raw any value. Returns
// "" for non-string or whitespace-only inputs.
func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// decodeStructured marshals raw → JSON → out so we can read interface{}
// values pulled out of a JSONB column into the typed structs declared
// in models/enrollment/form_schema.go without writing per-type
// destructuring code.
func decodeStructured(raw any, out any) error {
	bs, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, out)
}

func (s *decisionService) resolveReviewerStaffID(ctx context.Context, reviewerAccountID int64) (int64, error) {
	if reviewerAccountID <= 0 {
		return 0, fmt.Errorf("reviewer account id is required")
	}
	if s.personRepo == nil || s.staffRepo == nil {
		return 0, fmt.Errorf("reviewer staff lookup is unavailable")
	}
	person, err := s.personRepo.FindByAccountID(ctx, reviewerAccountID)
	if err != nil {
		return 0, fmt.Errorf("find reviewer person: %w", err)
	}
	if person == nil {
		return 0, fmt.Errorf("reviewer account %d has no linked person", reviewerAccountID)
	}
	staff, err := s.staffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("reviewer account %d has no linked staff", reviewerAccountID)
		}
		return 0, fmt.Errorf("find reviewer staff: %w", err)
	}
	if staff == nil {
		return 0, fmt.Errorf("reviewer account %d has no linked staff", reviewerAccountID)
	}
	return staff.ID, nil
}

// dispatchWeekdaySchedule inserts one pickup or arrival schedule row
// per non-empty weekday entry. isPickup=true targets pickup_schedules,
// false targets arrival_schedules.
func (s *decisionService) dispatchWeekdaySchedule(ctx context.Context, raw any, studentID int64, reviewedBy int64, isPickup bool) error {
	if (isPickup && s.pickupScheduleRepo == nil) || (!isPickup && s.arrivalScheduleRepo == nil) {
		return nil
	}
	var sched enrollmentModels.WeekdaySchedule
	if err := decodeStructured(raw, &sched); err != nil {
		return fmt.Errorf("decode weekday_schedule: %w", err)
	}
	if err := sched.Validate(); err != nil {
		return err
	}
	createdBy, err := s.resolveReviewerStaffID(ctx, reviewedBy)
	if err != nil {
		return err
	}
	weekdayInt := map[string]int{
		"mon": scheduleModels.WeekdayMonday,
		"tue": scheduleModels.WeekdayTuesday,
		"wed": scheduleModels.WeekdayWednesday,
		"thu": scheduleModels.WeekdayThursday,
		"fri": scheduleModels.WeekdayFriday,
	}
	for day, hhmm := range sched {
		hhmm = strings.TrimSpace(hhmm)
		if hhmm == "" {
			continue
		}
		t, err := time.Parse("15:04", hhmm)
		if err != nil {
			return fmt.Errorf("parse %s time %q: %w", day, hhmm, err)
		}
		t = timezone.WallClock(t)
		if isPickup {
			row := &scheduleModels.StudentPickupSchedule{
				StudentID:  studentID,
				Weekday:    weekdayInt[day],
				PickupTime: t,
				CreatedBy:  createdBy,
			}
			if err := s.pickupScheduleRepo.UpsertSchedule(ctx, row); err != nil {
				return fmt.Errorf("upsert pickup %s: %w", day, err)
			}
		} else {
			row := &scheduleModels.StudentArrivalSchedule{
				StudentID:       studentID,
				Weekday:         weekdayInt[day],
				ExpectedArrival: t,
				CreatedBy:       createdBy,
			}
			if err := s.arrivalScheduleRepo.Create(ctx, row); err != nil {
				return fmt.Errorf("create arrival %s: %w", day, err)
			}
		}
	}
	return nil
}

// dispatchContactList creates one additional guardian_profile (or
// reuses an existing one matched by email) per submitted contact,
// links it to the student via users.students_guardians, and inserts
// any submitted phone numbers. Mirrors the dedup-by-email behaviour
// of the CSV importer at services/import/student_import_config.go.
func contactGuardianRole(isEmergencyContact, canPickup bool) string {
	if canPickup {
		return authorize.GuardianRolePickupOnly
	}
	if isEmergencyContact {
		return authorize.GuardianRoleEmergency
	}
	return authorize.GuardianRoleCustom
}

func contactIdentityName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func contactIdentityPhones(entry enrollmentModels.ContactEntry) map[string]bool {
	phones := map[string]bool{}
	for _, phone := range entry.PhoneNumbers {
		number := strings.TrimSpace(phone.PhoneNumber)
		if number != "" {
			phones[number] = true
		}
	}
	return phones
}

func (s *decisionService) phoneOnlyContactProfilesForStudent(
	ctx context.Context,
	studentID int64,
	entry enrollmentModels.ContactEntry,
) ([]*users.GuardianProfile, error) {
	if studentID <= 0 ||
		s.studentGuardianRepo == nil ||
		s.guardianProfileRepo == nil ||
		s.guardianPhoneRepo == nil {
		return nil, nil
	}
	firstName := contactIdentityName(entry.FirstName)
	lastName := contactIdentityName(entry.LastName)
	phones := contactIdentityPhones(entry)
	if firstName == "" || lastName == "" || len(phones) == 0 {
		return nil, nil
	}

	links, err := s.studentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	profileIDs := make([]int64, 0, len(links))
	seenProfileIDs := map[int64]bool{}
	for _, link := range links {
		if link == nil ||
			link.IsPrimary ||
			authorize.IsFullGuardianRole(link.GuardianRole) ||
			link.GuardianProfileID <= 0 ||
			seenProfileIDs[link.GuardianProfileID] {
			continue
		}
		seenProfileIDs[link.GuardianProfileID] = true
		profileIDs = append(profileIDs, link.GuardianProfileID)
	}
	if len(profileIDs) == 0 {
		return nil, nil
	}

	profiles, err := s.guardianProfileRepo.FindByIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	phonesByProfile, err := s.guardianPhoneRepo.FindByGuardianIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}

	matches := make([]*users.GuardianProfile, 0, 1)
	for _, profileID := range profileIDs {
		profile := profiles[profileID]
		if profile == nil ||
			contactIdentityName(profile.FirstName) != firstName ||
			contactIdentityName(profile.LastName) != lastName {
			continue
		}
		for _, phone := range phonesByProfile[profileID] {
			if phone != nil && phones[strings.TrimSpace(phone.PhoneNumber)] {
				matches = append(matches, profile)
				break
			}
		}
	}
	return matches, nil
}

func (s *decisionService) upsertContactStudentGuardianLink(ctx context.Context, rel *users.StudentGuardian) error {
	if rel == nil {
		return errors.New("contact student guardian link cannot be nil")
	}
	if err := rel.Validate(); err != nil {
		return err
	}

	existing, err := s.studentGuardianRepo.FindByStudentAndGuardianForUpdate(ctx, rel.StudentID, rel.GuardianProfileID)
	if err != nil {
		if !errors.Is(err, users.ErrStudentGuardianNotFound) {
			return err
		}
		inserted, linkErr := s.studentGuardianRepo.LinkIfNotExists(ctx, rel)
		if linkErr != nil || inserted {
			return linkErr
		}
		existing, err = s.studentGuardianRepo.FindByStudentAndGuardianForUpdate(ctx, rel.StudentID, rel.GuardianProfileID)
		if err != nil {
			return err
		}
	}
	if existing.IsPrimary || authorize.IsFullGuardianRole(existing.GuardianRole) {
		return nil
	}

	existing.RelationshipType = rel.RelationshipType
	existing.IsEmergencyContact = rel.IsEmergencyContact
	existing.CanPickup = rel.CanPickup
	existing.EmergencyPriority = rel.EmergencyPriority
	existing.GuardianRole = rel.GuardianRole
	existing.Permissions = rel.Permissions
	updated, err := s.studentGuardianRepo.UpdateColumns(
		ctx,
		existing,
		"relationship_type",
		"is_emergency_contact",
		"can_pickup",
		"emergency_priority",
		"guardian_role",
		"permissions",
	)
	if err != nil {
		return err
	}
	if updated == 0 {
		return users.ErrStudentGuardianNotFound
	}
	return nil
}

func (s *decisionService) dispatchContactList(ctx context.Context, raw any, studentID int64) (map[int64]bool, error) {
	linkedProfileIDs := map[int64]bool{}
	if s.guardianProfileRepo == nil || s.studentGuardianRepo == nil {
		return linkedProfileIDs, nil
	}
	var entries []enrollmentModels.ContactEntry
	if err := decodeStructured(raw, &entries); err != nil {
		return linkedProfileIDs, fmt.Errorf("decode contact_list: %w", err)
	}
	for i := range entries {
		c := entries[i]
		if err := c.Validate(); err != nil {
			return linkedProfileIDs, err
		}

		var profile *users.GuardianProfile
		emailLC := strings.ToLower(strings.TrimSpace(c.Email))
		if emailLC != "" {
			existing, err := s.guardianProfileRepo.FindByEmail(ctx, emailLC)
			if err != nil && !errors.Is(err, users.ErrGuardianProfileNotFound) {
				return linkedProfileIDs, fmt.Errorf("find contact profile by email: %w", err)
			}
			if err == nil {
				profile = existing
			}
		} else {
			matches, err := s.phoneOnlyContactProfilesForStudent(ctx, studentID, c)
			if err != nil {
				return linkedProfileIDs, fmt.Errorf("resolve phone-only contact profile: %w", err)
			}
			if len(matches) > 0 {
				profile = matches[0]
			}
		}
		if profile == nil {
			profile = &users.GuardianProfile{
				FirstName:              c.FirstName,
				LastName:               c.LastName,
				PreferredContactMethod: "phone",
				LanguagePreference:     "de",
			}
			if emailLC != "" {
				profile.Email = &emailLC
			}
			if err := s.guardianProfileRepo.Create(ctx, profile); err != nil {
				return linkedProfileIDs, fmt.Errorf("create contact profile %s %s: %w", c.FirstName, c.LastName, err)
			}
		}
		linkedProfileIDs[profile.ID] = true

		// Phone numbers — append, dedup by unique index.
		if s.guardianPhoneRepo != nil {
			for j := range c.PhoneNumbers {
				p := c.PhoneNumbers[j]
				label := p.Label
				phone := &users.GuardianPhoneNumber{
					GuardianProfileID: profile.ID,
					PhoneNumber:       p.PhoneNumber,
					PhoneType:         users.PhoneType(p.PhoneType),
					IsPrimary:         p.IsPrimary,
				}
				if label != "" {
					phone.Label = &label
				}
				if err := s.guardianPhoneRepo.Create(ctx, phone); err != nil {
					if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
						return linkedProfileIDs, fmt.Errorf("create contact phone: %w", err)
					}
				}
			}
		}

		// students_guardians link with the parent-submitted flags.
		// Relationship type goes through the same German→enum mapping
		// the CSV importer uses; unknown values land on "other".
		rel := &users.StudentGuardian{
			StudentID:          studentID,
			GuardianProfileID:  profile.ID,
			RelationshipType:   importsvc.MapRelationshipType(c.RelationshipType),
			IsPrimary:          false,
			IsEmergencyContact: c.IsEmergencyContact,
			CanPickup:          c.CanPickup,
		}
		authorize.ApplyStudentGuardianRole(rel, contactGuardianRole(c.IsEmergencyContact, c.CanPickup))
		if c.EmergencyPriority > 0 {
			rel.EmergencyPriority = c.EmergencyPriority
		}
		if err := s.upsertContactStudentGuardianLink(ctx, rel); err != nil {
			return linkedProfileIDs, fmt.Errorf("link contact to student: %w", err)
		}
	}
	return linkedProfileIDs, nil
}
