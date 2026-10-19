package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentGuardianPhoneReads binds the owner public snapshot and canonical
// normalization facet without another store or a changed query.
func NewParentGuardianPhoneReads(source peopledirectory.GuardianPhoneReads) parentportal.GuardianPhones {
	return parentportal.NewGuardianPhoneReads(parentportal.GuardianPhoneReadCallbacks{
		ForProfile: func(ctx context.Context, id int64) ([]*parentportal.GuardianPhoneNumber, error) {
			rows, err := source.GuardianPhonesForProfile(ctx, id)
			return parentGuardianPhones(rows), err
		},
		ForProfiles: func(ctx context.Context, ids []int64) (map[int64][]*parentportal.GuardianPhoneNumber, error) {
			rows, err := source.GuardianPhonesForProfiles(ctx, ids)
			if rows == nil {
				return nil, err
			}
			result := make(map[int64][]*parentportal.GuardianPhoneNumber, len(rows))
			for id, phones := range rows {
				result[id] = parentGuardianPhones(phones)
			}
			return result, err
		},
		Normalize: func(value *parentportal.GuardianPhoneNumber) error {
			row := peopledirectory.GuardianProfilePhoneSnapshot(*value)
			err := source.NormalizeGuardianPhone(&row)
			*value = parentportal.GuardianPhoneNumber(row)
			return err
		},
		PhoneTypeOrMobile: source.GuardianPhoneTypeOrMobile,
	})
}

func parentGuardianPhones(rows []*peopledirectory.GuardianProfilePhoneSnapshot) []*parentportal.GuardianPhoneNumber {
	if rows == nil {
		return nil
	}
	result := make([]*parentportal.GuardianPhoneNumber, len(rows))
	for i, row := range rows {
		if row != nil {
			value := parentportal.GuardianPhoneNumber(*row)
			result[i] = &value
		}
	}
	return result
}
