package staff

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	authmodel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { testutil.SeedTestJWTConfig() }

type pinAccountStub struct {
	hasPIN    bool
	verifyPIN func(string) bool
}

func (a pinAccountStub) HasPIN() bool {
	return a.hasPIN
}

func (a pinAccountStub) VerifyPIN(pin string) bool {
	if a.verifyPIN != nil {
		return a.verifyPIN(pin)
	}
	return false
}

func TestResource_GetLogger(t *testing.T) {
	t.Parallel()

	customLogger := slog.Default()

	assert.Same(t, customLogger, (&Resource{logger: customLogger}).getLogger())
	assert.NotNil(t, (&Resource{}).getLogger())
}

func TestResource_Router(t *testing.T) {
	t.Parallel()
	router := (&Resource{}).Router()
	require.NotNil(t, router)
}

func TestResource_CrossStaffTimeAndAbsenceReadsRejectUsersRead(t *testing.T) {
	t.Parallel()
	claims := testutil.DefaultTestClaims()
	claims.Roles = []string{"user"}
	claims.Permissions = []string{"users:read"}
	claims.IsAdmin = false
	// Mint before building the router: MintTestJWT resolves this test's own
	// tenant (#2419), which initializes the test DB and loads .env — and with
	// it AUTH_JWT_SECRET. A router built before that load would sign-check
	// against a different secret than the token carries.
	token := testutil.MintTestJWT(t, claims)
	resource := &Resource{}
	router := resource.Router()

	cases := []string{
		"/123/time-tracking/history?from=2026-06-01&to=2026-06-07",
		"/123/time-tracking/export?from=2026-06-01&to=2026-06-07&format=csv",
		"/absences/pending",
		"/123/vacation/quota?year=2026",
		"/123/time-tracking/sessions/456/edits",
		"/123/absences?from=2026-06-01&to=2026-06-07",
		"/123/schedule",
	}
	for _, path := range cases {
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token))
		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertForbidden(t, rr)
	}
}

// A caller on the personnel tier sees the whole record.
func TestNewStaffResponse_PersonnelTierSeesFullRecord(t *testing.T) {
	t.Parallel()

	employmentType := users.EmploymentTypePartTime
	tagID := "04A1B2C3"
	response := buildStaffResponse(staffFieldAccess{notes: true, qualifications: true, personnel: true}, &users.Staff{
		Model:          base.Model{ID: 42},
		PersonID:       420,
		StaffNotes:     "Vertretung montags",
		EmploymentType: &employmentType,
		Person:         &users.Person{Model: base.Model{ID: 420}, FirstName: "Mara", LastName: "Kühn", TagID: &tagID},
	}, false, false, "", "sick", "user", "mara@example.com", "")

	assert.Equal(t, "Vertretung montags", response.StaffNotes)
	assert.Equal(t, "sick", response.AbsenceType)
	require.NotNil(t, response.EmploymentType)
	require.NotNil(t, response.Person)
	assert.Equal(t, tagID, response.Person.TagID)
	assert.Equal(t, "mara@example.com", response.Person.Email)
}

// The minimal colleague view: an ordinary Betreuer holding only users:read
// sees name, work e-mail, account role and today's presence — never the
// personnel-file fields (#2906).
func TestNewStaffResponse_DirectoryTierRedactsPersonnelFields(t *testing.T) {
	t.Parallel()

	employmentType := users.EmploymentTypePartTime
	tagID := "04A1B2C3"
	response := buildStaffResponse(staffFieldAccess{}, &users.Staff{
		Model:          base.Model{ID: 42},
		PersonID:       420,
		StaffNotes:     "Vertretung montags",
		EmploymentType: &employmentType,
		Person:         &users.Person{Model: base.Model{ID: 420}, FirstName: "Mara", LastName: "Kühn", TagID: &tagID},
	}, false, true, "working", "sick", "user", "mara@example.com", "avatar.png")

	assert.Empty(t, response.StaffNotes)
	assert.Empty(t, response.AbsenceType)
	assert.Nil(t, response.EmploymentType)
	require.NotNil(t, response.Person)
	assert.Empty(t, response.Person.TagID)

	// The minimal view stays useful.
	assert.Equal(t, "Mara", response.Person.FirstName)
	assert.Equal(t, "mara@example.com", response.Person.Email)
	assert.Equal(t, "avatar.png", response.Person.Avatar)
	assert.Equal(t, "user", response.AccountRole)
	assert.True(t, response.WasPresentToday)
	assert.Equal(t, "working", response.WorkStatus)
}

// staff:manage maintains the staff record — notes and qualifications — and
// nothing else: the personnel-file data (employment type, absence reason, NFC
// tag) stays with the personnel and time-management roles (#2906).
func TestNewStaffResponse_RecordTierWithoutPersonnelTier(t *testing.T) {
	t.Parallel()

	employmentType := users.EmploymentTypePartTime
	tagID := "04A1B2C3"
	response := buildStaffResponse(staffFieldAccess{notes: true, qualifications: true}, &users.Staff{
		Model:          base.Model{ID: 42},
		PersonID:       420,
		StaffNotes:     "Vertretung montags",
		EmploymentType: &employmentType,
		Person:         &users.Person{Model: base.Model{ID: 420}, FirstName: "Mara", LastName: "Kühn", TagID: &tagID},
	}, false, false, "", "sick", "user", "mara@example.com", "")

	assert.Equal(t, "Vertretung montags", response.StaffNotes)
	assert.Empty(t, response.AbsenceType)
	assert.Nil(t, response.EmploymentType)
	require.NotNil(t, response.Person)
	assert.Empty(t, response.Person.TagID)
}

// staff:stammdaten maintains the personnel file — qualifications and the
// personnel-file fields — but the private staff notes stay with the
// permission that writes them, staff:manage (#2906).
func TestStaffFieldAccess_StammdatenDoesNotReadStaffNotes(t *testing.T) {
	t.Parallel()

	access := staffFieldAccessFor([]string{permissions.UsersRead, permissions.StaffStammdaten})

	assert.False(t, access.notes, "staff:stammdaten must not read the private staff notes")
	assert.True(t, access.qualifications)
	assert.True(t, access.personnel)

	response := buildStaffResponse(access, &users.Staff{
		Model:      base.Model{ID: 42},
		PersonID:   420,
		StaffNotes: "Vertretung montags",
	}, false, false, "", "sick", "user", "", "")

	assert.Empty(t, response.StaffNotes)
	assert.Equal(t, "sick", response.AbsenceType)
}

// The time-management view reads today's absence reason, but not the notes the
// staff record's maintainer writes.
func TestNewStaffResponse_PersonnelTierWithoutRecordTier(t *testing.T) {
	t.Parallel()

	employmentType := users.EmploymentTypePartTime
	response := buildStaffResponse(staffFieldAccess{personnel: true}, &users.Staff{
		Model:          base.Model{ID: 42},
		PersonID:       420,
		StaffNotes:     "Vertretung montags",
		EmploymentType: &employmentType,
	}, false, false, "", "sick", "user", "", "")

	assert.Empty(t, response.StaffNotes)
	assert.Equal(t, "sick", response.AbsenceType)
	require.NotNil(t, response.EmploymentType)
}

// Free-text qualifications are personnel-file data; the pedagogical labels the
// group and substitution screens render are not.
func TestNewTeacherResponse_DirectoryTierRedactsQualifications(t *testing.T) {
	t.Parallel()

	staff := &users.Staff{Model: base.Model{ID: 42}, PersonID: 420}
	teacher := &users.Teacher{
		Model:          base.Model{ID: 7},
		Specialization: "Sport",
		Role:           "Gruppenleitung",
		Qualifications: "Erzieherin, Erste-Hilfe-Kurs 2025",
	}

	redacted := buildTeacherResponse(staffFieldAccess{}, staff, teacher, false, "", "", "", "", "")
	assert.Empty(t, redacted.Qualifications)
	assert.Equal(t, "Sport", redacted.Specialization)
	assert.Equal(t, "Gruppenleitung", redacted.Role)

	full := buildTeacherResponse(staffFieldAccess{notes: true, qualifications: true, personnel: true}, staff, teacher, false, "", "", "", "", "")
	assert.Equal(t, "Erzieherin, Erste-Hilfe-Kurs 2025", full.Qualifications)
}

func TestNewStaffResponse_IncludesEmploymentType(t *testing.T) {
	t.Parallel()

	employmentType := users.EmploymentTypePartTime
	response := buildStaffResponse(staffFieldAccess{notes: true, qualifications: true, personnel: true}, &users.Staff{
		Model:          base.Model{ID: 42},
		PersonID:       420,
		EmploymentType: &employmentType,
	}, false, false, "", "", "", "", "")

	require.NotNil(t, response.EmploymentType)
	assert.Equal(t, users.EmploymentTypePartTime, *response.EmploymentType)
}

// The staff screens send person_id back to identify the person they edit. It
// leaves as a decimal string so a bigint survives the JSON.parse in the Next.js
// proxy — as a number, an id past 2^53 would be rounded into a valid id for a
// different person before any mapper could preserve it (#2222).
func TestStaffResponse_PersonIDIsDecimalString(t *testing.T) {
	t.Parallel()

	const bigPersonID = int64(9007199254740993) // 2^53 + 1

	encoded, err := json.Marshal(buildStaffResponse(staffFieldAccess{}, &users.Staff{
		Model:    base.Model{ID: 42},
		PersonID: bigPersonID,
	}, false, false, "", "", "", "", ""))
	require.NoError(t, err)

	assert.Contains(t, string(encoded), `"person_id":"9007199254740993"`)

	var decoded StaffResponse
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, bigPersonID, decoded.PersonID)
}

func TestResource_ListActiveCaregiversRequiresDirectoryAwarePersonService(t *testing.T) {
	t.Parallel()

	resource := &Resource{}

	caregivers, err := resource.listActiveCaregivers(context.Background())
	require.Error(t, err)
	assert.Nil(t, caregivers)
	assert.Contains(t, err.Error(), "caregiver directory")
}

func TestResource_CheckAccountLocked(t *testing.T) {
	t.Parallel()

	// The lockout decision moved off the model into AuthService.IsPINLocked
	// (issue #586). IsPINLocked is a pure read of account.PINLockedUntil, so a
	// zero-value Service suffices here.
	resource := &Resource{AuthService: &authSvc.Service{}}

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	assert.Nil(t, resource.checkAccountLocked(context.Background(),
		&authmodel.Account{PINLockedUntil: &past}))
	assert.Nil(t, resource.checkAccountLocked(context.Background(),
		&authmodel.Account{PINLockedUntil: nil}))
	assert.NotNil(t, resource.checkAccountLocked(context.Background(),
		&authmodel.Account{PINLockedUntil: &future}))
}

func TestVerifyCurrentPIN(t *testing.T) {
	t.Parallel()

	t.Run("skips verification when no pin exists", func(t *testing.T) {
		result, renderer := verifyCurrentPIN(pinAccountStub{hasPIN: false}, nil)
		assert.Equal(t, pinVerificationNotRequired, result)
		assert.Nil(t, renderer)
	})

	t.Run("requires current pin when account already has one", func(t *testing.T) {
		result, renderer := verifyCurrentPIN(pinAccountStub{hasPIN: true}, nil)
		assert.Equal(t, pinVerificationMissingInput, result)
		assert.NotNil(t, renderer)
	})

	t.Run("rejects incorrect current pin", func(t *testing.T) {
		current := "0000"
		result, renderer := verifyCurrentPIN(pinAccountStub{
			hasPIN: true,
			verifyPIN: func(pin string) bool {
				return pin == "1234"
			},
		}, &current)
		assert.Equal(t, pinVerificationFailed, result)
		assert.NotNil(t, renderer)
	})

	t.Run("accepts correct current pin", func(t *testing.T) {
		current := "1234"
		result, renderer := verifyCurrentPIN(pinAccountStub{
			hasPIN: true,
			verifyPIN: func(pin string) bool {
				return pin == "1234"
			},
		}, &current)
		assert.Equal(t, pinVerificationPassed, result)
		assert.Nil(t, renderer)
	})
}
