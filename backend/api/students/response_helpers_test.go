package students

// Unit tests for populatePhotoFields — the helper that decides how a
// student's photo URL and consent metadata surface in the JSON response.
// Pinning these branches matters because they are the GDPR boundary the
// list and detail responses share: skipping a check here would leak photo
// URLs to staff outside the student's group, or leave URLs visible after
// a school disabled the photo feature.

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopulatePublicStudentFields_PreservesNonCanonicalPickupStatus(t *testing.T) {
	t.Parallel()

	status := "Bitte nur an Oma oder Opa übergeben"
	student := &users.Student{
		PickupStatus: &status,
		DepartureDays: users.DepartureDays{
			users.PickupDayMonday: users.DepartureBus,
		},
	}
	resp := StudentResponse{}

	populatePublicStudentFields(&resp, student)

	assert.Equal(t, status, resp.PickupStatus)
	assert.Equal(t, users.DepartureBus, resp.DepartureDays.ModeFor(users.PickupDayMonday))
	assert.True(t, resp.BusDays[users.PickupDayMonday])
	assert.True(t, resp.Bus)
}

func TestPopulateSnapshotPublicFields_PreservesNonCanonicalPickupStatus(t *testing.T) {
	t.Parallel()

	status := "Abholung nur nach telefonischer Rücksprache"
	student := &users.Student{
		PickupStatus: &status,
		DepartureDays: users.DepartureDays{
			users.PickupDayWednesday: users.DeparturePickup,
		},
	}
	resp := StudentResponse{}

	populateSnapshotPublicFields(&resp, student)

	assert.Equal(t, status, resp.PickupStatus)
	assert.Equal(t, users.DeparturePickup, resp.DepartureDays.ModeFor(users.PickupDayWednesday))
	assert.True(t, resp.PickupDays[users.PickupDayWednesday])
}

func TestResponsePickupStatus_UsesDerivedStatusForCanonicalStoredText(t *testing.T) {
	t.Parallel()

	status := users.PickupStatusPickedUp
	student := &users.Student{
		PickupStatus:  &status,
		DepartureDays: users.DepartureDays{},
	}
	resp := StudentResponse{}

	populatePublicStudentFields(&resp, student)

	assert.Equal(t, users.PickupStatusGoesAlone, resp.PickupStatus)
	assert.False(t, resp.PickupDays.HasAny())
	assert.True(t, resp.DepartureRuleConfigured)
}

func TestPopulatePublicStudentFields_LeavesMissingDepartureRuleUnconfigured(t *testing.T) {
	t.Parallel()

	resp := StudentResponse{}

	populatePublicStudentFields(&resp, &users.Student{})

	assert.False(t, resp.DepartureRuleConfigured)
}

// TestPopulatePublicStudentFields_BusAndAccompaniedSameDayKeepsAccompanied is
// the read-path guard for #1694. A child allowed BOTH Bus and "Mit anderem
// Kind" on the same weekday is a self-goer in the exclusive DepartureDays()
// projection (bus outranks accompanied there), so deriving the legacy
// pickup_status from that projection would return this safety-sensitive child
// to legacy list/search/admin consumers as "Geht alleine nach Hause". The
// response must derive from the FULL non-exclusive AllowedDepartureModes set
// instead, even though the stored canonical status gets recomputed.
func TestPopulatePublicStudentFields_BusAndAccompaniedSameDayKeepsAccompanied(t *testing.T) {
	t.Parallel()

	stored := users.PickupStatusAccompanied
	student := &users.Student{
		PickupStatus: &stored,
		AllowedDepartureModes: users.AllowedDepartureModes{
			users.PickupDayMonday: {users.DepartureBus, users.DepartureAccompanied},
		},
	}
	resp := StudentResponse{}

	populatePublicStudentFields(&resp, student)

	assert.Equal(t, users.PickupStatusAccompanied, resp.PickupStatus,
		"bus+accompanied on the same day must not be bucketed as a self-goer")
	assert.True(t, resp.BusDays[users.PickupDayMonday], "the bus signal must still surface")
}

func TestPopulateSnapshotPublicFields_BusAndAccompaniedSameDayKeepsAccompanied(t *testing.T) {
	t.Parallel()

	stored := users.PickupStatusAccompanied
	student := &users.Student{
		PickupStatus: &stored,
		AllowedDepartureModes: users.AllowedDepartureModes{
			users.PickupDayTuesday: {users.DepartureBus, users.DepartureAccompanied},
		},
	}
	resp := StudentResponse{}

	populateSnapshotPublicFields(&resp, student)

	assert.Equal(t, users.PickupStatusAccompanied, resp.PickupStatus,
		"bus+accompanied on the same day must not be bucketed as a self-goer")
	assert.True(t, resp.BusDays[users.PickupDayTuesday], "the bus signal must still surface")
	assert.True(t, resp.DepartureRuleConfigured)
}

func TestPopulateStudentAddressFields(t *testing.T) {
	t.Parallel()

	street := "Musterstraße 12"
	city := "Köln"
	postalCode := "50667"
	student := &users.Student{
		AddressStreet:     &street,
		AddressCity:       &city,
		AddressPostalCode: &postalCode,
	}
	resp := StudentResponse{}

	populateStudentAddressFields(&resp, student)

	assert.Equal(t, street, resp.AddressStreet)
	assert.Equal(t, city, resp.AddressCity)
	assert.Equal(t, postalCode, resp.AddressPostalCode)
}

// helper: build a Student with a given ID + photo state.
func makeStudent(id int64, photoPath *string, consentAt *time.Time, consentBy *int64) *users.Student {
	s := &users.Student{
		PhotoPath:           photoPath,
		PhotoConsentGivenAt: consentAt,
		PhotoConsentGivenBy: consentBy,
	}
	s.ID = id
	return s
}

// TestPopulatePhotoFields_FeatureDisabled — when the tenant has the
// photo feature off, the helper MUST leave every photo-related field at
// its zero value so callers cannot infer state from the response. In
// particular PhotoConsentGiven (a *bool) must remain nil so the
// frontend can't confuse "feature off" with "consent withdrawn".
func TestPopulatePhotoFields_FeatureDisabled(t *testing.T) {
	t.Parallel()

	now := time.Now()
	by := int64(42)
	url := "/uploads/student-photos/x.jpg"
	student := makeStudent(7, &url, &now, &by)
	resp := StudentResponse{}

	populatePhotoFields(&resp, student, false, true)

	assert.Empty(t, resp.PhotoURL, "feature off must suppress photo_url")
	assert.Nil(t, resp.PhotoConsentGiven, "feature off must leave consent flag unset (nil)")
	assert.Nil(t, resp.PhotoConsentGivenAt)
	assert.Nil(t, resp.PhotoConsentGivenBy)
}

// TestPopulatePhotoFields_FeatureEnabled_NoFullAccess — even with
// the feature on, a caller without full access to this student must not
// see the photo URL. The boolean PhotoConsentGiven flag may surface (it
// isn't GDPR-sensitive on its own — the URL is the byte gate). This
// mirrors what serveStudentPhoto enforces, so list/detail JSON never
// hands out a URL that the same session would 403 against.
func TestPopulatePhotoFields_FeatureEnabled_NoFullAccess(t *testing.T) {
	t.Parallel()

	now := time.Now()
	by := int64(42)
	url := "/uploads/student-photos/abc.jpg"
	student := makeStudent(11, &url, &now, &by)
	resp := StudentResponse{}

	populatePhotoFields(&resp, student, true, false /* hasFullAccess */)

	assert.Empty(t, resp.PhotoURL, "no full access must suppress photo_url even when feature is on")
	require.NotNil(t, resp.PhotoConsentGiven, "consent flag must still surface so the UI can render the badge")
	assert.True(t, *resp.PhotoConsentGiven)
	require.NotNil(t, resp.PhotoConsentGivenAt)
	assert.True(t, resp.PhotoConsentGivenAt.Equal(now))
	require.NotNil(t, resp.PhotoConsentGivenBy)
	assert.Equal(t, int64(42), *resp.PhotoConsentGivenBy)
}

// TestPopulatePhotoFields_FullAccess_PhotoAndConsent — the canonical
// happy path: feature on, caller has full access, student has both a
// photo and recorded consent. The URL must be the rewritten,
// authenticated /api/students/{id}/photo/{filename} form so the
// browser can fetch via cookie auth — never the raw /uploads/... path
// stored in the DB.
func TestPopulatePhotoFields_FullAccess_PhotoAndConsent(t *testing.T) {
	t.Parallel()

	now := time.Now()
	by := int64(99)
	url := "/uploads/student-photos/foo.jpg"
	student := makeStudent(123, &url, &now, &by)
	resp := StudentResponse{}

	populatePhotoFields(&resp, student, true, true)

	assert.Equal(t, "/api/students/123/photo/foo.jpg", resp.PhotoURL,
		"full-access response must serialize the rewritten serve URL, not the raw stored path")
	require.NotNil(t, resp.PhotoConsentGiven)
	assert.True(t, *resp.PhotoConsentGiven)
	require.NotNil(t, resp.PhotoConsentGivenAt)
	assert.True(t, resp.PhotoConsentGivenAt.Equal(now))
	require.NotNil(t, resp.PhotoConsentGivenBy)
	assert.Equal(t, by, *resp.PhotoConsentGivenBy)
}

// TestPopulatePhotoFields_FullAccess_ConsentNoPhoto — staff member
// recorded consent but never uploaded an actual photo. The URL must
// remain empty (omitempty drops it), but the consent_given flag must
// be true so the UI can show the green checkmark even before bytes
// exist on disk.
func TestPopulatePhotoFields_FullAccess_ConsentNoPhoto(t *testing.T) {
	t.Parallel()

	now := time.Now()
	by := int64(7)
	student := makeStudent(55, nil, &now, &by)
	resp := StudentResponse{}

	populatePhotoFields(&resp, student, true, true)

	assert.Empty(t, resp.PhotoURL)
	require.NotNil(t, resp.PhotoConsentGiven)
	assert.True(t, *resp.PhotoConsentGiven, "consent without a photo must still surface as true")
	require.NotNil(t, resp.PhotoConsentGivenAt)
	require.NotNil(t, resp.PhotoConsentGivenBy)
}

// TestPopulatePhotoFields_FullAccess_NoConsent — feature on, full
// access, but the parent has not recorded consent. PhotoURL must
// remain empty and PhotoConsentGiven must be a non-nil *bool pointing
// at false so the frontend can render the checkbox unticked. The nil
// case is reserved for "feature off" (TestPopulatePhotoFields_FeatureDisabled).
func TestPopulatePhotoFields_FullAccess_NoConsent(t *testing.T) {
	t.Parallel()

	student := makeStudent(8, nil, nil, nil)
	resp := StudentResponse{}

	populatePhotoFields(&resp, student, true, true)

	assert.Empty(t, resp.PhotoURL)
	require.NotNil(t, resp.PhotoConsentGiven, "feature on with no consent must surface false (not nil)")
	assert.False(t, *resp.PhotoConsentGiven,
		"nil reserved for feature-off; explicit false here lets the UI distinguish 'never consented' from 'feature off'")
	assert.Nil(t, resp.PhotoConsentGivenAt)
	assert.Nil(t, resp.PhotoConsentGivenBy)
}

// TestPopulatePhotoFields_FullAccess_PhotoButNoConsentAt — defensive:
// a row with a photo_path but no consent timestamp would be a bug in
// the upload path, but the helper must still shape a sensible
// response: surface the URL (since hasFullAccess=true and the byte
// gate won't 403), and emit consent_given=false so the UI can flag
// the broken state instead of silently hiding it.
func TestPopulatePhotoFields_FullAccess_PhotoButNoConsentAt(t *testing.T) {
	t.Parallel()

	url := "/uploads/student-photos/orphan.jpg"
	student := makeStudent(2, &url, nil, nil)
	resp := StudentResponse{}

	populatePhotoFields(&resp, student, true, true)

	assert.Equal(t, "/api/students/2/photo/orphan.jpg", resp.PhotoURL,
		"hasFullAccess + photo_path must still surface the URL even when consent metadata is missing")
	require.NotNil(t, resp.PhotoConsentGiven)
	assert.False(t, *resp.PhotoConsentGiven)
	assert.Nil(t, resp.PhotoConsentGivenAt)
	assert.Nil(t, resp.PhotoConsentGivenBy)
}
