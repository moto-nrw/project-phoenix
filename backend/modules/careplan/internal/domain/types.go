package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrCareOfferingNotFound       = errors.New("care offering not found")
	ErrOfferingChangeNotFound     = errors.New("offering change request not found")
	ErrOfferingChangeNotPending   = errors.New("offering change request is not pending")
	ErrOfferingChangeAlreadyOpen  = errors.New("offering change request already pending")
	ErrCareOfferingTriggerInvalid = errors.New("care offering auto trigger is outside the tenant")
	ErrCareDocumentNotFound       = errors.New("student care document not found")
)

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

type CareOfferingFilter struct {
	IDs              []int64
	PhaseIDs         []int64
	ActivityGroupIDs []int64
	ActiveOnly       bool
	LockForUpdate    bool
	Order            string
}

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

type OperationStats struct {
	Queries           int64
	Rows              int64
	Conflicts         int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.Conflicts += other.Conflicts
	s.StatementDuration += other.StatementDuration
}
