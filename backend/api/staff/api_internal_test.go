package staff

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	authmodel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	customLogger := slog.Default()

	assert.Same(t, customLogger, (&Resource{logger: customLogger}).getLogger())
	assert.NotNil(t, (&Resource{}).getLogger())
}

func TestResource_Router(t *testing.T) {
	viper.Set("auth_jwt_secret", "test-jwt-secret-for-staff-router-32-chars")
	viper.Set("auth_jwt_expiry", "15m")
	viper.Set("auth_jwt_refresh_expiry", "24h")

	router := (&Resource{}).Router()
	require.NotNil(t, router)
}

func TestResource_CrossStaffTimeAndAbsenceReadsRejectUsersRead(t *testing.T) {
	testutil.SeedTestJWTConfig()
	resource := &Resource{}
	router := resource.Router()
	claims := testutil.DefaultTestClaims()
	claims.Roles = []string{"user"}
	claims.Permissions = []string{"users:read"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)

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

func TestNewStaffResponse_IncludesEmploymentType(t *testing.T) {
	employmentType := users.EmploymentTypePartTime
	response := newStaffResponse(&users.Staff{
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
	const bigPersonID = int64(9007199254740993) // 2^53 + 1

	encoded, err := json.Marshal(newStaffResponse(&users.Staff{
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
	resource := &Resource{}

	caregivers, err := resource.listActiveCaregivers(context.Background())
	require.Error(t, err)
	assert.Nil(t, caregivers)
	assert.Contains(t, err.Error(), "caregiver directory")
}

func TestBuildSubstitutionInfoList(t *testing.T) {
	start := timezone.NewDate(2026, time.April, 1)
	substitutions := []*education.GroupSubstitution{
		{
			Model:     base.Model{ID: 11},
			GroupID:   101,
			StartDate: start,
			EndDate:   start,
			Group:     &education.Group{Name: "Gruppe Blau"},
		},
		{
			Model:     base.Model{ID: 12},
			GroupID:   102,
			StartDate: start,
			EndDate:   start.AddDays(2),
		},
	}

	result := buildSubstitutionInfoList(substitutions)

	require.Len(t, result, 2)
	assert.Equal(t, int64(11), result[0].ID)
	assert.Equal(t, "Gruppe Blau", result[0].GroupName)
	assert.True(t, result[0].IsTransfer)
	assert.Equal(t, "2026-04-01", result[0].StartDate)
	assert.Equal(t, "2026-04-01", result[0].EndDate)
	assert.False(t, result[1].IsTransfer)
	assert.Equal(t, "", result[1].GroupName)
}

func TestResource_CheckAccountLocked(t *testing.T) {
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
