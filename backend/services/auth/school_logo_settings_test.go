package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// schoolLogoURLFromSettings extracts the school logo URL from the
// JSON settings blob on platform.schools.settings. Used by the
// guardian invitation email renderer so the parent sees the school's
// branding. Two keys are checked in order: logoUrl, then loginImageUrl
// — the second is a legacy compatibility key for schools that only
// have the login screen image set up.

func TestSchoolLogoURLFromSettings_EmptyReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", schoolLogoURLFromSettings(""))
}

func TestSchoolLogoURLFromSettings_WhitespaceReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", schoolLogoURLFromSettings("   "))
}

func TestSchoolLogoURLFromSettings_BrokenJSONReturnsEmpty(t *testing.T) {
	t.Parallel()

	// Malformed settings JSON must not crash the email renderer — the
	// invitation just goes out without a school logo (the moto logo
	// always renders separately).
	assert.Equal(t, "", schoolLogoURLFromSettings("{not valid json"))
}

func TestSchoolLogoURLFromSettings_NeitherKeyReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", schoolLogoURLFromSettings(`{"other":"x"}`))
}

func TestSchoolLogoURLFromSettings_LogoURLKeyWins(t *testing.T) {
	t.Parallel()

	// When both keys are present, logoUrl takes precedence — it's the
	// purpose-built field; loginImageUrl is the legacy fallback.
	got := schoolLogoURLFromSettings(`{"logoUrl":"https://logo","loginImageUrl":"https://login"}`)
	assert.Equal(t, "https://logo", got)
}

func TestSchoolLogoURLFromSettings_LoginImageURLFallback(t *testing.T) {
	t.Parallel()

	// Schools migrated from the older single-image-per-school setup
	// only have loginImageUrl. Use it when logoUrl is missing so we
	// still render their branding.
	got := schoolLogoURLFromSettings(`{"loginImageUrl":"https://login"}`)
	assert.Equal(t, "https://login", got)
}

func TestSchoolLogoURLFromSettings_TrimsValue(t *testing.T) {
	t.Parallel()

	got := schoolLogoURLFromSettings(`{"logoUrl":"  https://logo  "}`)
	assert.Equal(t, "https://logo", got)
}

func TestSchoolLogoURLFromSettings_NonStringValueFallsThrough(t *testing.T) {
	t.Parallel()

	// JSON allows the value to be any type; non-string triggers the
	// type-assertion fallthrough so the other key gets a chance.
	got := schoolLogoURLFromSettings(`{"logoUrl":42,"loginImageUrl":"https://login"}`)
	assert.Equal(t, "https://login", got,
		"non-string logoUrl must fall through to loginImageUrl")
}

func TestSchoolLogoURLFromSettings_EmptyStringFallsThrough(t *testing.T) {
	t.Parallel()

	// Empty string is treated as "not configured" so the legacy key
	// gets a chance, same as the missing-key path.
	got := schoolLogoURLFromSettings(`{"logoUrl":"","loginImageUrl":"https://login"}`)
	assert.Equal(t, "https://login", got)
}
