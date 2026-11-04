package defaults

import (
	"math"
	"strconv"
	"testing"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBrandingColorPresetsAreReadableOnWhite holds the decision of #5239:
// six to eight preset colours for the letters of the Träger, each with a
// contrast of at least 4.5:1 to white paper (WCAG 2.2 AA for normal text),
// and the default among them so a Träger without a choice sees it marked.
func TestBrandingColorPresetsAreReadableOnWhite(t *testing.T) {
	t.Parallel()
	definition := config.DefaultRegistry().GetDefinition(config.KeyBrandingPrimaryColor)
	require.NotNil(t, definition)
	require.NotNil(t, definition.Options)
	presets := definition.Options.Static
	assert.GreaterOrEqual(t, len(presets), 6)
	assert.LessOrEqual(t, len(presets), 8)

	seenValues := map[string]bool{}
	seenLabels := map[string]bool{}
	for _, preset := range presets {
		value, ok := preset.Value.(string)
		require.True(t, ok, preset.Label)
		assert.False(t, seenValues[value], "duplicate colour %s", value)
		assert.False(t, seenLabels[preset.Label], "duplicate name %s", preset.Label)
		seenValues[value], seenLabels[preset.Label] = true, true
		assert.GreaterOrEqual(t, contrastToWhite(t, value), 4.5, "%s %s", preset.Label, value)
	}
	assert.True(t, seenValues[definition.Default.(string)], "default is a preset")
	assert.NotContains(t, definition.Description, "Farbcode")
}

// The contrast formula itself, against values the WCAG reference tools give.
func TestContrastToWhite(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 21.0, contrastToWhite(t, "#000000"), 0.01)
	assert.InDelta(t, 1.0, contrastToWhite(t, "#FFFFFF"), 0.01)
	assert.InDelta(t, 4.54, contrastToWhite(t, "#767676"), 0.01)
}

// contrastToWhite is the WCAG 2.2 contrast ratio of an #RRGGBB colour to
// white: (1 + 0.05) / (L + 0.05) with L the relative luminance.
func contrastToWhite(t *testing.T, hex string) float64 {
	t.Helper()
	require.Len(t, hex, 7)
	weights := [3]float64{0.2126, 0.7152, 0.0722}
	luminance := 0.0
	for i, weight := range weights {
		channel, err := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		require.NoError(t, err)
		c := float64(channel) / 255
		if c <= 0.04045 {
			c /= 12.92
		} else {
			c = math.Pow((c+0.055)/1.055, 2.4)
		}
		luminance += weight * c
	}
	return 1.05 / (luminance + 0.05)
}
