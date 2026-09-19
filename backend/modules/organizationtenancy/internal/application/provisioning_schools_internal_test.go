package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCreateSchool(organizationID int64) *organizationtenancy.CreateSchool {
	return &organizationtenancy.CreateSchool{
		OrganizationID: organizationID, Name: " Grundschule Burbach ", Slug: "Burbach",
		Subdomain: " Burbach-GS ", Active: true, City: "Burbach",
	}
}

func TestProvisioningCreateSchool(t *testing.T) {
	t.Parallel()

	t.Run("creates the school with default categories and the manual web device", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)

		created, err := h.svc.CreateSchool(context.Background(), validCreateSchool(1), testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, "Grundschule Burbach", created.Name)
		assert.Equal(t, "burbach", created.Slug)
		assert.Equal(t, "burbach-gs", created.Subdomain)
		assert.Equal(t, "Burbach", created.City)
		require.Contains(t, h.engine.schools, created.ID)

		categories := h.categories.seeded[created.ID]
		require.Len(t, categories, 10)
		assert.Contains(t, categories, domain.ActivityCategory{
			Name: "Mensa", Description: "Aktivitäten rund um das Mittagessen", Color: "#FF9500",
		}, "Essenszeiten need the Mensa category (#2131)")
		assert.Equal(t, "Sport", categories[0].Name)

		require.Len(t, h.devices.created, 1)
		device := h.devices.created[0]
		assert.Equal(t, created.ID, device.TenantID)
		assert.Equal(t, domain.WebManualDeviceID, device.DeviceID)
		assert.Equal(t, domain.DeviceTypeVirtual, device.DeviceType)
		assert.Equal(t, domain.DeviceStatusActive, device.Status)
		require.NotNil(t, device.Name)
		assert.Equal(t, "Web-Portal (Manuell)", *device.Name)
		assert.Nil(t, device.APIKey, "the virtual device authenticates no kiosk")

		entry := h.onlyAudit(t, domain.AuditActionCreate, domain.AuditResourceSchool, created.ID)
		assert.Equal(t, map[string]any{
			"name": "Grundschule Burbach", "slug": "burbach", "subdomain": "burbach-gs", "organizationID": float64(1),
		}, auditChanges(t, entry))
	})

	t.Run("the same slug is allowed in another organisation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "One", "one", false)
		h.addOrg(2, "Two", "two", false)
		h.addSchool(10, 1, "Burbach", "burbach").Subdomain = "burbach-one"

		created, err := h.svc.CreateSchool(context.Background(), validCreateSchool(2), testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, int64(2), created.OrganizationID)
		assert.Equal(t, "burbach", created.Slug)
	})

	t.Run("an existing manual web device is kept", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.devices.createErrs = []error{domain.ErrDeviceIDTaken}

		created, err := h.svc.CreateSchool(context.Background(), validCreateSchool(1), testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Len(t, h.audit.entries, 1)
	})

	t.Run("audit failure does not fail the creation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.audit.err = errBoom

		created, err := h.svc.CreateSchool(context.Background(), validCreateSchool(1), testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Contains(t, h.logs.String(), "failed to create operator audit log")
	})

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		created, err := h.svc.CreateSchool(context.Background(), nil, testOperatorID, operatorIP)

		assert.Nil(t, created)
		var invalid *organizationtenancy.InvalidProvisioningDataError
		require.ErrorAs(t, err, &invalid)
		assert.Zero(t, h.tx.adminRuns)
	})

	t.Run("does not change the caller's input", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		input := validCreateSchool(1)

		_, err := h.svc.CreateSchool(context.Background(), input, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, "Burbach", input.Slug)
	})

	for name, mutate := range map[string]func(*organizationtenancy.CreateSchool){
		"missing organisation id": func(s *organizationtenancy.CreateSchool) { s.OrganizationID = 0 },
		"missing name":            func(s *organizationtenancy.CreateSchool) { s.Name = "  " },
		"invalid slug":            func(s *organizationtenancy.CreateSchool) { s.Slug = "-bad-" },
		"reserved subdomain":      func(s *organizationtenancy.CreateSchool) { s.Subdomain = "api" },
	} {
		t.Run("rejects "+name+" before reading", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			input := validCreateSchool(1)
			mutate(input)

			created, err := h.svc.CreateSchool(context.Background(), input, testOperatorID, operatorIP)

			assert.Nil(t, created)
			var invalid *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &invalid)
			require.ErrorIs(t, err, organizationtenancy.ErrInvalidSchool)
			assert.Zero(t, h.tx.adminRuns)
		})
	}

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing organisation": {
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OrganizationNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(1), notFound.OrganizationID)
			},
		},
		"deleted organisation": {
			seed: func(h *provisioningHarness) { h.addOrg(1, "Gone", "gone", true) },
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.OrganizationDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(1), deleted.OrganizationID)
			},
		},
		"organisation lookup failure": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.engine.fail["FindForSchoolMutation"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"slug taken in the organisation": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Other", "burbach").Subdomain = "other"
			},
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Contains(t, conflict.Error(), "slug")
			},
		},
		"subdomain taken": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addOrg(2, "Other", "other", false)
				h.addSchool(10, 2, "Other", "other").Subdomain = "burbach-gs"
			},
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Contains(t, conflict.Error(), "subdomain")
			},
		},
		"slug lookup failure": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.engine.fail["FindSchoolByOrganizationAndSlug"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"subdomain lookup failure": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.engine.fail["FindSchoolBySubdomain"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"conflict raised by the owner write": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.engine.fail["CreateSchool"] = organizationtenancy.ErrSchoolDomainConflict
			},
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Contains(t, conflict.Error(), "subdomain")
			},
		},
		"organisation deleted during the write": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.engine.fail["CreateSchool"] = organizationtenancy.ErrOrganizationDeleted
			},
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.OrganizationDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(1), deleted.OrganizationID)
			},
		},
		"unknown write error": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.engine.fail["CreateSchool"] = errBoom
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

			created, err := h.svc.CreateSchool(context.Background(), validCreateSchool(1), testOperatorID, operatorIP)

			assert.Nil(t, created)
			tc.check(t, err)
			assert.Empty(t, h.categories.seeded)
			assert.Empty(t, h.devices.created)
			assert.Empty(t, h.audit.entries)
		})
	}

	t.Run("category seeding failure stops the creation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.categories.err = errBoom

		created, err := h.svc.CreateSchool(context.Background(), validCreateSchool(1), testOperatorID, operatorIP)

		assert.Nil(t, created)
		require.ErrorIs(t, err, errBoom)
		require.ErrorIs(t, h.tx.lastErr, errBoom)
		assert.Empty(t, h.devices.created)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("manual web device failure stops the creation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.devices.createErrs = []error{errBoom}

		created, err := h.svc.CreateSchool(context.Background(), validCreateSchool(1), testOperatorID, operatorIP)

		assert.Nil(t, created)
		require.ErrorIs(t, err, errBoom)
		assert.Contains(t, err.Error(), "create web manual device for tenant")
		require.ErrorIs(t, h.tx.lastErr, errBoom)
		assert.Empty(t, h.audit.entries)
	})
}

func TestProvisioningListSchools(t *testing.T) {
	t.Parallel()

	t.Run("attaches each school's organisation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "One", "one", false)
		h.addOrg(2, "Two", "two", true)
		h.addSchool(10, 1, "A", "a")
		h.addSchool(11, 2, "B", "b").DeletedAt = deletedNow()
		h.addSchool(12, 3, "Orphan", "orphan")

		schools, err := h.svc.ListSchools(context.Background())

		require.NoError(t, err)
		require.Len(t, schools, 3)
		require.NotNil(t, schools[0].Organization)
		assert.Equal(t, "One", schools[0].Organization.Name)
		require.NotNil(t, schools[1].Organization)
		assert.True(t, schools[1].Organization.IsDeleted())
		assert.Nil(t, schools[2].Organization)
	})

	t.Run("no schools", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		schools, err := h.svc.ListSchools(context.Background())

		require.NoError(t, err)
		assert.Empty(t, schools)
		assert.False(t, h.engine.called("ListByIDs"))
	})

	for _, method := range []string{"ListSchools", "ListByIDs"} {
		t.Run(method+" failure", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.addOrg(1, "One", "one", false)
			h.addSchool(10, 1, "A", "a")
			h.engine.fail[method] = errBoom

			schools, err := h.svc.ListSchools(context.Background())

			assert.Nil(t, schools)
			require.ErrorIs(t, err, errBoom)
		})
	}
}

func schoolChanges(organizationID int64) organizationtenancy.SchoolChanges {
	return organizationtenancy.SchoolChanges{
		OrganizationID: organizationID, Name: "Burbach", Slug: "burbach", Subdomain: "burbach", Active: true,
	}
}

func TestProvisioningUpdateSchool(t *testing.T) {
	t.Parallel()

	t.Run("updates every field and records the audited changes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		school := h.addSchool(10, 1, "Burbach", "burbach")
		school.Settings = `{"theme":"blue"}`
		school.DevicePinHash = "pin-hash"

		updated, err := h.svc.UpdateSchool(context.Background(), 10, organizationtenancy.SchoolChanges{
			OrganizationID: 1, Name: "Walbach", Slug: "walbach", Subdomain: "walbach-gs",
			Address: "Hauptstr. 1", City: "Walbach", Zip: "57299", Phone: "0271", Email: "info@walbach.test",
			Active: false, Hidden: true,
		}, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, "Walbach", updated.Name)
		assert.Equal(t, "walbach-gs", updated.Subdomain)
		assert.Equal(t, "57299", updated.Zip)
		require.Len(t, h.engine.updates, 1)
		write := h.engine.updates[0]
		assert.Equal(t, `{"theme":"blue"}`, write.Settings, "settings are not operator-editable and must be kept")
		assert.Equal(t, "pin-hash", write.DevicePinHash, "the device PIN hash must be kept")
		assert.Equal(t, "Hauptstr. 1", write.Address)
		assert.Equal(t, "info@walbach.test", write.Email)

		entry := h.onlyAudit(t, domain.AuditActionUpdate, domain.AuditResourceSchool, 10)
		assert.Equal(t, map[string]any{
			"slug":      map[string]any{"old": "burbach", "new": "walbach"},
			"subdomain": map[string]any{"old": "burbach", "new": "walbach-gs"},
			"name":      map[string]any{"old": "Burbach", "new": "Walbach"},
			"active":    map[string]any{"old": true, "new": false},
			"hidden":    map[string]any{"old": false, "new": true},
		}, auditChanges(t, entry))
	})

	t.Run("moves the school to another organisation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "One", "one", false)
		h.addOrg(2, "Two", "two", false)
		h.addSchool(10, 1, "Burbach", "burbach")

		updated, err := h.svc.UpdateSchool(context.Background(), 10, schoolChanges(2), testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, int64(2), updated.OrganizationID)
		entry := h.onlyAudit(t, domain.AuditActionUpdate, domain.AuditResourceSchool, 10)
		assert.Equal(t, map[string]any{
			"organization_id": map[string]any{"old": float64(1), "new": float64(2)},
		}, auditChanges(t, entry))
	})

	t.Run("unchanged school records an entry without changes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.addSchool(10, 1, "Burbach", "burbach")

		_, err := h.svc.UpdateSchool(context.Background(), 10, schoolChanges(1), testOperatorID, operatorIP)

		require.NoError(t, err)
		entry := h.onlyAudit(t, domain.AuditActionUpdate, domain.AuditResourceSchool, 10)
		assert.Nil(t, entry.Changes)
		assert.False(t, h.engine.called("FindSchoolByOrganizationAndSlug"))
		assert.False(t, h.engine.called("FindSchoolBySubdomain"))
	})

	t.Run("audit failure does not fail the update", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.addSchool(10, 1, "Burbach", "burbach")
		h.audit.err = errBoom

		updated, err := h.svc.UpdateSchool(context.Background(), 10, schoolChanges(1), testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, updated)
	})

	for name, tc := range map[string]struct {
		seed    func(*provisioningHarness)
		changes organizationtenancy.SchoolChanges
		check   func(*testing.T, error)
	}{
		"missing school": {
			changes: schoolChanges(1),
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(10), notFound.SchoolID)
			},
		},
		"deleted school": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach").DeletedAt = deletedNow()
			},
			changes: schoolChanges(1),
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(10), deleted.SchoolID)
			},
		},
		"school lookup failure": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
				h.engine.fail["FindSchoolByID"] = errBoom
			},
			changes: schoolChanges(1),
			check:   func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"slug taken": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
				h.addSchool(11, 1, "Walbach", "walbach")
			},
			changes: func() organizationtenancy.SchoolChanges {
				c := schoolChanges(1)
				c.Slug = "walbach"
				return c
			}(),
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Contains(t, conflict.Error(), "slug")
			},
		},
		"slug taken in the target organisation": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "One", "one", false)
				h.addOrg(2, "Two", "two", false)
				h.addSchool(10, 1, "Burbach", "burbach")
				h.addSchool(11, 2, "Burbach Two", "burbach").Subdomain = "burbach-two"
			},
			changes: schoolChanges(2),
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Contains(t, conflict.Error(), "slug")
			},
		},
		"subdomain taken": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
				h.addSchool(11, 1, "Walbach", "walbach")
			},
			changes: func() organizationtenancy.SchoolChanges {
				c := schoolChanges(1)
				c.Subdomain = "walbach"
				return c
			}(),
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Contains(t, conflict.Error(), "subdomain")
			},
		},
		"missing target organisation": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
			},
			changes: schoolChanges(99),
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OrganizationNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(99), notFound.OrganizationID)
			},
		},
		"deleted target organisation": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addOrg(2, "Gone", "gone", true)
				h.addSchool(10, 1, "Burbach", "burbach")
			},
			changes: schoolChanges(2),
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.OrganizationDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(2), deleted.OrganizationID)
			},
		},
		"invalid changes": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
			},
			changes: func() organizationtenancy.SchoolChanges {
				c := schoolChanges(1)
				c.Name = ""
				return c
			}(),
			check: func(t *testing.T, err error) {
				var invalid *organizationtenancy.InvalidProvisioningDataError
				require.ErrorAs(t, err, &invalid)
			},
		},
		"conflict raised by the owner write": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
				h.engine.fail["UpdateSchool"] = organizationtenancy.ErrSchoolSlugConflict
			},
			changes: schoolChanges(1),
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
			},
		},
		"unknown write error is invalid data": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
				h.engine.fail["UpdateSchool"] = errBoom
			},
			changes: schoolChanges(1),
			check: func(t *testing.T, err error) {
				var invalid *organizationtenancy.InvalidProvisioningDataError
				require.ErrorAs(t, err, &invalid)
				require.ErrorIs(t, err, errBoom)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			if tc.seed != nil {
				tc.seed(h)
			}

			updated, err := h.svc.UpdateSchool(context.Background(), 10, tc.changes, testOperatorID, operatorIP)

			assert.Nil(t, updated)
			tc.check(t, err)
			assert.Empty(t, h.audit.entries)
		})
	}
}

func TestProvisioningSoftDeleteSchool(t *testing.T) {
	t.Parallel()

	t.Run("deletes the school and revokes its access", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		h.addSchool(10, 1, "Burbach", "burbach")

		err := h.svc.SoftDeleteSchool(context.Background(), 10, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.True(t, h.engine.schools[10].IsDeleted())
		assert.Equal(t, []int64{10}, h.identity.revoked)
		assert.Equal(t, []int64{10}, h.identity.invalidated)
		entry := h.onlyAudit(t, domain.AuditActionSoftDelete, domain.AuditResourceSchool, 10)
		assert.Equal(t, map[string]any{
			"name": "Burbach", "slug": "burbach", "subdomain": "burbach",
			"revoked_tokens": float64(4), "invalidated_invites": float64(2),
		}, auditChanges(t, entry))
	})

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing school": {
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(10), notFound.SchoolID)
			},
		},
		"already deleted": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach").DeletedAt = deletedNow()
			},
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(10), deleted.SchoolID)
			},
		},
		"concurrent deletion maps to already deleted": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
				h.engine.fail["SoftDeleteSchool"] = organizationtenancy.ErrSchoolAlreadyDeleted
			},
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(10), deleted.SchoolID)
			},
		},
		"school lookup failure": {
			seed: func(h *provisioningHarness) {
				h.addSchool(10, 1, "Burbach", "burbach")
				h.engine.fail["FindSchoolByID"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"unknown owner error": {
			seed: func(h *provisioningHarness) {
				h.addSchool(10, 1, "Burbach", "burbach")
				h.engine.fail["SoftDeleteSchool"] = errBoom
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

			err := h.svc.SoftDeleteSchool(context.Background(), 10, testOperatorID, operatorIP)

			tc.check(t, err)
			assert.Empty(t, h.identity.revoked)
			assert.Empty(t, h.identity.invalidated)
			assert.Empty(t, h.audit.entries)
		})
	}

	t.Run("session revocation failure fails the deletion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addSchool(10, 1, "Burbach", "burbach")
		h.identity.fail["RevokeSchoolSessions"] = errBoom

		err := h.svc.SoftDeleteSchool(context.Background(), 10, testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		assert.Contains(t, err.Error(), "revoke tokens for school 10")
		require.ErrorIs(t, h.tx.lastErr, errBoom, "the deletion must roll back")
		assert.Empty(t, h.identity.invalidated)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("invitation invalidation failure fails the deletion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addSchool(10, 1, "Burbach", "burbach")
		h.identity.fail["InvalidatePendingInvitations"] = errBoom

		err := h.svc.SoftDeleteSchool(context.Background(), 10, testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		assert.Contains(t, err.Error(), "invalidate invitations for school 10")
		require.ErrorIs(t, h.tx.lastErr, errBoom)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("audit failure does not fail the deletion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addSchool(10, 1, "Burbach", "burbach")
		h.audit.err = errBoom

		require.NoError(t, h.svc.SoftDeleteSchool(context.Background(), 10, testOperatorID, operatorIP))
		assert.True(t, h.engine.schools[10].IsDeleted())
	})
}

func TestProvisioningRestoreSchool(t *testing.T) {
	t.Parallel()

	t.Run("restores the school and keeps it inactive", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.addOrg(1, "Talent", "talent", false)
		school := h.addSchool(10, 1, "Burbach", "burbach")
		school.DeletedAt = deletedNow()
		school.Active = false

		err := h.svc.RestoreSchool(context.Background(), 10, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.False(t, h.engine.schools[10].IsDeleted())
		assert.False(t, h.engine.schools[10].Active)
		entry := h.onlyAudit(t, domain.AuditActionRestore, domain.AuditResourceSchool, 10)
		assert.Equal(t, map[string]any{"name": "Burbach", "slug": "burbach", "subdomain": "burbach"}, auditChanges(t, entry))
	})

	deletedSchool := func(h *provisioningHarness) {
		h.addSchool(10, 1, "Burbach", "burbach").DeletedAt = deletedNow()
	}
	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing school": {
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
			},
		},
		"not deleted": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				h.addSchool(10, 1, "Burbach", "burbach")
			},
			check: func(t *testing.T, err error) {
				var notDeleted *organizationtenancy.SchoolNotDeletedError
				require.ErrorAs(t, err, &notDeleted)
				assert.Equal(t, int64(10), notDeleted.SchoolID)
			},
		},
		"deleted organisation": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Gone", "gone", true)
				deletedSchool(h)
			},
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.OrganizationDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(1), deleted.OrganizationID)
			},
		},
		"missing organisation": {
			seed: deletedSchool,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OrganizationNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(1), notFound.OrganizationID)
			},
		},
		"organisation lookup failure": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				deletedSchool(h)
				h.engine.fail["FindForSchoolMutation"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"concurrent restore maps to not deleted": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				deletedSchool(h)
				h.engine.fail["RestoreSchool"] = organizationtenancy.ErrSchoolNotDeleted
			},
			check: func(t *testing.T, err error) {
				var notDeleted *organizationtenancy.SchoolNotDeletedError
				require.ErrorAs(t, err, &notDeleted)
				assert.Equal(t, int64(10), notDeleted.SchoolID)
			},
		},
		"unknown owner error": {
			seed: func(h *provisioningHarness) {
				h.addOrg(1, "Talent", "talent", false)
				deletedSchool(h)
				h.engine.fail["RestoreSchool"] = errBoom
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

			err := h.svc.RestoreSchool(context.Background(), 10, testOperatorID, operatorIP)

			tc.check(t, err)
			assert.Empty(t, h.audit.entries)
		})
	}
}
