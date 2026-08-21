package enrollment

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/stretchr/testify/assert"
)

// schoolEmailFrom controls who "the email is from" in the parent's
// inbox. A blank school name must fall back to the default sender —
// no school = use the moto noreply address as the From name too. The
// address never changes — DMARC requires that.

func TestSchoolEmailFrom_EmptySchoolFallsBackToDefault(t *testing.T) {
	t.Parallel()

	def := email.NewEmail("moto noreply", "noreply@moto-app.de")
	got := schoolEmailFrom(def, "")
	assert.Equal(t, def, got)
}

func TestSchoolEmailFrom_WhitespaceSchoolFallsBackToDefault(t *testing.T) {
	t.Parallel()

	def := email.NewEmail("moto noreply", "noreply@moto-app.de")
	got := schoolEmailFrom(def, "   ")
	assert.Equal(t, def, got)
}

func TestSchoolEmailFrom_SchoolNameReplacesName(t *testing.T) {
	t.Parallel()

	def := email.NewEmail("moto noreply", "noreply@moto-app.de")
	got := schoolEmailFrom(def, "OGS Sonnenschule")
	assert.Equal(t, "OGS Sonnenschule", got.Name)
	assert.Equal(t, "noreply@moto-app.de", got.Address, "address never changes — DMARC")
}

func TestSchoolEmailFrom_TrimsSchoolName(t *testing.T) {
	t.Parallel()

	def := email.NewEmail("moto noreply", "noreply@moto-app.de")
	got := schoolEmailFrom(def, "  Schule  ")
	assert.Equal(t, "Schule", got.Name)
}

// payloadStringSlice handles the JSON-roundtrip case where an outbox
// payload column comes back as []any after json.Unmarshal. The
// renderer needs []string, so we coerce element-by-element and skip
// non-string values rather than panicking.

func TestPayloadStringSlice_MissingKey(t *testing.T) {
	t.Parallel()

	assert.Nil(t, payloadStringSlice(map[string]any{"other": "x"}, "names"))
}

func TestPayloadStringSlice_NilPayload(t *testing.T) {
	t.Parallel()

	assert.Nil(t, payloadStringSlice(nil, "names"))
}

func TestPayloadStringSlice_DirectStringSlice(t *testing.T) {
	t.Parallel()

	got := payloadStringSlice(map[string]any{
		"names": []string{"Anna", "Bert"},
	}, "names")
	assert.Equal(t, []string{"Anna", "Bert"}, got)
}

func TestPayloadStringSlice_JSONRoundtripAnySlice(t *testing.T) {
	t.Parallel()

	got := payloadStringSlice(map[string]any{
		"names": []any{"Anna", "Bert"},
	}, "names")
	assert.Equal(t, []string{"Anna", "Bert"}, got)
}

func TestPayloadStringSlice_MixedAnySliceDropsNonStrings(t *testing.T) {
	t.Parallel()

	got := payloadStringSlice(map[string]any{
		"names": []any{"Anna", 42, nil, "Bert"},
	}, "names")
	assert.Equal(t, []string{"Anna", "Bert"}, got)
}

func TestPayloadStringSlice_WrongTypeReturnsNil(t *testing.T) {
	t.Parallel()

	got := payloadStringSlice(map[string]any{
		"names": "just one string",
	}, "names")
	assert.Nil(t, got, "non-slice value must return nil rather than panic")
}

func TestPayloadStringSlice_EmptyAnySlice(t *testing.T) {
	t.Parallel()

	got := payloadStringSlice(map[string]any{"names": []any{}}, "names")
	assert.Empty(t, got)
	assert.NotNil(t, got, "empty []any must yield a (zero-len) []string, not nil")
}
