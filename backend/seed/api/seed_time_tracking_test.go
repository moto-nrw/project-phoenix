package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	t.Run("rejects payload without id", func(t *testing.T) {
		payload := []byte(`{"status":"success","data":{}}`)
		_, err := extractSessionID(payload)
		assert.Error(t, err)
	})

	t.Run("rejects unparseable JSON", func(t *testing.T) {
		_, err := extractSessionID([]byte("not json"))
		assert.Error(t, err)
	})
}
