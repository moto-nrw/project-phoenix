package domain

import "time"

type ClassArrivalPlan struct {
	ID           int64
	TenantID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SchoolClass  string
	ArrivalTimes map[string]string
	UpdatedBy    *int64
}

type ClassArrivalException struct {
	ID          int64
	TenantID    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SchoolClass string
	Date        string
	ArrivalTime time.Time
	Reason      *string
	CreatedBy   *int64
	Origin      string
}
