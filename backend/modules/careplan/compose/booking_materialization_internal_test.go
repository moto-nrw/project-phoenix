package compose

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type recordingPickupStaff struct{ tenants []int64 }

func (s *recordingPickupStaff) PickupScheduleChanged(tenantID int64) error {
	s.tenants = append(s.tenants, tenantID)
	return nil
}

type recordingPickupGuardians struct{ students [][2]int64 }

func (g *recordingPickupGuardians) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	g.students = append(g.students, [2]int64{tenantID, studentID})
}

// A changed offering pickup projection is announced only once the tenant
// transaction committed (#3560): a rolled-back reset or adjustment must not
// wake staff or guardian clients.
func TestPickupAnnouncementWaitsForTheTenantCommit(t *testing.T) {
	t.Parallel()

	staff, guardians := &recordingPickupStaff{}, &recordingPickupGuardians{}
	announce := application.AnnounceOfferingPickupChange(staff, guardians, TenantHooks{}, slog.Default())
	require.NotNil(t, announce)
	ctx, commit := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 7))

	announce(ctx, []int64{42, 43})
	assert.Empty(t, staff.tenants, "nobody may wake before commit")

	commit()
	assert.Equal(t, []int64{7}, staff.tenants)
	assert.Equal(t, [][2]int64{{7, 42}, {7, 43}}, guardians.students)
}

func TestPickupAnnouncementKeepsAMissingAnnouncerMissing(t *testing.T) {
	t.Parallel()

	assert.Nil(t, application.AnnounceOfferingPickupChange(nil, nil, TenantHooks{}, nil))
}
