package email_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The school-brand header of the enrollment mails names the school's care
// with its own term (#6042): the logo stand-in and the name stand-in say
// "GTS" for a school that chose it, and "OGS" for every other school.
func TestSchoolBrandTemplate_CareFacilityTerm(t *testing.T) {
	t.Parallel()

	templatesDir, err := filepath.Abs(".")
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	require.NoError(t, err)

	render := func(data map[string]any) string {
		t.Helper()
		var buf bytes.Buffer
		require.NoError(t, tpl.ExecuteTemplate(&buf, "enrollment-submitted.html", data))
		return buf.String()
	}
	base := func() map[string]any {
		return map[string]any{
			"GuardianFirstName": "Erika", "GuardianLastName": "Mustermann",
			"StatusURL": "https://parents.example.com/status/abc",
		}
	}

	chosen := base()
	chosen["CareFacilityTerm"] = "GTS"
	chosen["CareFacilityBadge"] = "GTS"
	out := render(chosen)
	assert.True(t, strings.Contains(out, `<div class="school-logo-fallback">GTS</div>`), "logo stand-in names GTS")
	assert.True(t, strings.Contains(out, `<p class="school-name">Ihre GTS</p>`), "name stand-in names GTS")
	assert.False(t, strings.Contains(out, ">OGS<"), "no OGS stand-in left")
	assert.False(t, strings.Contains(out, "Ihre OGS"), "no OGS name stand-in left")

	escaped := base()
	escaped["CareFacilityTerm"] = "<b>GTS</b>"
	escaped["CareFacilityBadge"] = "<b>"
	assert.False(t, strings.Contains(render(escaped), "<b>GTS</b>"), "a term is text, never markup")

	// A long own name keeps the short badge and the full name as text.
	long := base()
	long["CareFacilityTerm"] = "Betreuung an der Schule am Markt"
	long["CareFacilityBadge"] = "BAD"
	out = render(long)
	assert.True(t, strings.Contains(out, `<div class="school-logo-fallback">BAD</div>`), "badge stays short")
	assert.True(t, strings.Contains(out, `<p class="school-name">Ihre Betreuung an der Schule am Markt</p>`), "full name as text")

	unchanged := render(base())
	assert.True(t, strings.Contains(unchanged, `<div class="school-logo-fallback">OGS</div>`), "default logo stand-in")
	assert.True(t, strings.Contains(unchanged, `<p class="school-name">Ihre OGS</p>`), "default name stand-in")
}
