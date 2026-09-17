package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvisioningRequiresEveryDependency(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	complete := ProvisioningDependencies{
		Organizations: organizationtenancy.NewModule(h.engine), Transaction: h.tx, Dashboard: h.dashboard,
		Identity: h.identity, Devices: h.devices, People: h.people, Presence: h.presence,
		Categories: h.categories, Settings: h.settings, Audit: h.audit, Secrets: h.secrets,
	}
	svc, err := NewProvisioning(complete)
	require.NoError(t, err, "a nil logger falls back to the default")
	require.NotNil(t, svc)

	for name, drop := range map[string]func(*ProvisioningDependencies){
		"organizations": func(d *ProvisioningDependencies) { d.Organizations = nil },
		"transaction":   func(d *ProvisioningDependencies) { d.Transaction = nil },
		"dashboard":     func(d *ProvisioningDependencies) { d.Dashboard = nil },
		"identity":      func(d *ProvisioningDependencies) { d.Identity = nil },
		"devices":       func(d *ProvisioningDependencies) { d.Devices = nil },
		"people":        func(d *ProvisioningDependencies) { d.People = nil },
		"presence":      func(d *ProvisioningDependencies) { d.Presence = nil },
		"categories":    func(d *ProvisioningDependencies) { d.Categories = nil },
		"settings":      func(d *ProvisioningDependencies) { d.Settings = nil },
		"audit":         func(d *ProvisioningDependencies) { d.Audit = nil },
		"secrets":       func(d *ProvisioningDependencies) { d.Secrets = nil },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			deps := complete
			drop(&deps)
			svc, err := NewProvisioning(deps)
			require.Error(t, err)
			assert.Nil(t, svc)
		})
	}
}

func TestProvisioningCreateOrganization(t *testing.T) {
	t.Parallel()

	t.Run("creates the normalised organisation and records the action", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		created, err := h.svc.CreateOrganization(context.Background(), &organizationtenancy.CreateOrganization{
			Name: "  Talent OGS ", Slug: " Talent-OGS ", Active: true,
		}, testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, "Talent OGS", created.Name)
		assert.Equal(t, "talent-ogs", created.Slug)
		assert.True(t, created.Active)
		entry := h.onlyAudit(t, domain.AuditActionCreate, domain.AuditResourceOrganization, created.ID)
		assert.Equal(t, map[string]any{"name": "Talent OGS", "slug": "talent-ogs"}, auditChanges(t, entry))
		assert.Equal(t, 1, h.tx.adminRuns)
	})

	t.Run("nil input is invalid data", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		created, err := h.svc.CreateOrganization(context.Background(), nil, testOperatorID, operatorIP)

		assert.Nil(t, created)
		var invalid *organizationtenancy.InvalidProvisioningDataError
		require.ErrorAs(t, err, &invalid)
		assert.Zero(t, h.tx.adminRuns)
	})

	t.Run("owner validation is invalid data", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		created, err := h.svc.CreateOrganization(context.Background(), &organizationtenancy.CreateOrganization{
			Name: "", Slug: "valid",
		}, testOperatorID, operatorIP)

		assert.Nil(t, created)
		var invalid *organizationtenancy.InvalidProvisioningDataError
		require.ErrorAs(t, err, &invalid)
		require.ErrorIs(t, err, organizationtenancy.ErrInvalidOrganization)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("duplicate slug is a conflict", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Existing", "talent-ogs", false)

		created, err := h.svc.CreateOrganization(context.Background(), &organizationtenancy.CreateOrganization{
			Name: "Talent", Slug: "talent-ogs",
		}, testOperatorID, operatorIP)

		assert.Nil(t, created)
		var conflict *organizationtenancy.ProvisioningConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Contains(t, conflict.Error(), "organization slug already exists")
		assert.Empty(t, h.audit.entries)
	})

	t.Run("unknown owner error passes through", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.engine.fail["Create"] = errBoom

		created, err := h.svc.CreateOrganization(context.Background(), &organizationtenancy.CreateOrganization{
			Name: "Talent", Slug: "talent",
		}, testOperatorID, operatorIP)

		assert.Nil(t, created)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("audit failure fails the transaction", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.audit.err = errBoom

		created, err := h.svc.CreateOrganization(context.Background(), &organizationtenancy.CreateOrganization{
			Name: "Talent", Slug: "talent",
		}, testOperatorID, operatorIP)

		assert.Nil(t, created)
		require.ErrorIs(t, err, errBoom)
		require.ErrorIs(t, h.tx.lastErr, errBoom, "the error must leave the transaction callback so it rolls back")
	})
}

func TestProvisioningListOrganizations(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.addOrg(1, "Alpha", "alpha", false)
	h.addOrg(2, "Beta", "beta", true)

	organizations, err := h.svc.ListOrganizations(context.Background())

	require.NoError(t, err)
	require.Len(t, organizations, 2)
	assert.Equal(t, "alpha", organizations[0].Slug)
	assert.True(t, organizations[1].IsDeleted())

	h.engine.fail["List"] = errBoom
	_, err = h.svc.ListOrganizations(context.Background())
	require.ErrorIs(t, err, errBoom)
}

func TestProvisioningUpdateOrganization(t *testing.T) {
	t.Parallel()

	t.Run("records every changed field", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Old Name", "old-slug", false)

		updated, err := h.svc.UpdateOrganization(context.Background(), 1, organizationtenancy.OrganizationChanges{
			Name: "New Name", Slug: "NEW-SLUG", Active: false,
		}, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, "new-slug", updated.Slug)
		assert.Equal(t, "New Name", updated.Name)
		assert.False(t, updated.Active)
		entry := h.onlyAudit(t, domain.AuditActionUpdate, domain.AuditResourceOrganization, 1)
		assert.Equal(t, map[string]any{
			"slug":   map[string]any{"old": "old-slug", "new": "new-slug"},
			"name":   map[string]any{"old": "Old Name", "new": "New Name"},
			"active": map[string]any{"old": true, "new": false},
		}, auditChanges(t, entry))
	})

	t.Run("keeping the slug is no conflict and records no slug change", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Old Name", "same", false)

		_, err := h.svc.UpdateOrganization(context.Background(), 1, organizationtenancy.OrganizationChanges{
			Name: "Renamed", Slug: "same", Active: true,
		}, testOperatorID, operatorIP)

		require.NoError(t, err)
		entry := h.onlyAudit(t, domain.AuditActionUpdate, domain.AuditResourceOrganization, 1)
		assert.Equal(t, map[string]any{"name": map[string]any{"old": "Old Name", "new": "Renamed"}}, auditChanges(t, entry))
	})

	t.Run("no change records an entry without changes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Same", "same", false)

		_, err := h.svc.UpdateOrganization(context.Background(), 1, organizationtenancy.OrganizationChanges{
			Name: "Same", Slug: "same", Active: true,
		}, testOperatorID, operatorIP)

		require.NoError(t, err)
		entry := h.onlyAudit(t, domain.AuditActionUpdate, domain.AuditResourceOrganization, 1)
		assert.Nil(t, entry.Changes)
	})

	t.Run("missing organisation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		updated, err := h.svc.UpdateOrganization(context.Background(), 99, organizationtenancy.OrganizationChanges{
			Name: "Name", Slug: "slug",
		}, testOperatorID, operatorIP)

		assert.Nil(t, updated)
		var notFound *organizationtenancy.OrganizationNotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, int64(99), notFound.OrganizationID)
	})

	t.Run("deleted organisation is not updated", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Gone", "gone", true)

		_, err := h.svc.UpdateOrganization(context.Background(), 1, organizationtenancy.OrganizationChanges{
			Name: "Name", Slug: "slug",
		}, testOperatorID, operatorIP)

		var deleted *organizationtenancy.OrganizationAlreadyDeletedError
		require.ErrorAs(t, err, &deleted)
		assert.Equal(t, int64(1), deleted.OrganizationID)
		assert.False(t, h.engine.called("Update"))
		assert.Empty(t, h.audit.entries)
	})

	t.Run("slug taken by another organisation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "One", "one", false)
		h.addOrg(2, "Two", "two", false)

		_, err := h.svc.UpdateOrganization(context.Background(), 1, organizationtenancy.OrganizationChanges{
			Name: "One", Slug: "two", Active: true,
		}, testOperatorID, operatorIP)

		var conflict *organizationtenancy.ProvisioningConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("invalid changes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "One", "one", false)

		_, err := h.svc.UpdateOrganization(context.Background(), 1, organizationtenancy.OrganizationChanges{
			Name: "One", Slug: "api",
		}, testOperatorID, operatorIP)

		var invalid *organizationtenancy.InvalidProvisioningDataError
		require.ErrorAs(t, err, &invalid)
	})

	t.Run("unknown update error passes through", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "One", "one", false)
		h.engine.fail["Update"] = errBoom

		_, err := h.svc.UpdateOrganization(context.Background(), 1, organizationtenancy.OrganizationChanges{
			Name: "One", Slug: "one",
		}, testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("audit failure fails the update", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "One", "one", false)
		h.audit.err = errBoom

		updated, err := h.svc.UpdateOrganization(context.Background(), 1, organizationtenancy.OrganizationChanges{
			Name: "Two", Slug: "one",
		}, testOperatorID, operatorIP)

		assert.Nil(t, updated)
		require.ErrorIs(t, err, errBoom)
	})
}

func TestProvisioningSoftDeleteOrganization(t *testing.T) {
	t.Parallel()

	t.Run("deletes an organisation without schools", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.addSchool(10, 1, "Deleted", "deleted").DeletedAt = deletedNow()

		err := h.svc.SoftDeleteOrganization(context.Background(), 1, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.True(t, h.engine.orgs[1].IsDeleted())
		entry := h.onlyAudit(t, domain.AuditActionSoftDelete, domain.AuditResourceOrganization, 1)
		assert.Equal(t, map[string]any{"name": "Talent", "slug": "talent"}, auditChanges(t, entry))
	})

	t.Run("organisation with live schools keeps the school count", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.addSchool(10, 1, "A", "a")
		h.addSchool(11, 1, "B", "b")

		err := h.svc.SoftDeleteOrganization(context.Background(), 1, testOperatorID, operatorIP)

		var hasSchools *organizationtenancy.OrganizationHasSchoolsError
		require.ErrorAs(t, err, &hasSchools)
		assert.Equal(t, 2, hasSchools.SchoolCount)
		assert.False(t, h.engine.orgs[1].IsDeleted())
		assert.Empty(t, h.audit.entries)
	})

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		id    int64
		check func(*testing.T, error)
	}{
		"missing": {
			id: 99,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OrganizationNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(99), notFound.OrganizationID)
			},
		},
		"already deleted": {
			seed: func(h *provisioningHarness) { h.addOrg(1, "Gone", "gone", true) },
			id:   1,
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.OrganizationAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(1), deleted.OrganizationID)
			},
		},
		"invalid id": {
			id: 0,
			check: func(t *testing.T, err error) {
				var invalid *organizationtenancy.InvalidProvisioningDataError
				require.ErrorAs(t, err, &invalid)
			},
		},
		"unknown owner error": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.engine.fail["SoftDelete"] = errBoom
			},
			id:    1,
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			if tc.seed != nil {
				tc.seed(h)
			}
			err := h.svc.SoftDeleteOrganization(context.Background(), tc.id, testOperatorID, operatorIP)
			tc.check(t, err)
			assert.Empty(t, h.audit.entries)
		})
	}

	t.Run("audit failure fails the deletion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.audit.err = errBoom

		err := h.svc.SoftDeleteOrganization(context.Background(), 1, testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		require.ErrorIs(t, h.tx.lastErr, errBoom)
	})
}

func TestProvisioningRestoreOrganization(t *testing.T) {
	t.Parallel()

	t.Run("restores a deleted organisation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", true)

		err := h.svc.RestoreOrganization(context.Background(), 1, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.False(t, h.engine.orgs[1].IsDeleted())
		entry := h.onlyAudit(t, domain.AuditActionRestore, domain.AuditResourceOrganization, 1)
		assert.Equal(t, map[string]any{"name": "Talent", "slug": "talent"}, auditChanges(t, entry))
	})

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing": {
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OrganizationNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(1), notFound.OrganizationID)
			},
		},
		"not deleted": {
			seed: func(h *provisioningHarness) { h.addOrg(1, "Live", "live", false) },
			check: func(t *testing.T, err error) {
				var notDeleted *organizationtenancy.OrganizationNotDeletedError
				require.ErrorAs(t, err, &notDeleted)
				assert.Equal(t, int64(1), notDeleted.OrganizationID)
			},
		},
		"concurrent restore maps to not deleted": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", true)
				h.engine.fail["Restore"] = organizationtenancy.ErrOrganizationNotDeleted
			},
			check: func(t *testing.T, err error) {
				var notDeleted *organizationtenancy.OrganizationNotDeletedError
				require.ErrorAs(t, err, &notDeleted)
			},
		},
		"unknown owner error": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", true)
				h.engine.fail["Restore"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			if tc.seed != nil {
				tc.seed(h)
			}
			err := h.svc.RestoreOrganization(context.Background(), 1, testOperatorID, operatorIP)
			tc.check(t, err)
			assert.Empty(t, h.audit.entries)
		})
	}
}

func TestMapOrganizationErrorPreservesOperatorContracts(t *testing.T) {
	t.Parallel()

	hasSchools := &organizationtenancy.OrganizationHasSchoolsError{SchoolCount: 3}
	for name, tc := range map[string]struct {
		err   error
		check func(*testing.T, error)
	}{
		"not found": {organizationtenancy.ErrOrganizationNotFound, func(t *testing.T, err error) {
			var target *organizationtenancy.OrganizationNotFoundError
			require.ErrorAs(t, err, &target)
			assert.Equal(t, int64(5), target.OrganizationID)
		}},
		"slug conflict": {organizationtenancy.ErrOrganizationSlugConflict, func(t *testing.T, err error) {
			var target *organizationtenancy.ProvisioningConflictError
			require.ErrorAs(t, err, &target)
		}},
		"already deleted": {organizationtenancy.ErrOrganizationAlreadyDeleted, func(t *testing.T, err error) {
			var target *organizationtenancy.OrganizationAlreadyDeletedError
			require.ErrorAs(t, err, &target)
		}},
		"not deleted": {organizationtenancy.ErrOrganizationNotDeleted, func(t *testing.T, err error) {
			var target *organizationtenancy.OrganizationNotDeletedError
			require.ErrorAs(t, err, &target)
		}},
		"has schools": {hasSchools, func(t *testing.T, err error) {
			require.Same(t, hasSchools, err)
		}},
		"invalid": {&organizationtenancy.InvalidOrganizationError{Reason: "bad"}, func(t *testing.T, err error) {
			var target *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &target)
			require.ErrorIs(t, err, organizationtenancy.ErrInvalidOrganization)
		}},
		"unknown": {errBoom, func(t *testing.T, err error) { require.Same(t, errBoom, err) }},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc.check(t, mapOrganizationError(tc.err, 5))
		})
	}
}

func TestMapSchoolErrorPreservesOperatorContracts(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		err   error
		check func(*testing.T, error)
	}{
		"subdomain conflict": {organizationtenancy.ErrSchoolDomainConflict, func(t *testing.T, err error) {
			var target *organizationtenancy.ProvisioningConflictError
			require.ErrorAs(t, err, &target)
			assert.Contains(t, target.Error(), "subdomain")
		}},
		"slug conflict": {organizationtenancy.ErrSchoolSlugConflict, func(t *testing.T, err error) {
			var target *organizationtenancy.ProvisioningConflictError
			require.ErrorAs(t, err, &target)
			assert.Contains(t, target.Error(), "slug")
		}},
		"school not found": {organizationtenancy.ErrSchoolNotFound, func(t *testing.T, err error) {
			var target *organizationtenancy.SchoolNotFoundError
			require.ErrorAs(t, err, &target)
			assert.Equal(t, int64(10), target.SchoolID)
		}},
		"already deleted": {organizationtenancy.ErrSchoolAlreadyDeleted, func(t *testing.T, err error) {
			var target *organizationtenancy.SchoolAlreadyDeletedError
			require.ErrorAs(t, err, &target)
			assert.Equal(t, int64(10), target.SchoolID)
		}},
		"not deleted": {organizationtenancy.ErrSchoolNotDeleted, func(t *testing.T, err error) {
			var target *organizationtenancy.SchoolNotDeletedError
			require.ErrorAs(t, err, &target)
		}},
		"organisation deleted": {organizationtenancy.ErrOrganizationDeleted, func(t *testing.T, err error) {
			var target *organizationtenancy.OrganizationDeletedError
			require.ErrorAs(t, err, &target)
			assert.Equal(t, int64(20), target.OrganizationID)
		}},
		"organisation not found": {organizationtenancy.ErrOrganizationNotFound, func(t *testing.T, err error) {
			var target *organizationtenancy.OrganizationNotFoundError
			require.ErrorAs(t, err, &target)
			assert.Equal(t, int64(20), target.OrganizationID)
		}},
		"invalid school": {&organizationtenancy.InvalidSchoolError{Reason: "bad"}, func(t *testing.T, err error) {
			var target *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &target)
		}},
		"unknown": {errBoom, func(t *testing.T, err error) { require.Same(t, errBoom, err) }},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc.check(t, mapSchoolError(tc.err, 10, 20))
		})
	}

	_, known := translateSchoolError(errBoom, 10, 20)
	assert.False(t, known)
}
