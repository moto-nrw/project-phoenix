package enrollment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	importsvc "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// guardianRoleName is the auth.roles.name value the guardian invitation
// flow uses on accept. Mirrored here so an approval that finds an
// existing account can attach the role for the new tenant directly.
const guardianRoleName = "guardian"

// openSchoolClassPlaceholder satisfies the required users.students.school_class
// column when enrollment deliberately did not collect a grade or concrete
// class. It is application-owned state, not an administrator-assigned class.
const openSchoolClassPlaceholder = "offen"

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
	// ErrGuardianAccountMismatch marks an approval whose authenticated
	// submitter account (request.guardian_account_id) conflicts with the
	// guardian profile the request's email resolves to: the email already
	// belongs to a DIFFERENT account's guardian profile at this school.
	// Approving would link the child to that other account, so we fail closed
	// rather than silently misattribute it (#1663). Mapped to 400.
	ErrGuardianAccountMismatch = errors.New("enrollment guardian email belongs to a different account")
	ErrWaitlistDisabled        = errors.New("waitlist decisions are disabled for this tenant")
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
	RequestID                  int64
	ChildID                    int64
	Status                     DecisionStatus
	Reason                     string // optional; surfaced to parent only when phase.show_status_reason_to_parent
	ReviewedBy                 int64  // admin's auth account id
	SuppressParentEmail        bool
	SuppressGuardianInvitation bool
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
	// EffectiveFrom turns the adjustment into a dated switch instead of a
	// retroactive correction: enrollment rows that already started keep their
	// history and are capped at this date, and the new selection starts here.
	// Nil keeps the correction semantics (replace the whole phase window),
	// which is what an admin fixing a typo in the original submission wants.
	EffectiveFrom *timezone.Date
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
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// StudentRolloverAuditor records tracked profile changes made while approving
// or synchronizing enrollment data for an existing student.
type StudentRolloverAuditor interface {
	RecordChangesForActor(ctx context.Context, before, after *users.Student, editedBy int64) error
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after users.StudentStatus) error
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
	TimeframeRepo            scheduleModels.TimeframeRepository
	ActivityExceptionRepo    scheduleModels.ActivityExceptionRepository
	AccountRepo              authModels.AccountRepository
	AccountTenantRepo        authModels.AccountTenantRepository
	AccountRoleRepo          authModels.AccountRoleRepository
	RoleRepo                 authModels.RoleRepository
	OutboxEnqueuer           platformModels.OutboxEnqueuer
	StudentAudit             StudentRolloverAuditor
	// Broadcaster announces student_updated + student_companions_changed after
	// an approved enrollment sync replaced a child's departure plan (the write
	// that can trim "läuft mit" links). Nil-safe: without it the sync still
	// works, open student and companion views just stay stale until their next
	// manual refresh.
	Broadcaster realtime.Broadcaster
	FrontendURL string                   // not used by parent-facing emails today; kept for future admin links
	ParentsURL  string                   // status link in approved/waitlisted/rejected emails. Falls back to FrontendURL when empty.
	Settings    DecisionSettingsResolver // resolves enrollment.default_activation_mode on approval; nil-safe (defaults to scheduled)
	// LockTemplateRecurrence serializes sourced roster writes with template
	// split/end/materialization. Production wires the schedule service's
	// transaction-scoped tenant recurrence gate; tests may leave it nil.
	LockTemplateRecurrence func(context.Context) error
	Logger                 *slog.Logger
}

type decisionService struct {
	DecisionServiceConfig
}

func NewDecisionService(cfg DecisionServiceConfig) DecisionService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	cfg.ParentsURL = strings.TrimRight(strings.TrimSpace(cfg.ParentsURL), "/")
	if cfg.ParentsURL == "" {
		cfg.ParentsURL = strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/")
	}
	return &decisionService{DecisionServiceConfig: cfg}
}

func (s *decisionService) List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error) {
	requests, err := s.RequestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{
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
	requests, err := s.RequestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{
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
	req, err := s.RequestRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	return s.assemble(ctx, req)
}

func (s *decisionService) assemble(ctx context.Context, req *enrollmentModels.Request) (*RequestSummary, error) {
	phase, err := s.PhaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil {
		// Phase may have been deleted under us - surface as "phase
		// missing" but don't drop the row from the list.
		s.Logger.Warn("decision: phase lookup failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("phase_id", req.PhaseID),
			slog.String("error", err.Error()))
		phase = nil
	}
	children, err := s.RequestChildRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("decision: list children for request %d: %w", req.ID, err)
	}
	var guardians []*enrollmentModels.RequestGuardian
	if s.RequestGuardianRepo != nil {
		guardians, err = s.RequestGuardianRepo.ListByRequestID(ctx, req.ID)
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
	request, err := s.RequestRepo.FindByID(ctx, requestID)
	if err != nil || request == nil {
		return nil, fmt.Errorf("decision: load request for offerings: %w", err)
	}
	phase, err := s.PhaseRepo.FindByID(ctx, request.PhaseID)
	if err != nil || phase == nil {
		return nil, fmt.Errorf("decision: load phase for offerings: %w", err)
	}
	children, err := s.RequestChildRepo.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("decision: list children for offerings: %w", err)
	}
	out := make(map[int64][]ChildOfferingRow, len(children))
	for _, child := range children {
		links, lerr := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, child.ID, phase.ServiceStartDate)
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
			if s.CareOfferingRepo != nil {
				if off, err := s.CareOfferingRepo.FindByID(ctx, link.CareOfferingID); err == nil && off != nil {
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

	requests, err := s.RequestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{PhaseID: phaseID})
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
	phase, err := s.PhaseRepo.FindByID(ctx, phaseID)
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

	children, err := s.RequestChildRepo.ListByRequestIDs(ctx, reqIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: export load children: %w", err)
	}
	childIDs := make([]int64, 0, len(children))
	for _, c := range children {
		childIDs = append(childIDs, c.ID)
	}

	links, err := s.RequestChildOfferingRepo.ListByRequestChildIDsAtDate(ctx, childIDs, reportOfferingDate(phase))
	if err != nil {
		return nil, fmt.Errorf("decision: export load offerings: %w", err)
	}

	offerings, err := s.CareOfferingRepo.ListByPhase(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: export load care offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, off := range offerings {
		offeringByID[off.ID] = off
	}

	// Group offering links per child, resolving each to its catalog
	// name/days, and children per request.
	offeringsByChild := groupOfferingsByChild(links, offeringByID, len(childIDs))
	childrenByRequest := groupChildrenByRequest(children, len(reqIDs))

	// Load + group the additional guardians (co-guardians) so the export
	// carries every submitted contact, matching the admin detail and the
	// public status page. Defensive against an unwired repo.
	guardiansByRequest := make(map[int64][]*enrollmentModels.RequestGuardian)
	if s.RequestGuardianRepo != nil {
		guardians, gerr := s.RequestGuardianRepo.ListByRequestIDs(ctx, reqIDs)
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
		fs, ferr := s.FormSchemaRepo.FindByID(ctx, *req.SchemaID)
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
			s.Logger.Error("decision: export schema lookup failed, aborting export",
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
	if s.StudentRepo == nil {
		return nil, fmt.Errorf("decision: export student: student repo not configured")
	}
	if _, err := s.StudentRepo.FindByID(ctx, studentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("decision: export student load student %d: %w", studentID, ErrDecisionStudentNotFound)
		}
		return nil, fmt.Errorf("decision: export student load student %d: %w", studentID, err)
	}

	requests, err := s.RequestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{CreatedStudentID: studentID})
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

	children, err := s.RequestChildRepo.ListByRequestIDs(ctx, reqIDs)
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

	links, err := s.RequestChildOfferingRepo.ListByRequestChildIDsAtDate(ctx, childIDs, timezone.TodayDate())
	if err != nil {
		return nil, fmt.Errorf("decision: export student load offerings: %w", err)
	}

	offeringByID := make(map[int64]*enrollmentModels.CareOffering)
	for phaseID := range phaseIDs {
		offerings, err := s.CareOfferingRepo.ListByPhase(ctx, phaseID)
		if err != nil {
			return nil, fmt.Errorf("decision: export student load care offerings: %w", err)
		}
		for _, off := range offerings {
			offeringByID[off.ID] = off
		}
	}

	offeringsByChild := groupOfferingsByChild(links, offeringByID, len(childIDs))
	childrenByRequest := groupChildrenByRequest(filteredChildren, len(reqIDs))

	phases := make(map[int64]*enrollmentModels.Phase, len(phaseIDs))
	for phaseID := range phaseIDs {
		phase, err := s.PhaseRepo.FindByID(ctx, phaseID)
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
		if s.FormSchemaRepo == nil {
			return nil, fmt.Errorf("decision: export student schema repo not configured")
		}
		schema, err := s.FormSchemaRepo.FindByID(ctx, *req.SchemaID)
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

// groupOfferingsByChild resolves each child->offering link against the
// offering catalog and groups the rows per request child. Shared by the
// phase export and the per-student export.
func groupOfferingsByChild(links []*enrollmentModels.RequestChildOffering, offeringByID map[int64]*enrollmentModels.CareOffering, childCount int) map[int64][]ChildOfferingRow {
	offeringsByChild := make(map[int64][]ChildOfferingRow, childCount)
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
	return offeringsByChild
}

// groupChildrenByRequest groups request children per request id.
func groupChildrenByRequest(children []*enrollmentModels.RequestChild, requestCount int) map[int64][]*enrollmentModels.RequestChild {
	childrenByRequest := make(map[int64][]*enrollmentModels.RequestChild, requestCount)
	for _, c := range children {
		childrenByRequest[c.RequestID] = append(childrenByRequest[c.RequestID], c)
	}
	return childrenByRequest
}

func (s *decisionService) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *enrollmentModels.Phase, format, statusFilter string, requestCount, childCount int) error {
	if s.DataAccessLogRepo == nil {
		return fmt.Errorf("decision: export audit: data access log repo not configured")
	}
	if phase == nil {
		return fmt.Errorf("decision: export audit: phase required")
	}
	// An empty filter means the export covered every child — record it as
	// "all" so the audit trail is explicit about the disclosed scope.
	statusFilterLabel := statusFilter
	if statusFilterLabel == "" {
		statusFilterLabel = "all"
	}

	entry, err := exportAuditEntry("decision: export audit", actorAccountID, actorRole,
		auditModels.ResourceTypeEnrollmentPhaseExport,
		phase.ServiceStartDate.BerlinMidnight(), phase.ServiceEndDate.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.SetMetadata("phase_id", phase.ID)
	entry.SetMetadata("format", format)
	entry.SetMetadata("status_filter", statusFilterLabel)
	entry.SetMetadata("request_count", requestCount)
	entry.SetMetadata("child_count", childCount)

	return writeExportAudit(ctx, s.DataAccessLogRepo, entry, "decision: export audit")
}

func (s *decisionService) recordStudentExportAudit(ctx context.Context, actorAccountID int64, actorRole string, data *StudentEnrollmentExport, format string, requestCount, childCount int) error {
	if s.DataAccessLogRepo == nil {
		return fmt.Errorf("decision: student export audit: data access log repo not configured")
	}
	if data == nil || data.StudentID <= 0 {
		return fmt.Errorf("decision: student export audit: student required")
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

	entry, err := exportAuditEntry("decision: student export audit", actorAccountID, actorRole,
		auditModels.ResourceTypeEnrollmentStudentExport, rangeStart, rangeEnd, now)
	if err != nil {
		return err
	}
	entry.StudentID = &data.StudentID
	entry.SetMetadata("format", format)
	entry.SetMetadata("request_count", requestCount)
	entry.SetMetadata("child_count", childCount)

	return writeExportAudit(ctx, s.DataAccessLogRepo, entry, "decision: student export audit")
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
	// Lock the parent before its children. Cleanup, editing, and change-request
	// paths use the same order; the notification-mode pin updates the parent and
	// must not introduce a parent/child lock inversion.
	request, err := s.RequestRepo.FindByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	// Lock every sibling, in the repository's stable sort_order/id order, before
	// inspecting or changing any status. Decisions for two children in the same
	// request must serialize so the second transaction sees the first one's
	// committed state when deciding whether the digest is complete.
	children, err := s.RequestChildRepo.ListByRequestIDForUpdate(ctx, input.RequestID)
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

	// Block transitions out of a terminal status before resolving settings or
	// loading phase data. A retry of an invalid transition must keep its stable
	// conflict contract even during an unrelated settings outage.
	if target.IsTerminal() {
		return nil, ErrDecisionAlreadyTerminal
	}

	if input.Status == DecisionWaitlisted {
		enabled, resolveErr := s.resolveDecisionBool(ctx, configModel.KeyEnrollmentWaitlistEnabled, true)
		if resolveErr != nil {
			return nil, fmt.Errorf("decision: resolve waitlist setting: %w", resolveErr)
		}
		if !enabled {
			return nil, ErrWaitlistDisabled
		}
	}
	autoInviteEnabled := true
	if input.Status == DecisionApproved {
		autoInviteEnabled, err = s.resolveDecisionBool(ctx, configModel.KeyEnrollmentAutoInviteGuardianOnApprove, true)
		if err != nil {
			return nil, fmt.Errorf("decision: resolve guardian invitation setting: %w", err)
		}
	}
	phase, err := s.PhaseRepo.FindByID(ctx, request.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: load phase: %w", err)
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
		if autoInviteEnabled && !input.SuppressGuardianInvitation {
			outcome.PendingInvite = invite
		}
	}

	if err := s.RequestChildRepo.UpdateStatus(ctx, target.ID, string(input.Status), reasonPtr, input.ReviewedBy); err != nil {
		return nil, fmt.Errorf("decision: update child status: %w", err)
	}
	// Read the DB-authored review generation before constructing either
	// immediate or digest idempotency keys. Status alone is insufficient: a
	// supported rejected -> under_review -> rejected cycle is a new decision
	// even though it ends at the same status vector.
	children, err = s.RequestChildRepo.ListByRequestID(ctx, input.RequestID)
	if err != nil {
		return nil, fmt.Errorf("decision: refresh children after status update: %w", err)
	}
	target = nil
	for _, child := range children {
		if child.ID == input.ChildID {
			target = child
			break
		}
	}
	if target == nil {
		return nil, ErrDecisionChildNotFound
	}

	s.Logger.Info("enrollment decision applied",
		slog.Int64("request_id", input.RequestID),
		slog.Int64("child_id", input.ChildID),
		slog.String("status", string(input.Status)),
		slog.Int64("reviewed_by", input.ReviewedBy),
		slog.Bool("created_records", input.Status == DecisionApproved),
	)

	// Enqueue the parent decision email in the same transaction. An enqueue
	// failure rolls back the decision so a retry can safely enqueue it; the
	// tenant-scoped idempotency key prevents duplicate rows after retries.
	if !input.SuppressParentEmail && isParentVisibleDecision(input.Status) {
		if err := enqueueDecisionNotifications(ctx, decisionNotificationDependencies{
			requests:   s.RequestRepo,
			settings:   s.Settings,
			outbox:     s.OutboxEnqueuer,
			schools:    s.SchoolRepo,
			parentsURL: s.ParentsURL,
		}, request, children, phase, map[int64]struct{}{target.ID: {}}); err != nil {
			return nil, err
		}
	}
	outcome.Child = target
	return outcome, nil
}

func isParentVisibleDecision(status DecisionStatus) bool {
	return status == DecisionApproved || status == DecisionRejected || status == DecisionWaitlisted
}

func (s *decisionService) resolveDecisionBool(ctx context.Context, key string, registryDefault bool) (bool, error) {
	if s.Settings == nil {
		return registryDefault, nil
	}
	return s.Settings.ResolveBool(ctx, key)
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
	if s.Settings == nil {
		return configModel.EnrollmentActivationModeScheduled
	}
	mode, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentDefaultActivationMode)
	if err != nil {
		s.Logger.Warn("decision: resolve activation mode failed, defaulting to scheduled",
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
	if err := s.RequestChildRepo.UpdateActivationPlan(ctx, requestChildID, plan.Mode, plan.ActivateOn); err != nil {
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
	if s.PersonRepo == nil || s.StudentRepo == nil || s.GuardianProfileRepo == nil ||
		s.StudentGuardianRepo == nil {
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
		return s.applyApprovalRollover(ctx, request, child, phase, reviewedBy)
	}

	// Existing-student re-enrollment branch (migration 1.15.221): an
	// existing_students phase matched this child to an already-enrolled
	// student at submission and pinned its id. Renew that student instead of
	// creating a duplicate Person + Student — the whole point of the
	// existing_students audience is re-enrollment of children the school
	// already has (#1663). A student deleted between submission and approval
	// nulls this reference via ON DELETE SET NULL, so we only reach here with
	// a live student and otherwise fall through to a fresh create.
	if child.MatchedStudentID != nil {
		s.Logger.Info("decision: existing-student re-enrollment — updating matched student",
			slog.Int64("request_child_id", child.ID),
			slog.Int64("student_id", *child.MatchedStudentID),
		)
		// syncTargetedFields=true: an existing_students submission is a full
		// parent form, so the submitted targeted fields (health info, departure/
		// arrival schedules, contact lists, consent flags) must land on the
		// matched student — unlike the annual rollover, which carries no fresh
		// form (#1663).
		return s.attachApprovalToExistingStudent(ctx, request, child, phase, *child.MatchedStudentID, reviewedBy, true)
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
	if err := s.attachGuardianAccountIfPresent(ctx, request, guardian, profileWasNew); err != nil {
		return nil, err
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
	if err := s.PersonRepo.Create(ctx, person); err != nil {
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
	schoolClass := s.resolveSchoolClass(child)
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
	if err := s.StudentRepo.Create(ctx, student); err != nil {
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
	if err := s.StudentGuardianRepo.Create(ctx, rel); err != nil {
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
	// The plan-synced flag is deliberately dropped here: this student row was
	// created moments ago in this transaction, so no "läuft mit" link can
	// exist yet and a companion broadcast would only wake every open editor
	// once per mass approval for a change that cannot have touched a link.
	if _, err := s.applyTargetedFields(ctx, request, child, student, guardian, reviewedBy, targetedFieldSyncOptions{}); err != nil {
		s.Logger.Warn("decision: targeted-field dispatch had errors",
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
	careOfferingsEnabled, err := s.resolveDecisionBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled, true)
	if err != nil {
		return nil, fmt.Errorf("decision: resolve care offerings setting: %w", err)
	}
	if careOfferingsEnabled {
		if err := s.materializeEnrollments(ctx, child.ID, student.ID, phase); err != nil {
			return nil, err
		}
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
	return s.pendingGuardianInvite(guardian, reviewedBy, profileWasNew), nil
}

// attachGuardianAccountIfPresent runs the cross-tenant account check for a
// resolved guardian profile: if the email (or the submitting parent's
// JWT-derived account id) already has a global auth.accounts row, attach the
// new tenant + this profile to it directly. This bypasses the invitation flow
// entirely — the invitation accept path overwrites the password hash, which is
// the wrong UX when the parent already has a working password from another
// school.
//
// The by-ID lookup wins when the request carries guardian_account_id (parent
// submitted while logged in): a parent who edits their email in the form would
// otherwise miss the attach step and trigger an invitation that overwrites
// their existing password. It is also strictly cheaper — no platform-wide
// email index hit.
//
// Shared by the fresh-create approval and the existing-student re-enrollment
// approval; profileWasNew is logging context only.
func (s *decisionService) attachGuardianAccountIfPresent(
	ctx context.Context,
	request *enrollmentModels.Request,
	guardian *users.GuardianProfile,
	profileWasNew bool,
) error {
	if guardian == nil {
		return nil
	}
	// A profile that already carries an account_id needs no attach — but the
	// link alone does NOT prove the account can reach this school. account_id
	// survives an offboarding that flipped auth.account_tenants to inactive,
	// and pendingGuardianInvite deliberately sends nothing for a linked
	// profile, so no other step would repair the mapping: the child would be
	// approved and stay invisible to the parent. Approving is the
	// administrative act that grants access, so re-assert it here.
	if guardian.AccountID != nil {
		return s.ensureGuardianTenantAccess(ctx, *guardian.AccountID, "ensure linked guardian access")
	}
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
		return fmt.Errorf("decision: attach existing account: %w", err)
	}
	if linked {
		s.Logger.Info("decision: linked approval to existing global account",
			slog.Int64("guardian_profile_id", guardian.ID),
			slog.Int64("tenant_id", tenant.FromContext(ctx)),
			slog.Bool("profile_was_new", profileWasNew),
			slog.Bool("via_request_account_id", request.GuardianAccountID != nil),
		)
	}
	return nil
}

// pendingGuardianInvite reports the post-commit invitation an approval owes the
// submitted primary guardian: none when they already hold a portal account (per
// the design Q answer: "when they already have an account we do not need to
// create a new one") or when there is no address to invite.
func (s *decisionService) pendingGuardianInvite(
	guardian *users.GuardianProfile,
	reviewedBy int64,
	profileWasNew bool,
) *PendingGuardianInvite {
	if guardian == nil || guardian.HasAccount {
		return nil
	}
	if guardian.Email == nil || strings.TrimSpace(*guardian.Email) == "" {
		return nil
	}
	s.Logger.Debug("decision: scheduling guardian invitation",
		slog.Int64("guardian_profile_id", guardian.ID),
		slog.Bool("profile_was_new", profileWasNew),
	)
	return &PendingGuardianInvite{
		GuardianProfileID: guardian.ID,
		CreatedBy:         reviewedBy,
	}
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
	reviewedBy int64,
) (*PendingGuardianInvite, error) {
	source, err := s.RequestChildRepo.FindByID(ctx, *child.RolloverSourceChildID)
	if err != nil || source == nil || source.CreatedStudentID == nil {
		s.Logger.Warn("decision: rollover source has no created_student, falling back to fresh approval",
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
		return s.applyApproval(ctx, request, &clone, phase, reviewedBy)
	}

	s.Logger.Info("decision: rollover approval — updating existing student",
		slog.Int64("request_child_id", child.ID),
		slog.Int64("student_id", *source.CreatedStudentID),
	)
	// syncTargetedFields=false: a rolled-over request_child is carried forward
	// from last year's approval without a fresh parent submission, so there are
	// no newly submitted targeted fields to dispatch. reviewedBy isn't tracked
	// on this path (see applyApprovalRollover's fallback note) — pass 0.
	return s.attachApprovalToExistingStudent(ctx, request, child, phase, *source.CreatedStudentID, 0, false)
}

// attachApprovalToExistingStudent is the shared approval tail for a child that
// resolves to an already-existing student rather than a fresh Person + Student:
// the annual rollover flow (source row's created_student_id) and the
// existing_students re-enrollment audience (matched_student_id) both land here.
// It renews the student's class + enrollment window, materializes the phase's
// care offerings, stamps the activation plan and back-links the request_child.
//
// syncTargetedFields separates the two callers: it is false for the annual
// rollover (no fresh parent submission exists, so there is nothing to apply
// beyond class + window + offerings) and true for an existing_students
// re-enrollment, which IS a full parent form and therefore also reconciles the
// submitted guardian — primary link, phone, portal invitation — exactly like
// the fresh-create path. Assuming the matched student already carries the
// submitted guardian is wrong for an anonymous submission, for an imported or
// manually created child with no guardian link at all, and for the other parent
// submitting this year's renewal; without the reconciliation the approval
// silently discards the submitted guardian and their portal access (#1663).
func (s *decisionService) attachApprovalToExistingStudent(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
	studentID int64,
	reviewedBy int64,
	syncTargetedFields bool,
) (*PendingGuardianInvite, error) {
	existing, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("decision: load existing student %d: %w", studentID, err)
	}
	if existing == nil {
		return nil, fmt.Errorf("decision: existing student %d not found", studentID)
	}
	beforeStatus := existing.Status

	activationPlan := s.approvalActivationPlan(ctx, phase)

	// Update school_class / enrollment window. Already-active children
	// stay active even for a future phase, so current attendance
	// workflows are not interrupted. Inactive/pending children follow the
	// approval-time activation plan. The window itself follows the phase
	// KIND (see renewedEnrollmentWindow): only a school-year renewal may
	// replace the master enrollment window.
	existing.SchoolClass = s.resolveRolloverSchoolClass(child, existing.SchoolClass)
	enrolledFrom, enrolledUntil := renewedEnrollmentWindow(phase, existing.EnrolledFrom, existing.EnrolledUntil)
	existing.EnrolledFrom = &enrolledFrom
	existing.EnrolledUntil = &enrolledUntil
	if existing.Status != users.StudentStatusActive {
		existing.Status = activationPlan.StudentStatus
	}
	// A full re-enrollment form re-states the guardian's contact data, so the
	// student's denormalized guardian_email / guardian_phone follow the fresh
	// submission instead of keeping last year's address (same rule the approved
	// child sync applies). The rollover carries no submission and keeps them.
	if syncTargetedFields {
		if email := strings.TrimSpace(strings.ToLower(request.GuardianEmail)); email != "" {
			existing.GuardianEmail = &email
		}
		existing.GuardianPhone = request.GuardianPhone
	}
	if err := s.StudentRepo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("decision: update existing student: %w", err)
	}
	if beforeStatus != existing.Status && s.StudentAudit != nil {
		if reviewedBy > 0 {
			before := &users.Student{Status: beforeStatus}
			after := &users.Student{Status: existing.Status}
			after.ID = existing.ID
			if err := s.StudentAudit.RecordChangesForActor(ctx, before, after, reviewedBy); err != nil {
				return nil, fmt.Errorf("decision: audit rollover student status: %w", err)
			}
		} else if err := s.StudentAudit.RecordSystemStatusChange(
			ctx,
			existing.ID,
			beforeStatus,
			existing.Status,
		); err != nil {
			return nil, fmt.Errorf("decision: audit rollover student status: %w", err)
		}
	}

	// Reconcile the submitted primary guardian BEFORE the targeted-field
	// dispatch, so the resolved profile is the one the dispatch enriches with
	// the submitted phone number.
	//
	// pruneStalePrimary=false: a guardian already holding the primary link on
	// the matched student comes from a different source than this submission
	// (last year's approval, an import, the other parent), so they keep their
	// link — and with it their pickup authority and parent-portal access — and
	// are only demoted from primary by the DB trigger. See
	// reconcilePrimaryGuardianLink (#1663).
	var guardian *users.GuardianProfile
	if syncTargetedFields {
		resolved, err := s.reconcilePrimaryGuardianLink(ctx, request, studentID, false)
		if err != nil {
			return nil, err
		}
		guardian = resolved
		// Same cross-tenant account check the fresh-create approval runs: a
		// parent who already has a portal account (this school or another) gets
		// the tenant + profile attached directly instead of an invitation that
		// would overwrite their password.
		if err := s.attachGuardianAccountIfPresent(ctx, request, guardian, false); err != nil {
			return nil, err
		}
	}

	// Dispatch every targeted form field the parent submitted onto the existing
	// record (health/extra info, departure + arrival schedules, contact lists,
	// consent-flag propagation). Without this the existing-student approval
	// silently drops everything the form collected beyond class/window/care
	// offerings. Rollover passes false because its request_child carries no
	// fresh submission (#1663).
	//
	// ReplaceSchedules: the matched student most likely already has arrival /
	// pickup schedule rows from its original enrollment. A plain insert of the
	// resubmitted weekdays would collide with the unique (tenant_id, student_id,
	// weekday) key and leave removed weekdays behind; ReplaceSchedules deletes
	// the student's existing rows for each resubmitted schedule target before
	// re-inserting, giving this full re-enrollment form proper replacement
	// semantics for schedules while every other field stays additive (#1663).
	//
	// Unlike the fresh-create path, a dispatch failure here is FATAL, not
	// best-effort: ReplaceSchedules has already deleted the matched student's
	// live arrival / pickup rows before the re-insert runs, so swallowing the
	// error would commit the approval with the existing schedule deleted but
	// never rebuilt (or only partially rebuilt) — silent, destructive schedule
	// loss on a live student. Returning the error rolls the whole approval back
	// through the surrounding tenant tx (see Decide), leaving the original
	// schedule intact for a clean retry (#1663).
	//
	// ReplaceConsent: this form re-asks every configured consent, so a box the
	// guardian left unchecked WITHDRAWS the consent the matched student still
	// carries from an earlier enrollment. Without it an approval would leave
	// photo or email-contact consent active against the guardian's explicit
	// answer (#1663).
	if syncTargetedFields {
		if _, err := s.applyTargetedFields(ctx, request, child, existing, guardian, reviewedBy, targetedFieldSyncOptions{
			ReplaceSchedules: true,
			ReplaceConsent:   true,
		}); err != nil {
			return nil, fmt.Errorf("decision: targeted-field dispatch on existing student: %w", err)
		}
		// Materialize any co-guardians the parent added on this full form. A
		// newly submitted co-guardian needs a users.students_guardians link +
		// phone or the AdditionalGuardians contact data is silently dropped.
		// Idempotent, so re-approval is safe. Fatal like the fresh-create path —
		// losing a pickup-authorized emergency contact is a data-integrity
		// failure, not best-effort field noise (#1663).
		if err := s.linkAdditionalGuardians(ctx, request, studentID); err != nil {
			return nil, fmt.Errorf("decision: link additional guardians on existing student: %w", err)
		}
	}

	// Materialize the phase's care offerings under this student.
	careOfferingsEnabled, err := s.resolveDecisionBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled, true)
	if err != nil {
		return nil, fmt.Errorf("decision: resolve care offerings setting: %w", err)
	}
	if careOfferingsEnabled {
		if err := s.materializeEnrollments(ctx, child.ID, studentID, phase); err != nil {
			return nil, err
		}
	}

	if err := s.stampActivationPlan(ctx, child.ID, activationPlan); err != nil {
		return nil, err
	}

	// Link this request_child to the student so the admin UI can navigate
	// from the submission to the (single) student row.
	if err := s.linkCreatedStudent(ctx, child.ID, studentID); err != nil {
		return nil, fmt.Errorf("decision: link existing student: %w", err)
	}

	// Invite the submitted guardian when they have no portal account yet — the
	// re-enrollment form is exactly the moment a school hands a family portal
	// access, and the matched student's existing links say nothing about
	// whether THIS submitter can reach it. nil on the rollover path (guardian
	// stays nil there) and whenever the account attach above already linked
	// them.
	return s.pendingGuardianInvite(guardian, reviewedBy, false), nil
}

// renewedEnrollmentWindow decides the enrollment window an approval writes
// onto an ALREADY EXISTING student, from the phase kind (#1663):
//
//   - school_year: the phase IS the child's new master enrollment window
//     (annual rollover / re-enrollment), so it replaces the old one wholesale.
//   - holiday / custom: the phase describes a limited-time care period, NOT
//     the child's school membership. Overwriting the master window with it
//     would cut an annually enrolled child's enrollment short — a holiday
//     phase ending in October would satisfy FindActiveDueForDeactivation and
//     the scheduler would mark a perfectly active child inactive. So the
//     existing window is preserved and only WIDENED where the phase's service
//     period reaches beyond it (an inactive child re-enrolled for a holiday
//     block still needs a window that covers that block).
//
// A missing bound (nil) is treated as "not set" and takes the phase's date, so
// a legacy student without a window still ends up with one.
func renewedEnrollmentWindow(
	phase *enrollmentModels.Phase,
	currentFrom, currentUntil *timezone.Date,
) (timezone.Date, timezone.Date) {
	from := phase.ServiceStartDate
	until := phase.ServiceEndDate
	if phase.Kind == enrollmentModels.PhaseKindSchoolYear {
		return from, until
	}
	if currentFrom != nil && currentFrom.Before(from) {
		from = *currentFrom
	}
	if currentUntil != nil && currentUntil.After(until) {
		until = *currentUntil
	}
	return from, until
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

	authAccountID := int64(0)
	if request.GuardianAccountID != nil && *request.GuardianAccountID > 0 {
		authAccountID = *request.GuardianAccountID
	}

	// Authenticated submit: the JWT-derived account is authoritative over the
	// parent-editable email field. Resolve THIS account's own guardian profile
	// at the tenant first, so a parent who edited the email in the form is
	// never routed onto a different account's profile (#1663). FindByAccountID
	// is tenant-scoped (RLS), so a returning parent at this school matches here
	// and their possibly-changed email no longer decides the linkage.
	if authAccountID > 0 {
		own, err := s.GuardianProfileRepo.FindByAccountID(ctx, authAccountID)
		switch {
		case err == nil && own != nil:
			return own, false, nil
		case err != nil && !errors.Is(err, users.ErrGuardianProfileNotFound):
			// A database/driver failure must NOT degrade into the email path:
			// that would silently hand the linkage decision to the
			// parent-editable email field (or create a duplicate profile) on a
			// transient outage. Fail the approval so it can be retried.
			return nil, false, fmt.Errorf("decision: resolve guardian profile by account: %w", err)
		}
		// Only the explicit not-found sentinel flows through: a first-time
		// applicant at this school has no profile yet and is handled by the
		// email/create paths below.
	}

	if email != "" {
		existing, err := s.GuardianProfileRepo.FindByEmail(ctx, email)
		if err == nil && existing != nil {
			// Guard against an authenticated parent claiming an email that
			// already belongs to a DIFFERENT account's guardian profile at this
			// school. Without this, applyApproval skips the by-id attach (the
			// resolved profile already has an account) and links the child to
			// that other account. Fail closed (#1663).
			if authAccountID > 0 && existing.AccountID != nil && *existing.AccountID != authAccountID {
				return nil, false, fmt.Errorf("%w: guardian_profile_id %d", ErrGuardianAccountMismatch, existing.ID)
			}
			// An UNLINKED profile (account_id IS NULL) is worse, not better: the
			// by-id attach in applyApproval would bind that whole profile — and
			// every child already hanging off it — to the caller's JWT account.
			// guardian_email stays parent-editable, so a logged-in parent could
			// otherwise type a stranger's address at a school where they have no
			// profile yet and claim that family's record. Only the caller's OWN
			// address makes the profile claimable; anything else fails closed
			// exactly like the already-linked mismatch above (#1663).
			if authAccountID > 0 && existing.AccountID == nil {
				owns, ownErr := s.submitterOwnsEmail(ctx, authAccountID, email)
				if ownErr != nil {
					return nil, false, ownErr
				}
				if !owns {
					return nil, false, fmt.Errorf("%w: guardian_profile_id %d", ErrGuardianAccountMismatch, existing.ID)
				}
			}
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
	if err := s.GuardianProfileRepo.Create(ctx, profile); err != nil {
		return nil, false, fmt.Errorf("decision: create guardian profile: %w", err)
	}
	return profile, true, nil
}

// submitterOwnsEmail reports whether the submitted guardian email is the
// authenticated submitter's OWN account address. It is the ownership proof
// resolveGuardianProfile requires before letting an authenticated approval
// claim an unlinked guardian profile.
//
// Two configurations answer true without a comparison, both deliberately:
//
//   - AccountRepo unwired: attachExistingAccountByID short-circuits on the
//     same nil check, so no account linkage can happen at all — there is
//     nothing to protect and the legacy accept stands.
//   - account deleted between submission and decision: the by-id attach
//     already falls back to the email-owner lookup, which can only bind the
//     profile to whoever owns THAT address, never to the caller.
func (s *decisionService) submitterOwnsEmail(ctx context.Context, accountID int64, email string) (bool, error) {
	if s.AccountRepo == nil {
		return true, nil
	}
	account, err := s.AccountRepo.FindByID(ctx, accountID)
	if err != nil {
		return false, fmt.Errorf("decision: load submitting account %d: %w", accountID, err)
	}
	if account == nil {
		if s.Logger != nil {
			s.Logger.Warn("decision: submitting account no longer resolvable, skipping email ownership check",
				slog.Int64("guardian_account_id", accountID),
			)
		}
		return true, nil
	}
	return strings.EqualFold(strings.TrimSpace(account.Email), email), nil
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
	if err := s.GuardianProfileRepo.Update(ctx, profile); err != nil {
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
	if s.RequestGuardianRepo == nil {
		return nil
	}
	extras, err := s.RequestGuardianRepo.ListByRequestID(ctx, request.ID)
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
				s.Logger.Warn("decision: persist co-guardian phone failed",
					slog.Int64("guardian_profile_id", profileID),
					slog.String("error", err.Error()))
			}
		}
	}
	return nil
}

// reconcilePrimaryGuardianLink resolves the request's primary guardian profile
// and makes sure the student carries it as the primary students_guardians link,
// creating the link when the student has none (imported / manually created
// children, and children whose original enrollment predates the guardian link).
//
// pruneStalePrimary decides what happens to a DIFFERENT guardian who currently
// holds the primary link:
//
//   - true (admin edit sync): the edit rewrites this request's guardian, so the
//     link it produced earlier is repointed / removed — the old profile was the
//     same submission's answer and is now wrong.
//   - false (existing-student re-enrollment): the matched student may carry a
//     primary guardian from an ENTIRELY different source — last year's approval,
//     an import, the other parent. Deleting or repointing that row would strip a
//     real guardian of their pickup authority and parent-portal access because
//     the other parent happened to submit this year's renewal. The submitted
//     guardian still becomes primary (the DB trigger
//     enforce_single_primary_student_guardian demotes the previous holder to
//     is_primary=false), but their link and permissions survive (#1663).
func (s *decisionService) reconcilePrimaryGuardianLink(
	ctx context.Context,
	request *enrollmentModels.Request,
	studentID int64,
	pruneStalePrimary bool,
) (*users.GuardianProfile, error) {
	guardian, _, err := s.resolveGuardianProfile(ctx, request)
	if err != nil {
		return nil, err
	}
	if s.StudentGuardianRepo == nil {
		return guardian, nil
	}
	links, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
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
		if err := s.StudentGuardianRepo.Update(ctx, currentLink); err != nil {
			return nil, fmt.Errorf("decision: update current primary guardian link: %w", err)
		}
		if pruneStalePrimary && primaryLink != nil && primaryLink.ID != currentLink.ID {
			if err := s.StudentGuardianRepo.Delete(ctx, primaryLink.ID); err != nil {
				return nil, fmt.Errorf("decision: remove stale primary guardian link: %w", err)
			}
		}
		return guardian, nil
	}

	if pruneStalePrimary && primaryLink != nil {
		primaryLink.GuardianProfileID = guardian.ID
		primaryLink.RelationshipType = "guardian"
		primaryLink.IsPrimary = true
		primaryLink.IsEmergencyContact = true
		primaryLink.CanPickup = true
		authorize.ApplyStudentGuardianRole(primaryLink, authorize.GuardianRolePrimaryGuardian)
		if err := primaryLink.Validate(); err != nil {
			return nil, fmt.Errorf("decision: validate primary guardian link: %w", err)
		}
		if err := s.StudentGuardianRepo.Update(ctx, primaryLink); err != nil {
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
	if err := s.StudentGuardianRepo.Create(ctx, rel); err != nil {
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
	if s.RequestGuardianRepo == nil || s.StudentGuardianRepo == nil {
		return currentProfileIDs, nil
	}
	if err := s.linkAdditionalGuardians(ctx, request, studentID); err != nil {
		return currentProfileIDs, fmt.Errorf("decision: relink additional guardians: %w", err)
	}
	current, err := s.RequestGuardianRepo.ListByRequestID(ctx, request.ID)
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
	if len(previous) == 0 || s.StudentGuardianRepo == nil {
		return nil
	}
	links, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link == nil || link.IsPrimary || authorize.IsFullGuardianRole(link.GuardianRole) {
			continue
		}
		if previous[link.GuardianProfileID] && !keep[link.GuardianProfileID] {
			if err := s.StudentGuardianRepo.Delete(ctx, link.ID); err != nil {
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
	if snapshot == nil || child == nil || s.GuardianProfileRepo == nil {
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
		profile, err := s.GuardianProfileRepo.FindByEmail(ctx, email)
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
// is a no-op. The helper is idempotent because approval can relink the same
// guardian more than once while syncing targeted contact fields.
func (s *decisionService) createGuardianPhoneNumber(ctx context.Context, profileID int64, phone string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" || s.GuardianPhoneRepo == nil {
		return nil
	}
	existing, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profileID)
	if err != nil {
		return fmt.Errorf("find existing guardian phone numbers: %w", err)
	}
	for _, current := range existing {
		if current != nil && strings.TrimSpace(current.PhoneNumber) == phone {
			return nil
		}
	}
	row := &users.GuardianPhoneNumber{
		GuardianProfileID: profileID,
		PhoneNumber:       phone,
		PhoneType:         users.PhoneTypeMobile,
		IsPrimary:         true,
	}
	if err := s.GuardianPhoneRepo.Create(ctx, row); err != nil {
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
		if existing, err := s.GuardianProfileRepo.FindByEmail(ctx, email); err == nil && existing != nil {
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
		if err := s.GuardianProfileRepo.Create(ctx, profile); err != nil {
			return 0, fmt.Errorf("create co-guardian profile: %w", err)
		}
		profileID = profile.ID
	}

	// Stamp the resolved profile back so the request's other children
	// reuse it. Non-fatal on failure: the link is created regardless;
	// worst case a later child creates a duplicate email-less profile.
	if err := s.RequestGuardianRepo.StampResolvedProfile(ctx, extra.ID, profileID); err != nil {
		s.Logger.Warn("decision: stamp co-guardian profile failed",
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
		// users.students.school_class is required even when the enrollment
		// tenant deliberately does not collect a grade. Persist a neutral,
		// non-grade placeholder while request_children keeps the grade NULL.
		return openSchoolClassPlaceholder
	}
	return strconv.Itoa(int(*grade))
}

// isBareGradePlaceholderClass reports whether a student's school_class is an
// un-customized grade placeholder — empty or all digits ("", "1", "2"), as
// produced by gradeToClass — rather than a concrete class an admin assigned
// ("2a"). Bare placeholders may be safely re-derived from a changed grade;
// concrete classes must be preserved. Issue #1833.
func isBareGradePlaceholderClass(class string) bool {
	class = strings.TrimSpace(class)
	if strings.EqualFold(class, openSchoolClassPlaceholder) {
		return true
	}
	for _, r := range class {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// concreteSchoolClass returns the trimmed concrete class the parent
// chose at enrollment (e.g. "2a"), or "" when none was collected
// ("Klasse offen"). Issue #1833.
func (s *decisionService) concreteSchoolClass(child *enrollmentModels.RequestChild) string {
	if child.TargetSchoolClass == nil {
		return ""
	}
	return strings.TrimSpace(*child.TargetSchoolClass)
}

// resolveSchoolClass is what a freshly-created student's school_class
// should be: the concrete class when the parent picked one, otherwise
// the bare grade number as a placeholder the admin renames later. Used
// only for brand-new student rows — never to overwrite an existing
// student, where clobbering a concrete "2a" with a bare grade number
// would lose information (see the rollover/adjustment paths). Issue #1833.
func (s *decisionService) resolveSchoolClass(child *enrollmentModels.RequestChild) string {
	if concrete := s.concreteSchoolClass(child); concrete != "" {
		return concrete
	}
	return s.gradeToClass(child.TargetGradeLevel)
}

// resolveRolloverSchoolClass decides what an EXISTING student's
// school_class becomes when a rollover approval updates the row in place.
// Pure (no DB) so it is unit-testable. Issue #1833.
//
// Rules, in order:
//   - rollover carries a concrete class (e.g. "3a") -> use it.
//   - no concrete class and no target grade -> keep the existing class. The
//     tenant deliberately did not collect a grade, so there is no replacement
//     class information to apply.
//   - no concrete class, but the current class is still an un-customized
//     bare grade placeholder (empty or all digits, e.g. "1") -> re-derive
//     the new grade number so grade bumps still track ("1" -> "2"). Also
//     covers legacy/admin-reviewed rows whose source grade is nil.
//   - no concrete class and the current class is a customised concrete
//     class whose grade still matches the rollover's target grade ("3b"
//     rolling into grade 3, or any half-year rollover that keeps the
//     grade) -> keep it. The letter is still valid for the new grade.
//   - no concrete class and the current class is a concrete class from a
//     DIFFERENT grade than the rollover target ("2a" while the grade
//     bumps to 3) -> the letter is stale for the new grade, so fall back
//     to the bare grade placeholder ("3"). Keeping "2a" would strand a
//     grade-3 student in a grade-2 class on every class-based view
//     (rosters, filters) until someone noticed and fixed it by hand; the
//     admin instead reassigns the concrete class in the new grade
//     ("manuell zuordnen"). Classes with no numeric prefix ("Bienen")
//     carry no derivable grade and are left untouched.
//
// Mirrors the bare-placeholder check in SyncApprovedChildData (#1833).
func (s *decisionService) resolveRolloverSchoolClass(child *enrollmentModels.RequestChild, existingClass string) string {
	if concrete := s.concreteSchoolClass(child); concrete != "" {
		return concrete
	}
	if child.TargetGradeLevel == nil || *child.TargetGradeLevel == 0 {
		return existingClass
	}
	if isBareGradePlaceholderClass(existingClass) {
		return s.gradeToClass(child.TargetGradeLevel)
	}
	if newGradeClass := s.gradeToClass(child.TargetGradeLevel); newGradeClass != "" {
		if prefix := schoolclass.GradePrefix(existingClass); prefix != "" && prefix != newGradeClass {
			return newGradeClass
		}
	}
	return existingClass
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
	return s.materializeEnrollmentsFrom(ctx, requestChildID, studentID, phase, nil)
}

// materializeEnrollmentsFrom is materializeEnrollments with an optional start
// override. startFrom is set when a dated adjustment replaces only the part of
// the phase window from that date onward; the rows before it were capped, not
// deleted, so the new rows must not reach back over them.
func (s *decisionService) materializeEnrollmentsFrom(
	ctx context.Context,
	requestChildID, studentID int64,
	phase *enrollmentModels.Phase,
	startFrom *timezone.Date,
) error {
	if !s.hasEnrollmentMaterializationDependencies() {
		// Wired without the offering repos: skip silently. Approvals
		// will still create the student record; the admin can attach
		// activity groups later via the activity admin UI.
		s.Logger.Warn("decision: enrollment repos missing; skipping activity materialization",
			slog.Int64("request_child_id", requestChildID),
			slog.Int64("student_id", studentID))
		return nil
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	drafts, err := s.careEnrollmentDraftsForChild(ctx, requestChildID, phase)
	if err != nil {
		return err
	}
	return s.persistCareEnrollmentDrafts(ctx, requestChildID, studentID, phase, drafts, startFrom)
}

func (s *decisionService) hasEnrollmentMaterializationDependencies() bool {
	return s.RequestChildOfferingRepo != nil &&
		s.CareOfferingRepo != nil &&
		s.StudentEnrollmentRepo != nil &&
		s.ActivityGroupRepo != nil &&
		s.ActivityScheduleRepo != nil &&
		s.CalendarPeriodRepo != nil &&
		s.TimeframeRepo != nil &&
		s.ActivityExceptionRepo != nil
}

func (s *decisionService) careEnrollmentDraftsForChild(
	ctx context.Context,
	requestChildID int64,
	phase *enrollmentModels.Phase,
) (map[int64]*careEnrollmentDraft, error) {
	links, err := s.RequestChildOfferingRepo.ListByRequestChildID(ctx, requestChildID)
	if err != nil {
		return nil, fmt.Errorf("decision: list child offerings: %w", err)
	}
	return s.careEnrollmentDraftsForLinks(ctx, requestChildID, links, phase)
}

// careEnrollmentDraftsForLinks materializes an explicitly supplied selection.
// Dated offering changes use it after scheduling their future links: reading
// the current links at that point would correctly return the old selection,
// but incorrectly materialize that old selection at the switch date.
func (s *decisionService) careEnrollmentDraftsForLinks(
	ctx context.Context,
	requestChildID int64,
	links []*enrollmentModels.RequestChildOffering,
	phase *enrollmentModels.Phase,
) (map[int64]*careEnrollmentDraft, error) {
	if len(links) == 0 {
		return map[int64]*careEnrollmentDraft{}, nil
	}
	offeringIDs := uniqueCareOfferingIDs(links)
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("decision: list linked care offerings: %w", err)
	}
	drafts, err := s.buildCareEnrollmentDrafts(ctx, requestChildID, links, offerings, phase)
	if err != nil {
		return nil, err
	}
	return drafts, nil
}

func (s *decisionService) persistCareEnrollmentDrafts(
	ctx context.Context,
	requestChildID, studentID int64,
	phase *enrollmentModels.Phase,
	drafts map[int64]*careEnrollmentDraft,
	startFrom *timezone.Date,
) error {
	groupIDs := make([]int64, 0, len(drafts))
	for groupID := range drafts {
		groupIDs = append(groupIDs, groupID)
	}
	slices.Sort(groupIDs)
	for _, groupID := range groupIDs {
		row := studentEnrollmentFromCareDraft(requestChildID, studentID, phase, drafts[groupID], startFrom)
		if err := row.Validate(); err != nil {
			return fmt.Errorf("decision: validate enrollment: %w", err)
		}
		if err := s.StudentEnrollmentRepo.Create(ctx, row); err != nil {
			return fmt.Errorf("decision: create enrollment: %w", err)
		}
	}
	return nil
}

func studentEnrollmentFromCareDraft(
	requestChildID, studentID int64,
	phase *enrollmentModels.Phase,
	draft *careEnrollmentDraft,
	startFrom *timezone.Date,
) *activities.StudentEnrollment {
	validUntil := phase.ServiceEndDate.AddDays(1)
	validFrom := phase.ServiceStartDate
	// A dated switch may start mid-phase; a phase that already began must not
	// pull the new row back to its service start. Clamped so an effective date
	// before the phase window cannot widen it either.
	if startFrom != nil && startFrom.After(validFrom) {
		validFrom = *startFrom
	}
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
	return row
}

func (s *decisionService) lockTemplateRecurrence(ctx context.Context) error {
	if s.LockTemplateRecurrence == nil {
		return nil
	}
	if err := s.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("decision: lock template recurrence: %w", err)
	}
	return nil
}

type careEnrollmentDraft struct {
	activityGroupID  int64
	calendarPeriodID *int64
	selectedWeekday  map[int]bool
	allWeekdays      bool
}

func uniqueCareOfferingIDs(links []*enrollmentModels.RequestChildOffering) []int64 {
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, link := range links {
		if link.CareOfferingID <= 0 || seen[link.CareOfferingID] {
			continue
		}
		seen[link.CareOfferingID] = true
		ids = append(ids, link.CareOfferingID)
	}
	return ids
}

func (s *decisionService) buildCareEnrollmentDrafts(
	ctx context.Context,
	requestChildID int64,
	links []*enrollmentModels.RequestChildOffering,
	offerings []*enrollmentModels.CareOffering,
	phase *enrollmentModels.Phase,
) (map[int64]*careEnrollmentDraft, error) {
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		offeringByID[offering.ID] = offering
	}
	drafts := make(map[int64]*careEnrollmentDraft)
	for _, link := range links {
		offering := offeringByID[link.CareOfferingID]
		if offering == nil {
			s.Logger.Warn("decision: care offering missing for child link",
				slog.Int64("request_child_id", requestChildID),
				slog.Int64("care_offering_id", link.CareOfferingID))
			continue
		}
		if err := s.addCareOfferingDrafts(ctx, drafts, offering, link, phase); err != nil {
			return nil, err
		}
	}
	return drafts, nil
}

func (s *decisionService) addCareOfferingDrafts(
	ctx context.Context,
	drafts map[int64]*careEnrollmentDraft,
	offering *enrollmentModels.CareOffering,
	link *enrollmentModels.RequestChildOffering,
	phase *enrollmentModels.Phase,
) error {
	if offering.ActivityGroupID == nil || *offering.ActivityGroupID == 0 {
		return nil
	}
	segments, err := resolveCareOfferingLinkedGroupsForPhase(ctx, careOfferingTemplateDeps{
		activityGroupRepo:    s.ActivityGroupRepo,
		activityScheduleRepo: s.ActivityScheduleRepo,
		calendarPeriodRepo:   s.CalendarPeriodRepo,
	}, *offering.ActivityGroupID, phase)
	if err != nil {
		return fmt.Errorf("decision: validate linked activity group for care offering %d: %w", link.CareOfferingID, err)
	}
	isTemplate := len(segments) > 0 && segments[0].group.IsTemplate
	if isTemplate && len(offering.AvailableDays) == 0 && len(link.SelectedDays) == 0 {
		return fmt.Errorf("decision: care offering %d links to a timetable template but has no selected or available days", link.CareOfferingID)
	}
	days, err := effectiveOfferingDaysForEnrollment(offering, link)
	if err != nil {
		return fmt.Errorf("decision: resolve selected days for care offering %d: %w", link.CareOfferingID, err)
	}
	if err := validateCareOfferingTemplateSegments(segments, phase, days, true); err != nil {
		return fmt.Errorf("decision: care offering %d is not materializable: %w", link.CareOfferingID, err)
	}
	if err := validateCareOfferingMaterializability(
		ctx,
		careOfferingMaterializationDeps{
			timeframeRepo:         s.TimeframeRepo,
			activityExceptionRepo: s.ActivityExceptionRepo,
		},
		segments,
		phase,
		days,
		careOfferingMaterializationChange{},
	); err != nil {
		return fmt.Errorf("decision: care offering %d is not materializable: %w", link.CareOfferingID, err)
	}
	for _, segment := range segments {
		if err := mergeCareEnrollmentDraft(drafts, segment, days, link.CareOfferingID); err != nil {
			return err
		}
	}
	return nil
}

func mergeCareEnrollmentDraft(
	drafts map[int64]*careEnrollmentDraft,
	segment linkedCareOfferingGroup,
	days []string,
	offeringID int64,
) error {
	var periodID *int64
	if segment.period != nil {
		periodID = &segment.period.ID
	}
	draft := drafts[segment.group.ID]
	if draft != nil && !sameOptionalInt64(draft.calendarPeriodID, periodID) {
		return fmt.Errorf("decision: care offering %d resolves to conflicting calendar_period_id", offeringID)
	}
	if draft == nil {
		draft = &careEnrollmentDraft{
			activityGroupID:  segment.group.ID,
			calendarPeriodID: periodID,
			selectedWeekday:  make(map[int]bool),
		}
		drafts[segment.group.ID] = draft
	}
	if !segment.group.IsTemplate || len(days) == 0 {
		draft.allWeekdays = true
		return nil
	}
	if draft.allWeekdays {
		return nil
	}
	for _, day := range days {
		weekday, ok := enrollmentDayToISOWeekday(day)
		if !ok {
			return fmt.Errorf("decision: invalid selected day %q for care offering %d", day, offeringID)
		}
		draft.selectedWeekday[weekday] = true
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
	return enrollmentModels.CanonicalDayToISOWeekday(day)
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
	return s.RequestChildRepo.LinkCreatedStudent(ctx, requestChildID, studentID)
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
	if s.AccountRepo == nil || s.AccountTenantRepo == nil ||
		s.AccountRoleRepo == nil || s.RoleRepo == nil {
		// Auth repos not wired - fall back to the original invitation
		// flow. Test factories that don't bring up the auth side will
		// hit this path.
		return false, nil
	}
	if guardian.Email == nil || strings.TrimSpace(*guardian.Email) == "" {
		return false, nil
	}

	email := strings.TrimSpace(strings.ToLower(*guardian.Email))
	account, err := s.AccountRepo.FindByEmail(ctx, email)
	if err != nil {
		// Not-found is the common case (parent has no portal account
		// yet) - treat it as "nothing to attach", let the invitation
		// flow run. We don't import the auth package's notfound
		// detection here; instead we rely on the FindByEmail wrapper
		// returning a typed DatabaseError on real failures. Logging
		// at debug level covers both branches.
		s.Logger.Debug("decision: account lookup result",
			slog.String("email", email),
			slog.String("error", err.Error()),
		)
		return false, nil
	}
	if account == nil {
		return false, nil
	}

	return s.attachAccountToGuardian(ctx, guardian, account, "attach")
}

// attachAccountToGuardian runs the shared attach tail for both account
// resolution paths: account_tenants mapping (idempotent create), guardian
// role for this tenant, and LinkAccount on the per-tenant profile.
// errPrefix keeps the historical per-path error wording ("attach" /
// "attach by id").
func (s *decisionService) attachAccountToGuardian(
	ctx context.Context,
	guardian *users.GuardianProfile,
	account *authModels.Account,
	errPrefix string,
) (bool, error) {
	// 1 + 2. Active account_tenants mapping and guardian role for this tenant.
	if err := s.ensureGuardianTenantAccess(ctx, account.ID, errPrefix); err != nil {
		return false, err
	}

	// 3. Link the per-tenant guardian profile row to the global
	// account. LinkAccount also flips has_account=true so future
	// approvals for the same profile see the linked state.
	if err := s.GuardianProfileRepo.LinkAccount(ctx, guardian.ID, account.ID); err != nil {
		return false, fmt.Errorf("%s: link profile: %w", errPrefix, err)
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
	if s.AccountRepo == nil || s.AccountTenantRepo == nil ||
		s.AccountRoleRepo == nil || s.RoleRepo == nil {
		return false, nil
	}
	account, err := s.AccountRepo.FindByID(ctx, accountID)
	if err != nil || account == nil {
		// Account was deleted between submission and decision - fall
		// back to email lookup so the approval still goes through.
		s.Logger.Warn("decision: request guardian_account_id no longer resolvable, falling back to email",
			slog.Int64("guardian_account_id", accountID),
		)
		if guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "" {
			return s.attachExistingAccountIfPresent(ctx, guardian)
		}
		return false, nil
	}

	return s.attachAccountToGuardian(ctx, guardian, account, "attach by id")
}

// ensureGuardianTenantAccess makes an account's guardian membership in the
// CURRENT tenant usable: the auth.account_tenants mapping is created OR
// REACTIVATED, and the guardian base role is assigned for this tenant.
//
// EnsureActive (not Create) is deliberate: Create is an ON CONFLICT DO NOTHING
// insert, so an existing row left inactive by a previous offboarding would
// survive an approval untouched — mapping present, status 'inactive', parent
// locked out of the school they were just approved for.
//
// errPrefix keeps the caller's historical error wording ("attach" /
// "attach by id" / the already-linked path).
func (s *decisionService) ensureGuardianTenantAccess(ctx context.Context, accountID int64, errPrefix string) error {
	if s.AccountTenantRepo == nil || s.AccountRoleRepo == nil || s.RoleRepo == nil {
		// Auth repos not wired — the invitation flow stays responsible, same
		// short-circuit the attach paths use.
		return nil
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return fmt.Errorf("%s: tenant not in context", errPrefix)
	}

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.AccountTenantRepo.EnsureActive(ctx, mapping); err != nil {
		return fmt.Errorf("%s: account_tenants: %w", errPrefix, err)
	}

	// Guardian role for this tenant. AccountRoleRepo.Create has no ON CONFLICT,
	// so ensureGuardianRoleForTenant checks first and only creates when missing.
	return s.ensureGuardianRoleForTenant(ctx, accountID)
}

// ensureGuardianRoleForTenant assigns the guardian base role for the
// current tenant, idempotently. Mirrors the linkProfileToAccount step
// in services/auth.guardianInvitationService so a parent linked here
// gets the same role footprint as one who came in via the invite
// accept flow.
func (s *decisionService) ensureGuardianRoleForTenant(ctx context.Context, accountID int64) error {
	role, err := s.RoleRepo.FindByName(ctx, guardianRoleName)
	if err != nil {
		return fmt.Errorf("attach: guardian role lookup: %w", err)
	}
	if role == nil {
		return fmt.Errorf("attach: guardian role not found")
	}

	existing, err := s.AccountRoleRepo.FindByAccountAndRole(ctx, accountID, role.ID)
	if err == nil && existing != nil {
		// Already assigned for this tenant (FindByAccountAndRole
		// honours tenant scope) - nothing to do.
		return nil
	}

	assignment := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    role.ID,
	}
	if err := s.AccountRoleRepo.Create(ctx, assignment); err != nil {
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
// + per-child records have already been written by the caller. The one
// exception to the opaque combined string are the companion sentinels a
// departure-plan student write can raise (users.ErrCompanionWouldLoseDeparture,
// users.ErrCompanionLockBusy): they stay reachable via errors.Is so the
// enrollment handlers can answer with the actionable 4xx the student PUT
// gives instead of a blind 500.
//
// The returned bool reports whether the student write actually TRIMMED a
// "läuft mit" link — read from the write itself via
// users.CompanionChangeRecorder, not from the fact that the payload carried a
// departure plan. It is the caller's signal for the student_companions_changed
// broadcast, and a false positive there costs somebody an in-progress
// companion edit.
type targetedFieldSyncOptions struct {
	Replace bool
	// ReplaceSchedules deletes the student's existing arrival / pickup schedule
	// rows before re-inserting the resubmitted weekdays, WITHOUT the full-form
	// clearing semantics of Replace. Used by the existing_students re-enrollment
	// approval, where the matched student already has schedule rows that would
	// otherwise collide with the unique (tenant_id, student_id, weekday) key.
	// Implied by Replace.
	ReplaceSchedules bool
	// ReplaceConsent applies the submitted consent flags as the complete
	// consent state instead of an additive OR: a flag the parent left
	// unchecked CLEARS the matching timestamp on the student row. Used by the
	// existing_students re-enrollment approval, where the submission is a full
	// renewal of a student that already carries consent from a previous year —
	// without it a guardian who withdraws photo or email-contact consent on the
	// renewal form stays recorded as consenting (#1663). Implied by Replace.
	ReplaceConsent         bool
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
) (bool, error) {
	if s.FormSchemaRepo == nil || request.SchemaID == nil {
		return false, nil
	}
	schema, err := s.FormSchemaRepo.FindByID(ctx, *request.SchemaID)
	if err != nil || schema == nil {
		return false, nil
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

	// Replace implies schedule and consent replacement; ReplaceSchedules /
	// ReplaceConsent ask for one of them alone (existing_students
	// re-enrollment) without the full-form clearing the rest of Replace applies.
	replaceSchedules := options.Replace || options.ReplaceSchedules
	replaceConsent := options.Replace || options.ReplaceConsent

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
			if replaceSchedules && s.PickupScheduleRepo != nil && !pickupScheduleDeleted {
				pickupScheduleDeleted = true
				if err := s.PickupScheduleRepo.DeleteByStudentID(ctx, student.ID); err != nil {
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
			if replaceSchedules && s.ArrivalScheduleRepo != nil && !arrivalScheduleDeleted {
				arrivalScheduleDeleted = true
				if err := s.ArrivalScheduleRepo.DeleteByStudentID(ctx, student.ID); err != nil {
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
		// Withdrawal. A replacing submission answers the whole consent
		// question, so a block the guardian left unchecked must CLEAR the
		// matching timestamp instead of leaving last year's consent standing.
		//
		// Clearing keys on PRESENCE, not on falsiness: the form submits every
		// configured legal block (an unchecked optional box arrives as an
		// explicit false) and filterConsentFlags drops everything else, so an
		// absent key means "this form never asked" — not "withdrawn". Wiping
		// those would let a renewal phase that configures no photo block erase
		// a photo consent the original enrollment recorded (#1663).
		if replaceConsent {
			if photo, ok := request.ConsentFlags[enrollmentModels.ConsentKeyPhoto].(bool); ok && !photo {
				student.PhotoConsentGivenAt = nil
				student.PhotoConsentGivenBy = nil
				studentDirty = true
			}
			if agb, ok := request.ConsentFlags[enrollmentModels.ConsentKeyAGB].(bool); ok && !agb {
				student.AGBAcceptedAt = nil
				studentDirty = true
			}
			if dp, ok := request.ConsentFlags[enrollmentModels.ConsentKeyDataProcessing].(bool); ok && !dp {
				student.DataProcessingAcceptedAt = nil
				studentDirty = true
			}
			if email, ok := request.ConsentFlags[enrollmentModels.ConsentKeyEmailContact].(bool); ok && !email {
				student.EmailContactAcceptedAt = nil
				studentDirty = true
			}
		}
	}
	// guardian is nil on the annual rollover path: that request_child carries no
	// fresh parent submission, so there is no newly submitted phone number and
	// no profile to enrich — skip rather than nil-panic. Every path that DOES
	// carry a submission (fresh create, existing-student re-enrollment, admin
	// edit sync) passes the resolved primary guardian.
	if request.GuardianPhone != nil && guardian != nil {
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
			note = strutil.TruncateRunes(note, users.MaxDepartureCompanionNoteLen, "")
			student.DepartureCompanionNote = &note
			studentDirty = true
		}
	}

	departurePlanSynced := false
	// The companion refusals are kept as a WRAPPED error, not flattened into
	// the string list: StudentRepository.Update reconciles the "läuft mit"
	// edges for every caller, and these two sentinels are expected,
	// user-actionable refusals (fix the other child's Heimweg first / retry
	// after the concurrent edit). Reducing them to text — as every other
	// best-effort field error is — would turn a legitimate enrollment change
	// into an opaque 500 at the handler (#1694).
	var companionRefusal error
	if studentDirty {
		// Carrying a departure plan is a NECESSARY, not a sufficient, condition
		// for a companion change: writing the same modes back trims no edge, and
		// announcing student_companions_changed for such a write makes every open
		// companion editor in the school discard or block its draft for nothing.
		// Only the write path knows the difference, so read it from there
		// (users.CompanionChangeRecorder) instead of inferring it from the payload.
		updateCtx, companionChanges := users.ContextWithCompanionChangeRecorder(ctx)
		if err := s.StudentRepo.Update(updateCtx, student); err != nil {
			if errors.Is(err, users.ErrCompanionWouldLoseDeparture) || errors.Is(err, users.ErrCompanionLockBusy) {
				companionRefusal = fmt.Errorf("update student: %w", err)
			} else {
				errs = append(errs, fmt.Sprintf("update student: %v", err))
			}
		} else {
			departurePlanSynced = companionChanges.Changed()
		}
	}

	if companionRefusal != nil {
		if len(errs) > 0 {
			return departurePlanSynced, fmt.Errorf("%s; %w", strings.Join(errs, "; "), companionRefusal)
		}
		return departurePlanSynced, companionRefusal
	}
	if len(errs) > 0 {
		return departurePlanSynced, errors.New(strings.Join(errs, "; "))
	}
	return departurePlanSynced, nil
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
	return decodeWeekdayBooleanDays[users.BusDays](raw, users.BusDayOrder)
}

// decodeWeekdayBooleanDays decodes a weekday_boolean map and projects it
// onto the given day order. Shared core of the bus and pickup decoders;
// their divergent legacy branches (bool flag vs pickup answer string)
// stay with each decoder.
func decodeWeekdayBooleanDays[M ~map[string]bool](raw any, order []string) (M, error) {
	var days enrollmentModels.WeekdayBoolean
	if err := decodeStructured(raw, &days); err != nil {
		return nil, fmt.Errorf("decode weekday_boolean: %w", err)
	}
	if err := days.Validate(); err != nil {
		return nil, err
	}
	out := M{}
	for _, day := range order {
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
	return decodeWeekdayBooleanDays[users.PickupDays](raw, users.PickupDayOrder)
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
	if s.PersonRepo == nil || s.StaffRepo == nil {
		return 0, fmt.Errorf("reviewer staff lookup is unavailable")
	}
	person, err := s.PersonRepo.FindByAccountID(ctx, reviewerAccountID)
	if err != nil {
		return 0, fmt.Errorf("find reviewer person: %w", err)
	}
	if person == nil {
		return 0, fmt.Errorf("reviewer account %d has no linked person", reviewerAccountID)
	}
	staff, err := s.StaffRepo.FindByPersonID(ctx, person.ID)
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
	if (isPickup && s.PickupScheduleRepo == nil) || (!isPickup && s.ArrivalScheduleRepo == nil) {
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
			if err := s.PickupScheduleRepo.UpsertSchedule(ctx, row); err != nil {
				return fmt.Errorf("upsert pickup %s: %w", day, err)
			}
		} else {
			row := &scheduleModels.StudentArrivalSchedule{
				StudentID:       studentID,
				Weekday:         weekdayInt[day],
				ExpectedArrival: t,
				CreatedBy:       createdBy,
			}
			if err := s.ArrivalScheduleRepo.Create(ctx, row); err != nil {
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
		s.StudentGuardianRepo == nil ||
		s.GuardianProfileRepo == nil ||
		s.GuardianPhoneRepo == nil {
		return nil, nil
	}
	firstName := contactIdentityName(entry.FirstName)
	lastName := contactIdentityName(entry.LastName)
	phones := contactIdentityPhones(entry)
	if firstName == "" || lastName == "" || len(phones) == 0 {
		return nil, nil
	}

	links, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
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

	profiles, err := s.GuardianProfileRepo.FindByIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	phonesByProfile, err := s.GuardianPhoneRepo.FindByGuardianIDs(ctx, profileIDs)
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

	existing, err := s.StudentGuardianRepo.FindByStudentAndGuardianForUpdate(ctx, rel.StudentID, rel.GuardianProfileID)
	if err != nil {
		if !errors.Is(err, users.ErrStudentGuardianNotFound) {
			return err
		}
		inserted, linkErr := s.StudentGuardianRepo.LinkIfNotExists(ctx, rel)
		if linkErr != nil || inserted {
			return linkErr
		}
		existing, err = s.StudentGuardianRepo.FindByStudentAndGuardianForUpdate(ctx, rel.StudentID, rel.GuardianProfileID)
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
	updated, err := s.StudentGuardianRepo.UpdateColumns(
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
	if s.GuardianProfileRepo == nil || s.StudentGuardianRepo == nil {
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
			existing, err := s.GuardianProfileRepo.FindByEmail(ctx, emailLC)
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
			if err := s.GuardianProfileRepo.Create(ctx, profile); err != nil {
				return linkedProfileIDs, fmt.Errorf("create contact profile %s %s: %w", c.FirstName, c.LastName, err)
			}
		}
		linkedProfileIDs[profile.ID] = true

		// Phone numbers — append, dedup by unique index.
		if s.GuardianPhoneRepo != nil {
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
				if err := s.GuardianPhoneRepo.Create(ctx, phone); err != nil {
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
