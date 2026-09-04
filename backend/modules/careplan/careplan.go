// Package careplan is the public Care Plan capability. It owns care offerings,
// offering-change requests, exits, companions, care documents, and the
// reversible removal ledger. Other owners use Query or Command; the Postgres
// tables stay hidden behind the module.
package careplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const DateLayout = "2006-01-02"

// Date is a calendar day in canonical YYYY-MM-DD form.
type Date string

func (d Date) String() string         { return string(d) }
func (d Date) IsZero() bool           { return d == "" }
func (d Date) Before(other Date) bool { return d < other }
func (d Date) After(other Date) bool  { return d > other }
func (d Date) Compare(other Date) int {
	switch {
	case d < other:
		return -1
	case d > other:
		return 1
	default:
		return 0
	}
}

const (
	OfferingOrderCatalog      = "catalog"
	OfferingOrderID           = "id"
	OfferingOrderPhaseCatalog = "phase_catalog"

	ChangeOrderReviewed = "reviewed"
	ChangeOrderCreated  = "created"
	ChangeOrderUpdated  = "updated"

	OfferingChangePending   = "pending"
	OfferingChangeApproved  = "approved"
	OfferingChangeRejected  = "rejected"
	OfferingChangeWithdrawn = "withdrawn"
	OfferingChangeDone      = "done"
	OfferingChangeCareEnded = "care_ended"

	GuardianPermissionPortalAccess     = "parent_portal.access"
	GuardianPermissionEnrollmentsView  = "parent_portal.enrollments.view"
	GuardianPermissionEnrollmentSubmit = "parent_portal.enrollment.submit"

	StudentStatusPending = "pending"
	StudentStatusActive  = "active"
	StudentStatusAlumnus = "alumnus"
)

var (
	ErrCareOfferingNotFound       = errors.New("care offering not found")
	ErrOfferingChangeNotFound     = errors.New("offering change request not found")
	ErrOfferingChangeNotPending   = errors.New("offering change request is not pending")
	ErrOfferingChangeAlreadyOpen  = errors.New("offering change request already pending")
	ErrInvalidCareOffering        = errors.New("invalid care offering")
	ErrInvalidOfferingChange      = errors.New("invalid offering change request")
	ErrCareOfferingTriggerInvalid = errors.New("care offering auto trigger is outside the tenant")
)

type InvalidError struct {
	Kind   error
	Reason string
}

func (e *InvalidError) Error() string { return e.Reason }
func (e *InvalidError) Unwrap() error { return e.Kind }

// CareOffering is the owner view of one enrollment.care_offerings row.
// Calendar dates do not occur on the row. JSON-backed rules remain raw so the
// capability does not leak the enrollment package's model types.
type CareOffering struct {
	ID                        int64
	TenantID                  int64
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	PhaseID                   int64
	ActivityGroupID           *int64
	Name                      string
	Description               *string
	DaysOfWeekMode            string
	AvailableDays             []string
	IncludesHolidayCare       bool
	IncludesLunch             bool
	Capacity                  *int
	PriceCents                *int
	IsActive                  bool
	IsRequired                bool
	CountsAsCare              bool
	AutoAddGradeLevels        []int
	AvailabilityRule          json.RawMessage
	SortOrder                 int
	SelectionGroup            string
	SelectionRule             string
	PickupTimes               map[string]string
	AutoAddTriggerOfferingIDs []int64
}

// CareOfferingFields is the writable catalog state. Trigger IDs are part of
// the command so the offering and its selection links commit atomically.
type CareOfferingFields struct {
	PhaseID                   int64
	ActivityGroupID           *int64
	Name                      string
	Description               *string
	DaysOfWeekMode            string
	AvailableDays             []string
	IncludesHolidayCare       bool
	IncludesLunch             bool
	Capacity                  *int
	PriceCents                *int
	IsActive                  bool
	IsRequired                bool
	CountsAsCare              bool
	AutoAddGradeLevels        []int
	AvailabilityRule          json.RawMessage
	SortOrder                 int
	SelectionGroup            string
	SelectionRule             string
	PickupTimes               map[string]string
	AutoAddTriggerOfferingIDs []int64
}

type CreateCareOffering struct{ CareOfferingFields }

type UpdateCareOffering struct {
	ID int64
	CareOfferingFields
}

// CareOfferingFilter has closed filtering and ordering semantics. Nil IDs
// means no ID filter; an explicitly empty slice matches no rows.
type CareOfferingFilter struct {
	IDs              []int64
	PhaseIDs         []int64
	ActivityGroupIDs []int64
	ActiveOnly       bool
	LockForUpdate    bool
	Order            string
}

// OfferingChangeRequest is one requested effective booking change. Dates use
// DateLayout. Payload and snapshot are deliberately opaque JSON contracts.
type OfferingChangeRequest struct {
	ID                          int64
	TenantID                    int64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
	StudentID                   int64
	RequestChildID              int64
	SubmittedBy                 int64
	CompleteWithdrawalConfirmed bool
	WithdrawalConfirmedBy       *int64
	WithdrawalConfirmedAt       *time.Time
	ApprovedCompleteWithdrawal  bool
	Payload                     json.RawMessage
	EffectiveFrom               string
	ParentNote                  *string
	Status                      string
	DecisionReason              *string
	DecisionSnapshot            json.RawMessage
	ReviewedBy                  *int64
	ReviewedAt                  *time.Time
	AppliedAt                   *time.Time
}

type OfferingChangeFilter struct {
	IDs           []int64
	StudentID     int64
	StudentIDs    []int64
	Statuses      []string
	UrgentOnly    *bool
	UrgentDate    string
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
	LockForUpdate bool
	Order         string
}

type UpdatePendingOfferingChange struct {
	ID            int64
	Payload       json.RawMessage
	EffectiveFrom string
	ParentNote    *string
}

type DecideOfferingChange struct {
	ID         int64
	Status     string
	Reason     *string
	ReviewedBy *int64
	Applied    bool
}

type Query interface {
	CareRecordsQuery
	StudentSchedulesQuery
	FindCareOffering(context.Context, int64) (CareOffering, error)
	ListCareOfferings(context.Context, CareOfferingFilter) ([]CareOffering, error)
	CountCareOfferingsByPhase(context.Context, int64) (int, error)

	FindOfferingChange(context.Context, int64, bool) (OfferingChangeRequest, error)
	ListOfferingChanges(context.Context, OfferingChangeFilter) ([]OfferingChangeRequest, error)
}

type Command interface {
	CareRecordsCommand
	StudentSchedulesCommand
	CreateCareOffering(context.Context, CreateCareOffering) (CareOffering, error)
	UpdateCareOffering(context.Context, UpdateCareOffering) (CareOffering, error)
	DeleteCareOffering(context.Context, int64) error
	ReplaceAutoAddTriggers(context.Context, int64, []int64) error

	CreateOfferingChange(context.Context, OfferingChangeRequest) (OfferingChangeRequest, error)
	UpdateOfferingChangeEffectiveFrom(context.Context, int64, string) error
	UpdateApprovedCompleteWithdrawal(context.Context, int64, bool) error
	UpdatePendingOfferingChange(context.Context, UpdatePendingOfferingChange) error
	DecideOfferingChange(context.Context, DecideOfferingChange) error
	UpdateOfferingChangeSnapshot(context.Context, int64, json.RawMessage) error
	ClosePendingOfferingChanges(context.Context, []int64, string, *int64, time.Time) (int64, error)
}

type Capability interface {
	Query
	Command
}

type engine interface {
	Query
	Command
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("care plan: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) FindCareOffering(ctx context.Context, id int64) (CareOffering, error) {
	if id <= 0 {
		return CareOffering{}, invalid(ErrInvalidCareOffering, "care offering ID is required")
	}
	return m.engine.FindCareOffering(ctx, id)
}

func (m *Module) ListCareOfferings(ctx context.Context, filter CareOfferingFilter) ([]CareOffering, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.PhaseIDs = uniquePositive(filter.PhaseIDs)
	filter.ActivityGroupIDs = uniquePositive(filter.ActivityGroupIDs)
	if filter.Order == "" {
		filter.Order = OfferingOrderCatalog
	}
	switch filter.Order {
	case OfferingOrderCatalog, OfferingOrderID, OfferingOrderPhaseCatalog:
	default:
		return nil, invalid(ErrInvalidCareOffering, "care offering order is invalid")
	}
	return m.engine.ListCareOfferings(ctx, filter)
}

func (m *Module) CountCareOfferingsByPhase(ctx context.Context, phaseID int64) (int, error) {
	if phaseID <= 0 {
		return 0, invalid(ErrInvalidCareOffering, "phase ID is required")
	}
	return m.engine.CountCareOfferingsByPhase(ctx, phaseID)
}

func (m *Module) CreateCareOffering(ctx context.Context, input CreateCareOffering) (CareOffering, error) {
	fields, err := normalizeCareOfferingFields(input.CareOfferingFields)
	if err != nil {
		return CareOffering{}, err
	}
	input.CareOfferingFields = fields
	input.AutoAddTriggerOfferingIDs = normalizedTriggers(0, input.AutoAddTriggerOfferingIDs)
	return m.engine.CreateCareOffering(ctx, input)
}

func (m *Module) UpdateCareOffering(ctx context.Context, input UpdateCareOffering) (CareOffering, error) {
	if input.ID <= 0 {
		return CareOffering{}, invalid(ErrInvalidCareOffering, "care offering ID is required")
	}
	fields, err := normalizeCareOfferingFields(input.CareOfferingFields)
	if err != nil {
		return CareOffering{}, err
	}
	input.CareOfferingFields = fields
	input.AutoAddTriggerOfferingIDs = normalizedTriggers(input.ID, input.AutoAddTriggerOfferingIDs)
	return m.engine.UpdateCareOffering(ctx, input)
}

func (m *Module) DeleteCareOffering(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid(ErrInvalidCareOffering, "care offering ID is required")
	}
	return m.engine.DeleteCareOffering(ctx, id)
}

func (m *Module) ReplaceAutoAddTriggers(ctx context.Context, targetID int64, triggerIDs []int64) error {
	if targetID <= 0 {
		return invalid(ErrInvalidCareOffering, "target care offering ID is required")
	}
	return m.engine.ReplaceAutoAddTriggers(ctx, targetID, normalizedTriggers(targetID, triggerIDs))
}

func (m *Module) FindOfferingChange(ctx context.Context, id int64, lock bool) (OfferingChangeRequest, error) {
	if id <= 0 {
		return OfferingChangeRequest{}, invalid(ErrInvalidOfferingChange, "offering change request ID is required")
	}
	return m.engine.FindOfferingChange(ctx, id, lock)
}

func (m *Module) ListOfferingChanges(ctx context.Context, filter OfferingChangeFilter) ([]OfferingChangeRequest, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StudentIDs = uniquePositive(filter.StudentIDs)
	for _, status := range filter.Statuses {
		if !validOfferingChangeStatus(status) {
			return nil, invalid(ErrInvalidOfferingChange, "offering change status filter is invalid")
		}
	}
	if filter.UrgentOnly != nil {
		if err := validateDate(filter.UrgentDate, "urgent date", ErrInvalidOfferingChange); err != nil {
			return nil, err
		}
	}
	if filter.Limit < 0 {
		return nil, invalid(ErrInvalidOfferingChange, "limit must not be negative")
	}
	if filter.Order == "" {
		filter.Order = ChangeOrderCreated
	}
	switch filter.Order {
	case ChangeOrderReviewed, ChangeOrderCreated, ChangeOrderUpdated:
	default:
		return nil, invalid(ErrInvalidOfferingChange, "offering change order is invalid")
	}
	return m.engine.ListOfferingChanges(ctx, filter)
}

func (m *Module) CreateOfferingChange(ctx context.Context, row OfferingChangeRequest) (OfferingChangeRequest, error) {
	if row.StudentID <= 0 || row.RequestChildID <= 0 || row.SubmittedBy <= 0 {
		return OfferingChangeRequest{}, invalid(ErrInvalidOfferingChange, "student, request child, and submitter are required")
	}
	if row.Status != OfferingChangePending {
		return OfferingChangeRequest{}, invalid(ErrInvalidOfferingChange, "a new offering change request must be pending")
	}
	if !validRequiredJSON(row.Payload) {
		return OfferingChangeRequest{}, invalid(ErrInvalidOfferingChange, "payload must be valid non-null JSON")
	}
	if err := validateDate(row.EffectiveFrom, "effective_from", ErrInvalidOfferingChange); err != nil {
		return OfferingChangeRequest{}, err
	}
	return m.engine.CreateOfferingChange(ctx, row)
}

func (m *Module) UpdateOfferingChangeEffectiveFrom(ctx context.Context, id int64, date string) error {
	if id <= 0 {
		return invalid(ErrInvalidOfferingChange, "offering change request ID is required")
	}
	if err := validateDate(date, "effective_from", ErrInvalidOfferingChange); err != nil {
		return err
	}
	return m.engine.UpdateOfferingChangeEffectiveFrom(ctx, id, date)
}

func (m *Module) UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error {
	if id <= 0 {
		return invalid(ErrInvalidOfferingChange, "offering change request ID is required")
	}
	return m.engine.UpdateApprovedCompleteWithdrawal(ctx, id, complete)
}

func (m *Module) UpdatePendingOfferingChange(ctx context.Context, input UpdatePendingOfferingChange) error {
	if input.ID <= 0 {
		return invalid(ErrInvalidOfferingChange, "offering change request ID is required")
	}
	if err := validateDate(input.EffectiveFrom, "effective_from", ErrInvalidOfferingChange); err != nil {
		return err
	}
	if !validRequiredJSON(input.Payload) {
		return invalid(ErrInvalidOfferingChange, "payload must be valid non-null JSON")
	}
	return m.engine.UpdatePendingOfferingChange(ctx, input)
}

func (m *Module) DecideOfferingChange(ctx context.Context, input DecideOfferingChange) error {
	if input.ID <= 0 || !terminalOfferingChangeStatus(input.Status) {
		return invalid(ErrInvalidOfferingChange, "offering change request ID and terminal status are required")
	}
	return m.engine.DecideOfferingChange(ctx, input)
}

func (m *Module) UpdateOfferingChangeSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	if id <= 0 {
		return invalid(ErrInvalidOfferingChange, "offering change request ID is required")
	}
	if len(snapshot) > 0 && !json.Valid(snapshot) {
		return invalid(ErrInvalidOfferingChange, "decision snapshot must be valid JSON")
	}
	return m.engine.UpdateOfferingChangeSnapshot(ctx, id, snapshot)
}

func (m *Module) ClosePendingOfferingChanges(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, error) {
	studentIDs = uniquePositive(studentIDs)
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if strings.TrimSpace(reason) == "" || at.IsZero() {
		return 0, invalid(ErrInvalidOfferingChange, "care-end reason and timestamp are required")
	}
	return m.engine.ClosePendingOfferingChanges(ctx, studentIDs, reason, reviewedBy, at)
}

func normalizeCareOfferingFields(fields CareOfferingFields) (CareOfferingFields, error) {
	if err := validateCareOfferingScalars(&fields); err != nil {
		return fields, err
	}
	var err error
	if fields.AvailableDays, err = normalizeDays(fields.AvailableDays); err != nil {
		return fields, err
	}
	if fields.AutoAddGradeLevels, err = normalizeGrades(fields.AutoAddGradeLevels); err != nil {
		return fields, err
	}
	if fields.AvailabilityRule, err = normalizeAvailabilityRule(fields.AvailabilityRule); err != nil {
		return fields, err
	}
	if fields.PickupTimes, err = normalizePickupTimes(fields.PickupTimes, fields.AvailableDays); err != nil {
		return fields, err
	}
	return fields, nil
}

func validateCareOfferingScalars(fields *CareOfferingFields) error {
	if fields.PhaseID <= 0 {
		return invalid(ErrInvalidCareOffering, "phase ID is required")
	}
	fields.Name = strings.TrimSpace(fields.Name)
	if fields.Name == "" {
		return invalid(ErrInvalidCareOffering, "care offering name is required")
	}
	if fields.DaysOfWeekMode == "" {
		fields.DaysOfWeekMode = "fixed"
	}
	if fields.DaysOfWeekMode != "fixed" && fields.DaysOfWeekMode != "parent_choice" {
		return invalid(ErrInvalidCareOffering, "days_of_week_mode must be fixed or parent_choice")
	}
	if (fields.Capacity != nil && *fields.Capacity < 0) || (fields.PriceCents != nil && *fields.PriceCents < 0) {
		return invalid(ErrInvalidCareOffering, "capacity and price_cents must be non-negative")
	}
	if fields.IsRequired && fields.Capacity != nil {
		return invalid(ErrInvalidCareOffering, "a required care offering must not have a capacity limit")
	}
	fields.SelectionGroup = strings.TrimSpace(fields.SelectionGroup)
	if fields.SelectionRule == "" {
		fields.SelectionRule = "optional"
	}
	if !slices.Contains([]string{"optional", "exactly_one", "at_least_one", "at_most_one"}, fields.SelectionRule) {
		return invalid(ErrInvalidCareOffering, "selection_rule is invalid")
	}
	if fields.SelectionRule != "optional" && fields.SelectionGroup == "" {
		return invalid(ErrInvalidCareOffering, "a selection rule requires a selection_group name")
	}
	return nil
}

var canonicalDays = map[string]struct{}{"mon": {}, "tue": {}, "wed": {}, "thu": {}, "fri": {}, "sat": {}, "sun": {}}

func normalizeDays(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, invalid(ErrInvalidCareOffering, "available_days must contain at least one day")
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		day := strings.ToLower(strings.TrimSpace(value))
		if _, ok := canonicalDays[day]; !ok {
			return nil, invalid(ErrInvalidCareOffering, fmt.Sprintf("available_days entry %q is invalid", value))
		}
		result = append(result, day)
	}
	return result, nil
}

func normalizeGrades(values []int) ([]int, error) {
	result := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value < 1 || value > 13 {
			return nil, invalid(ErrInvalidCareOffering, fmt.Sprintf("invalid grade %d", value))
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

type availabilityRule struct {
	Match      string                  `json:"match"`
	Conditions []availabilityCondition `json:"conditions"`
}

type availabilityCondition struct {
	Source   string `json:"source"`
	Operator string `json:"operator"`
	Value    []int  `json:"value"`
}

func normalizeAvailabilityRule(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var rule availabilityRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, invalid(ErrInvalidCareOffering, "availability_rule must be valid JSON")
	}
	if len(rule.Conditions) == 0 {
		return nil, nil
	}
	if rule.Match != "all" && rule.Match != "any" {
		return nil, invalid(ErrInvalidCareOffering, "availability_rule.match must be all or any")
	}
	for i := range rule.Conditions {
		condition := &rule.Conditions[i]
		if condition.Source != "grade_level" || condition.Operator != "in" && condition.Operator != "not_in" || len(condition.Value) == 0 {
			return nil, invalid(ErrInvalidCareOffering, fmt.Sprintf("availability_rule condition %d is invalid", i+1))
		}
		grades, err := normalizeGrades(condition.Value)
		if err != nil {
			return nil, err
		}
		slices.Sort(grades)
		condition.Value = grades
	}
	encoded, err := json.Marshal(rule)
	if err != nil {
		return nil, invalid(ErrInvalidCareOffering, "availability_rule cannot be encoded")
	}
	return encoded, nil
}

func normalizePickupTimes(values map[string]string, days []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	available := make(map[string]struct{}, len(days))
	for _, day := range days {
		available[day] = struct{}{}
	}
	result := make(map[string]string, len(values))
	for rawDay, rawValue := range values {
		day, value := strings.ToLower(strings.TrimSpace(rawDay)), strings.TrimSpace(rawValue)
		if value == "" {
			continue
		}
		if _, ok := canonicalDays[day]; !ok || day == "sat" || day == "sun" {
			return nil, invalid(ErrInvalidCareOffering, fmt.Sprintf("pickup_times day %q is invalid", rawDay))
		}
		if _, ok := available[day]; !ok {
			return nil, invalid(ErrInvalidCareOffering, fmt.Sprintf("pickup_times day %q is not available", day))
		}
		parsed, err := time.Parse("15:04", value)
		if err != nil {
			return nil, invalid(ErrInvalidCareOffering, fmt.Sprintf("pickup_times value for %q must be HH:MM", day))
		}
		result[day] = parsed.Format("15:04")
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func validateDate(value, label string, kind error) error {
	if value == "" {
		return invalid(kind, label+" is required")
	}
	if _, err := time.Parse(DateLayout, value); err != nil {
		return invalid(kind, label+" must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

func validRequiredJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{") && json.Valid(raw)
}

func terminalOfferingChangeStatus(status string) bool {
	return status != OfferingChangePending && validOfferingChangeStatus(status)
}

func validOfferingChangeStatus(status string) bool {
	switch status {
	case OfferingChangePending, OfferingChangeApproved, OfferingChangeRejected,
		OfferingChangeWithdrawn, OfferingChangeDone, OfferingChangeCareEnded:
		return true
	default:
		return false
	}
}

func normalizedTriggers(targetID int64, ids []int64) []int64 {
	ids = uniquePositive(ids)
	result := ids[:0]
	for _, id := range ids {
		if id != targetID {
			result = append(result, id)
		}
	}
	return result
}

func uniquePositive(ids []int64) []int64 {
	if ids == nil {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func invalid(kind error, reason string) error {
	return &InvalidError{Kind: kind, Reason: reason}
}

// ErrorCode is the stable low-cardinality label used by runtime evidence.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrCareOfferingNotFound), errors.Is(err, ErrOfferingChangeNotFound), errors.Is(err, ErrCareDocumentNotFound), errors.Is(err, ErrStudentScheduleNotFound):
		return "not_found"
	case errors.Is(err, ErrOfferingChangeNotPending):
		return "not_pending"
	case errors.Is(err, ErrOfferingChangeAlreadyOpen):
		return "already_pending"
	case errors.Is(err, ErrInvalidCareOffering), errors.Is(err, ErrInvalidOfferingChange),
		errors.Is(err, ErrInvalidCareExit), errors.Is(err, ErrInvalidCompanion), errors.Is(err, ErrInvalidCareDocument),
		errors.Is(err, ErrCareExitInvalidReason), errors.Is(err, ErrCareExitNoteRequired),
		errors.Is(err, ErrCareExitNoteNotAllowed), errors.Is(err, ErrCareExitNoteTooLong), errors.Is(err, ErrInvalidStudentSchedule):
		return "invalid"
	case errors.Is(err, ErrCareOfferingTriggerInvalid):
		return "invalid_trigger"
	default:
		return "internal_error"
	}
}

var _ Capability = (*Module)(nil)
