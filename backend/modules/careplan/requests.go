package careplan

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrExcusedRequestNotFound        = errors.New("excused absence request not found")
	ErrExcusedRequestNotPending      = errors.New("excused absence request is not pending")
	ErrExcusedRequestNotDecided      = errors.New("excused absence request is not decided")
	ErrCareScheduleRequestNotFound   = errors.New("care schedule change request not found")
	ErrCareScheduleRequestNotPending = errors.New("care schedule change request is not pending")
	ErrCareScheduleRequestNotDecided = errors.New("care schedule change request is not decided")
	ErrStudentDataRequestNotFound    = errors.New("student data change request not found")
	ErrStudentDataRequestNotPending  = errors.New("student data change request is not pending")
	ErrStudentDataRequestNotDecided  = errors.New("student data change request is not decided")
)

// RequestQueueFilter is the owner-neutral paging contract shared by the
// immediate-notice and approval-request queues.
type RequestQueueFilter struct {
	UrgentOnly    *bool
	UrgentDate    string
	StudentIDs    []int64
	StudentID     int64
	Search        string
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
}

type ExcusedAbsenceRequest struct {
	ID             int64
	TenantID       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StudentID      int64
	SubmittedBy    int64
	Dates          []Date
	Note           string
	AbsenceStatus  string
	Status         string
	DecisionReason *string
	ReviewedBy     *int64
	ReviewedAt     *time.Time
	AppliedAt      *time.Time
}

type ExcusedAbsenceRequestFilter struct {
	StudentID   int64
	Statuses    []string
	RecentSince time.Time
	Queue       *RequestQueueFilter
	Options     *StudentScheduleQueryOptions
}

type ExcusedAbsenceDecision struct {
	ID         int64
	Status     string
	Reason     *string
	ReviewedBy *int64
	Applied    bool
}

type CareScheduleChangeRequest struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StudentID        int64
	SubmittedBy      int64
	RequestKind      string
	Payload          json.RawMessage
	Status           string
	DecisionReason   *string
	ReviewedBy       *int64
	ReviewedAt       *time.Time
	AppliedAt        *time.Time
	DecisionSnapshot json.RawMessage
}

type CareScheduleRequestFilter struct {
	StudentID    int64
	RequestKinds []string
	Statuses     []string
	RecentSince  time.Time
	Queue        *RequestQueueFilter
}

type CareScheduleRequestDecision struct {
	ID         int64
	Status     string
	Reason     *string
	ReviewedBy *int64
	Applied    bool
}

type StudentDataChangeRequest struct {
	ID           int64
	TenantID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	StudentID    int64
	SubmittedBy  int64
	Target       string
	TargetRefID  *int64
	FieldKey     string
	OldValue     json.RawMessage
	NewValue     json.RawMessage
	Status       string
	ReviewReason *string
	ReviewedBy   *int64
	ReviewedAt   *time.Time
	AppliedAt    *time.Time
}

type StudentDataRequestFilter struct {
	StudentID     int64
	Statuses      []string
	ParentVisible bool
	Limit         int
	Queue         *RequestQueueFilter
}

type StudentDataRequestDecision struct {
	ID         int64
	Status     string
	Reason     *string
	ReviewedBy int64
	Applied    bool
}

type CareRequestsQuery interface {
	FindExcusedAbsenceRequest(context.Context, int64, bool) (ExcusedAbsenceRequest, error)
	FindPendingExcusedAbsenceRequest(context.Context, int64) (ExcusedAbsenceRequest, error)
	ListExcusedAbsenceRequests(context.Context, ExcusedAbsenceRequestFilter) ([]ExcusedAbsenceRequest, error)

	FindCareScheduleRequest(context.Context, int64, bool) (CareScheduleChangeRequest, error)
	FindPendingCareScheduleRequest(context.Context, int64) (CareScheduleChangeRequest, error)
	ListCareScheduleRequests(context.Context, CareScheduleRequestFilter) ([]CareScheduleChangeRequest, error)

	FindStudentDataRequest(context.Context, int64, bool) (StudentDataChangeRequest, error)
	FindPendingStudentDataRequest(context.Context, int64) (StudentDataChangeRequest, error)
	ListStudentDataRequests(context.Context, StudentDataRequestFilter) ([]StudentDataChangeRequest, error)
	HasPendingStudentDataRequest(context.Context, int64, string, string) (bool, error)
	CountOpenCareRequests(context.Context, []int64) (map[int64]int, error)
}

type CareRequestsCommand interface {
	CreateExcusedAbsenceRequest(context.Context, ExcusedAbsenceRequest) (ExcusedAbsenceRequest, error)
	LockExcusedAbsenceRequests(context.Context, int64) error
	UpdatePendingExcusedAbsenceRequest(context.Context, int64, []Date, string, string) error
	DecideExcusedAbsenceRequest(context.Context, ExcusedAbsenceDecision) error
	RedecideExcusedAbsenceRequest(context.Context, ExcusedAbsenceDecision) error

	CreateCareScheduleRequest(context.Context, CareScheduleChangeRequest) (CareScheduleChangeRequest, error)
	UpdatePendingCareScheduleRequest(context.Context, int64, json.RawMessage) error
	DecideCareScheduleRequest(context.Context, CareScheduleRequestDecision) error
	RedecideCareScheduleRequest(context.Context, CareScheduleRequestDecision) error
	UpdateCareScheduleRequestSnapshot(context.Context, int64, json.RawMessage) error

	CreateStudentDataRequest(context.Context, StudentDataChangeRequest) (StudentDataChangeRequest, error)
	UpdatePendingStudentDataRequest(context.Context, int64, json.RawMessage) error
	DecideStudentDataRequest(context.Context, StudentDataRequestDecision) error
	RedecideStudentDataRequest(context.Context, StudentDataRequestDecision) error
	LockOpenCareRequests(context.Context, []int64) error
	CloseOpenCareRequests(context.Context, []int64, string, *int64, time.Time) (int64, error)
}

func (m *Module) FindExcusedAbsenceRequest(ctx context.Context, id int64, lock bool) (ExcusedAbsenceRequest, error) {
	return m.engine.FindExcusedAbsenceRequest(ctx, id, lock)
}
func (m *Module) FindPendingExcusedAbsenceRequest(ctx context.Context, id int64) (ExcusedAbsenceRequest, error) {
	return m.engine.FindPendingExcusedAbsenceRequest(ctx, id)
}
func (m *Module) ListExcusedAbsenceRequests(ctx context.Context, filter ExcusedAbsenceRequestFilter) ([]ExcusedAbsenceRequest, error) {
	filter.Statuses = uniqueStrings(filter.Statuses)
	return m.engine.ListExcusedAbsenceRequests(ctx, filter)
}
func (m *Module) CreateExcusedAbsenceRequest(ctx context.Context, value ExcusedAbsenceRequest) (ExcusedAbsenceRequest, error) {
	return m.engine.CreateExcusedAbsenceRequest(ctx, value)
}
func (m *Module) LockExcusedAbsenceRequests(ctx context.Context, studentID int64) error {
	return m.engine.LockExcusedAbsenceRequests(ctx, studentID)
}
func (m *Module) UpdatePendingExcusedAbsenceRequest(ctx context.Context, id int64, dates []Date, note, status string) error {
	return m.engine.UpdatePendingExcusedAbsenceRequest(ctx, id, dates, note, status)
}
func (m *Module) DecideExcusedAbsenceRequest(ctx context.Context, value ExcusedAbsenceDecision) error {
	return m.engine.DecideExcusedAbsenceRequest(ctx, value)
}
func (m *Module) RedecideExcusedAbsenceRequest(ctx context.Context, value ExcusedAbsenceDecision) error {
	return m.engine.RedecideExcusedAbsenceRequest(ctx, value)
}

func (m *Module) FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (CareScheduleChangeRequest, error) {
	return m.engine.FindCareScheduleRequest(ctx, id, lock)
}
func (m *Module) FindPendingCareScheduleRequest(ctx context.Context, id int64) (CareScheduleChangeRequest, error) {
	return m.engine.FindPendingCareScheduleRequest(ctx, id)
}
func (m *Module) ListCareScheduleRequests(ctx context.Context, filter CareScheduleRequestFilter) ([]CareScheduleChangeRequest, error) {
	filter.RequestKinds = uniqueStrings(filter.RequestKinds)
	filter.Statuses = uniqueStrings(filter.Statuses)
	return m.engine.ListCareScheduleRequests(ctx, filter)
}
func (m *Module) CreateCareScheduleRequest(ctx context.Context, value CareScheduleChangeRequest) (CareScheduleChangeRequest, error) {
	return m.engine.CreateCareScheduleRequest(ctx, value)
}
func (m *Module) UpdatePendingCareScheduleRequest(ctx context.Context, id int64, payload json.RawMessage) error {
	return m.engine.UpdatePendingCareScheduleRequest(ctx, id, payload)
}
func (m *Module) DecideCareScheduleRequest(ctx context.Context, value CareScheduleRequestDecision) error {
	return m.engine.DecideCareScheduleRequest(ctx, value)
}
func (m *Module) RedecideCareScheduleRequest(ctx context.Context, value CareScheduleRequestDecision) error {
	return m.engine.RedecideCareScheduleRequest(ctx, value)
}
func (m *Module) UpdateCareScheduleRequestSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	return m.engine.UpdateCareScheduleRequestSnapshot(ctx, id, snapshot)
}

func (m *Module) FindStudentDataRequest(ctx context.Context, id int64, lock bool) (StudentDataChangeRequest, error) {
	return m.engine.FindStudentDataRequest(ctx, id, lock)
}
func (m *Module) FindPendingStudentDataRequest(ctx context.Context, id int64) (StudentDataChangeRequest, error) {
	return m.engine.FindPendingStudentDataRequest(ctx, id)
}
func (m *Module) ListStudentDataRequests(ctx context.Context, filter StudentDataRequestFilter) ([]StudentDataChangeRequest, error) {
	filter.Statuses = uniqueStrings(filter.Statuses)
	return m.engine.ListStudentDataRequests(ctx, filter)
}
func (m *Module) HasPendingStudentDataRequest(ctx context.Context, studentID int64, target, field string) (bool, error) {
	return m.engine.HasPendingStudentDataRequest(ctx, studentID, target, field)
}
func (m *Module) CreateStudentDataRequest(ctx context.Context, value StudentDataChangeRequest) (StudentDataChangeRequest, error) {
	return m.engine.CreateStudentDataRequest(ctx, value)
}
func (m *Module) UpdatePendingStudentDataRequest(ctx context.Context, id int64, value json.RawMessage) error {
	return m.engine.UpdatePendingStudentDataRequest(ctx, id, value)
}
func (m *Module) DecideStudentDataRequest(ctx context.Context, value StudentDataRequestDecision) error {
	return m.engine.DecideStudentDataRequest(ctx, value)
}
func (m *Module) RedecideStudentDataRequest(ctx context.Context, value StudentDataRequestDecision) error {
	return m.engine.RedecideStudentDataRequest(ctx, value)
}

func (m *Module) CountOpenCareRequests(ctx context.Context, studentIDs []int64) (map[int64]int, error) {
	ids := uniquePositive(studentIDs)
	if len(ids) == 0 {
		return map[int64]int{}, nil
	}
	return m.engine.CountOpenCareRequests(ctx, ids)
}

func (m *Module) LockOpenCareRequests(ctx context.Context, studentIDs []int64) error {
	ids := uniquePositive(studentIDs)
	if len(ids) == 0 {
		return nil
	}
	return m.engine.LockOpenCareRequests(ctx, ids)
}

func (m *Module) CloseOpenCareRequests(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, error) {
	ids := uniquePositive(studentIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	return m.engine.CloseOpenCareRequests(ctx, ids, reason, reviewedBy, at)
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
