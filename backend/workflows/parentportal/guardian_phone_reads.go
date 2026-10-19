package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// GuardianPhoneNumber is defined from the workflow's own native snapshot.
type GuardianPhoneNumber care.GuardianPhoneNumber

type GuardianPhoneReadCallbacks struct {
	ForProfile        func(context.Context, int64) ([]*GuardianPhoneNumber, error)
	ForProfiles       func(context.Context, []int64) (map[int64][]*GuardianPhoneNumber, error)
	Normalize         func(*GuardianPhoneNumber) error
	PhoneTypeOrMobile func(string) string
}

type GuardianPhones interface{ care.GuardianPhoneReads }

// NewGuardianPhoneReads binds root callbacks to the application-owned port.
func NewGuardianPhoneReads(c GuardianPhoneReadCallbacks) GuardianPhones { return guardianPhoneReads{c} }

type guardianPhoneReads struct{ callbacks GuardianPhoneReadCallbacks }

func (r guardianPhoneReads) FindByGuardianID(ctx context.Context, id int64) ([]*care.GuardianPhoneNumber, error) {
	rows, err := r.callbacks.ForProfile(ctx, id)
	return applicationGuardianPhones(rows), err
}
func (r guardianPhoneReads) FindByGuardianIDs(ctx context.Context, ids []int64) (map[int64][]*care.GuardianPhoneNumber, error) {
	rows, err := r.callbacks.ForProfiles(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make(map[int64][]*care.GuardianPhoneNumber, len(rows))
	for id, phones := range rows {
		result[id] = applicationGuardianPhones(phones)
	}
	return result, err
}
func (r guardianPhoneReads) NormalizePhone(p *care.GuardianPhoneNumber) error {
	return r.callbacks.Normalize((*GuardianPhoneNumber)(p))
}
func (r guardianPhoneReads) PhoneTypeOrMobile(input string) string {
	return r.callbacks.PhoneTypeOrMobile(input)
}
func applicationGuardianPhones(rows []*GuardianPhoneNumber) []*care.GuardianPhoneNumber {
	if rows == nil {
		return nil
	}
	result := make([]*care.GuardianPhoneNumber, len(rows))
	for i, row := range rows {
		result[i] = (*care.GuardianPhoneNumber)(row)
	}
	return result
}
