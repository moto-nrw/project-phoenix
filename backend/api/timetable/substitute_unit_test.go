// Pure unit tests for the substitute helpers — no DB. Complements the
// DB-backed integration coverage in substitute_test.go.
package timetable

import (
	"strings"
	"testing"
	"unicode/utf8"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strptr(s string) *string { return &s }

func TestTrimReason(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		assert.Nil(t, trimReason(nil))
	})
	t.Run("blank normalizes to nil", func(t *testing.T) {
		assert.Nil(t, trimReason(strptr("   ")))
	})
	t.Run("trims surrounding whitespace", func(t *testing.T) {
		got := trimReason(strptr("  krank  "))
		require.NotNil(t, got)
		assert.Equal(t, "krank", *got)
	})
	t.Run("caps by rune count, not byte length", func(t *testing.T) {
		got := trimReason(strptr(strings.Repeat("x", understaffedAckNoteMaxLength+50)))
		require.NotNil(t, got)
		assert.Equal(t, understaffedAckNoteMaxLength, utf8.RuneCountInString(*got))
	})
	t.Run("multi-byte truncation stays valid UTF-8", func(t *testing.T) {
		// 499 ASCII + a 2-byte rune is 501 bytes but 500 runes: a byte-slice at
		// 500 would split the final rune and produce invalid UTF-8 that
		// PostgreSQL rejects. A rune-count cap keeps the whole string.
		input := strings.Repeat("a", understaffedAckNoteMaxLength-1) + "ä"
		got := trimReason(strptr(input))
		require.NotNil(t, got)
		assert.True(t, utf8.ValidString(*got), "truncated reason must be valid UTF-8")
		assert.Equal(t, input, *got, "a 500-rune value is under the cap and kept intact")
	})
	t.Run("over-cap multi-byte truncation never splits a rune", func(t *testing.T) {
		// All multi-byte runes, well over the cap. Truncation must land on a
		// rune boundary regardless of byte offsets.
		got := trimReason(strptr(strings.Repeat("ä", understaffedAckNoteMaxLength+10)))
		require.NotNil(t, got)
		assert.True(t, utf8.ValidString(*got))
		assert.Equal(t, understaffedAckNoteMaxLength, utf8.RuneCountInString(*got))
	})
}

func TestIsPlannableInstance(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{scheduleModel.InstanceStatusPlanned, true},
		{scheduleModel.InstanceStatusActive, true},
		{scheduleModel.InstanceStatusCompleted, false},
		{scheduleModel.InstanceStatusCancelled, false},
	}
	for _, tc := range cases {
		inst := &scheduleModel.ActivityInstance{Status: tc.status}
		assert.Equalf(t, tc.want, isPlannableInstance(inst), "status=%s", tc.status)
	}
}
