package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// schoolLoginImageURL extracts the school's brand logo URL from the
// JSON settings blob on platform.schools.settings. The function is
// pure (no DB) and must fail closed on malformed input — a broken
// settings JSON means "no school logo", not a crash inside the
// outbox worker.

func TestSchoolLoginImageURL_EmptyStringReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", schoolLoginImageURL(""))
}

func TestSchoolLoginImageURL_WhitespaceOnlyReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", schoolLoginImageURL("   "))
}

func TestSchoolLoginImageURL_InvalidJSONReturnsEmpty(t *testing.T) {
	t.Parallel()

	// A broken settings blob must not crash the renderer — the email
	// just goes out without a school logo. The MOTO logo always fires.
	assert.Equal(t, "", schoolLoginImageURL("{not valid"))
}

func TestSchoolLoginImageURL_NoLoginImageKeyReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", schoolLoginImageURL(`{"otherKey":"x"}`))
}

func TestSchoolLoginImageURL_LoginImageKeyNotStringReturnsEmpty(t *testing.T) {
	t.Parallel()

	// JSON allows any type for the value; coerce-or-empty so the
	// renderer always gets a string back.
	assert.Equal(t, "", schoolLoginImageURL(`{"loginImageUrl":42}`))
}

func TestSchoolLoginImageURL_HappyPath(t *testing.T) {
	t.Parallel()

	got := schoolLoginImageURL(`{"loginImageUrl":"/uploads/login-images/2_V3EjlEtM.jpg"}`)
	assert.Equal(t, "/uploads/login-images/2_V3EjlEtM.jpg", got)
}

func TestSchoolLoginImageURL_TrimsWhitespaceFromValue(t *testing.T) {
	t.Parallel()

	got := schoolLoginImageURL(`{"loginImageUrl":"  /uploads/login-images/x.jpg  "}`)
	assert.Equal(t, "/uploads/login-images/x.jpg", got)
}

// motoLogoURL pins the moto logo path against baseURL — the renderer
// always sets a MOTO logo even when the school has no custom logo,
// so the bottom of every email has consistent branding.

func TestMotoLogoURL_AppendsImagePath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://parents.localhost:3000/images/moto-logo-mit-schriftzug.png",
		motoLogoURL("https://parents.localhost:3000"))
}

func TestMotoLogoURL_TrimsTrailingSlash(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://parents.localhost:3000/images/moto-logo-mit-schriftzug.png",
		motoLogoURL("https://parents.localhost:3000/"))
}

func TestMotoLogoURL_EmptyBaseReturnsRelative(t *testing.T) {
	t.Parallel()

	// No base URL → return the relative path. The email template will
	// render a broken image; the test pins that the function doesn't
	// silently inject "http://localhost" or similar.
	assert.Equal(t, "/images/moto-logo-mit-schriftzug.png", motoLogoURL(""))
}

// absoluteBrandURL resolves brand assets against the parents-portal
// base URL. Already-absolute URLs pass through unchanged so a
// custom CDN works without extra config.

func TestAbsoluteBrandURL_EmptyURLReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", absoluteBrandURL("https://example.test", ""))
}

func TestAbsoluteBrandURL_WhitespaceURLReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", absoluteBrandURL("https://example.test", "   "))
}

func TestAbsoluteBrandURL_HTTPSPassesThrough(t *testing.T) {
	t.Parallel()

	got := absoluteBrandURL("https://example.test", "https://cdn.example/logo.png")
	assert.Equal(t, "https://cdn.example/logo.png", got)
}

func TestAbsoluteBrandURL_HTTPPassesThrough(t *testing.T) {
	t.Parallel()

	got := absoluteBrandURL("https://example.test", "http://cdn.example/logo.png")
	assert.Equal(t, "http://cdn.example/logo.png", got)
}

func TestAbsoluteBrandURL_RelativeWithLeadingSlash(t *testing.T) {
	t.Parallel()

	got := absoluteBrandURL("https://example.test", "/logo.png")
	assert.Equal(t, "https://example.test/logo.png", got)
}

func TestAbsoluteBrandURL_RelativeWithoutLeadingSlash(t *testing.T) {
	t.Parallel()

	got := absoluteBrandURL("https://example.test", "logo.png")
	assert.Equal(t, "https://example.test/logo.png", got)
}

func TestAbsoluteBrandURL_TrimsTrailingSlashOnBase(t *testing.T) {
	t.Parallel()

	got := absoluteBrandURL("https://example.test/", "/logo.png")
	assert.Equal(t, "https://example.test/logo.png", got,
		"trailing slash on base must not produce double-slash in result")
}

func TestAbsoluteBrandURL_EmptyBaseReturnsRawURL(t *testing.T) {
	t.Parallel()

	// No base, relative path → return as-is. The email renderer treats
	// this as "broken image" rather than crashing.
	got := absoluteBrandURL("", "/logo.png")
	assert.Equal(t, "/logo.png", got)
}

func TestAbsoluteBrandURL_TrimsBaseWhitespace(t *testing.T) {
	t.Parallel()

	got := absoluteBrandURL("  https://example.test  ", "/logo.png")
	assert.Equal(t, "https://example.test/logo.png", got)
}
