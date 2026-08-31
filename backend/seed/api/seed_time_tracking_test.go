package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMostRecentWeekday(t *testing.T) {
	t.Parallel()
	loc := time.UTC

	tests := []struct {
		name   string
		from   time.Time
		target time.Weekday
		want   time.Time
	}{
		{
			name:   "from Monday targeting Monday returns same day",
			from:   time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			target: time.Monday,
			want:   time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Wednesday targeting Tuesday returns yesterday",
			from:   time.Date(2026, 5, 6, 0, 0, 0, 0, loc),
			target: time.Tuesday,
			want:   time.Date(2026, 5, 5, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Tuesday targeting Wednesday wraps to previous week",
			from:   time.Date(2026, 5, 5, 0, 0, 0, 0, loc),
			target: time.Wednesday,
			want:   time.Date(2026, 4, 29, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Sunday targeting Monday returns previous Monday",
			from:   time.Date(2026, 5, 3, 0, 0, 0, 0, loc),
			target: time.Monday,
			want:   time.Date(2026, 4, 27, 0, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mostRecentWeekday(tt.from, tt.target)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.target, got.Weekday(),
				"resolved date must land on the requested weekday")
		})
	}
}

func TestNextWeekday(t *testing.T) {
	t.Parallel()
	loc := time.UTC

	tests := []struct {
		name   string
		from   time.Time
		target time.Weekday
		want   time.Time
	}{
		{
			name:   "from Monday targeting Tuesday returns tomorrow",
			from:   time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			target: time.Tuesday,
			want:   time.Date(2026, 5, 5, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Tuesday targeting Tuesday returns same day",
			from:   time.Date(2026, 5, 5, 0, 0, 0, 0, loc),
			target: time.Tuesday,
			want:   time.Date(2026, 5, 5, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Friday targeting Tuesday wraps to next week",
			from:   time.Date(2026, 5, 8, 0, 0, 0, 0, loc),
			target: time.Tuesday,
			want:   time.Date(2026, 5, 12, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Sunday targeting Tuesday returns upcoming Tuesday",
			from:   time.Date(2026, 5, 10, 0, 0, 0, 0, loc),
			target: time.Tuesday,
			want:   time.Date(2026, 5, 12, 0, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextWeekday(tt.from, tt.target)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.target, got.Weekday(),
				"resolved date must land on the requested weekday")
		})
	}
}

func TestToDateKey(t *testing.T) {
	t.Parallel()

	d := time.Date(2026, 5, 4, 12, 34, 56, 0, time.UTC)
	assert.Equal(t, "2026-05-04", toDateKey(d))
}

func TestExtractSessionID(t *testing.T) {
	t.Parallel()

	t.Run("extracts id from a wrapped success payload", func(t *testing.T) {
		payload := []byte(`{"status":"success","data":{"id":42,"staff_id":1}}`)
		id, err := extractSessionID(payload)
		assert.NoError(t, err)
		assert.Equal(t, int64(42), id)
	})

	// Work-session responses quote the id so an int64 past 2^53 survives
	// JSON.parse in the browser (#2402). The seeder walks the same public
	// endpoints, so it has to read the quoted form.
	t.Run("extracts id from a string-encoded id", func(t *testing.T) {
		payload := []byte(`{"status":"success","data":{"id":"9007199254740993","staff_id":1}}`)
		id, err := extractSessionID(payload)
		assert.NoError(t, err)
		assert.Equal(t, int64(9007199254740993), id)
	})

	t.Run("rejects payload without id", func(t *testing.T) {
		payload := []byte(`{"status":"success","data":{}}`)
		_, err := extractSessionID(payload)
		assert.Error(t, err)
	})

	t.Run("rejects a non-numeric id", func(t *testing.T) {
		payload := []byte(`{"status":"success","data":{"id":"abc"}}`)
		_, err := extractSessionID(payload)
		assert.Error(t, err)
	})

	t.Run("rejects unparseable JSON", func(t *testing.T) {
		_, err := extractSessionID([]byte("not json"))
		assert.Error(t, err)
	})
}

func TestExtractAbsenceID(t *testing.T) {
	t.Parallel()

	t.Run("extracts id from a wrapped success payload", func(t *testing.T) {
		payload := []byte(`{"status":"success","data":{"id":42,"absence_type":"vacation"}}`)
		id, err := extractAbsenceID(payload)
		assert.NoError(t, err)
		assert.Equal(t, int64(42), id)
	})

	t.Run("rejects payload without id", func(t *testing.T) {
		payload := []byte(`{"status":"success","data":{}}`)
		_, err := extractAbsenceID(payload)
		assert.Error(t, err)
	})

	t.Run("rejects unparseable JSON", func(t *testing.T) {
		_, err := extractAbsenceID([]byte("not json"))
		assert.Error(t, err)
	})
}

func TestSeedTimeTrackingCoverageCreatesQuotaOpeningAndBreak(t *testing.T) {
	t.Parallel()

	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"91"}}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Token: "admin"}}
	require.NoError(t, seedTimeTrackingCoverage(rt, 17, 2026))
	assert.Equal(t, []string{
		"/api/staff/17/vacation/quota",
		"/api/staff/17/time-tracking/opening",
		"/api/staff/17/time-tracking/adjustments",
		"/api/staff/17/time-tracking/adjustments/91",
	}, paths)

	paths = nil
	rt.Client.BindAuth(AuthRef{Token: "staff"})
	require.NoError(t, seedOneWorkSessionBreak(rt))
	assert.Equal(t, []string{
		"/api/time-tracking/break/start",
		"/api/time-tracking/break/end",
	}, paths)
}
