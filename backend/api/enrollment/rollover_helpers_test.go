package enrollment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// --- parseDateField -------------------------------------------------------

func TestParseDateField_HappyPath(t *testing.T) {
	t.Parallel()

	got, err := parseDateField("service_start_date", "2027-09-01")
	require.NoError(t, err)
	assert.Equal(t, timezone.NewDate(2027, 9, 1), got)
}

func TestParseDateField_RejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := parseDateField("service_start_date", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_start_date is required",
		"error must include the field name so the admin can correct it")
}

func TestParseDateField_RejectsRFC3339(t *testing.T) {
	t.Parallel()

	// parseDateField is for DATE-typed columns. RFC3339 input must be
	// rejected so we don't silently lop off the time portion.
	_, err := parseDateField("service_start_date", "2027-09-01T00:00:00Z")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestParseDateField_RejectsBogusString(t *testing.T) {
	t.Parallel()

	_, err := parseDateField("svc", "not a date")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "svc must be YYYY-MM-DD")
}

// --- parseDateTimeField ---------------------------------------------------

func TestParseDateTimeField_HappyPath(t *testing.T) {
	t.Parallel()

	got, err := parseDateTimeField("enrollment_open_at", "2027-08-01T08:30:00Z")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2027, 8, 1, 8, 30, 0, 0, time.UTC), got)
}

func TestParseDateTimeField_PreservesOffset(t *testing.T) {
	t.Parallel()

	got, err := parseDateTimeField("enrollment_open_at", "2027-08-01T08:30:00+02:00")
	require.NoError(t, err)
	// The parsed Time keeps its zone offset; converting to UTC should
	// land at 06:30.
	assert.Equal(t, 8, got.Hour(), "hour stays in the original zone")
	assert.Equal(t, 6, got.UTC().Hour(), "UTC shift matches the +02:00 offset")
}

func TestParseDateTimeField_RejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := parseDateTimeField("rollover_deadline", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_deadline is required")
}

func TestParseDateTimeField_RejectsPlainDate(t *testing.T) {
	t.Parallel()

	// RFC3339 requires the T + time portion. A bare date must fail
	// loudly so the form sends a deadline-with-time, not just a day.
	_, err := parseDateTimeField("rollover_deadline", "2027-09-01")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RFC3339")
}

// --- parseRolloverCreateRequest -------------------------------------------

func validBody() *RolloverCreateRequest {
	return &RolloverCreateRequest{
		Name:             "Schuljahr 2027/28",
		ServiceStartDate: "2027-09-01",
		ServiceEndDate:   "2028-07-31",
		RolloverDeadline: "2027-07-15T18:00:00Z",
		RolloverMode:     enrollmentModels.PhaseRolloverModeOptOut,
	}
}

func TestParseRolloverCreateRequest_HappyPath(t *testing.T) {
	t.Parallel()

	req, err := parseRolloverCreateRequest(42, validBody(), 99)
	require.NoError(t, err)
	assert.Equal(t, int64(42), req.SourcePhaseID)
	assert.Equal(t, "Schuljahr 2027/28", req.Name)
	assert.Equal(t, enrollmentModels.PhaseKindSchoolYear, req.Kind, "kind defaults to school_year")
	assert.Equal(t, enrollmentModels.PhaseRolloverModeOptOut, req.RolloverMode)
	assert.True(t, req.RolloverBumpsGrade, "bumps_grade defaults to true (yearly cadence)")
	assert.Equal(t, int64(99), req.AdminAccountID)
	assert.Equal(t, timezone.NewDate(2027, 9, 1), req.ServiceStartDate)
	assert.Equal(t, timezone.NewDate(2028, 7, 31), req.ServiceEndDate)
	assert.Equal(t, time.Date(2027, 7, 15, 18, 0, 0, 0, time.UTC), req.RolloverDeadline)
}

func TestParseRolloverCreateRequest_BumpsGradeFalseHonored(t *testing.T) {
	t.Parallel()

	body := validBody()
	f := false
	body.RolloverBumpsGrade = &f
	req, err := parseRolloverCreateRequest(42, body, 99)
	require.NoError(t, err)
	assert.False(t, req.RolloverBumpsGrade)
}

func TestParseRolloverCreateRequest_KindPassesThrough(t *testing.T) {
	t.Parallel()

	body := validBody()
	body.Kind = enrollmentModels.PhaseKindHoliday
	req, err := parseRolloverCreateRequest(42, body, 99)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.PhaseKindHoliday, req.Kind)
}

func TestParseRolloverCreateRequest_OptionalOpenAtSet(t *testing.T) {
	t.Parallel()

	body := validBody()
	openStr := "2027-08-01T00:00:00Z"
	body.EnrollmentOpenAt = &openStr
	req, err := parseRolloverCreateRequest(42, body, 99)
	require.NoError(t, err)
	require.NotNil(t, req.EnrollmentOpenAt)
	assert.Equal(t, time.Date(2027, 8, 1, 0, 0, 0, 0, time.UTC), *req.EnrollmentOpenAt)
}

func TestParseRolloverCreateRequest_OptionalOpenAtBlankStringStaysNil(t *testing.T) {
	t.Parallel()

	// Treating the empty string as "not provided" lets the frontend send
	// an empty input field without forcing the user to clear it
	// entirely. Matches the "if raw == "" { return nil }" branch.
	body := validBody()
	empty := ""
	body.EnrollmentOpenAt = &empty
	req, err := parseRolloverCreateRequest(42, body, 99)
	require.NoError(t, err)
	assert.Nil(t, req.EnrollmentOpenAt)
}

func TestParseRolloverCreateRequest_OptionalOpenAtBadValueErrors(t *testing.T) {
	t.Parallel()

	body := validBody()
	bad := "yesterday"
	body.EnrollmentOpenAt = &bad
	_, err := parseRolloverCreateRequest(42, body, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment_open_at")
}

func TestParseRolloverCreateRequest_OptionalCloseAtSet(t *testing.T) {
	t.Parallel()

	body := validBody()
	closeStr := "2027-08-31T23:59:00Z"
	body.EnrollmentCloseAt = &closeStr
	req, err := parseRolloverCreateRequest(42, body, 99)
	require.NoError(t, err)
	require.NotNil(t, req.EnrollmentCloseAt)
	assert.Equal(t, 23, req.EnrollmentCloseAt.Hour())
}

func TestParseRolloverCreateRequest_OptionalCloseAtBadValueErrors(t *testing.T) {
	t.Parallel()

	body := validBody()
	bad := "not-iso"
	body.EnrollmentCloseAt = &bad
	_, err := parseRolloverCreateRequest(42, body, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment_close_at")
}

func TestParseRolloverCreateRequest_FormSchemaIDPassesThrough(t *testing.T) {
	t.Parallel()

	body := validBody()
	id := int64(7777)
	body.FormSchemaID = &id
	req, err := parseRolloverCreateRequest(42, body, 99)
	require.NoError(t, err)
	require.NotNil(t, req.FormSchemaID)
	assert.Equal(t, int64(7777), *req.FormSchemaID)
}

func TestParseRolloverCreateRequest_NilFormSchemaIDMeansInherit(t *testing.T) {
	t.Parallel()

	req, err := parseRolloverCreateRequest(42, validBody(), 99)
	require.NoError(t, err)
	assert.Nil(t, req.FormSchemaID, "nil = copy source phase's schema id (per docstring)")
}

func TestParseRolloverCreateRequest_AutoApprovePassesThrough(t *testing.T) {
	t.Parallel()

	body := validBody()
	body.RolloverAutoApprove = true
	req, err := parseRolloverCreateRequest(42, body, 99)
	require.NoError(t, err)
	assert.True(t, req.RolloverAutoApprove)
}

func TestParseRolloverCreateRequest_RejectsBadStartDate(t *testing.T) {
	t.Parallel()

	body := validBody()
	body.ServiceStartDate = "yesterday"
	_, err := parseRolloverCreateRequest(42, body, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_start_date")
}

func TestParseRolloverCreateRequest_RejectsBadEndDate(t *testing.T) {
	t.Parallel()

	body := validBody()
	body.ServiceEndDate = "tomorrow"
	_, err := parseRolloverCreateRequest(42, body, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_end_date")
}

func TestParseRolloverCreateRequest_RejectsBadDeadline(t *testing.T) {
	t.Parallel()

	body := validBody()
	body.RolloverDeadline = "soon"
	_, err := parseRolloverCreateRequest(42, body, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_deadline")
}
