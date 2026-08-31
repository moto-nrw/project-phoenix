package active

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ErrManagerControlledAbsence marks absence mutations that staff may view but
// only a time-tracking manager may create, change, or delete.
var ErrManagerControlledAbsence = errors.New("absence type is manager-controlled")

// ErrVacationQuotaInvalid marks invalid quota input supplied by a caller.
var ErrVacationQuotaInvalid = errors.New("invalid vacation quota")

// Comp_time absences may overdraw the Stundenkonto since #2873: the former
// ErrCompTimeExceedsBalance overdraft rejection was replaced by the
// PreviewCompTimeBalance projection, which the frontend shows before the
// Leitung deliberately confirms a booking into the negative. The overdraft
// guard on balance ADJUSTMENTS (payout etc.) is unchanged (#1420).

// CreateAbsenceRequest defines the request for creating an absence
type CreateAbsenceRequest struct {
	AbsenceType string `json:"absence_type"`
	// AbsenceTypeID picks a school-defined Abwesenheitsart (#2403). It names
	// the absence; the arithmetic still comes from AbsenceType, which the
	// service overwrites with the art's base type so a client cannot pair a
	// custom name with a calculation it was not created for.
	AbsenceTypeID *int64 `json:"absence_type_id"`
	DateStart     string `json:"date_start"`
	DateEnd       string `json:"date_end"`
	HalfDay       bool   `json:"half_day"`
	Note          string `json:"note"`
}

// UnmarshalJSON accepts decimal strings for int64 identifiers. JavaScript
// cannot represent every database BIGINT as a number, so browser clients send
// custom absence IDs as strings.
func (r *CreateAbsenceRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		AbsenceType   string          `json:"absence_type"`
		AbsenceTypeID json.RawMessage `json:"absence_type_id"`
		DateStart     string          `json:"date_start"`
		DateEnd       string          `json:"date_end"`
		HalfDay       bool            `json:"half_day"`
		Note          string          `json:"note"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.AbsenceType, r.DateStart, r.DateEnd, r.HalfDay, r.Note = raw.AbsenceType, raw.DateStart, raw.DateEnd, raw.HalfDay, raw.Note
	if len(raw.AbsenceTypeID) != 0 {
		id, err := parseAbsenceTypeID(raw.AbsenceTypeID)
		if err != nil {
			return err
		}
		r.AbsenceTypeID = id
	}
	return nil
}

// UpdateAbsenceRequest defines the request for updating an absence
type UpdateAbsenceRequest struct {
	AbsenceType *string `json:"absence_type"`
	// AbsenceTypeID re-points the named art. A present pointer to 0 clears it
	// back to the plain standard type; omitted (nil) leaves it untouched.
	AbsenceTypeID *int64 `json:"absence_type_id"`
	// AbsenceTypeIDSet distinguishes an omitted field from JSON null, which
	// explicitly clears a previously selected school-defined art.
	AbsenceTypeIDSet bool    `json:"-"`
	DateStart        *string `json:"date_start"`
	DateEnd          *string `json:"date_end"`
	HalfDay          *bool   `json:"half_day"`
	Note             *string `json:"note"`
}

func (r *UpdateAbsenceRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		AbsenceType   *string         `json:"absence_type"`
		AbsenceTypeID json.RawMessage `json:"absence_type_id"`
		DateStart     *string         `json:"date_start"`
		DateEnd       *string         `json:"date_end"`
		HalfDay       *bool           `json:"half_day"`
		Note          *string         `json:"note"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.AbsenceType, r.DateStart, r.DateEnd, r.HalfDay, r.Note = raw.AbsenceType, raw.DateStart, raw.DateEnd, raw.HalfDay, raw.Note
	if len(raw.AbsenceTypeID) == 0 {
		return nil
	}
	r.AbsenceTypeIDSet = true
	id, err := parseAbsenceTypeID(raw.AbsenceTypeID)
	if err != nil {
		return err
	}
	r.AbsenceTypeID = id
	return nil
}

func parseAbsenceTypeID(raw json.RawMessage) (*int64, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		value = string(raw)
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ungültige Abwesenheitsart-ID: %w", err)
	}
	return &id, nil
}

// maxSickAbsenceRangeDays bounds the synchronous plan cascade. Sick reports
// legitimately span longer than the week-sized plan views, but allowing an
// arbitrary multi-year range would hold the tenant transaction while the
// cascade expands and locks every civil day.
const maxSickAbsenceRangeDays = 366

// StaffAbsenceResponse wraps an absence with calculated fields
type StaffAbsenceResponse struct {
	*activeModels.StaffAbsence
	DurationDays int `json:"duration_days"`
	// AbsenceTypeID shadows the embedded model field on the wire, so browser
	// clients receive this BIGINT as a lossless decimal string.
	AbsenceTypeID *string `json:"absence_type_id,omitempty"`
}

// StaffAbsenceRequestItem is one absence request in the Anfragen module's
// display format: the request itself plus the person it belongs to and, in
// the history, who decided it.
type StaffAbsenceRequestItem struct {
	*StaffAbsenceResponse
	StaffName     string `json:"staff_name"`
	DecidedByName string `json:"decided_by_name,omitempty"`
}

// AbsenceRequestListQuery is what the Anfragen module asks for: the open work
// list or the decided history, narrowed by absence type and staff name.
type AbsenceRequestListQuery struct {
	// History selects decided requests instead of the open work list.
	History bool
	Types   []string
	Search  string
}

// StaffAbsenceListFilter selects a staff member's absences by overlapping date
// range, status, or both. From and To must either both be set or both be nil.
type StaffAbsenceListFilter struct {
	From   *timezone.Date
	To     *timezone.Date
	Status string
}

// RequestVacationRequest is what the MA submits via "Urlaub beantragen".
// StartHalfDay/EndHalfDay are Personio-style boundary half-days: 0.5 day
// off the first / last day of the range, full days for everything between.
type RequestVacationRequest struct {
	DateStart         string `json:"date_start"`
	DateEnd           string `json:"date_end"`
	StartHalfDay      bool   `json:"start_half_day"`
	EndHalfDay        bool   `json:"end_half_day"`
	Note              string `json:"note"`
	SubstituteStaffID *int64 `json:"substitute_staff_id,omitempty"`
}

// VacationDecisionRequest is the admin's approve/deny payload
type VacationDecisionRequest struct {
	DecisionNote string `json:"decision_note"`
}

// VacationQuotaSummary aggregates entitled/taken/reserved for a staff/year
type VacationQuotaSummary struct {
	StaffID       int64   `json:"staff_id"`
	Year          int     `json:"year"`
	EntitledDays  float64 `json:"entitled_days"`
	CarryoverDays float64 `json:"carryover_days"`
	// TakenBeforeDays is the vacation takeover from the moto introduction
	// (#2132): days already taken before the Stichtag in the old system.
	// 0 without an opening row (or before its Stichtag).
	TakenBeforeDays float64 `json:"taken_before_days"`
	TakenDays       float64 `json:"taken_days"`
	ReservedDays    float64 `json:"reserved_days"`
	RemainingDays   float64 `json:"remaining_days"`
	// Opening carries the takeover row for the summary's year, if any —
	// Stichtag, entered Resturlaub, note, actor for the admin UI.
	Opening *activeModels.StaffVacationOpening `json:"opening,omitempty"`
}

// StaffAbsenceService defines operations for staff absence management
type StaffAbsenceService interface {
	CreateAbsence(ctx context.Context, staffID int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error)
	CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error)
	// CreateAbsenceFor separates the absence's subject from its creator so an
	// admin can file a sick report on a staff member's behalf (#1843). A sick
	// full-day absence cascades into the plans via the ShiftPlanSyncer inside
	// the caller's tenant tx; a cascade error aborts the whole create.
	CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error)
	UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, req UpdateAbsenceRequest) (*StaffAbsenceResponse, error)
	DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) error
	DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) error
	// DeleteAbsenceFor is DeleteAbsence with an explicit actor (admin delete,
	// #1843): deleting a sick report reverses its plan cascade first.
	DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) error
	// PreviewCompTimeBalance projects the Stundenkonto effect of a planned
	// Freizeitausgleich before the Leitung confirms it (#2873) — informative
	// only, an overdraft no longer blocks the create.
	PreviewCompTimeBalance(ctx context.Context, staffID int64, start, end timezone.Date, halfDay bool) (*CompTimeBalancePreview, error)
	// SetShiftPlanSyncer injects the #1843 plan cascade (wired in the factory
	// after the schedule services exist; mirrors SetStaffShiftRepo).
	SetShiftPlanSyncer(syncer ShiftPlanSyncer)
	GetAbsencesForRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAbsenceResponse, error)
	ListAbsences(ctx context.Context, staffID int64, filter StaffAbsenceListFilter) ([]*StaffAbsenceResponse, error)
	HasAbsenceOnDate(ctx context.Context, staffID int64, date timezone.Date) (bool, *activeModels.StaffAbsence, error)

	// GetTodayAbsenceMap returns staff ID -> absence type for today (issue
	// #584 lookup; repository result returned verbatim).
	GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error)

	// GetTodayAbsenceLabelMap returns staff ID -> the school's own wording for
	// today's winning absence (#2403). Only staff whose absence carries a
	// school-defined Abwesenheitsart appear; everyone else keeps the standard
	// label the client derives from GetTodayAbsenceMap.
	GetTodayAbsenceLabelMap(ctx context.Context) (map[int64]string, error)

	// Vacation workflow (Tranche 4)
	RequestVacation(ctx context.Context, staffID int64, req RequestVacationRequest) (*StaffAbsenceResponse, error)
	ApproveAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, note string) (*StaffAbsenceResponse, error)
	DenyAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, reason string) (*StaffAbsenceResponse, error)
	// QuestionAbsence moves a requested absence into "question" (Rückfrage)
	// with a mandatory note from the Leitung (#1419 4d). No decision is
	// recorded yet — the actor is captured in the audit row.
	QuestionAbsence(ctx context.Context, absenceID int64, actorAccountID int64, note string) (*StaffAbsenceResponse, error)
	// ResubmitAbsence lets the MA amend their note and move a "question"
	// absence back to "requested" for another decision round (#1419 4d).
	ResubmitAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64, note string) (*StaffAbsenceResponse, error)
	CancelAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64) error
	GetVacationQuotaSummary(ctx context.Context, staffID int64, year int) (*VacationQuotaSummary, error)
	UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error
	// Vacation takeover at the moto introduction (#2132): one row per staff
	// and year; corrections are delete + re-create (deletion tombstone).
	GetVacationOpening(ctx context.Context, staffID int64, year int) (*activeModels.StaffVacationOpening, error)
	SetVacationOpening(ctx context.Context, staffID, decidedBy int64, req SetVacationOpeningRequest) (*activeModels.StaffVacationOpening, error)
	DeleteVacationOpening(ctx context.Context, staffID, deletedBy int64, year int) error
	// ValidateVacationOpeningAbsencesBefore is the read-only half of the
	// takeover guard, used by the bulk import's dry-run preview. Part of the
	// contract because the import lives in another package.
	ValidateVacationOpeningAbsencesBefore(ctx context.Context, staffID int64, effectiveDate timezone.Date) error
	ListPendingRequests(ctx context.Context) ([]*StaffAbsenceResponse, error)
	// ListAbsenceRequests serves the Mitarbeitende-Reiter of the Anfragen
	// module (#2433): the open work list or the decided history, both with
	// the names the list shows, filtered by absence type and staff name.
	ListAbsenceRequests(ctx context.Context, req AbsenceRequestListQuery) ([]*StaffAbsenceRequestItem, error)
}

// GetTodayAbsenceMap returns staff ID -> absence type for today.
func (s *staffAbsenceService) GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error) {
	return s.absenceRepo.GetAbsenceMapForDate(ctx, timezone.TodayDate())
}

// GetTodayAbsenceLabelMap returns staff ID -> school-defined absence name for
// today.
func (s *staffAbsenceService) GetTodayAbsenceLabelMap(ctx context.Context) (map[int64]string, error) {
	if s.absenceTypes == nil {
		return map[int64]string{}, nil
	}
	typeIDs, err := s.absenceRepo.GetAbsenceTypeIDMapForDate(ctx, timezone.TodayDate())
	if err != nil {
		return nil, err
	}
	if len(typeIDs) == 0 {
		return map[int64]string{}, nil
	}
	names, err := s.absenceTypes.LabelsByID(ctx)
	if err != nil {
		return nil, err
	}
	labels := make(map[int64]string, len(typeIDs))
	for staffID, typeID := range typeIDs {
		if name, ok := names[typeID]; ok {
			labels[staffID] = name
		}
	}
	return labels, nil
}

// staffAbsenceService implements StaffAbsenceService
type staffAbsenceService struct {
	absenceRepo     activeModels.StaffAbsenceRepository
	workSessionRepo activeModels.WorkSessionRepository
	quotaRepo       activeModels.StaffVacationQuotaRepository
	// openingRepo carries the vacation takeover rows (#2132); setter
	// injection (SetVacationOpeningRepository), nil in bare unit fixtures.
	openingRepo activeModels.StaffVacationOpeningRepository
	auditRepo   activeModels.StaffAbsenceAuditRepository
	settings    monthSettingsResolver
	// monthService provides the daily-target and closing-balance math for
	// the comp_time overdraft guard; nil in bare-constructed unit tests.
	monthService    WorkTimeMonthService
	shiftPlanSyncer ShiftPlanSyncer
	// emailDeps is nil unless SetAbsenceEmailDeps wired it (factory only);
	// nil means no absence emails are sent (#1419 4d).
	emailDeps   *AbsenceEmailDeps
	broadcaster realtime.Broadcaster
	// deletionRepo writes append-only tombstones for deleted absences
	// (#1417): the absence's own audit trail is ON DELETE CASCADE, so
	// without the tombstone a delete erases its whole history. Setter
	// injection (SetDeletionAudit) like SetBroadcaster; nil makes deletes
	// fail.
	deletionRepo auditModels.TimeTrackingDeletionRepository
	// absenceTypes resolves school-defined Abwesenheitsarten (#2403). Setter
	// injection (SetAbsenceTypeService) like the others; nil in bare-constructed
	// unit fixtures, where every absence is a plain standard type.
	absenceTypes StaffAbsenceTypeService
	todayFunc    func() timezone.Date
}

func (s *staffAbsenceService) today() timezone.Date {
	if s.todayFunc != nil {
		return s.todayFunc()
	}
	return timezone.TodayDate()
}

// SetAbsenceTypeService wires the school-defined absence names (#2403).
func (s *staffAbsenceService) SetAbsenceTypeService(svc StaffAbsenceTypeService) {
	s.absenceTypes = svc
}

// resolveAbsenceTypeSelection validates a requested Abwesenheitsart and returns
// the canonical absence type the row must carry. A nil/zero ID leaves the
// caller's own type in place.
//
// The base type is taken from the art, never from the client: that is the whole
// point of the split — a school can name a day "Regenerationstag", but it stays
// arithmetically the type the art was created as (v1: always "Sonstige").
func (s *staffAbsenceService) resolveAbsenceTypeSelection(ctx context.Context, typeID *int64, fallbackType string) (*int64, string, error) {
	if typeID == nil || *typeID <= 0 {
		return nil, fallbackType, nil
	}
	if s.absenceTypes == nil {
		return nil, "", ErrAbsenceTypeNotFound
	}
	resolved, err := s.absenceTypes.ResolveForAbsence(ctx, *typeID)
	if err != nil {
		return nil, "", err
	}
	id := resolved.ID
	return &id, resolved.BaseType, nil
}

// withLabels stamps the school's own wording onto the given responses and
// returns them unchanged otherwise, so read paths can wrap their return value
// in one call.
func (s *staffAbsenceService) withLabels(ctx context.Context, responses ...*StaffAbsenceResponse) []*StaffAbsenceResponse {
	absences := make([]*activeModels.StaffAbsence, 0, len(responses))
	for _, r := range responses {
		if r != nil && r.StaffAbsence != nil {
			absences = append(absences, r.StaffAbsence)
		}
	}
	StampAbsenceTypeLabels(ctx, s.absenceTypes, absences)
	return responses
}

// withLabel is the single-response form of withLabels.
func (s *staffAbsenceService) withLabel(ctx context.Context, response *StaffAbsenceResponse) *StaffAbsenceResponse {
	s.withLabels(ctx, response)
	return response
}

// SetDeletionAudit wires the deletion tombstone writer (#1417).
func (s *staffAbsenceService) SetDeletionAudit(repo auditModels.TimeTrackingDeletionRepository) {
	s.deletionRepo = repo
}

// SetShiftPlanSyncer wires the #1843 plan cascade (setter injection because
// the implementation lives in services/schedule, which imports this package).
func (s *staffAbsenceService) SetShiftPlanSyncer(syncer ShiftPlanSyncer) {
	s.shiftPlanSyncer = syncer
}

// SetBroadcaster injects the tenant-wide SSE broadcaster. It stays outside
// StaffAbsenceService so existing API-layer mocks do not need a no-op setter.
func (s *staffAbsenceService) SetBroadcaster(broadcaster realtime.Broadcaster) {
	s.broadcaster = broadcaster
}

func (s *staffAbsenceService) broadcastTimeTrackingChanged(ctx context.Context) {
	queueStaffTimeTrackingChanged(ctx, s.broadcaster, nil)
}

// planSyncer is nil-safe: bare-constructed services (unit tests) cascade into
// a no-op.
func (s *staffAbsenceService) planSyncer() ShiftPlanSyncer {
	if s.shiftPlanSyncer != nil {
		return s.shiftPlanSyncer
	}
	return noopShiftPlanSyncer{}
}

// NewStaffAbsenceService creates a new staff absence service
func NewStaffAbsenceService(
	absenceRepo activeModels.StaffAbsenceRepository,
	workSessionRepo activeModels.WorkSessionRepository,
	quotaRepo activeModels.StaffVacationQuotaRepository,
	auditRepo activeModels.StaffAbsenceAuditRepository,
	settings monthSettingsResolver,
	monthService WorkTimeMonthService,
	today ...func() timezone.Date,
) StaffAbsenceService {
	service := &staffAbsenceService{
		absenceRepo:     absenceRepo,
		workSessionRepo: workSessionRepo,
		quotaRepo:       quotaRepo,
		auditRepo:       auditRepo,
		settings:        settings,
		monthService:    monthService,
	}
	if len(today) > 0 {
		service.todayFunc = today[0]
	}
	return service
}

// defaultEntitledDays is the fallback when no per-staff quota row exists.
// 30 working days = TVöD-Allgemein standard. Pro-Tenant override via settings
// will come in a follow-up, see Issue #1375 Tranche 4a.
const defaultEntitledDays = 30.0

// CreateAbsence creates a new absence record for the caller themself.
func (s *staffAbsenceService) CreateAbsence(ctx context.Context, staffID int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	return s.CreateOwnAbsence(ctx, staffID, nil, req)
}

// CreateOwnAbsence is the self-service entry point. Comp-time absences change
// the Stundenkonto and therefore require the manager-authorized staff route.
func (s *staffAbsenceService) CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	if req.AbsenceType == activeModels.AbsenceTypeCompTime {
		return nil, ErrManagerControlledAbsence
	}
	return s.CreateAbsenceFor(ctx, staffID, staffID, actorAccountID, req)
}

// CreateAbsenceFor creates an absence for subjectStaffID on createdByStaffID's
// behalf (#1843 admin sick reporting; self-service passes the same id twice).
// A full-day sick absence cascades into the plans; the merge path cascades
// over the MERGED range of the primary row, which the idempotency guards in
// the syncer make safe to re-run.
func (s *staffAbsenceService) CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	// Resolve the school-defined art first (#2403): it, not the client, decides
	// the canonical type every guard below and every later calculation reads.
	typeID, baseType, err := s.resolveAbsenceTypeSelection(ctx, req.AbsenceTypeID, req.AbsenceType)
	if err != nil {
		return nil, err
	}
	req.AbsenceTypeID, req.AbsenceType = typeID, baseType

	if req.AbsenceType == activeModels.AbsenceTypeVacation {
		return nil, fmt.Errorf("vacation absences must be requested through the vacation flow")
	}

	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}
	if err = validateSickAbsenceRange(req.AbsenceType, dateStart, dateEnd); err != nil {
		return nil, err
	}
	if err := s.rejectPreAccountCompTime(ctx, req.AbsenceType, dateStart, dateEnd); err != nil {
		return nil, err
	}
	if err := s.lockStaffAbsenceWrites(ctx, subjectStaffID); err != nil {
		return nil, err
	}

	// Check for overlapping absences, merge if same type, reject if different type.
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, subjectStaffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}

	var resp *StaffAbsenceResponse
	if blocking := filterBlockingAbsences(existing); len(blocking) > 0 {
		resp, err = s.mergeOverlappingAbsences(ctx, blocking, dateStart, dateEnd, createdByStaffID, req)
	} else {
		if err := validateSingleDayHalfDayAbsence(req.AbsenceType, req.HalfDay, dateStart, dateEnd); err != nil {
			return nil, err
		}
		s.warnIfWorkSessionsExist(ctx, subjectStaffID, dateStart, dateEnd)
		resp, err = s.createNewAbsence(ctx, subjectStaffID, createdByStaffID, dateStart, dateEnd, req)
	}
	if err != nil {
		return nil, err
	}

	if err := s.cascadeSickReport(ctx, resp.StaffAbsence, createdByStaffID, actorAccountID); err != nil {
		return nil, err
	}
	s.broadcastTimeTrackingChanged(ctx)
	return resp, nil
}

// cascadeSickReport fans a full-day sick report out into the plans (#1843).
// Fail-closed: an error aborts the surrounding create so a sick report whose
// plan effects half-applied never commits.
func (s *staffAbsenceService) cascadeSickReport(ctx context.Context, absence *activeModels.StaffAbsence, actorStaffID int64, actorAccountID *int64) error {
	if absence.AbsenceType != activeModels.AbsenceTypeSick || absence.HalfDay {
		return nil
	}
	if err := s.planSyncer().MarkSickForRange(ctx, SickCascadeInput{
		SubjectStaffID: absence.StaffID,
		DateStart:      absence.DateStart,
		DateEnd:        absence.DateEnd,
		SkipStartDay:   absence.StartHalfDay,
		SkipEndDay:     absence.EndHalfDay,
		AbsenceID:      absence.ID,
		ActorStaffID:   actorStaffID,
		ActorAccountID: actorAccountID,
	}); err != nil {
		return fmt.Errorf("sick report saved nothing — plan cascade failed: %w", err)
	}
	return nil
}

// parseDateRange parses start and end date strings in ISO format.
func parseDateRange(startStr, endStr string) (timezone.Date, timezone.Date, error) {
	dateStart, err := timezone.ParseDate(startStr)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
	}
	dateEnd, err := timezone.ParseDate(endStr)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")
	}
	// Reject reversed ranges here instead of letting them hit the DB check
	// constraint chk_sa_dates, which would surface as a 500 (#1420 review).
	// The "invalid" prefix is what the handlers classify as a 400.
	if dateEnd.Before(dateStart) {
		return timezone.Date{}, timezone.Date{}, fmt.Errorf("invalid date range: date_end must not be before date_start")
	}
	return dateStart, dateEnd, nil
}

// mergeOverlappingAbsences handles overlapping absences: rejects if different type, merges if same type.
func (s *staffAbsenceService) mergeOverlappingAbsences(
	ctx context.Context,
	existing []*activeModels.StaffAbsence,
	dateStart, dateEnd timezone.Date,
	actorStaffID int64,
	req CreateAbsenceRequest,
) (*StaffAbsenceResponse, error) {
	// Check if all overlapping absences have the same type
	if err := validateSameAbsenceType(existing, req.AbsenceType); err != nil {
		return nil, err
	}
	if err := validateSameAbsenceTypeName(existing, req.AbsenceTypeID); err != nil {
		return nil, err
	}
	if err := validateSameMergeDuration(existing, req); err != nil {
		return nil, err
	}

	// Calculate merged date range
	mergedStart, mergedEnd := calculateMergedDateRange(existing, dateStart, dateEnd)
	if err := validateSickAbsenceRange(req.AbsenceType, mergedStart, mergedEnd); err != nil {
		return nil, err
	}
	if err := validateSingleDayHalfDayAbsence(req.AbsenceType, req.HalfDay, mergedStart, mergedEnd); err != nil {
		return nil, err
	}

	// Update the primary absence with merged range
	primary := existing[0]
	primary.DateStart = mergedStart
	primary.DateEnd = mergedEnd
	if req.Note != "" && primary.Note == "" {
		primary.Note = req.Note
	}
	primary.UpdatedAt = time.Now()

	if err := s.absenceRepo.Update(ctx, primary); err != nil {
		return nil, fmt.Errorf("failed to merge absence: %w", err)
	}

	// The secondaries are about to be deleted; their plan stamps must move to
	// the surviving primary or its eventual deletion would miss those rows
	// (#1843). Fail closed — a dangling stamp breaks the reversal invariant.
	if primary.AbsenceType == activeModels.AbsenceTypeSick {
		for _, secondary := range existing[1:] {
			if err := s.planSyncer().ReassignSickStamps(ctx, secondary.ID, primary.ID); err != nil {
				return nil, fmt.Errorf("failed to merge absence — plan stamp reassignment failed: %w", err)
			}
		}
	}

	// Delete remaining overlapping absences. The reassigned provenance and the
	// primary update must roll back if even one secondary cannot be removed.
	if err := s.deleteRemainingAbsences(ctx, existing[1:], actorStaffID); err != nil {
		return nil, err
	}

	return s.withLabel(ctx, toAbsenceResponse(primary)), nil
}

// validateSameMergeDuration rejects merging half-day with full-day rows for
// the duration-sensitive types: sick cascades per day, comp_time deducts the
// Stundenkonto. The merge keeps the primary's HalfDay flag, so a mismatch
// would silently rewrite the requested duration (#1420).
func validateSameMergeDuration(existing []*activeModels.StaffAbsence, req CreateAbsenceRequest) error {
	if req.AbsenceType != activeModels.AbsenceTypeSick && req.AbsenceType != activeModels.AbsenceTypeCompTime {
		return nil
	}
	for _, absence := range existing {
		if absence.HalfDay != req.HalfDay {
			if req.AbsenceType == activeModels.AbsenceTypeCompTime {
				return fmt.Errorf("invalid comp_time absence: half-day and full-day entries cannot be merged")
			}
			return fmt.Errorf("invalid sick absence: half-day and full-day reports cannot be merged")
		}
	}
	return nil
}

func validateSickAbsenceRange(absenceType string, start, end timezone.Date) error {
	if absenceType != activeModels.AbsenceTypeSick || start.IsZero() || end.IsZero() || end.Before(start) {
		return nil
	}
	if start.DaysUntil(end)+1 > maxSickAbsenceRangeDays {
		return fmt.Errorf("invalid sick absence: date range exceeds %d days", maxSickAbsenceRangeDays)
	}
	return nil
}

func validateSingleDayHalfDayAbsence(absenceType string, halfDay bool, start, end timezone.Date) error {
	if !halfDay || start == end {
		return nil
	}
	switch absenceType {
	case activeModels.AbsenceTypeSick:
		return fmt.Errorf("invalid sick absence: half-day reports must cover exactly one date")
	case activeModels.AbsenceTypeCompTime:
		return fmt.Errorf("invalid comp_time absence: half-day reports must cover exactly one date")
	}
	return nil
}

// rejectPreAccountCompTime fails a comp_time absence dated before the account
// start: balance aggregation begins at the anchor, so such an absence would
// appear in the history without ever reducing the Stundenkonto (mirrors
// rejectPreAccountDate on balance adjustments, #1420).
func (s *staffAbsenceService) rejectPreAccountCompTime(ctx context.Context, absenceType string, dateStart, dateEnd timezone.Date) error {
	if absenceType != activeModels.AbsenceTypeCompTime {
		return nil
	}
	anchor, err := resolveAccountAnchor(ctx, s.settings, slog.Default(), monthOf(s.today()))
	if err != nil {
		return fmt.Errorf("failed to resolve account start for comp_time absence: %w", err)
	}
	if dateStart.Before(anchor) {
		return fmt.Errorf("invalid comp_time absence: date_start %s lies before the account start %s", dateStart.String(), anchor.String())
	}
	// Mirror the Monatskarte's future bound: the overdraft guard reads the
	// closing balance before dateStart, which has no defined result beyond
	// the carry-chain horizon (#1420 review).
	if horizon := monthOf(s.today()).addMonths(maxFutureMonths); horizon.before(monthOf(dateEnd)) {
		return fmt.Errorf("invalid comp_time absence: date_end %s is more than %d months ahead", dateEnd.String(), maxFutureMonths)
	}
	return nil
}

// CompTimeBalancePreview is what the create modal shows before the Leitung
// confirms a Freizeitausgleich (#2873). All values are minutes.
type CompTimeBalancePreview struct {
	// CurrentBalanceMinutes is the live Stundenkonto as of today.
	CurrentBalanceMinutes int `json:"current_balance_minutes"`
	// DeductionMinutes is what the requested range additionally deducts
	// (overlapping existing comp-time rows already reserve their share).
	DeductionMinutes int `json:"deduction_minutes"`
	// RealizedDeductionMinutes is the part of DeductionMinutes on days up to
	// today — already contained in CurrentBalanceMinutes, since a day without
	// recorded work shows its missing target as minus either way.
	RealizedDeductionMinutes int `json:"realized_deduction_minutes"`
	// FutureCommitmentMinutes sums the deductions of OTHER comp-time entries
	// on days after today, which the current balance does not yet include.
	FutureCommitmentMinutes int `json:"future_commitment_minutes"`
	// ProjectedBalanceMinutes = current − future commitments − the unrealized
	// part of the new deduction: the account once every named day has passed.
	ProjectedBalanceMinutes int `json:"projected_balance_minutes"`
}

// PreviewCompTimeBalance projects what a comp_time absence over [start, end]
// does to the Stundenkonto (#2873). It replaces the former overdraft
// rejection: the numbers inform the Leitung before the deliberate confirm in
// the frontend, they never block the booking.
func (s *staffAbsenceService) PreviewCompTimeBalance(
	ctx context.Context,
	staffID int64,
	start, end timezone.Date,
	halfDay bool,
) (*CompTimeBalancePreview, error) {
	if s.monthService == nil {
		return nil, fmt.Errorf("comp_time preview requires the month service")
	}
	if end.Before(start) {
		return nil, fmt.Errorf("invalid date range: date_end must not be before date_start")
	}
	if err := validateSingleDayHalfDayAbsence(activeModels.AbsenceTypeCompTime, halfDay, start, end); err != nil {
		return nil, err
	}
	if err := s.rejectPreAccountCompTime(ctx, activeModels.AbsenceTypeCompTime, start, end); err != nil {
		return nil, err
	}

	today := s.today()
	currentBalance := 0
	anchor, err := resolveAccountAnchor(ctx, s.settings, slog.Default(), monthOf(today))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve account start for comp_time preview: %w", err)
	}
	if !today.Before(anchor) {
		currentBalance, err = s.monthService.GetClosingBalanceAsOf(ctx, staffID, today)
		if err != nil {
			return nil, fmt.Errorf("failed to compute current balance for comp_time preview: %w", err)
		}
	}

	// Existing comp-time rows overlapping the requested range already reserve
	// their minutes; a create over them merges, so only the additional part
	// counts — the same reservation arithmetic the former guard used.
	overlapping, err := s.effectiveCompTimeAbsences(ctx, staffID, start, end)
	if err != nil {
		return nil, err
	}
	deduction, err := s.getAdditionalCompTimeDeduction(ctx, staffID, start, end, halfDay, overlapping)
	if err != nil {
		return nil, err
	}

	unrealizedDeduction := 0
	if end.After(today) {
		clippedStart := start
		if tomorrow := today.AddDays(1); clippedStart.Before(tomorrow) {
			clippedStart = tomorrow
		}
		unrealizedDeduction, err = s.getAdditionalCompTimeDeduction(ctx, staffID, clippedStart, end, halfDay, overlapping)
		if err != nil {
			return nil, err
		}
	}

	futureCommitment, err := s.monthService.GetFutureCompTimeCommitmentMinutes(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute future comp_time commitments: %w", err)
	}

	return &CompTimeBalancePreview{
		CurrentBalanceMinutes:    currentBalance,
		DeductionMinutes:         deduction,
		RealizedDeductionMinutes: deduction - unrealizedDeduction,
		FutureCommitmentMinutes:  futureCommitment,
		ProjectedBalanceMinutes:  currentBalance - futureCommitment - unrealizedDeduction,
	}, nil
}

// effectiveCompTimeAbsences returns the comp_time rows overlapping [start,
// end] that actually reserve balance (reported/approved) — the set a create
// over the same range would merge with.
func (s *staffAbsenceService) effectiveCompTimeAbsences(ctx context.Context, staffID int64, start, end timezone.Date) ([]*activeModels.StaffAbsence, error) {
	rows, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing comp_time absences: %w", err)
	}
	compTime := make([]*activeModels.StaffAbsence, 0, len(rows))
	for _, row := range rows {
		if row.AbsenceType != activeModels.AbsenceTypeCompTime {
			continue
		}
		if row.Status != activeModels.AbsenceStatusReported && row.Status != activeModels.AbsenceStatusApproved {
			continue
		}
		compTime = append(compTime, row)
	}
	return compTime, nil
}

func (s *staffAbsenceService) getAdditionalCompTimeDeduction(
	ctx context.Context,
	staffID int64,
	start, end timezone.Date,
	halfDay bool,
	existing []*activeModels.StaffAbsence,
) (int, error) {
	deduction, err := s.monthService.GetCompTimeDeductionMinutes(ctx, staffID, start, end, halfDay)
	if err != nil {
		return 0, fmt.Errorf("failed to compute comp_time deduction: %w", err)
	}

	reservedDeduction := 0
	for _, absence := range existing {
		overlapStart := absence.DateStart
		if overlapStart.Before(start) {
			overlapStart = start
		}
		overlapEnd := absence.DateEnd
		if overlapEnd.After(end) {
			overlapEnd = end
		}
		if overlapEnd.Before(overlapStart) {
			continue
		}
		reserved, err := s.monthService.GetCompTimeDeductionMinutes(
			ctx,
			staffID,
			overlapStart,
			overlapEnd,
			absence.HalfDay,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to compute existing comp_time deduction: %w", err)
		}
		reservedDeduction += reserved
	}
	return deduction - reservedDeduction, nil
}

func (s *staffAbsenceService) lockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	if err := s.absenceRepo.LockStaffAbsenceWrites(ctx, staffID); err != nil {
		return fmt.Errorf("failed to lock staff absence writes: %w", err)
	}
	return nil
}

// validateSameAbsenceTypeName keeps the merge honest for school-defined
// Abwesenheitsarten (#2403). Two overlapping rows can share the canonical type
// "Sonstige" and still be a "Regenerationstag" and a "Ferienzeit" — merging
// them would silently relabel one of them, so an unequal art is an overlap
// conflict, exactly like an unequal type.
func validateSameAbsenceTypeName(existing []*activeModels.StaffAbsence, absenceTypeID *int64) error {
	for _, e := range existing {
		if !sameAbsenceTypeID(e.AbsenceTypeID, absenceTypeID) {
			return fmt.Errorf("absence overlaps with an existing absence of a different Abwesenheitsart from %s to %s",
				e.DateStart,
				e.DateEnd)
		}
	}
	return nil
}

func sameAbsenceTypeID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// validateSameAbsenceType checks that all existing absences match the requested type.
func validateSameAbsenceType(existing []*activeModels.StaffAbsence, absenceType string) error {
	for _, e := range existing {
		if e.AbsenceType != absenceType {
			return fmt.Errorf("absence overlaps with existing %s absence from %s to %s",
				e.AbsenceType,
				e.DateStart,
				e.DateEnd)
		}
	}
	return nil
}

// calculateMergedDateRange expands the date range to cover all overlapping absences.
func calculateMergedDateRange(existing []*activeModels.StaffAbsence, dateStart, dateEnd timezone.Date) (timezone.Date, timezone.Date) {
	mergedStart := dateStart
	mergedEnd := dateEnd
	for _, e := range existing {
		if e.DateStart.Before(mergedStart) {
			mergedStart = e.DateStart
		}
		if e.DateEnd.After(mergedEnd) {
			mergedEnd = e.DateEnd
		}
	}
	return mergedStart, mergedEnd
}

// deleteRemainingAbsences deletes absences that were merged into the primary.
func (s *staffAbsenceService) deleteRemainingAbsences(ctx context.Context, absences []*activeModels.StaffAbsence, actorStaffID int64) error {
	for _, e := range absences {
		if err := s.writeAbsenceDeletionAudit(ctx, e, actorStaffID); err != nil {
			return fmt.Errorf("failed to audit merged absence %d: %w", e.ID, err)
		}
		if err := s.absenceRepo.Delete(ctx, e.ID); err != nil {
			return fmt.Errorf("failed to delete merged absence %d: %w", e.ID, err)
		}
	}
	return nil
}

// warnIfWorkSessionsExist logs a warning if work sessions exist in the date range.
func (s *staffAbsenceService) warnIfWorkSessionsExist(ctx context.Context, staffID int64, dateStart, dateEnd timezone.Date) {
	sessions, err := s.workSessionRepo.GetHistoryByStaffID(ctx, staffID, dateStart, dateEnd)
	if err == nil && len(sessions) > 0 {
		slog.Default().WarnContext(ctx, "work sessions exist in absence range",
			slog.Int("session_count", len(sessions)),
			slog.Int64("staff_id", staffID),
			slog.String("date_start", dateStart.String()),
			slog.String("date_end", dateEnd.String()))
	}
}

// createNewAbsence creates a new absence record in the database. createdBy
// may differ from staffID when an admin files the report (#1843).
func (s *staffAbsenceService) createNewAbsence(
	ctx context.Context,
	staffID int64,
	createdBy int64,
	dateStart, dateEnd timezone.Date,
	req CreateAbsenceRequest,
) (*StaffAbsenceResponse, error) {
	now := time.Now()
	absence := &activeModels.StaffAbsence{
		StaffID:       staffID,
		AbsenceType:   req.AbsenceType,
		AbsenceTypeID: req.AbsenceTypeID,
		DateStart:     dateStart,
		DateEnd:       dateEnd,
		HalfDay:       req.HalfDay,
		Note:          req.Note,
		Status:        activeModels.AbsenceStatusReported,
		CreatedBy:     createdBy,
	}
	absence.CreatedAt = now
	absence.UpdatedAt = now

	absence.SetTenantID(tenant.FromContext(ctx))
	if err := s.absenceRepo.Create(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to create absence: %w", err)
	}

	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

// UpdateAbsence updates an existing absence record
func (s *staffAbsenceService) UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, req UpdateAbsenceRequest) (*StaffAbsenceResponse, error) {
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}

	// Verify ownership
	if absence.StaffID != staffID {
		return nil, fmt.Errorf("can only update own absences")
	}
	if absence.AbsenceType == activeModels.AbsenceTypeCompTime ||
		(req.AbsenceType != nil && *req.AbsenceType == activeModels.AbsenceTypeCompTime) {
		return nil, ErrManagerControlledAbsence
	}
	before := *absence
	if err := validateAbsenceUpdate(absence, req); err != nil {
		return nil, err
	}

	// Apply updates from request
	if err := applyAbsenceUpdates(absence, req); err != nil {
		return nil, err
	}
	// A re-pointed Abwesenheitsart (#2403) also re-pins the canonical type, so
	// the name and the arithmetic can never drift apart on an edit either. An
	// unchanged retired art is deliberately preserved: historic entries remain
	// editable, while resolving a newly selected retired art still rejects it.
	if req.AbsenceTypeIDSet || req.AbsenceTypeID != nil {
		if before.AbsenceType == activeModels.AbsenceTypeSick &&
			!sameAbsenceTypeID(req.AbsenceTypeID, before.AbsenceTypeID) {
			return nil, fmt.Errorf("invalid absence type change: sick absences must be deleted and re-created, not converted")
		}
		if sameAbsenceTypeID(req.AbsenceTypeID, before.AbsenceTypeID) {
			absence.AbsenceTypeID = before.AbsenceTypeID
			// Only a kept custom art re-pins the canonical type. With no art on
			// either side there is nothing to pin to, and restoring the old type
			// here would swallow a plain standard-type change: the client sends
			// absence_type_id: null alongside every standard selection, so this
			// branch is exactly the "Fortbildung → Sonstige" case.
			if before.AbsenceTypeID != nil {
				absence.AbsenceType = before.AbsenceType
			}
		} else {
			typeID, baseType, err := s.resolveAbsenceTypeSelection(ctx, req.AbsenceTypeID, absence.AbsenceType)
			if err != nil {
				return nil, err
			}
			absence.AbsenceTypeID, absence.AbsenceType = typeID, baseType
		}
	} else if req.AbsenceType != nil && before.AbsenceTypeID != nil {
		// A canonical type update without an art ID means the standard type. Do
		// not retain the old custom ID, as that pair violates the DB constraint.
		absence.AbsenceTypeID = nil
	}
	if err := validateSickAbsenceRange(absence.AbsenceType, absence.DateStart, absence.DateEnd); err != nil {
		return nil, err
	}
	if err := validateSingleDayHalfDayAbsence(absence.AbsenceType, absence.HalfDay, absence.DateStart, absence.DateEnd); err != nil {
		return nil, err
	}
	if err := s.rejectVacationBeforeOpening(ctx, absence); err != nil {
		return nil, err
	}

	// Check for overlapping absences (excluding self)
	if err := s.checkOverlapExcludingSelf(ctx, staffID, absenceID, absence.DateStart, absence.DateEnd); err != nil {
		return nil, err
	}

	absence.UpdatedAt = time.Now()

	if err := absence.Validate(); err != nil {
		return nil, fmt.Errorf("invalid absence data: %w", err)
	}

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to update absence: %w", err)
	}

	if err := s.reconcileUpdatedSickAbsence(ctx, &before, absence, staffID, actorAccountID); err != nil {
		return nil, err
	}

	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func validateAbsenceUpdate(absence *activeModels.StaffAbsence, req UpdateAbsenceRequest) error {
	if isVacationWorkflowAbsence(absence) {
		return fmt.Errorf("vacation workflow absences must be changed through the vacation flow")
	}
	if req.AbsenceType != nil && *req.AbsenceType == activeModels.AbsenceTypeVacation {
		return fmt.Errorf("vacation absences must be requested through the vacation flow")
	}
	// A type change into or out of "sick" would desync the #1843 plan cascade:
	// sick→other keeps the cancellations/marks but skips the reversal on
	// delete forever; other→sick pretends a cascade that never ran. Delete
	// and re-create instead. Date edits remain supported and reconcile their
	// plan-day difference below.
	if req.AbsenceType != nil && *req.AbsenceType != absence.AbsenceType &&
		(absence.AbsenceType == activeModels.AbsenceTypeSick || *req.AbsenceType == activeModels.AbsenceTypeSick) {
		return fmt.Errorf("invalid absence type change: sick absences must be deleted and re-created, not converted")
	}
	// Same reasoning for the half-day flag: flipping it would desync the
	// already-applied (or deliberately skipped) cascade from the record.
	if absence.AbsenceType == activeModels.AbsenceTypeSick &&
		req.HalfDay != nil && *req.HalfDay != absence.HalfDay {
		return fmt.Errorf("invalid absence change: sick absences cannot switch between half and full days — delete and re-create the report")
	}
	return nil
}

func (s *staffAbsenceService) reconcileUpdatedSickAbsence(ctx context.Context, before, after *activeModels.StaffAbsence, actorStaffID int64, actorAccountID *int64) error {
	if before.AbsenceType == activeModels.AbsenceTypeSick && !before.HalfDay &&
		(before.DateStart != after.DateStart || before.DateEnd != after.DateEnd) {
		if err := s.planSyncer().ReconcileSickRange(
			ctx,
			sickCascadeInput(before, actorStaffID, actorAccountID),
			sickCascadeInput(after, actorStaffID, actorAccountID),
		); err != nil {
			return fmt.Errorf("absence update saved nothing — plan cascade reconciliation failed: %w", err)
		}
	}
	return nil
}

func sickCascadeInput(absence *activeModels.StaffAbsence, actorStaffID int64, actorAccountID *int64) SickCascadeInput {
	return SickCascadeInput{
		SubjectStaffID: absence.StaffID,
		DateStart:      absence.DateStart,
		DateEnd:        absence.DateEnd,
		SkipStartDay:   absence.StartHalfDay,
		SkipEndDay:     absence.EndHalfDay,
		AbsenceID:      absence.ID,
		ActorStaffID:   actorStaffID,
		ActorAccountID: actorAccountID,
	}
}

// applyAbsenceUpdates applies partial updates from the request to the absence.
func applyAbsenceUpdates(absence *activeModels.StaffAbsence, req UpdateAbsenceRequest) error {
	if req.AbsenceType != nil {
		absence.AbsenceType = *req.AbsenceType
	}
	if req.DateStart != nil {
		dateStart, err := timezone.ParseDate(*req.DateStart)
		if err != nil {
			return fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
		}
		absence.DateStart = dateStart
	}
	if req.DateEnd != nil {
		dateEnd, err := timezone.ParseDate(*req.DateEnd)
		if err != nil {
			return fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")
		}
		absence.DateEnd = dateEnd
	}
	if req.HalfDay != nil {
		absence.HalfDay = *req.HalfDay
	}
	if req.Note != nil {
		absence.Note = *req.Note
	}
	return nil
}

// checkOverlapExcludingSelf checks for overlapping absences, excluding the given absence ID.
func (s *staffAbsenceService) checkOverlapExcludingSelf(ctx context.Context, staffID, absenceID int64, dateStart, dateEnd timezone.Date) error {
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return fmt.Errorf("failed to check existing absences: %w", err)
	}
	for _, e := range existing {
		if e.ID != absenceID && blocksAbsenceRange(e.Status) {
			return fmt.Errorf("updated dates overlap with existing absence from %s to %s",
				e.DateStart,
				e.DateEnd)
		}
	}
	return nil
}

// DeleteAbsence deletes an absence record (self-service; the caller is the
// absence's owner and the acting person).
func (s *staffAbsenceService) DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) error {
	return s.DeleteOwnAbsence(ctx, staffID, nil, absenceID)
}

// DeleteOwnAbsence is the self-service delete path. Manager-created comp-time
// absences remain read-only for the affected staff member.
func (s *staffAbsenceService) DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) error {
	return s.deleteAbsenceFor(ctx, staffID, staffID, actorAccountID, absenceID, false)
}

// DeleteAbsenceFor deletes subjectStaffID's absence on actorStaffID's behalf
// (#1843 admin delete). Deleting a sick report first reverses its plan
// cascade — reactivates the shifts it cancelled and clears the block
// absences it stamped — in the same tenant tx, so a failed reversal aborts
// the delete. CancelAbsence needs no such hook: it only accepts
// requested/approved vacation-flow rows, never a reported sick absence.
func (s *staffAbsenceService) DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) error {
	return s.deleteAbsenceFor(ctx, subjectStaffID, actorStaffID, actorAccountID, absenceID, true)
}

func (s *staffAbsenceService) deleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64, allowManagerControlled bool) error {
	if err := s.lockStaffAbsenceWrites(ctx, subjectStaffID); err != nil {
		return err
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return fmt.Errorf("absence not found")
	}

	// Verify ownership
	if absence.StaffID != subjectStaffID {
		return fmt.Errorf("can only delete own absences")
	}
	if absence.AbsenceType == activeModels.AbsenceTypeCompTime && !allowManagerControlled {
		return ErrManagerControlledAbsence
	}
	if isVacationWorkflowAbsence(absence) {
		return fmt.Errorf("vacation workflow absences must be canceled through the vacation flow")
	}

	// The reversal runs UNCONDITIONALLY: it is keyed purely by provenance
	// stamps (sick_absence_id == this id), so an absence that never cascaded
	// is a cheap no-op, while gating on type/half_day would let a mutated row
	// (e.g. half_day flipped after the cascade ran) orphan its stamps forever.
	if err := s.planSyncer().ClearSickForRange(ctx, SickCascadeInput{
		SubjectStaffID: absence.StaffID,
		DateStart:      absence.DateStart,
		DateEnd:        absence.DateEnd,
		SkipStartDay:   absence.StartHalfDay,
		SkipEndDay:     absence.EndHalfDay,
		AbsenceID:      absence.ID,
		ActorStaffID:   actorStaffID,
		ActorAccountID: actorAccountID,
	}); err != nil {
		return fmt.Errorf("absence not deleted — plan cascade reversal failed: %w", err)
	}

	// Tombstone first, delete second, same tenant tx: the absence's own
	// audit rows CASCADE away with it, so this row is the only surviving
	// trace (#1417).
	if err := s.writeAbsenceDeletionAudit(ctx, absence, actorStaffID); err != nil {
		return err
	}
	if err := s.absenceRepo.Delete(ctx, absenceID); err != nil {
		return fmt.Errorf("failed to delete absence: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *staffAbsenceService) writeAbsenceDeletionAudit(ctx context.Context, absence *activeModels.StaffAbsence, actorStaffID int64) error {
	if s.deletionRepo == nil {
		return fmt.Errorf("time tracking deletion audit repository is not configured")
	}
	StampAbsenceTypeLabels(ctx, s.absenceTypes, []*activeModels.StaffAbsence{absence})
	payload, err := json.Marshal(absence)
	if err != nil {
		return fmt.Errorf("failed to snapshot absence for deletion audit: %w", err)
	}
	if err := s.deletionRepo.Create(ctx, &auditModels.TimeTrackingDeletion{
		StaffID:   absence.StaffID,
		Source:    auditModels.TimeTrackingDeletionSourceAbsence,
		SourceID:  absence.ID,
		DeletedBy: actorStaffID,
		Payload:   payload,
		Note:      absence.Note,
	}); err != nil {
		return fmt.Errorf("failed to write deletion audit: %w", err)
	}
	return nil
}

// ListAbsences returns a staff member's absences for a date range, status, or
// both. The generic repository filter keeps tenant scoping in one place.
func (s *staffAbsenceService) ListAbsences(ctx context.Context, staffID int64, filter StaffAbsenceListFilter) ([]*StaffAbsenceResponse, error) {
	if (filter.From == nil) != (filter.To == nil) {
		return nil, fmt.Errorf("from and to must be provided together")
	}
	if filter.From == nil && filter.Status == "" {
		return nil, fmt.Errorf("absence list filter is required")
	}
	if filter.Status != "" && !slices.Contains(activeModels.ValidAbsenceStatuses, filter.Status) {
		return nil, fmt.Errorf("invalid absence status")
	}

	options := modelBase.NewQueryOptions()
	options.Filter.Equal("staff_id", staffID)
	if filter.From != nil && filter.To != nil {
		options.Filter.
			LessThanOrEqual("date_start", *filter.To).
			GreaterThanOrEqual("date_end", *filter.From)
	}
	if filter.Status != "" {
		options.Filter.Equal("status", filter.Status)
	}

	sorting := &modelBase.Sorting{}
	if filter.From != nil {
		sorting.AddField("date_start", modelBase.SortAsc)
	} else {
		sorting.AddField("requested_at", modelBase.SortDesc)
	}
	options.Sorting = sorting

	absences, err := s.absenceRepo.List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences: %w", err)
	}

	responses := make([]*StaffAbsenceResponse, len(absences))
	for i, a := range absences {
		responses[i] = toAbsenceResponse(a)
	}

	return s.withLabels(ctx, responses...), nil
}

// GetAbsencesForRange preserves the range-only service contract used by staff
// detail views.
func (s *staffAbsenceService) GetAbsencesForRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAbsenceResponse, error) {
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences: %w", err)
	}

	responses := make([]*StaffAbsenceResponse, len(absences))
	for i, a := range absences {
		responses[i] = toAbsenceResponse(a)
	}

	return s.withLabels(ctx, responses...), nil
}

// HasAbsenceOnDate checks if a staff member has an absence on a specific date
func (s *staffAbsenceService) HasAbsenceOnDate(ctx context.Context, staffID int64, date timezone.Date) (bool, *activeModels.StaffAbsence, error) {
	absence, err := s.absenceRepo.GetByStaffAndDate(ctx, staffID, date)
	if err != nil {
		return false, nil, fmt.Errorf("failed to check absence: %w", err)
	}
	if absence == nil {
		return false, nil, nil
	}
	if !isEffectiveAbsenceStatus(absence.Status) {
		return false, nil, nil
	}
	return true, absence, nil
}

func toAbsenceResponse(a *activeModels.StaffAbsence) *StaffAbsenceResponse {
	var absenceTypeID *string
	if a.AbsenceTypeID != nil {
		value := strconv.FormatInt(*a.AbsenceTypeID, 10)
		absenceTypeID = &value
	}
	return &StaffAbsenceResponse{
		StaffAbsence:  a,
		DurationDays:  a.DurationDays(),
		AbsenceTypeID: absenceTypeID,
	}
}

func isEffectiveAbsenceStatus(status string) bool {
	return status == activeModels.AbsenceStatusReported ||
		status == activeModels.AbsenceStatusApproved
}

func blocksAbsenceRange(status string) bool {
	return status == activeModels.AbsenceStatusReported ||
		status == activeModels.AbsenceStatusRequested ||
		status == activeModels.AbsenceStatusQuestion ||
		status == activeModels.AbsenceStatusApproved
}

func isVacationWorkflowAbsence(absence *activeModels.StaffAbsence) bool {
	if absence.AbsenceType != activeModels.AbsenceTypeVacation {
		return false
	}
	return absence.Status == activeModels.AbsenceStatusRequested ||
		absence.Status == activeModels.AbsenceStatusQuestion ||
		absence.Status == activeModels.AbsenceStatusApproved ||
		absence.Status == activeModels.AbsenceStatusDeclined ||
		absence.Status == activeModels.AbsenceStatusCanceled
}

func filterBlockingAbsences(rows []*activeModels.StaffAbsence) []*activeModels.StaffAbsence {
	filtered := make([]*activeModels.StaffAbsence, 0, len(rows))
	for _, row := range rows {
		if blocksAbsenceRange(row.Status) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func effectiveBoundaryHalfDays(a *activeModels.StaffAbsence) (bool, bool) {
	if a.HalfDay && !a.StartHalfDay && !a.EndHalfDay {
		return true, true
	}
	return a.StartHalfDay, a.EndHalfDay
}

func isWorkingDay(d timezone.Date) bool {
	w := d.Weekday()
	return w != time.Saturday && w != time.Sunday
}

// Vacation workflow (Tranche 4)

// countWorkingDays returns Mon-Fri count between two dates inclusive,
// minus 0.5 for each boundary half-day. Personio-style: first day half
// and last day half are independent flags. Feiertage are NOT excluded
// yet. That lives in Tranche 3 (Issue #1231).
func countWorkingDays(from, to timezone.Date, startHalf, endHalf bool) float64 {
	if to.Before(from) {
		return 0
	}
	count := 0
	for d := from; !d.After(to); d = d.AddDays(1) {
		if isWorkingDay(d) {
			count++
		}
	}
	result := float64(count)
	// Single-day range: only one boundary exists, so a single half flag halves it.
	if from == to {
		if isWorkingDay(from) && (startHalf || endHalf) {
			result -= 0.5
		}
		return result
	}
	if startHalf && isWorkingDay(from) {
		result -= 0.5
	}
	if endHalf && isWorkingDay(to) {
		result -= 0.5
	}
	return result
}

func (s *staffAbsenceService) RequestVacation(ctx context.Context, staffID int64, req RequestVacationRequest) (*StaffAbsenceResponse, error) {
	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}
	if isBeforeLocalToday(dateStart, time.Now()) {
		return nil, fmt.Errorf("vacation request must start today or in the future")
	}
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, err
	}

	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}
	// Declined and canceled rows do not block the time slot, the staff
	// member is free to request the same dates again. Only requested,
	// approved and reported absences hold the range.
	for _, e := range existing {
		if blocksAbsenceRange(e.Status) {
			return nil, fmt.Errorf("dates overlap with an existing absence")
		}
	}

	workingDays := countWorkingDays(dateStart, dateEnd, req.StartHalfDay, req.EndHalfDay)
	if workingDays <= 0 {
		return nil, fmt.Errorf("vacation range contains no working days")
	}

	now := time.Now()
	// HalfDay legacy flag stays in sync: true iff either boundary is half.
	halfDay := req.StartHalfDay || req.EndHalfDay
	absence := &activeModels.StaffAbsence{
		StaffID:           staffID,
		AbsenceType:       activeModels.AbsenceTypeVacation,
		DateStart:         dateStart,
		DateEnd:           dateEnd,
		HalfDay:           halfDay,
		StartHalfDay:      req.StartHalfDay,
		EndHalfDay:        req.EndHalfDay,
		Note:              req.Note,
		Status:            activeModels.AbsenceStatusRequested,
		CreatedBy:         staffID,
		WorkingDays:       &workingDays,
		RequestedAt:       now,
		SubstituteStaffID: req.SubstituteStaffID,
	}
	absence.CreatedAt = now
	absence.UpdatedAt = now
	absence.SetTenantID(tenant.FromContext(ctx))

	if err := s.absenceRepo.Create(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to create vacation request: %w", err)
	}
	s.notifyAbsenceRequested(ctx, absence)
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) ApproveAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, note string) (*StaffAbsenceResponse, error) {
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	absence, err = s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != activeModels.AbsenceStatusRequested &&
		absence.Status != activeModels.AbsenceStatusQuestion {
		return nil, fmt.Errorf("only requested absences can be approved")
	}
	if err := s.rejectVacationBeforeOpening(ctx, absence); err != nil {
		return nil, err
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = activeModels.AbsenceStatusApproved
	absence.ApprovedBy = &decidedByStaffID
	absence.ApprovedAt = &now
	absence.DecisionNote = note
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to approve absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, note); err != nil {
		return nil, fmt.Errorf("failed to audit absence approval: %w", err)
	}
	s.notifyAbsenceDecision(ctx, absence)
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) DenyAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, reason string) (*StaffAbsenceResponse, error) {
	if reason == "" {
		return nil, fmt.Errorf("decline reason is required")
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	absence, err = s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != activeModels.AbsenceStatusRequested &&
		absence.Status != activeModels.AbsenceStatusQuestion {
		return nil, fmt.Errorf("only requested absences can be declined")
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = activeModels.AbsenceStatusDeclined
	absence.ApprovedBy = &decidedByStaffID
	absence.ApprovedAt = &now
	absence.DecisionNote = reason
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to decline absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, reason); err != nil {
		return nil, fmt.Errorf("failed to audit absence decline: %w", err)
	}
	s.notifyAbsenceDecision(ctx, absence)
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) QuestionAbsence(ctx context.Context, absenceID int64, actorAccountID int64, note string) (*StaffAbsenceResponse, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, fmt.Errorf("question note is required")
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	absence, err = s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != activeModels.AbsenceStatusRequested {
		return nil, fmt.Errorf("only requested absences can be questioned")
	}
	fromStatus := absence.Status
	absence.Status = activeModels.AbsenceStatusQuestion
	// The Leitung's question lives in decision_note; ApprovedBy/At stay nil
	// because no decision has been made yet. History is in the audit rows.
	absence.DecisionNote = note
	absence.UpdatedAt = time.Now()

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to question absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, note); err != nil {
		return nil, fmt.Errorf("failed to audit absence question: %w", err)
	}
	s.notifyAbsenceDecision(ctx, absence)
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) ResubmitAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64, note string) (*StaffAbsenceResponse, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, fmt.Errorf("resubmit note is required")
	}
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.StaffID != staffID {
		return nil, fmt.Errorf("can only resubmit own absences")
	}
	if absence.Status != activeModels.AbsenceStatusQuestion {
		return nil, fmt.Errorf("only absences with a question can be resubmitted")
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = activeModels.AbsenceStatusRequested
	absence.Note = note
	// Re-stamp so the request re-enters the inbox at its resubmit time.
	absence.RequestedAt = now
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to resubmit absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, note); err != nil {
		return nil, fmt.Errorf("failed to audit absence resubmit: %w", err)
	}
	// A resubmit re-enters the inbox, so approvers get the "received" mail again.
	s.notifyAbsenceRequested(ctx, absence)
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) CancelAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64) error {
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return err
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return fmt.Errorf("absence not found")
	}
	if absence.StaffID != staffID {
		return fmt.Errorf("can only cancel own absences")
	}
	// MA can cancel requested/questioned or approved future vacation. Past
	// approved absences become historical record and are not cancelable from
	// the UI.
	if absence.Status != activeModels.AbsenceStatusRequested &&
		absence.Status != activeModels.AbsenceStatusQuestion &&
		absence.Status != activeModels.AbsenceStatusApproved {
		return fmt.Errorf("only pending or approved absences can be canceled")
	}
	if absence.Status == activeModels.AbsenceStatusApproved &&
		isBeforeLocalToday(absence.DateStart, time.Now()) {
		return fmt.Errorf("past absences cannot be canceled")
	}
	fromStatus := absence.Status
	absence.Status = activeModels.AbsenceStatusCanceled
	absence.UpdatedAt = time.Now()
	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return fmt.Errorf("failed to cancel absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, ""); err != nil {
		return fmt.Errorf("failed to audit absence cancellation: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *staffAbsenceService) createAudit(ctx context.Context, absenceID int64, actorAccountID int64, fromStatus string, toStatus string, note string) error {
	if s.auditRepo == nil {
		return fmt.Errorf("staff absence audit repository is not configured")
	}
	audit := &activeModels.StaffAbsenceAudit{
		AbsenceID:  absenceID,
		FromStatus: &fromStatus,
		ToStatus:   toStatus,
		ActorID:    actorAccountID,
		Note:       note,
	}
	return s.auditRepo.Create(ctx, audit)
}

func isBeforeLocalToday(date timezone.Date, now time.Time) bool {
	return date.Before(timezone.DateFromTime(now))
}

func (s *staffAbsenceService) GetVacationQuotaSummary(ctx context.Context, staffID int64, year int) (*VacationQuotaSummary, error) {
	quota, err := s.quotaRepo.GetByStaffAndYear(ctx, staffID, year)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quota: %w", err)
	}
	entitled := defaultEntitledDays
	carryover := 0.0
	if quota != nil {
		entitled = quota.EntitledDays
		carryover = quota.CarryoverDays
	}

	yearStart := timezone.NewDate(year, time.January, 1)
	yearEnd := timezone.NewDate(year, time.December, 31)
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, yearStart, yearEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch year absences: %w", err)
	}
	var opening *activeModels.StaffVacationOpening
	if s.openingRepo != nil {
		opening, err = s.openingRepo.GetByStaffAndYear(ctx, staffID, year)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch vacation opening: %w", err)
		}
	}
	return computeVacationQuotaSummary(staffID, year, entitled, carryover, absences, opening), nil
}

// computeVacationQuotaSummary is the pure tail of GetVacationQuotaSummary: the
// two reads happen at the call site, the arithmetic lives here. Extracted so
// the cross-staff overview (#1417) can compute Resturlaub from prefetched rows
// through the SAME code, instead of a second implementation that would drift.
func computeVacationQuotaSummary(
	staffID int64,
	year int,
	entitled, carryover float64,
	absences []*activeModels.StaffAbsence,
	opening *activeModels.StaffVacationOpening,
) *VacationQuotaSummary {
	return computeVacationQuotaSummaryThrough(
		staffID,
		year,
		entitled,
		carryover,
		timezone.NewDate(year, time.December, 31),
		absences,
		opening,
	)
}

// computeVacationQuotaSummaryThrough is the historical counterpart of the
// full-year quota summary. Absence ranges are clipped at through so a past
// month does not spend vacation that lies later in the selected year.
func computeVacationQuotaSummaryThrough(
	staffID int64,
	year int,
	entitled, carryover float64,
	through timezone.Date,
	absences []*activeModels.StaffAbsence,
	opening *activeModels.StaffVacationOpening,
) *VacationQuotaSummary {
	yearStart := timezone.NewDate(year, time.January, 1)
	yearEnd := timezone.NewDate(year, time.December, 31)
	if through.Before(yearEnd) {
		yearEnd = through
	}

	// The takeover (#2132) counts only in its own year and only from its
	// Stichtag on — a historical view that ends before the introduction
	// keeps the pre-moto arithmetic.
	takenBefore := 0.0
	if opening != nil && opening.Year == year && !through.Before(opening.EffectiveDate) {
		takenBefore = opening.TakenBeforeDays
	} else {
		opening = nil
	}

	taken, reserved := 0.0, 0.0
	for _, a := range absences {
		if a.AbsenceType != activeModels.AbsenceTypeVacation {
			continue
		}
		days := vacationDaysInYear(a, yearStart, yearEnd)
		if a.WorkingDays != nil && dateWithinRange(a.DateStart, yearStart, yearEnd) && dateWithinRange(a.DateEnd, yearStart, yearEnd) {
			days = *a.WorkingDays
		}
		switch a.Status {
		case activeModels.AbsenceStatusApproved, activeModels.AbsenceStatusReported:
			taken += days
		case activeModels.AbsenceStatusRequested, activeModels.AbsenceStatusQuestion:
			reserved += days
		}
	}

	return &VacationQuotaSummary{
		StaffID:         staffID,
		Year:            year,
		EntitledDays:    entitled,
		CarryoverDays:   carryover,
		TakenBeforeDays: takenBefore,
		TakenDays:       taken,
		ReservedDays:    reserved,
		RemainingDays:   entitled + carryover - takenBefore - taken - reserved,
		Opening:         opening,
	}
}

func dateWithinRange(d, from, to timezone.Date) bool {
	return !d.Before(from) && !d.After(to)
}

func vacationDaysInYear(a *activeModels.StaffAbsence, yearStart, yearEnd timezone.Date) float64 {
	from := a.DateStart
	if from.Before(yearStart) {
		from = yearStart
	}
	to := a.DateEnd
	if to.After(yearEnd) {
		to = yearEnd
	}
	if to.Before(from) {
		return 0
	}

	startHalf, endHalf := effectiveBoundaryHalfDays(a)
	if from != a.DateStart {
		startHalf = false
	}
	if to != a.DateEnd {
		endHalf = false
	}
	return countWorkingDays(from, to, startHalf, endHalf)
}

func (s *staffAbsenceService) UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error {
	now := time.Now()
	quota := &activeModels.StaffVacationQuota{
		StaffID:       staffID,
		Year:          year,
		EntitledDays:  entitled,
		CarryoverDays: carryover,
	}
	quota.CreatedAt = now
	quota.UpdatedAt = now
	quota.SetTenantID(tenant.FromContext(ctx))
	if err := quota.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrVacationQuotaInvalid, err.Error())
	}
	if err := s.quotaRepo.Upsert(ctx, quota); err != nil {
		return err
	}
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *staffAbsenceService) ListAbsenceRequests(ctx context.Context, req AbsenceRequestListQuery) ([]*StaffAbsenceRequestItem, error) {
	filter := activeModels.AbsenceRequestFilter{
		Types:   req.Types,
		Search:  req.Search,
		Decided: req.History,
	}
	if req.History {
		// Only requests that went through the approval flow have a history.
		// Admin-direct entries (status "reported") were never requested.
		filter.Statuses = []string{
			activeModels.AbsenceStatusApproved,
			activeModels.AbsenceStatusDeclined,
			activeModels.AbsenceStatusCanceled,
		}
	} else {
		filter.Statuses = []string{
			activeModels.AbsenceStatusRequested,
			activeModels.AbsenceStatusQuestion,
		}
	}

	rows, err := s.absenceRepo.ListRequests(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list absence requests: %w", err)
	}
	responses := make([]*StaffAbsenceResponse, len(rows))
	for i, row := range rows {
		responses[i] = toAbsenceResponse(row.StaffAbsence)
	}
	s.withLabels(ctx, responses...)

	items := make([]*StaffAbsenceRequestItem, len(rows))
	for i, row := range rows {
		items[i] = &StaffAbsenceRequestItem{
			StaffAbsenceResponse: responses[i],
			StaffName:            row.StaffName,
			DecidedByName:        row.DecidedByName,
		}
	}
	return items, nil
}

func (s *staffAbsenceService) ListPendingRequests(ctx context.Context) ([]*StaffAbsenceResponse, error) {
	rows, err := s.absenceRepo.ListByStatuses(ctx, []string{
		activeModels.AbsenceStatusRequested,
		activeModels.AbsenceStatusQuestion,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pending requests: %w", err)
	}
	responses := make([]*StaffAbsenceResponse, len(rows))
	for i, r := range rows {
		responses[i] = toAbsenceResponse(r)
	}
	return s.withLabels(ctx, responses...), nil
}
