package timetracking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrManagerControlledAbsence marks absence mutations that staff may view but
// only a time-tracking manager may create, change, or delete.
var ErrManagerControlledAbsence = errors.New("absence type is manager-controlled")

// ErrAllowanceBookingOverlap keeps the entry date of every allowance booking
// intact. A merge would make newly added days look as though they were entered
// with the older booking, which could wrongly make expired carryover usable.
var ErrAllowanceBookingOverlap error = allowanceBookingOverlapError{}

type allowanceBookingOverlapError struct{}

func (allowanceBookingOverlapError) Error() string {
	return "Diese Buchung überschneidet sich. Bitte löschen Sie die alte Buchung. Tragen Sie alle Tage zusammen ein."
}

// ErrVacationQuotaInvalid marks invalid quota input supplied by a caller.
var ErrVacationQuotaInvalid = errors.New("invalid vacation quota")

// ErrVacationQuotaExceeded rejects vacation beyond the Resturlaub (#3256).
// Only the Stundenkonto may go negative; the vacation account may not.
var ErrVacationQuotaExceeded = errors.New("vacation quota exceeded")

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
	*StaffAbsence
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
	From   *calendar.Date
	To     *calendar.Date
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
	Opening *StaffVacationOpening `json:"opening,omitempty"`
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
	// RebookAbsences changes the type of stored absences with a reason
	// (#3258); DryRun only reports the effects.
	RebookAbsences(ctx context.Context, req RebookAbsencesRequest) (*AbsenceRebookingResult, error)
	// PreviewCompTimeBalance projects the Stundenkonto effect of a planned
	// Freizeitausgleich before the Leitung confirms it (#2873) — informative
	// only, an overdraft no longer blocks the create.
	PreviewCompTimeBalance(ctx context.Context, staffID int64, start, end calendar.Date, halfDay bool) (*CompTimeBalancePreview, error)
	GetAbsencesForRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*StaffAbsenceResponse, error)
	ListAbsences(ctx context.Context, staffID int64, filter StaffAbsenceListFilter) ([]*StaffAbsenceResponse, error)
	HasAbsenceOnDate(ctx context.Context, staffID int64, date calendar.Date) (bool, *StaffAbsence, error)

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
	// SetVacationQuota is the Leitung's reasoned change (#3256): like
	// UpsertVacationQuota, plus a row in the reason trail.
	SetVacationQuota(ctx context.Context, change VacationQuotaChange) error
	// Vacation takeover at the moto introduction (#2132): one row per staff
	// and year; corrections are delete + re-create (deletion tombstone).
	GetVacationOpening(ctx context.Context, staffID int64, year int) (*StaffVacationOpening, error)
	SetVacationOpening(ctx context.Context, staffID, decidedBy int64, req SetVacationOpeningRequest) (*StaffVacationOpening, error)
	DeleteVacationOpening(ctx context.Context, staffID, deletedBy int64, year int) error
	// ValidateVacationOpeningAbsencesBefore is the read-only half of the
	// takeover guard, used by the bulk import's dry-run preview. Part of the
	// contract because the import lives in another package.
	ValidateVacationOpeningAbsencesBefore(ctx context.Context, staffID int64, effectiveDate calendar.Date) error
	ListPendingRequests(ctx context.Context) ([]*StaffAbsenceResponse, error)
	// ListAbsenceRequests serves the Mitarbeitende-Reiter of the Anfragen
	// module (#2433): the open work list or the decided history, both with
	// the names the list shows, filtered by absence type and staff name.
	ListAbsenceRequests(ctx context.Context, req AbsenceRequestListQuery) ([]*StaffAbsenceRequestItem, error)
}

func (s *staffAbsenceService) GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error) {
	return s.absenceRepo.GetAbsenceMapForDate(ctx, calendar.TodayDate())
}

func (s *staffAbsenceService) GetTodayAbsenceLabelMap(ctx context.Context) (map[int64]string, error) {
	if s.absenceTypes == nil {
		return map[int64]string{}, nil
	}
	typeIDs, err := s.absenceRepo.GetAbsenceTypeIDMapForDate(ctx, calendar.TodayDate())
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
	runInTx         func(context.Context, func(context.Context) error) error
	absenceRepo     StaffAbsenceRepository
	workSessionRepo WorkSessionRepository
	quotaRepo       StaffVacationQuotaRepository
	// openingRepo carries the vacation takeover rows (#2132); supplied by
	// WithVacationOpenings, nil in bare unit fixtures.
	openingRepo StaffVacationOpeningRepository
	auditRepo   StaffAbsenceAuditRepository
	settings    monthSettingsResolver
	// monthService provides the daily-target and closing-balance math for
	// the comp_time overdraft guard; nil in bare-constructed unit tests.
	monthService    WorkTimeMonthService
	shiftPlanSyncer ShiftPlanSyncer
	// emailDeps is nil unless WithAbsenceEmail bound it (factory only);
	// nil means no absence emails are sent (#1419 4d).
	emailDeps   *AbsenceEmailDeps
	broadcaster EventPublisher
	// deletionRepo writes append-only tombstones for deleted absences
	// (#1417): the absence's own audit trail is ON DELETE CASCADE, so
	// without the tombstone a delete erases its whole history. Supplied by
	// WithAbsenceDeletionAudit; nil makes deletes
	// fail.
	deletionRepo TimeTrackingDeletionAudit
	// absenceTypes resolves school-defined Abwesenheitsarten (#2403). Supplied
	// by WithAbsenceTypes; nil in bare-constructed
	// unit fixtures, where every absence is a plain standard type.
	absenceTypes AbsenceTypeReader
	// snapshotRepo reads closed months (#1417); a rebooking inside one is
	// blocked (#3258). Supplied by WithAbsenceMonthSnapshots; nil means no
	// month counts as closed.
	snapshotRepo adjustmentFreezeReader
	// logger is supplied by the factory (WithAbsenceLogger); nil falls back to
	// slog.Default() via getLogger, like the other active services.
	logger    *slog.Logger
	todayFunc func() calendar.Date
}

func (s *staffAbsenceService) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

func (s *staffAbsenceService) today() calendar.Date {
	if s.todayFunc != nil {
		return s.todayFunc()
	}
	return calendar.TodayDate()
}

// resolveAbsenceTypeSelection validates a requested Abwesenheitsart and returns
// the canonical absence type the row must carry. A nil/zero ID leaves the
// caller's own type in place.
//
// The base type is taken from the art, never from the client: that is the whole
// point of the split — a school can name a day "Regenerationstag", but it stays
// arithmetically the type the art was created as (v1: always "Sonstige").
func (s *staffAbsenceService) resolveAbsenceTypeSelection(ctx context.Context, typeID *int64, fallbackType string) (*StaffAbsenceType, *int64, string, error) {
	if typeID == nil || *typeID <= 0 {
		return nil, nil, fallbackType, nil
	}
	if s.absenceTypes == nil {
		return nil, nil, "", ErrAbsenceTypeNotFound
	}
	resolved, err := s.absenceTypes.ResolveForAbsence(ctx, *typeID)
	if err != nil {
		return nil, nil, "", err
	}
	id := resolved.ID
	return resolved, &id, resolved.BaseType, nil
}

// withLabels stamps the school's own wording onto the given responses and
// returns them unchanged otherwise, so read paths can wrap their return value
// in one call.
func (s *staffAbsenceService) withLabels(ctx context.Context, responses ...*StaffAbsenceResponse) []*StaffAbsenceResponse {
	absences := make([]*StaffAbsence, 0, len(responses))
	for _, r := range responses {
		if r != nil && r.StaffAbsence != nil {
			absences = append(absences, r.StaffAbsence)
		}
	}
	StampAbsenceTypeLabels(ctx, s.absenceTypes, absences, s.getLogger())
	return responses
}

// withLabel is the single-response form of withLabels.
func (s *staffAbsenceService) withLabel(ctx context.Context, response *StaffAbsenceResponse) *StaffAbsenceResponse {
	s.withLabels(ctx, response)
	return response
}

func (s *staffAbsenceService) broadcastTimeTrackingChanged(ctx context.Context) {
	queueStaffTimeTrackingChanged(ctx, s.broadcaster, nil)
}

// NewStaffAbsenceService creates a new staff absence service
func NewStaffAbsenceService(
	absenceRepo StaffAbsenceRepository,
	workSessionRepo WorkSessionRepository,
	quotaRepo StaffVacationQuotaRepository,
	auditRepo StaffAbsenceAuditRepository,
	settings monthSettingsResolver,
	monthService WorkTimeMonthService,
	opts ...StaffAbsenceOption,
) StaffAbsenceService {
	service := &staffAbsenceService{
		absenceRepo:     absenceRepo,
		workSessionRepo: workSessionRepo,
		quotaRepo:       quotaRepo,
		auditRepo:       auditRepo,
		settings:        settings,
		monthService:    monthService,
	}
	for _, opt := range opts {
		opt(service)
	}
	if service.shiftPlanSyncer == nil {
		panic("staff absence service: the sick cascade is required")
	}
	if service.runInTx != nil {
		return &absenceCommands{service}
	}
	return service
}

// defaultEntitledDays is the fallback when no per-staff quota row exists.
// 30 working days = TVöD-Allgemein standard. Pro-Tenant override via settings
// will come in a follow-up, see Issue #1375 Tranche 4a.
const defaultEntitledDays = 30.0

type absenceCommands struct{ *staffAbsenceService }
