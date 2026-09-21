package staffgroups_test

import (
	"context"
	"testing"

	membershipcompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforcecompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// staffEmployment binds School Membership to the real Workforce employment
// owner (#2753), as the composition root does.
func staffEmployment(t *testing.T, db *bun.DB) membershipcompose.StaffEmployment {
	t.Helper()
	owner, err := workforcecompose.NewStaffEmployment(db)
	require.NoError(t, err)
	return employmentBinding{owner: owner}
}

type employmentBinding struct{ owner workforce.StaffEmployments }

func (b employmentBinding) StaffEmployments(ctx context.Context, ids []int64) (map[int64]membershipcompose.StaffEmploymentProfile, error) {
	values, err := b.owner.StaffEmployments(ctx, ids)
	result := make(map[int64]membershipcompose.StaffEmploymentProfile, len(values))
	for id, value := range values {
		result[id] = membershipcompose.StaffEmploymentProfile(value)
	}
	return result, err
}

func (b employmentBinding) SaveStaffEmployment(ctx context.Context, value membershipcompose.StaffEmploymentProfile) error {
	return b.owner.SaveStaffEmployment(ctx, workforce.StaffEmployment(value))
}

func (b employmentBinding) ClearStaffWorkTimeModel(ctx context.Context, id int64) error {
	return b.owner.ClearStaffWorkTimeModel(ctx, id)
}
