package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// GuardianProfile and GuardianProfilePhone are defined workflow DTOs, not
// aliases to another owner's models.
type GuardianProfile care.GuardianProfile
type GuardianProfilePhone care.GuardianProfilePhone

// WithGuardianProfilePhones retains an already-loaded phone relation,
// including nil versus empty and nil entries.
func WithGuardianProfilePhones(profile *GuardianProfile, phones []*GuardianProfilePhone) *GuardianProfile {
	if profile == nil {
		return nil
	}
	if phones == nil {
		profile.PhoneNumbers = nil
		return profile
	}
	profile.PhoneNumbers = make([]*care.GuardianProfilePhone, len(phones))
	for i, p := range phones {
		profile.PhoneNumbers[i] = (*care.GuardianProfilePhone)(p)
	}
	return profile
}

// GuardianProfileReadCallbacks is the root's binding surface. The factories
// translate only workflow-owned values into the application DTOs.
type GuardianProfileReadCallbacks struct {
	FindByID                      func(context.Context, int64) (*GuardianProfile, error)
	FindByIDs                     func(context.Context, []int64) (map[int64]*GuardianProfile, error)
	FindByEmail                   func(context.Context, string) (*GuardianProfile, error)
	FindByAccountID               func(context.Context, int64) (*GuardianProfile, error)
	FindActivePortalProfilesByIDs func(context.Context, []int64) (map[int64]*GuardianProfile, error)
	LockByIDForUpdate             func(context.Context, int64) error
	Normalize                     func(*GuardianProfile) error
}

// GuardianProfiles is the shared profile read surface of both workflow flows.
type GuardianProfiles interface {
	care.GuardianProfileReads
	FindActivePortalProfilesByIDs(context.Context, []int64) (map[int64]*care.GuardianProfile, error)
}

// NewGuardianProfileReads adapts the root callbacks to both workflow flows.
func NewGuardianProfileReads(c GuardianProfileReadCallbacks) GuardianProfiles {
	return guardianProfileReads{c}
}

type guardianProfileReads struct{ callbacks GuardianProfileReadCallbacks }

func (r guardianProfileReads) FindByID(ctx context.Context, id int64) (*care.GuardianProfile, error) {
	value, err := r.callbacks.FindByID(ctx, id)
	return (*care.GuardianProfile)(value), err
}
func (r guardianProfileReads) FindByIDs(ctx context.Context, ids []int64) (map[int64]*care.GuardianProfile, error) {
	value, err := r.callbacks.FindByIDs(ctx, ids)
	if value == nil {
		return nil, err
	}
	result := make(map[int64]*care.GuardianProfile, len(value))
	for id, row := range value {
		result[id] = (*care.GuardianProfile)(row)
	}
	return result, err
}
func (r guardianProfileReads) FindByEmail(ctx context.Context, email string) (*care.GuardianProfile, error) {
	value, err := r.callbacks.FindByEmail(ctx, email)
	return (*care.GuardianProfile)(value), err
}
func (r guardianProfileReads) FindByAccountID(ctx context.Context, id int64) (*care.GuardianProfile, error) {
	value, err := r.callbacks.FindByAccountID(ctx, id)
	return (*care.GuardianProfile)(value), err
}
func (r guardianProfileReads) FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*care.GuardianProfile, error) {
	value, err := r.callbacks.FindActivePortalProfilesByIDs(ctx, ids)
	if value == nil {
		return nil, err
	}
	result := make(map[int64]*care.GuardianProfile, len(value))
	for id, row := range value {
		result[id] = (*care.GuardianProfile)(row)
	}
	return result, err
}
func (r guardianProfileReads) LockByIDForUpdate(ctx context.Context, id int64) error {
	return r.callbacks.LockByIDForUpdate(ctx, id)
}
func (r guardianProfileReads) NormalizeProfile(p *care.GuardianProfile) error {
	return r.callbacks.Normalize((*GuardianProfile)(p))
}
