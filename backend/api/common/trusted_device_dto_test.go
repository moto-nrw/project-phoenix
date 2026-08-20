package common_test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// fixed reference time so the RFC3339 string assertions are stable.
var (
	testCreated  = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	testExpires  = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	testLastUsed = time.Date(2026, 5, 15, 9, 30, 0, 0, time.UTC)
)

func TestNewTrustedDeviceDTO_FullRow(t *testing.T) {
	t.Parallel()

	ua := "Mozilla/5.0 (testbot)"
	row := common.TrustedDeviceRow{
		ID:         42,
		UserAgent:  &ua,
		IPAddress:  net.ParseIP("203.0.113.10"),
		CreatedAt:  testCreated,
		ExpiresAt:  testExpires,
		LastUsedAt: &testLastUsed,
	}

	dto := common.NewTrustedDeviceDTO(row)

	assert.Equal(t, int64(42), dto.ID)
	assert.Equal(t, &ua, dto.UserAgent)
	assert.Equal(t, "203.0.113.10", dto.IPAddress)
	assert.Equal(t, "2026-05-01T12:00:00Z", dto.CreatedAt)
	assert.Equal(t, "2026-08-01T12:00:00Z", dto.ExpiresAt)
	if assert.NotNil(t, dto.LastUsedAt) {
		assert.Equal(t, "2026-05-15T09:30:00Z", *dto.LastUsedAt)
	}
}

func TestNewTrustedDeviceDTO_OmitsNilOptionalFields(t *testing.T) {
	t.Parallel()

	row := common.TrustedDeviceRow{
		ID:        7,
		CreatedAt: testCreated,
		ExpiresAt: testExpires,
		// UserAgent, IPAddress, LastUsedAt left nil — DTO must serialise
		// the omitempty fields as their empty values.
	}

	dto := common.NewTrustedDeviceDTO(row)

	assert.Equal(t, int64(7), dto.ID)
	assert.Nil(t, dto.UserAgent)
	assert.Equal(t, "", dto.IPAddress, "nil net.IP must not emit an address string")
	assert.Equal(t, "2026-05-01T12:00:00Z", dto.CreatedAt)
	assert.Equal(t, "2026-08-01T12:00:00Z", dto.ExpiresAt)
	assert.Nil(t, dto.LastUsedAt)
}

func TestNewTrustedDeviceDTO_NormalisesTimestampsToUTC(t *testing.T) {
	t.Parallel()

	// Build a non-UTC moment to verify the helper normalises to RFC3339-Z
	// rather than leaking the source location.
	cet, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("load CET: %v", err)
	}
	localCreated := time.Date(2026, 5, 1, 14, 0, 0, 0, cet) // 12:00 UTC
	row := common.TrustedDeviceRow{
		ID:        1,
		CreatedAt: localCreated,
		ExpiresAt: localCreated.Add(24 * time.Hour),
	}

	dto := common.NewTrustedDeviceDTO(row)

	assert.Equal(t, "2026-05-01T12:00:00Z", dto.CreatedAt)
	assert.Equal(t, "2026-05-02T12:00:00Z", dto.ExpiresAt)
}

func TestMapTrustedDevices_EmptyInput(t *testing.T) {
	t.Parallel()

	out := common.MapTrustedDevices(
		[]struct{ ID int64 }{},
		func(struct{ ID int64 }) common.TrustedDeviceRow {
			return common.TrustedDeviceRow{}
		},
	)
	assert.NotNil(t, out, "must return non-nil empty slice (callers may JSON-encode without nil-check)")
	assert.Empty(t, out)
}

func TestMapTrustedDevices_PreservesOrderAndProjection(t *testing.T) {
	t.Parallel()

	// Use a tiny stand-in struct so we don't pull a real model dependency.
	type fakeDevice struct {
		ID       int64
		UA       string
		Created  time.Time
		Expires  time.Time
		LastUsed *time.Time
	}
	ua1, ua2 := "agent-1", "agent-2"
	rows := []fakeDevice{
		{ID: 1, UA: ua1, Created: testCreated, Expires: testExpires, LastUsed: &testLastUsed},
		{ID: 2, UA: ua2, Created: testCreated.Add(time.Hour), Expires: testExpires},
	}

	out := common.MapTrustedDevices(rows, func(d fakeDevice) common.TrustedDeviceRow {
		return common.TrustedDeviceRow{
			ID:         d.ID,
			UserAgent:  &d.UA,
			CreatedAt:  d.Created,
			ExpiresAt:  d.Expires,
			LastUsedAt: d.LastUsed,
		}
	})

	if assert.Len(t, out, 2) {
		assert.Equal(t, int64(1), out[0].ID)
		assert.Equal(t, ua1, *out[0].UserAgent)
		assert.NotNil(t, out[0].LastUsedAt)
		assert.Equal(t, int64(2), out[1].ID)
		assert.Equal(t, ua2, *out[1].UserAgent)
		assert.Nil(t, out[1].LastUsedAt, "second row had nil LastUsed — must remain nil after projection")
	}
}
