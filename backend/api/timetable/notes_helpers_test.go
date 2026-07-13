// Pure unit tests for the Wochennotiz/Tagesnotiz helpers (#1837 follow-up):
// normalizeNotes (trim -> nil-or-value) and the split flow's presence-aware
// nullableStr JSON decoder. No database required.
package timetable

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeNotes(t *testing.T) {
	trimmed := "Raum erst ab 14 Uhr offen"

	cases := []struct {
		name string
		in   *string
		want *string
	}{
		{"nil stays nil", nil, nil},
		{"empty becomes nil", ptr(""), nil},
		{"whitespace only becomes nil", ptr("   \t\n "), nil},
		{"value is trimmed", ptr("  " + trimmed + "  "), &trimmed},
		{"already-clean value kept", ptr(trimmed), &trimmed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeNotes(tc.in)
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, *tc.want, *got)
		})
	}
}

func ptr(s string) *string { return &s }

func TestNullableStr_UnmarshalJSON(t *testing.T) {
	t.Run("omitted field leaves Set=false", func(t *testing.T) {
		// A struct whose nullableStr field is absent from the JSON must keep the
		// zero value (Set=false) so the split treats it as "inherit".
		var payload struct {
			Notes nullableStr `json:"notes"`
		}
		require.NoError(t, json.Unmarshal([]byte(`{}`), &payload))
		assert.False(t, payload.Notes.Set)
		assert.Nil(t, payload.Notes.Value)
	})

	t.Run("explicit null sets Set=true, Value=nil (clear)", func(t *testing.T) {
		var n nullableStr
		require.NoError(t, n.UnmarshalJSON([]byte(`null`)))
		assert.True(t, n.Set)
		assert.Nil(t, n.Value)
	})

	t.Run("string value sets Set=true and Value", func(t *testing.T) {
		var n nullableStr
		require.NoError(t, n.UnmarshalJSON([]byte(`"Wochennotiz"`)))
		assert.True(t, n.Set)
		require.NotNil(t, n.Value)
		assert.Equal(t, "Wochennotiz", *n.Value)
	})

	t.Run("invalid JSON returns an error", func(t *testing.T) {
		var n nullableStr
		require.Error(t, n.UnmarshalJSON([]byte(`{not-a-string`)))
	})
}
