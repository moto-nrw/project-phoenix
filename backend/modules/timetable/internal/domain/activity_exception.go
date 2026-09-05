package domain

import (
	"errors"
	"time"
)

var ErrActivityExceptionNotFound = errors.New("activity exception not found")

const ActivityExceptionUniqueConstraint = "unique_activity_exception"

type ActivityException struct {
	ID              int64
	TenantID        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ActivityGroupID int64
	ExceptionDate   string
	ExceptionType   string
	StartTime       *string
	EndTime         *string
	RoomID          *int64
	Reason          *string
	CreatedBy       *int64
}

type ActivityExceptionFields struct {
	ActivityGroupID int64
	ExceptionDate   string
	ExceptionType   string
	StartTime       *string
	EndTime         *string
	RoomID          *int64
	Reason          *string
	CreatedBy       *int64
}

type ActivityExceptionFilter struct {
	ActivityGroupID *int64
	ExceptionDate   *string
	FromDate        *string
	ToDate          *string
	BeforeDate      *string
	OrderByDate     bool
	Limit           int
	Offset          int
}
