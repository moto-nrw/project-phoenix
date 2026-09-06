package enrollment

import (
	"context"
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

func (m *Module) CreateRequestGuardian(ctx context.Context, guardian *RequestGuardian) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error { return m.engine.CreateRequestGuardian(txCtx, guardian) })
}

func (m *Module) RequestGuardians(ctx context.Context, requestIDs []int64) ([]*RequestGuardian, error) {
	var result []*RequestGuardian
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = m.engine.RequestGuardians(txCtx, requestIDs)
		return err
	})
	return result, err
}

func (m *Module) DeleteRequestGuardians(ctx context.Context, requestID int64) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error { return m.engine.DeleteRequestGuardians(txCtx, requestID) })
}

func (m *Module) StampRequestGuardianProfile(ctx context.Context, id, profileID int64) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error { return m.engine.StampRequestGuardianProfile(txCtx, id, profileID) })
}
