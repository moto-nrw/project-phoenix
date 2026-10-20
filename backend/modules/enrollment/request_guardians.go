package enrollment

import (
	"time"
)

// RequestGuardian is an additional guardian supplied with an application.
type RequestGuardian struct {
	ID                int64     `json:"id"`
	TenantID          int64     `json:"tenant_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	RequestID         int64     `json:"request_id"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	Email             *string   `json:"email,omitempty"`
	Phone             *string   `json:"phone,omitempty"`
	GuardianProfileID *int64    `json:"guardian_profile_id,omitempty"`
	SortOrder         int       `json:"sort_order"`
}
