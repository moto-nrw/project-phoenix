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

// seedDashboard stores organisation 1 with live schools 10 and 20 and the
// deleted school 50, organisation 2 with school 30, and the deleted
// organisation 3 without schools. Devices and persons sit in every school.
func seedDashboard(h *provisioningHarness) {
	seedDeviceSchools(h)
	h.addOrg(3, "Papierkorb OGS", "papierkorb", true)
	deleted := h.addSchool(50, 1, "Geloescht", "geloescht")
	deleted.DeletedAt = deletedNow()
	h.devices.add(terminal(1, 10, "A"))
	h.devices.add(terminal(2, 10, "B"))
	h.devices.add(terminal(3, 20, "C"))
	h.devices.add(terminal(4, 30, "D"))
	h.devices.add(terminal(5, 50, "E"))
	h.people.counts = map[int64]int{10: 5, 20: 3, 30: 7, 50: 11}
	h.dashboard.accounts = map[int64]int{1: 6, 2: 2, 10: 4, 20: 2, 30: 2, 50: 1}
}

func TestProvisioningGetProvisioningStats(t *testing.T) {
	t.Parallel()

	t.Run("counts devices of live schools only", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)
		h.dashboard.counts = domain.DashboardCounts{Organizations: 3, Schools: 7, Accounts: 12}

		stats, err := h.svc.GetProvisioningStats(context.Background())

		require.NoError(t, err)
		assert.Equal(t, &organizationtenancy.ProvisioningStats{
			TraegerCount: 3, SchulenCount: 7, KontenCount: 12, GeraeteCount: 4,
		}, stats)
	})

	for name, tc := range map[string]struct {
		fail    func(*provisioningHarness)
		message string
	}{
		"counts":        {func(h *provisioningHarness) { h.dashboard.fail["Counts"] = errBoom }, ""},
		"device counts": {func(h *provisioningHarness) { h.devices.fail["CountDevicesByTenant"] = errBoom }, "count devices for provisioning stats"},
		"school rows":   {func(h *provisioningHarness) { h.dashboard.fail["SchoolSummaries"] = errBoom }, "load schools for provisioning stats"},
	} {
		t.Run(name+" failure", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDashboard(h)
			tc.fail(h)

			stats, err := h.svc.GetProvisioningStats(context.Background())

			assert.Nil(t, stats)
			require.ErrorIs(t, err, errBoom)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestProvisioningListOrganizationSummaries(t *testing.T) {
	t.Parallel()

	t.Run("adds the device and person counts of live schools", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)

		summaries, err := h.svc.ListOrganizationSummaries(context.Background())

		require.NoError(t, err)
		require.Len(t, summaries, 3)
		assert.Equal(t, &organizationtenancy.OrganizationSummary{
			ID: 1, Name: "Talent OGS", Slug: "talent", Active: true, CreatedAt: fixedTime, UpdatedAt: fixedTime,
			SchulenCount: 2, KontenCount: 6, GeraeteCount: 3, PersonenCount: 8,
		}, summaries[0])
		assert.Equal(t, 1, summaries[1].GeraeteCount)
		assert.Equal(t, 7, summaries[1].PersonenCount)
		assert.Equal(t, int64(3), summaries[2].ID)
		require.NotNil(t, summaries[2].DeletedAt, "deleted organisations stay listed")
		assert.Zero(t, summaries[2].SchulenCount)
		assert.Zero(t, summaries[2].GeraeteCount)
	})

	t.Run("no organisations reads no counts", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.devices.fail["CountDevicesByTenant"] = errBoom

		summaries, err := h.svc.ListOrganizationSummaries(context.Background())

		require.NoError(t, err)
		assert.NotNil(t, summaries)
		assert.Empty(t, summaries)
		assert.Empty(t, h.people.calls)
	})

	for name, tc := range map[string]struct {
		fail    func(*provisioningHarness)
		message string
	}{
		"organisation rows": {func(h *provisioningHarness) { h.dashboard.fail["OrganizationSummaries"] = errBoom }, ""},
		"device counts":     {func(h *provisioningHarness) { h.devices.fail["CountDevicesByTenant"] = errBoom }, "count devices for organization summaries"},
		"person counts":     {func(h *provisioningHarness) { h.people.fail["CountPersonsByTenant"] = errBoom }, "count persons for organization summaries"},
		"school rows":       {func(h *provisioningHarness) { h.dashboard.fail["SchoolSummaries"] = errBoom }, "load schools for organization summaries"},
	} {
		t.Run(name+" failure", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDashboard(h)
			tc.fail(h)

			summaries, err := h.svc.ListOrganizationSummaries(context.Background())

			assert.Nil(t, summaries)
			require.ErrorIs(t, err, errBoom)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestProvisioningListSchoolSummaries(t *testing.T) {
	t.Parallel()

	t.Run("lists every school with its counts", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)
		school := h.engine.schools[10]
		school.Address, school.City, school.Zip = "Hauptstr. 1", "Burbach", "57299"
		school.Phone, school.Email, school.Settings, school.Hidden = "0271", "gs@burbach.test", `{"a":1}`, true

		summaries, err := h.svc.ListSchoolSummaries(context.Background())

		require.NoError(t, err)
		require.Len(t, summaries, 4)
		assert.Equal(t, &organizationtenancy.SchoolSummary{
			ID: 10, OrganizationID: 1, OrganizationName: "Talent OGS", Name: "Burbach", Slug: "burbach",
			Subdomain: "burbach", Active: true, Hidden: true, CreatedAt: fixedTime, UpdatedAt: fixedTime,
			Address: "Hauptstr. 1", City: "Burbach", Zip: "57299", Phone: "0271", Email: "gs@burbach.test",
			Settings: `{"a":1}`, KontenCount: 4, GeraeteCount: 2, PersonenCount: 5,
		}, summaries[0])
		assert.Equal(t, int64(50), summaries[3].ID)
		require.NotNil(t, summaries[3].DeletedAt, "deleted schools stay listed")
		assert.Equal(t, 1, summaries[3].GeraeteCount)
		assert.Equal(t, 11, summaries[3].PersonenCount)
		require.Len(t, h.dashboard.schoolFilter, 1)
		assert.Nil(t, h.dashboard.schoolFilter[0])
	})

	t.Run("no schools reads no counts", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		summaries, err := h.svc.ListSchoolSummaries(context.Background())

		require.NoError(t, err)
		assert.NotNil(t, summaries)
		assert.Empty(t, summaries)
		assert.Empty(t, h.people.calls)
	})

	for name, tc := range map[string]struct {
		fail    func(*provisioningHarness)
		message string
	}{
		"school rows":   {func(h *provisioningHarness) { h.dashboard.fail["SchoolSummaries"] = errBoom }, ""},
		"device counts": {func(h *provisioningHarness) { h.devices.fail["CountDevicesByTenant"] = errBoom }, "count devices for school summaries"},
		"person counts": {func(h *provisioningHarness) { h.people.fail["CountPersonsByTenant"] = errBoom }, "count persons for school summaries"},
	} {
		t.Run(name+" failure", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDashboard(h)
			tc.fail(h)

			summaries, err := h.svc.ListSchoolSummaries(context.Background())

			assert.Nil(t, summaries)
			require.ErrorIs(t, err, errBoom)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestProvisioningListOrganizationSchoolSummaries(t *testing.T) {
	t.Parallel()

	t.Run("lists the organisation's schools including deleted ones", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)

		summaries, err := h.svc.ListOrganizationSchoolSummaries(context.Background(), 1)

		require.NoError(t, err)
		require.Len(t, summaries, 3)
		assert.Equal(t, []int64{10, 20, 50}, []int64{summaries[0].ID, summaries[1].ID, summaries[2].ID})
		require.Len(t, h.dashboard.schoolFilter, 1)
		require.NotNil(t, h.dashboard.schoolFilter[0])
		assert.Equal(t, int64(1), *h.dashboard.schoolFilter[0])
	})

	t.Run("a deleted organisation still lists its schools", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)

		summaries, err := h.svc.ListOrganizationSchoolSummaries(context.Background(), 3)

		require.NoError(t, err)
		assert.Empty(t, summaries)
	})

	t.Run("missing organisation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		summaries, err := h.svc.ListOrganizationSchoolSummaries(context.Background(), 99)

		assert.Nil(t, summaries)
		var notFound *organizationtenancy.OrganizationNotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, int64(99), notFound.OrganizationID)
		assert.Empty(t, h.dashboard.schoolFilter)
	})

	t.Run("organisation lookup failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)
		h.engine.fail["FindByID"] = errBoom

		summaries, err := h.svc.ListOrganizationSchoolSummaries(context.Background(), 1)

		assert.Nil(t, summaries)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("school rows failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)
		h.dashboard.fail["SchoolSummaries"] = errBoom

		summaries, err := h.svc.ListOrganizationSchoolSummaries(context.Background(), 1)

		assert.Nil(t, summaries)
		require.ErrorIs(t, err, errBoom)
	})
}

func TestProvisioningGetSchoolPWAUsage(t *testing.T) {
	t.Parallel()

	t.Run("maps portal rows into the response shape", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)
		h.dashboard.pwaRows = []domain.PWAUsageRow{
			{TenantID: 10, Portal: "staff", StandaloneUsers: 3, EligibleUsers: 12},
			{TenantID: 10, Portal: "parent", StandaloneUsers: 47, EligibleUsers: 210},
			{TenantID: 10, Portal: "kiosk", StandaloneUsers: 99, EligibleUsers: 99},
		}

		usage, err := h.svc.GetSchoolPWAUsage(context.Background(), 10)

		require.NoError(t, err)
		assert.Equal(t, int64(10), h.dashboard.pwaTenantID)
		assert.Equal(t, 30*24*time.Hour, h.dashboard.pwaWindow)
		assert.Equal(t, &organizationtenancy.SchoolPWAUsage{
			WindowDays: 30,
			Staff:      organizationtenancy.PWAPortalUsage{StandaloneUsers: 3, EligibleUsers: 12},
			Parent:     organizationtenancy.PWAPortalUsage{StandaloneUsers: 47, EligibleUsers: 210},
		}, usage)
	})

	t.Run("missing buckets stay zero", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)

		usage, err := h.svc.GetSchoolPWAUsage(context.Background(), 10)

		require.NoError(t, err)
		assert.Equal(t, &organizationtenancy.SchoolPWAUsage{WindowDays: 30}, usage)
	})

	t.Run("a deleted school still reports its usage", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)

		_, err := h.svc.GetSchoolPWAUsage(context.Background(), 50)

		require.NoError(t, err)
		assert.Equal(t, int64(50), h.dashboard.pwaTenantID)
	})

	t.Run("unknown school", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		usage, err := h.svc.GetSchoolPWAUsage(context.Background(), 999)

		assert.Nil(t, usage)
		var notFound *organizationtenancy.SchoolNotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, int64(999), notFound.SchoolID)
		assert.Zero(t, h.dashboard.pwaTenantID)
	})

	t.Run("invalid school id is not found", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.svc.GetSchoolPWAUsage(context.Background(), 0)

		var notFound *organizationtenancy.SchoolNotFoundError
		require.ErrorAs(t, err, &notFound)
	})

	t.Run("usage failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDashboard(h)
		h.dashboard.fail["PWAUsage"] = errBoom

		usage, err := h.svc.GetSchoolPWAUsage(context.Background(), 10)

		assert.Nil(t, usage)
		require.ErrorIs(t, err, errBoom)
	})
}

func TestProvisioningListPWAUsage(t *testing.T) {
	t.Parallel()

	t.Run("returns every school's buckets for the window", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.dashboard.pwaTenantID = -1
		h.dashboard.pwaRows = []domain.PWAUsageRow{
			{TenantID: 10, Portal: "staff", StandaloneUsers: 1, EligibleUsers: 2},
			{TenantID: 20, Portal: "parent", StandaloneUsers: 3, EligibleUsers: 4},
		}

		rows, err := h.svc.ListPWAUsage(context.Background(), 7*24*time.Hour)

		require.NoError(t, err)
		assert.Equal(t, []organizationtenancy.SchoolPWAUsageRow{
			{TenantID: 10, Portal: "staff", StandaloneUsers: 1, EligibleUsers: 2},
			{TenantID: 20, Portal: "parent", StandaloneUsers: 3, EligibleUsers: 4},
		}, rows)
		assert.Zero(t, h.dashboard.pwaTenantID, "tenant 0 asks for every school")
		assert.Equal(t, 7*24*time.Hour, h.dashboard.pwaWindow)
	})

	t.Run("no usage is an empty list", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		rows, err := h.svc.ListPWAUsage(context.Background(), time.Hour)

		require.NoError(t, err)
		assert.NotNil(t, rows)
		assert.Empty(t, rows)
	})

	t.Run("usage failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.dashboard.fail["PWAUsage"] = errBoom

		rows, err := h.svc.ListPWAUsage(context.Background(), time.Hour)

		assert.Nil(t, rows)
		require.ErrorIs(t, err, errBoom)
	})
}
