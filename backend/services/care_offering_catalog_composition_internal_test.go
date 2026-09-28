package services

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The catalog interprets an offering's translation document with
// Enrollment's rules through this binding (#3377, #3559); the catalog turns
// a refusal into its invalid-configuration error.
func TestCareOfferingCatalogTranslations(t *testing.T) {
	t.Parallel()

	t.Run("stores the canonical document", func(t *testing.T) {
		t.Parallel()
		normalized, ok, err := careOfferingCatalogTranslations{}.NormalizeTranslations(
			json.RawMessage(`{"RU":{"name":{"text":" Продлёнка ","source":"OGS"},"description":{"text":" "}}}`))
		require.NoError(t, err)
		require.True(t, ok)
		assert.JSONEq(t, `{"ru":{"name":{"text":"Продлёнка","source":"OGS"}}}`, string(normalized))
	})

	t.Run("stores nothing when no translation remains", func(t *testing.T) {
		t.Parallel()
		normalized, ok, err := careOfferingCatalogTranslations{}.NormalizeTranslations(json.RawMessage(`{"ru":{"name":{"text":""}}}`))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Nil(t, normalized)
	})

	t.Run("refuses malformed and foreign documents", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{`[]`, `{"ru":{"label":{"text":"x"}}}`, `{"de":{"name":{"text":"x"}}}`} {
			_, ok, err := careOfferingCatalogTranslations{}.NormalizeTranslations(json.RawMessage(raw))
			assert.True(t, !ok || err != nil, raw)
		}
	})
}
