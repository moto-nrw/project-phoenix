package careplan

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
)

var (
	ErrExcusedRequestNotFound        = excusedrequests.ErrExcusedRequestNotFound
	ErrExcusedRequestNotPending      = excusedrequests.ErrExcusedRequestNotPending
	ErrExcusedRequestNotDecided      = excusedrequests.ErrExcusedRequestNotDecided
	ErrCareScheduleRequestNotFound   = carerequests.ErrNotFound
	ErrCareScheduleRequestNotPending = carerequests.ErrNotPending
	ErrCareScheduleRequestNotDecided = carerequests.ErrNotDecided
	ErrStudentDataRequestNotFound    = errors.New("student data change request not found")
	ErrStudentDataRequestNotPending  = errors.New("student data change request is not pending")
	// ErrStudentDataRequestFieldPending joins the raw database error when a
	// pending request for the same student, target, and field already exists.
	ErrStudentDataRequestFieldPending = errors.New("a student data change request for this field is already pending")
	ErrStudentDataRequestNotDecided   = errors.New("student data change request is not decided")
)

// RequestQueueFilter is the owner-neutral paging contract shared by the
// immediate-notice and approval-request queues.
type RequestQueueFilter = excusedrequests.QueueFilter

func ParseDate(value string) (Date, error) { return excusedrequests.ParseDate(value) }

// ExcusedAbsenceRequest is one excused-absence request row.
type ExcusedAbsenceRequest = excusedrequests.Request

type ExcusedAbsenceRequestFilter struct {
	// IDs restricts the listing to exact rows. Nil means no ID filter.
	IDs         []int64
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

type CareScheduleChangeRequest = carerequests.Request

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

type StudentDataChangeRequest = masterdatarequests.Request

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

// CareRequestsCommand writes the three parent request kinds and closes the
// open ones of a child together.
type CareRequestsCommand interface {
	ExcusedAbsenceRequestsCommand
	CareScheduleRequestsCommand
	StudentDataRequestsCommand
	LockOpenCareRequests(context.Context, []int64) error
	CloseOpenCareRequests(context.Context, []int64, string, *int64, time.Time) (int64, error)
}

// ExcusedAbsenceRequestsCommand writes excused-absence requests.
type ExcusedAbsenceRequestsCommand interface {
	CreateExcusedAbsenceRequest(context.Context, ExcusedAbsenceRequest) (ExcusedAbsenceRequest, error)
	LockExcusedAbsenceRequests(context.Context, int64) error
	UpdatePendingExcusedAbsenceRequest(context.Context, int64, []Date, string, string) error
	DecideExcusedAbsenceRequest(context.Context, ExcusedAbsenceDecision) error
	RedecideExcusedAbsenceRequest(context.Context, ExcusedAbsenceDecision) error
}

// CareScheduleRequestsCommand writes care schedule change requests.
type CareScheduleRequestsCommand interface {
	CreateCareScheduleRequest(context.Context, CareScheduleChangeRequest) (CareScheduleChangeRequest, error)
	UpdatePendingCareScheduleRequest(context.Context, int64, json.RawMessage) error
	DecideCareScheduleRequest(context.Context, CareScheduleRequestDecision) error
	RedecideCareScheduleRequest(context.Context, CareScheduleRequestDecision) error
	UpdateCareScheduleRequestSnapshot(context.Context, int64, json.RawMessage) error
}

// StudentDataRequestsCommand writes student data change requests.
type StudentDataRequestsCommand interface {
	CreateStudentDataRequest(context.Context, StudentDataChangeRequest) (StudentDataChangeRequest, error)
	UpdatePendingStudentDataRequest(context.Context, int64, json.RawMessage) error
	DecideStudentDataRequest(context.Context, StudentDataRequestDecision) error
	RedecideStudentDataRequest(context.Context, StudentDataRequestDecision) error
}
