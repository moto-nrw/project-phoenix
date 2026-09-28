package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withChildQuota(changes organizationtenancy.SchoolChanges, quota *organizationtenancy.ChildQuota) organizationtenancy.SchoolChanges {
	changes.ChildQuota = &organizationtenancy.ChildQuotaChange{Quota: quota}
	return changes
}

// TestProvisioningUpdateSchoolChildQuota pins how the operator maintains the
// Kinderkontingent (#3567) through the existing school update: set, change
// and remove it, each audited with old and new value.
func TestProvisioningUpdateSchoolChildQuota(t *testing.T) {
	t.Parallel()

	t.Run("sets it and audits old and new", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.addSchool(10, 1, "Burbach", "burbach")

		updated, err := h.svc.UpdateSchool(context.Background(), 10,
			withChildQuota(schoolChanges(1), &organizationtenancy.ChildQuota{Bundles: 2, BundleSize: 50}), testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, updated.ChildQuota())
		assert.Equal(t, 100, updated.ChildQuota().Limit())
		entry := h.onlyAudit(t, domain.AuditActionUpdate, domain.AuditResourceSchool, 10)
		assert.Equal(t, map[string]any{
			"child_quota": map[string]any{
				"old": nil,
				"new": map[string]any{"bundles": float64(2), "bundle_size": float64(50), "limit": float64(100)},
			},
		}, auditChanges(t, entry))
	})

	t.Run("lowers it below any count and removes it", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		school := h.addSchool(10, 1, "Burbach", "burbach")
		bundles := 3
		school.ChildQuotaBundles, school.ChildQuotaBundleSize = &bundles, 40

		updated, err := h.svc.UpdateSchool(context.Background(), 10,
			withChildQuota(schoolChanges(1), &organizationtenancy.ChildQuota{Bundles: 1, BundleSize: 40}), testOperatorID, operatorIP)
		require.NoError(t, err)
		assert.Equal(t, 40, updated.ChildQuota().Limit())

		updated, err = h.svc.UpdateSchool(context.Background(), 10, withChildQuota(schoolChanges(1), nil), testOperatorID, operatorIP)
		require.NoError(t, err)
		assert.Nil(t, updated.ChildQuota(), "a removed Kinderkontingent means no limit")
		require.Len(t, h.audit.entries, 2)
		assert.Equal(t, map[string]any{
			"child_quota": map[string]any{
				"old": map[string]any{"bundles": float64(1), "bundle_size": float64(40), "limit": float64(40)},
				"new": nil,
			},
		}, auditChanges(t, h.audit.entries[1]))
	})

	t.Run("an update that does not mention it keeps it", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		school := h.addSchool(10, 1, "Burbach", "burbach")
		bundles := 2
		school.ChildQuotaBundles, school.ChildQuotaBundleSize = &bundles, 50

		updated, err := h.svc.UpdateSchool(context.Background(), 10, schoolChanges(1), testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, updated.ChildQuota())
		assert.Equal(t, 100, updated.ChildQuota().Limit())
		assert.False(t, h.engine.called("SetSchoolChildQuota"))
	})

	t.Run("rejects a quota outside the contract model", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.addSchool(10, 1, "Burbach", "burbach")

		for _, quota := range []organizationtenancy.ChildQuota{{Bundles: 0, BundleSize: 50}, {Bundles: 1, BundleSize: 0}} {
			_, err := h.svc.UpdateSchool(context.Background(), 10, withChildQuota(schoolChanges(1), &quota), testOperatorID, operatorIP)
			var invalid *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &invalid)
		}
		assert.False(t, h.engine.called("SetSchoolChildQuota"))
		assert.Empty(t, h.audit.entries)
	})

	t.Run("a failed audit fails the change so the transaction rolls back", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.addSchool(10, 1, "Burbach", "burbach")
		h.audit.err = errBoom

		_, err := h.svc.UpdateSchool(context.Background(), 10,
			withChildQuota(schoolChanges(1), &organizationtenancy.ChildQuota{Bundles: 2, BundleSize: 50}), testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		require.ErrorIs(t, h.tx.lastErr, errBoom, "the administrative transaction sees the failure and does not commit")
	})
}
