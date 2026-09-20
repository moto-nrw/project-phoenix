package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The online checks compare against time.Since, so last-seen values sit far
// from the window edges.
func seenAgo(d time.Duration) *time.Time {
	seen := time.Now().Add(-d)
	return &seen
}

// seedDeviceSchools stores organisation 1 with schools 10 (Burbach) and 20
// (Walbach) and organisation 2 with school 30 (Anderswo).
func seedDeviceSchools(h *provisioningHarness) {
	h.addOrg(1, "Talent OGS", "talent", false)
	h.addOrg(2, "Andere OGS", "andere", false)
	h.addSchool(10, 1, "Burbach", "burbach")
	h.addSchool(20, 1, "Walbach", "walbach")
	h.addSchool(30, 2, "Anderswo", "anderswo")
}

func terminal(id, tenantID int64, deviceID string) domain.Device {
	key := "dev_" + deviceID + "-secret-key"
	name := "Terminal " + deviceID
	return domain.Device{
		ID: id, TenantID: tenantID, CreatedAt: fixedTime, UpdatedAt: fixedTime,
		DeviceID: deviceID, DeviceType: "terminal", Name: &name,
		Status: domain.DeviceStatusActive, APIKey: &key,
	}
}

func TestMaskAPIKey(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		key  *string
		want string
	}{
		"nil":        {nil, ""},
		"empty":      {strPtr(""), ""},
		"short":      {strPtr("abc"), "abc"},
		"ten chars":  {strPtr("0123456789"), "0123456789"},
		"long":       {strPtr("0123456789abcdef"), "0123456789..."},
		"eleven len": {strPtr("0123456789a"), "0123456789..."},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, maskAPIKey(tc.key))
		})
	}
}

func TestProvisioningListDevices(t *testing.T) {
	t.Parallel()

	seed := func(h *provisioningHarness) {
		seedDeviceSchools(h)
		h.addOrg(3, "Papierkorb OGS", "papierkorb", true)
		h.addSchool(40, 3, "Weg", "weg").DeletedAt = deletedNow()
		h.addSchool(50, 1, "Geloescht", "geloescht").DeletedAt = deletedNow()
		h.addSchool(60, 3, "Verwaist", "verwaist")

		online := terminal(1, 20, "B-ONLINE")
		online.LastSeen = seenAgo(time.Minute)
		offline := terminal(2, 10, "Z-OFFLINE")
		offline.LastSeen = seenAgo(10 * time.Minute)
		h.devices.add(online)
		h.devices.add(offline)
		h.devices.add(terminal(3, 10, "A-NEVER"))
		h.devices.add(terminal(4, 30, "OTHER-ORG"))
		h.devices.add(terminal(5, 50, "DELETED-SCHOOL"))
		h.devices.add(terminal(6, 60, "DELETED-ORG"))
		archived := terminal(7, 10, "ARCHIVED")
		archived.ArchivedAt = &fixedTime
		archived.APIKey = nil
		h.devices.add(archived)
	}

	t.Run("lists devices of live schools with derived fields", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)

		devices, err := h.svc.ListAllDevices(context.Background())

		require.NoError(t, err)
		ids := make([]string, 0, len(devices))
		for _, device := range devices {
			ids = append(ids, device.DeviceID)
		}
		assert.Equal(t, []string{"OTHER-ORG", "A-NEVER", "Z-OFFLINE", "B-ONLINE"}, ids,
			"sorted by organisation, school and device id; deleted schools and organisations are hidden")

		never, offline, online := devices[1], devices[2], devices[3]
		assert.Equal(t, int64(3), never.ID)
		assert.Equal(t, int64(10), never.SchoolID)
		assert.Equal(t, "Burbach", never.SchoolName)
		assert.Equal(t, int64(1), never.OrganizationID)
		assert.Equal(t, "Talent OGS", never.OrganizationName)
		assert.Equal(t, "terminal", never.DeviceType)
		assert.Equal(t, fixedTime, never.CreatedAt)
		require.NotNil(t, never.APIKey)
		assert.Equal(t, "dev_A-NEVE...", never.MaskedAPIKey)
		assert.False(t, never.IsOnline)
		assert.Nil(t, never.LastSeen)
		assert.False(t, offline.IsOnline)
		assert.True(t, online.IsOnline)
	})

	t.Run("school listing", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)

		devices, err := h.svc.ListSchoolDevices(context.Background(), 10)

		require.NoError(t, err)
		require.Len(t, devices, 2)
		assert.Equal(t, "A-NEVER", devices[0].DeviceID)
		assert.Equal(t, "Z-OFFLINE", devices[1].DeviceID)
	})

	t.Run("organisation listing", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)

		devices, err := h.svc.ListOrganizationDevices(context.Background(), 1)

		require.NoError(t, err)
		require.Len(t, devices, 3)
		for _, device := range devices {
			assert.Equal(t, int64(1), device.OrganizationID)
		}
	})

	t.Run("school without devices lists none", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)

		devices, err := h.svc.ListSchoolDevices(context.Background(), 20)

		require.NoError(t, err)
		assert.NotNil(t, devices)
		assert.Empty(t, devices)
	})

	for name, tc := range map[string]struct {
		list  func(*provisioningHarness) ([]organizationtenancy.OperatorDevice, error)
		check func(*testing.T, error)
	}{
		"missing school": {
			list: func(h *provisioningHarness) ([]organizationtenancy.OperatorDevice, error) {
				return h.svc.ListSchoolDevices(context.Background(), 99)
			},
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(99), notFound.SchoolID)
			},
		},
		"deleted school": {
			list: func(h *provisioningHarness) ([]organizationtenancy.OperatorDevice, error) {
				return h.svc.ListSchoolDevices(context.Background(), 50)
			},
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(50), deleted.SchoolID)
			},
		},
		"missing organisation": {
			list: func(h *provisioningHarness) ([]organizationtenancy.OperatorDevice, error) {
				return h.svc.ListOrganizationDevices(context.Background(), 99)
			},
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OrganizationNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(99), notFound.OrganizationID)
			},
		},
		"deleted organisation": {
			list: func(h *provisioningHarness) ([]organizationtenancy.OperatorDevice, error) {
				return h.svc.ListOrganizationDevices(context.Background(), 3)
			},
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.OrganizationDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(3), deleted.OrganizationID)
			},
		},
		"school rows failure": {
			list: func(h *provisioningHarness) ([]organizationtenancy.OperatorDevice, error) {
				h.engine.fail["ListSchools"] = errBoom
				return h.svc.ListAllDevices(context.Background())
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"organisation rows failure": {
			list: func(h *provisioningHarness) ([]organizationtenancy.OperatorDevice, error) {
				h.engine.fail["List"] = errBoom
				return h.svc.ListAllDevices(context.Background())
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"device listing failure": {
			list: func(h *provisioningHarness) ([]organizationtenancy.OperatorDevice, error) {
				h.devices.fail["ListDevicesByTenant"] = errBoom
				return h.svc.ListAllDevices(context.Background())
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seed(h)

			devices, err := tc.list(h)

			assert.Nil(t, devices)
			tc.check(t, err)
		})
	}
}

func TestProvisioningCreateDevice(t *testing.T) {
	t.Parallel()

	t.Run("generates a key when none is given", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.secrets.keys = []string{"dev_generated-key-1"}

		device, err := h.svc.CreateDevice(context.Background(), 10, "  BURBACH-1 ", " terminal ", strPtr("Eingang"), nil, testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, device)
		assert.Equal(t, "BURBACH-1", device.DeviceID)
		assert.Equal(t, "terminal", device.DeviceType)
		require.NotNil(t, device.Name)
		assert.Equal(t, "Eingang", *device.Name)
		assert.Equal(t, domain.DeviceStatusActive, device.Status)
		require.NotNil(t, device.APIKey)
		assert.Equal(t, "dev_generated-key-1", *device.APIKey)
		assert.Equal(t, "dev_genera...", device.MaskedAPIKey)
		assert.Equal(t, "Burbach", device.SchoolName)
		assert.Equal(t, "Talent OGS", device.OrganizationName)
		assert.Equal(t, 1, h.secrets.calls)
		require.Len(t, h.devices.created, 1)
		assert.Equal(t, int64(10), h.devices.created[0].TenantID)

		entry := h.onlyAudit(t, domain.AuditActionCreate, domain.AuditResourceDevice, device.ID)
		assert.Equal(t, map[string]any{
			"device_id": "BURBACH-1", "device_type": "terminal", "school_id": float64(10), "api_key_mode": "auto",
		}, auditChanges(t, entry))
	})

	t.Run("uses the operator's key", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)

		device, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, strPtr("manual-key-123"), testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, device.APIKey)
		assert.Equal(t, "manual-key-123", *device.APIKey)
		assert.Zero(t, h.secrets.calls)
		entry := h.onlyAudit(t, domain.AuditActionCreate, domain.AuditResourceDevice, device.ID)
		assert.Equal(t, "manual", auditChanges(t, entry)["api_key_mode"])
	})

	t.Run("an empty operator key counts as none", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)

		device, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, strPtr(""), testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, device)
		assert.Equal(t, 1, h.secrets.calls)
		assert.Equal(t, "auto", auditChanges(t, h.audit.entries[0])["api_key_mode"])
	})

	t.Run("regenerates a colliding generated key", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.devices.add(terminal(1, 20, "TAKEN"))
		h.secrets.keys = []string{"dev_TAKEN-secret-key", "dev_fresh-key"}

		device, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, nil, testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, device.APIKey)
		assert.Equal(t, "dev_fresh-key", *device.APIKey)
		assert.Equal(t, 2, h.secrets.calls)
		assert.Len(t, h.devices.created, 2)
		assert.Len(t, h.audit.entries, 1)
	})

	t.Run("gives up after three colliding keys", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		taken := domain.ErrDeviceAPIKeyTaken
		h.devices.createErrs = []error{taken, taken, taken}

		device, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, nil, testOperatorID, operatorIP)

		assert.Nil(t, device)
		require.ErrorContains(t, err, "failed to generate unique API key after 3 attempts")
		assert.Len(t, h.devices.created, 3)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("a taken operator key is a conflict without retry", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.devices.add(terminal(1, 20, "TAKEN"))

		device, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, strPtr("dev_TAKEN-secret-key"), testOperatorID, operatorIP)

		assert.Nil(t, device)
		var conflict *organizationtenancy.ProvisioningConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Contains(t, conflict.Error(), "api_key already in use")
		assert.Len(t, h.devices.created, 1)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("a device id taken at the school is a conflict", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.devices.add(terminal(1, 10, "BURBACH-1"))

		device, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, nil, testOperatorID, operatorIP)

		assert.Nil(t, device)
		var conflict *organizationtenancy.ProvisioningConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Contains(t, conflict.Error(), "device_id already exists for this school")
		assert.Len(t, h.devices.created, 1)
	})

	t.Run("key generation failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.secrets.err = errBoom

		_, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, nil, testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		assert.Contains(t, err.Error(), "generate API key")
		assert.Empty(t, h.devices.created)
	})

	t.Run("unknown write error passes through", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.devices.createErrs = []error{errBoom}

		_, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, nil, testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		assert.Len(t, h.devices.created, 1)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("re-query failure fails the creation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.engine.fail["List"] = errBoom

		device, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, nil, testOperatorID, operatorIP)

		assert.Nil(t, device)
		require.ErrorIs(t, err, errBoom)
		assert.Contains(t, err.Error(), "CreateDevice: re-query failed")
		require.ErrorIs(t, h.tx.lastErr, errBoom)
	})

	for name, tc := range map[string]struct {
		schoolID   int64
		deviceID   string
		deviceType string
	}{
		"missing school id":        {0, "D-1", "terminal"},
		"negative school id":       {-1, "D-1", "terminal"},
		"empty device id":          {10, "", "terminal"},
		"whitespace-only deviceID": {10, "   ", "terminal"},
		"empty device type":        {10, "D-1", " "},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)

			device, err := h.svc.CreateDevice(context.Background(), tc.schoolID, tc.deviceID, tc.deviceType, nil, nil, testOperatorID, operatorIP)

			assert.Nil(t, device)
			var invalid *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &invalid)
			assert.Zero(t, h.tx.adminRuns)
		})
	}

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
		"deleted school": {
			seed: func(h *provisioningHarness) { h.engine.schools[10].DeletedAt = deletedNow() },
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
			},
		},
		"inactive school": {
			seed: func(h *provisioningHarness) { h.engine.schools[10].Active = false },
			check: func(t *testing.T, err error) {
				var inactive *organizationtenancy.SchoolInactiveError
				require.ErrorAs(t, err, &inactive)
				assert.Equal(t, int64(10), inactive.SchoolID)
			},
		},
		"school lookup failure": {
			seed:  func(h *provisioningHarness) { h.engine.fail["FindSchoolByID"] = errBoom },
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			if tc.seed != nil {
				seedDeviceSchools(h)
				tc.seed(h)
			}

			device, err := h.svc.CreateDevice(context.Background(), 10, "BURBACH-1", "terminal", nil, nil, testOperatorID, operatorIP)

			assert.Nil(t, device)
			tc.check(t, err)
			assert.Empty(t, h.devices.created)
			assert.Empty(t, h.audit.entries)
		})
	}
}

func TestProvisioningSetDeviceAPIKey(t *testing.T) {
	t.Parallel()

	seed := func(h *provisioningHarness) {
		seedDeviceSchools(h)
		h.devices.add(terminal(1, 10, "BURBACH-1"))
		h.devices.add(terminal(2, 20, "WALBACH-1"))
	}

	t.Run("rotates to a generated key", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)
		h.secrets.keys = []string{"dev_rotated-key"}

		device, err := h.svc.SetDeviceAPIKey(context.Background(), 1, nil, testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, device.APIKey)
		assert.Equal(t, "dev_rotated-key", *device.APIKey)
		assert.Equal(t, "BURBACH-1", device.DeviceID)
		assert.Equal(t, "Burbach", device.SchoolName)
		require.Len(t, h.devices.updated, 1)
		assert.Equal(t, int64(1), h.devices.updated[0].ID)
		assert.Equal(t, int64(10), h.devices.updated[0].TenantID)
		entry := h.onlyAudit(t, domain.AuditActionRotateAPIKey, domain.AuditResourceDevice, 1)
		assert.Equal(t, map[string]any{
			"device_id": "BURBACH-1", "school_id": float64(10), "api_key_mode": "auto",
		}, auditChanges(t, entry))
	})

	t.Run("sets the operator's key", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)

		device, err := h.svc.SetDeviceAPIKey(context.Background(), 1, strPtr("manual-key"), testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, "manual-key", *device.APIKey)
		assert.Zero(t, h.secrets.calls)
		assert.Equal(t, "manual", auditChanges(t, h.audit.entries[0])["api_key_mode"])
	})

	t.Run("a taken operator key is a conflict without retry", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)

		device, err := h.svc.SetDeviceAPIKey(context.Background(), 1, strPtr("dev_WALBACH-1-secret-key"), testOperatorID, operatorIP)

		assert.Nil(t, device)
		var conflict *organizationtenancy.ProvisioningConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Len(t, h.devices.updated, 1)
		assert.Equal(t, "dev_BURBACH-1-secret-key", *h.devices.devices[1].APIKey)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("regenerates a colliding generated key", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)
		h.secrets.keys = []string{"dev_WALBACH-1-secret-key", "dev_fresh"}

		device, err := h.svc.SetDeviceAPIKey(context.Background(), 1, nil, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, "dev_fresh", *device.APIKey)
		assert.Len(t, h.devices.updated, 2)
	})

	t.Run("gives up after three colliding keys", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)
		taken := domain.ErrDeviceAPIKeyTaken
		h.devices.updateErrs = []error{taken, taken, taken}

		device, err := h.svc.SetDeviceAPIKey(context.Background(), 1, nil, testOperatorID, operatorIP)

		assert.Nil(t, device)
		require.ErrorContains(t, err, "SetDeviceAPIKey: failed to generate unique API key after 3 attempts")
		assert.Len(t, h.devices.updated, 3)
		assert.Empty(t, h.audit.entries)
	})

	t.Run("unknown write error passes through", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)
		h.devices.updateErrs = []error{errBoom}

		_, err := h.svc.SetDeviceAPIKey(context.Background(), 1, nil, testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		assert.Len(t, h.devices.updated, 1)
	})

	t.Run("key generation failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)
		h.secrets.err = errBoom

		_, err := h.svc.SetDeviceAPIKey(context.Background(), 1, nil, testOperatorID, operatorIP)

		require.ErrorIs(t, err, errBoom)
		assert.Empty(t, h.devices.updated)
	})

	for _, id := range []int64{0, -1} {
		t.Run("rejects an invalid id", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)

			device, err := h.svc.SetDeviceAPIKey(context.Background(), id, nil, testOperatorID, operatorIP)

			assert.Nil(t, device)
			var invalid *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &invalid)
			assert.Zero(t, h.tx.adminRuns)
		})
	}

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		id    int64
		check func(*testing.T, error)
	}{
		"missing device": {
			id: 99,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OperatorDeviceNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(99), notFound.DeviceID)
			},
		},
		"archived device": {
			seed: func(h *provisioningHarness) { h.devices.devices[1].ArchivedAt = &fixedTime },
			id:   1,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OperatorDeviceNotFoundError
				require.ErrorAs(t, err, &notFound)
			},
		},
		"device lookup failure": {
			seed:  func(h *provisioningHarness) { h.devices.fail["FindDeviceForUpdate"] = errBoom },
			id:    1,
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"missing school": {
			seed: func(h *provisioningHarness) { delete(h.engine.schools, 10) },
			id:   1,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(10), notFound.SchoolID)
			},
		},
		"deleted school": {
			seed: func(h *provisioningHarness) { h.engine.schools[10].DeletedAt = deletedNow() },
			id:   1,
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(10), deleted.SchoolID)
			},
		},
		"inactive school": {
			seed: func(h *provisioningHarness) { h.engine.schools[10].Active = false },
			id:   1,
			check: func(t *testing.T, err error) {
				var inactive *organizationtenancy.SchoolInactiveError
				require.ErrorAs(t, err, &inactive)
				assert.Equal(t, int64(10), inactive.SchoolID)
			},
		},
		"school lookup failure is wrapped": {
			seed: func(h *provisioningHarness) { h.engine.fail["FindSchoolByID"] = errBoom },
			id:   1,
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "SetDeviceAPIKey: lookup school")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seed(h)
			if tc.seed != nil {
				tc.seed(h)
			}

			device, err := h.svc.SetDeviceAPIKey(context.Background(), tc.id, nil, testOperatorID, operatorIP)

			assert.Nil(t, device)
			tc.check(t, err)
			assert.Empty(t, h.devices.updated)
			assert.Empty(t, h.audit.entries)
		})
	}
}

func TestProvisioningDeleteDevice(t *testing.T) {
	t.Parallel()

	seed := func(h *provisioningHarness) {
		seedDeviceSchools(h)
		h.devices.add(terminal(1, 10, "BURBACH-1"))
		manual := terminal(2, 10, domain.WebManualDeviceID)
		manual.DeviceType = domain.DeviceTypeVirtual
		h.devices.add(manual)
	}

	t.Run("deletes the device and records it", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)

		require.NoError(t, h.svc.DeleteDevice(context.Background(), 1, testOperatorID, operatorIP))

		assert.Equal(t, []int64{1}, h.devices.deleted)
		entry := h.onlyAudit(t, domain.AuditActionDelete, domain.AuditResourceDevice, 1)
		assert.Equal(t, map[string]any{
			"device_id": "BURBACH-1", "device_type": "terminal", "school_id": float64(10),
		}, auditChanges(t, entry))
	})

	t.Run("audit failure does not fail the deletion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)
		h.audit.err = errBoom

		require.NoError(t, h.svc.DeleteDevice(context.Background(), 1, testOperatorID, operatorIP))
		assert.Equal(t, []int64{1}, h.devices.deleted)
	})

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		id    int64
		check func(*testing.T, error)
	}{
		"invalid id": {
			id: 0,
			check: func(t *testing.T, err error) {
				var invalid *organizationtenancy.InvalidProvisioningDataError
				require.ErrorAs(t, err, &invalid)
			},
		},
		"missing device": {
			id: 99,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OperatorDeviceNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(99), notFound.DeviceID)
			},
		},
		"device lookup failure": {
			seed:  func(h *provisioningHarness) { h.devices.fail["FindDeviceForUpdate"] = errBoom },
			id:    1,
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"manual web device is protected": {
			id: 2,
			check: func(t *testing.T, err error) {
				var protected *organizationtenancy.DeviceProtectedError
				require.ErrorAs(t, err, &protected)
				assert.Equal(t, int64(2), protected.DeviceID)
				assert.Equal(t, "system device required for manual web check-ins", protected.Reason)
			},
		},
		"referenced device is in use": {
			seed: func(h *provisioningHarness) { h.devices.fail["DeleteDevice"] = domain.ErrDeviceReferenced },
			id:   1,
			check: func(t *testing.T, err error) {
				var inUse *organizationtenancy.DeviceInUseError
				require.ErrorAs(t, err, &inUse)
				assert.Equal(t, int64(1), inUse.DeviceID)
			},
		},
		"unknown delete error is wrapped": {
			seed: func(h *provisioningHarness) { h.devices.fail["DeleteDevice"] = errBoom },
			id:   1,
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "DeleteDevice:")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seed(h)
			if tc.seed != nil {
				tc.seed(h)
			}

			err := h.svc.DeleteDevice(context.Background(), tc.id, testOperatorID, operatorIP)

			tc.check(t, err)
			assert.Empty(t, h.devices.deleted)
			assert.Empty(t, h.audit.entries)
		})
	}
}

func TestProvisioningGetDeviceTransferStatus(t *testing.T) {
	t.Parallel()

	seed := func(h *provisioningHarness, lastSeen *time.Time) {
		device := terminal(200, 10, "BURBACH-2")
		device.LastSeen = lastSeen
		h.devices.add(device)
	}

	t.Run("reports an online device with an open session", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		lastSeen := seenAgo(time.Minute)
		seed(h, lastSeen)
		startedAt := fixedTime.Add(-time.Hour)
		h.presence.sessions[200] = &domain.DeviceSession{
			ID: 300, StartedAt: startedAt, ActivityName: strPtr("Mensa"), RoomName: strPtr("Speisesaal"),
		}

		status, err := h.svc.GetDeviceTransferStatus(context.Background(), 200)

		require.NoError(t, err)
		assert.False(t, status.CanTransfer)
		assert.True(t, status.IsOnline)
		assert.False(t, status.IsProtected)
		assert.Equal(t, lastSeen, status.LastSeen)
		require.NotNil(t, status.ActiveSession)
		assert.Equal(t, organizationtenancy.DeviceTransferSession{
			ID: 300, StartedAt: startedAt, ActivityName: strPtr("Mensa"), RoomName: strPtr("Speisesaal"),
		}, *status.ActiveSession)
		assert.Equal(t, [][2]int64{{10, 200}}, h.presence.sessionLookups, "the session is read at the device's school")
	})

	t.Run("an idle offline device can move", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h, nil)

		status, err := h.svc.GetDeviceTransferStatus(context.Background(), 200)

		require.NoError(t, err)
		assert.True(t, status.CanTransfer)
		assert.False(t, status.IsOnline)
		assert.Nil(t, status.ActiveSession)
		assert.Empty(t, h.settings.tenants, "a device never seen needs no online window")
	})

	t.Run("the manual web device is protected", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		manual := terminal(200, 10, domain.WebManualDeviceID)
		manual.LastSeen = seenAgo(time.Minute)
		h.devices.add(manual)

		status, err := h.svc.GetDeviceTransferStatus(context.Background(), 200)

		require.NoError(t, err)
		assert.True(t, status.IsProtected)
		assert.False(t, status.CanTransfer)
		assert.False(t, status.IsOnline)
		assert.Empty(t, h.presence.sessionLookups)
		assert.Empty(t, h.settings.tenants)
	})

	t.Run("uses the school's online window outside the admin transaction", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h, seenAgo(10*time.Minute))
		h.settings.minutes = 15

		status, err := h.svc.GetDeviceTransferStatus(context.Background(), 200)

		require.NoError(t, err)
		assert.Equal(t, []int64{10}, h.settings.tenants)
		assert.False(t, h.settings.adminCalled, "the setting resolves in its own tenant transaction")
		assert.True(t, status.IsOnline, "seen 10 minutes ago is inside a 15-minute window")
		assert.False(t, status.CanTransfer)
	})

	for name, settings := range map[string]fakeSettings{
		"resolve failure": {err: errBoom},
		"zero minutes":    {minutes: 0},
		"negative":        {minutes: -3},
	} {
		t.Run("falls back to five minutes on "+name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seed(h, seenAgo(10*time.Minute))
			*h.settings = settings

			status, err := h.svc.GetDeviceTransferStatus(context.Background(), 200)

			require.NoError(t, err)
			assert.False(t, status.IsOnline)
			assert.True(t, status.CanTransfer)
		})
	}

	t.Run("session lookup failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h, nil)
		h.presence.sessionErr = errBoom

		status, err := h.svc.GetDeviceTransferStatus(context.Background(), 200)

		assert.Nil(t, status)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		status, err := h.svc.GetDeviceTransferStatus(context.Background(), 0)

		assert.Nil(t, status)
		var invalid *organizationtenancy.InvalidProvisioningDataError
		require.ErrorAs(t, err, &invalid)
		assert.Zero(t, h.tx.adminRuns)
	})

	t.Run("missing device", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		status, err := h.svc.GetDeviceTransferStatus(context.Background(), 200)

		assert.Nil(t, status)
		var notFound *organizationtenancy.OperatorDeviceNotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, int64(200), notFound.DeviceID)
	})

	t.Run("device lookup failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.devices.fail["FindDeviceForUpdate"] = errBoom

		_, err := h.svc.GetDeviceTransferStatus(context.Background(), 200)

		require.ErrorIs(t, err, errBoom)
	})
}

func TestProvisioningTransferDevice(t *testing.T) {
	t.Parallel()

	seed := func(h *provisioningHarness) *domain.Device {
		seedDeviceSchools(h)
		source := terminal(200, 10, "BURBACH-2")
		registeredBy, room := int64(77), int64(88)
		source.RegisteredByID = &registeredBy
		source.RoomID = &room
		source.LastSeen = seenAgo(time.Hour)
		h.devices.add(source)
		return h.devices.devices[200]
	}

	t.Run("archives the source and moves identity and key", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		source := seed(h)
		originalKey := *source.APIKey

		result, err := h.svc.TransferDevice(context.Background(), 200, 20, testOperatorID, operatorIP)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, h.devices.created, 1)
		target := h.devices.created[0]
		assert.Equal(t, domain.NewDevice{
			TenantID: 20, DeviceID: "BURBACH-2", DeviceType: "terminal",
			Name: source.Name, Status: domain.DeviceStatusActive, APIKey: &originalKey,
		}, target)

		assert.Equal(t, int64(501), result.ID)
		assert.Equal(t, "BURBACH-2", result.DeviceID)
		assert.Equal(t, int64(20), result.SchoolID)
		assert.Equal(t, "Walbach", result.SchoolName)
		assert.Nil(t, result.LastSeen, "the new row starts without the source's presence")
		require.NotNil(t, result.APIKey)
		assert.Equal(t, originalKey, *result.APIKey)

		require.Len(t, h.devices.updated, 2)
		archived := h.devices.updated[0]
		require.NotNil(t, archived.ArchivedAt)
		assert.Nil(t, archived.APIKey)
		assert.Equal(t, domain.DeviceStatusInactive, archived.Status)
		assert.Nil(t, archived.RoomID)
		assert.Nil(t, archived.RegisteredByID)
		assert.Nil(t, archived.TransferredToDeviceID)
		linked := h.devices.updated[1]
		require.NotNil(t, linked.TransferredToDeviceID)
		assert.Equal(t, int64(501), *linked.TransferredToDeviceID)
		assert.Equal(t, archived.ArchivedAt, linked.ArchivedAt)

		entry := h.onlyAudit(t, domain.AuditActionTransfer, domain.AuditResourceDevice, 200)
		assert.Equal(t, map[string]any{
			"device_id": "BURBACH-2", "source_device_id": float64(200), "target_device_id": float64(501),
			"source_school_id": float64(10), "target_school_id": float64(20),
		}, auditChanges(t, entry))
		assert.Equal(t, []int64{10}, h.settings.tenants)
		assert.False(t, h.settings.adminCalled)
	})

	t.Run("an inactive source keeps its status at the target", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h).Status = domain.DeviceStatusInactive

		_, err := h.svc.TransferDevice(context.Background(), 200, 20, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, domain.DeviceStatusInactive, h.devices.created[0].Status)
	})

	t.Run("a deleted source school does not block the move", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seed(h)
		h.engine.schools[10].DeletedAt = deletedNow()

		result, err := h.svc.TransferDevice(context.Background(), 200, 20, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, int64(20), result.SchoolID)
	})

	for name, tc := range map[string]struct {
		id, target int64
	}{
		"missing device id": {0, 20},
		"missing target":    {200, 0},
		"negative ids":      {-1, -1},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)

			result, err := h.svc.TransferDevice(context.Background(), tc.id, tc.target, testOperatorID, operatorIP)

			assert.Nil(t, result)
			var invalid *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &invalid)
			assert.Zero(t, h.tx.adminRuns)
		})
	}

	for name, tc := range map[string]struct {
		seed   func(*provisioningHarness)
		id     int64
		target int64
		check  func(*testing.T, error)
	}{
		"missing device": {
			id: 999, target: 20,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OperatorDeviceNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(999), notFound.DeviceID)
			},
		},
		"manual web device": {
			seed: func(h *provisioningHarness) {
				h.devices.devices[200].DeviceID = domain.WebManualDeviceID
			},
			id: 200, target: 20,
			check: func(t *testing.T, err error) {
				var protected *organizationtenancy.DeviceTransferProtectedError
				require.ErrorAs(t, err, &protected)
				assert.Equal(t, int64(200), protected.DeviceID)
				assert.Equal(t, "system device required for manual web check-ins", protected.Reason)
			},
		},
		"same school": {
			id: 200, target: 10,
			check: func(t *testing.T, err error) {
				var same *organizationtenancy.DeviceTransferSameSchoolError
				require.ErrorAs(t, err, &same)
				assert.Equal(t, int64(10), same.SchoolID)
			},
		},
		"other organisation": {
			id: 200, target: 30,
			check: func(t *testing.T, err error) {
				var mismatch *organizationtenancy.DeviceTransferOrganizationMismatchError
				require.ErrorAs(t, err, &mismatch)
				assert.Equal(t, int64(10), mismatch.SourceSchoolID)
				assert.Equal(t, int64(30), mismatch.TargetSchoolID)
			},
		},
		"missing source school": {
			seed: func(h *provisioningHarness) { delete(h.engine.schools, 10) },
			id:   200, target: 20,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(10), notFound.SchoolID)
			},
		},
		"missing target school": {
			id: 200, target: 99,
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(99), notFound.SchoolID)
			},
		},
		"deleted target school": {
			seed: func(h *provisioningHarness) { h.engine.schools[20].DeletedAt = deletedNow() },
			id:   200, target: 20,
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(20), deleted.SchoolID)
			},
		},
		"inactive target school": {
			seed: func(h *provisioningHarness) { h.engine.schools[20].Active = false },
			id:   200, target: 20,
			check: func(t *testing.T, err error) {
				var inactive *organizationtenancy.SchoolInactiveError
				require.ErrorAs(t, err, &inactive)
				assert.Equal(t, int64(20), inactive.SchoolID)
			},
		},
		"school lookup failure is wrapped": {
			seed: func(h *provisioningHarness) { h.engine.fail["FindSchoolByID"] = errBoom },
			id:   200, target: 20,
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "TransferDevice: lookup source school")
			},
		},
		"online device": {
			seed: func(h *provisioningHarness) { h.devices.devices[200].LastSeen = seenAgo(time.Minute) },
			id:   200, target: 20,
			check: func(t *testing.T, err error) {
				var blocked *organizationtenancy.DeviceTransferBlockedError
				require.ErrorAs(t, err, &blocked)
				assert.Equal(t, int64(200), blocked.DeviceID)
				assert.Equal(t, organizationtenancy.DeviceTransferBlockedOnline, blocked.Reason)
			},
		},
		"open session": {
			seed: func(h *provisioningHarness) {
				h.presence.sessions[200] = &domain.DeviceSession{ID: 300, StartedAt: fixedTime}
			},
			id: 200, target: 20,
			check: func(t *testing.T, err error) {
				var blocked *organizationtenancy.DeviceTransferBlockedError
				require.ErrorAs(t, err, &blocked)
				assert.Equal(t, organizationtenancy.DeviceTransferBlockedActiveSession, blocked.Reason)
			},
		},
		"session lookup failure": {
			seed:   func(h *provisioningHarness) { h.presence.sessionErr = errBoom },
			id:     200,
			target: 20,
			check:  func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seed(h)
			if tc.seed != nil {
				tc.seed(h)
			}

			result, err := h.svc.TransferDevice(context.Background(), tc.id, tc.target, testOperatorID, operatorIP)

			assert.Nil(t, result)
			tc.check(t, err)
			assert.Empty(t, h.devices.updated, "a rejected transfer must not write")
			assert.Empty(t, h.devices.created, "a rejected transfer must not write")
			assert.Empty(t, h.audit.entries)
		})
	}

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"key taken at the target": {
			seed: func(h *provisioningHarness) { h.devices.createErrs = []error{domain.ErrDeviceAPIKeyTaken} },
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Contains(t, conflict.Error(), "api_key already in use")
			},
		},
		"device id taken at the target": {
			seed: func(h *provisioningHarness) { h.devices.add(terminal(300, 20, "BURBACH-2")) },
			check: func(t *testing.T, err error) {
				var conflict *organizationtenancy.ProvisioningConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Contains(t, conflict.Error(), "device_id already exists for target school")
			},
		},
		"target write failure": {
			seed: func(h *provisioningHarness) { h.devices.createErrs = []error{errBoom} },
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "TransferDevice: create target")
			},
		},
		"archive failure": {
			seed: func(h *provisioningHarness) { h.devices.updateErrs = []error{errBoom} },
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "TransferDevice: archive source")
			},
		},
		"history link failure": {
			seed: func(h *provisioningHarness) { h.devices.updateErrs = []error{nil, errBoom} },
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "TransferDevice: link source history")
			},
		},
	} {
		t.Run(name+" fails the transfer", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seed(h)
			tc.seed(h)

			result, err := h.svc.TransferDevice(context.Background(), 200, 20, testOperatorID, operatorIP)

			assert.Nil(t, result)
			tc.check(t, err)
			require.Error(t, h.tx.lastErr, "the archived source must roll back")
			assert.Empty(t, h.audit.entries)
		})
	}
}
