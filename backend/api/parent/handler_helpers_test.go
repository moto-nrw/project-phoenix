package parent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- submitParentEnrollment ----------------------------------------------

type parentSubmitAuthStub struct {
	authService.AuthService
	mapped bool
}

func (s *parentSubmitAuthStub) VerifyAccountTenantMembership(context.Context, int64, int64) (bool, error) {
	return s.mapped, nil
}

type parentSubmitSchoolStub struct {
	platformSvc.SchoolService
	school *platformModels.School
}

func (s *parentSubmitSchoolStub) GetSchoolBySlug(context.Context, string) (*platformModels.School, error) {
	return s.school, nil
}

type parentSubmitRequestStub struct {
	enrollmentService.RequestService
	called bool
	got    enrollmentService.SubmitRequest
}

func (s *parentSubmitRequestStub) Submit(_ context.Context, req enrollmentService.SubmitRequest) (*enrollmentService.SubmitResult, error) {
	s.called = true
	s.got = req
	request := &enrollmentModels.Request{StatusToken: "status-token"}
	request.ID = 99001
	return &enrollmentService.SubmitResult{Request: request, StatusURL: "/status/status-token"}, nil
}

func TestSubmitParentEnrollment_AllowsMappedAccountWithoutExistingGuardianPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	var tenantID int64 = 1
	school := &platformModels.School{Name: "Testschule", Slug: "testschule"}
	school.ID = tenantID
	requestSvc := &parentSubmitRequestStub{}
	rs := &Resource{
		AuthService: &parentSubmitAuthStub{mapped: true},
		// The submit path now resolves guardian facts via ParentService
		// (#1663); the zero-value status (no guardian link, no permission)
		// is exactly the pre-guardian-row state this test asserts on.
		ParentService:  &fakeParentService{},
		RequestService: requestSvc,
		SchoolService:  &parentSubmitSchoolStub{school: school},
		db:             db,
	}

	body, err := json.Marshal(enrollmentAPI.SubmitEnrollmentRequest{
		PhaseID:           4242,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "anna@example.test",
		LateInviteToken:   "parent-late-token",
		Children: []enrollmentAPI.SubmitChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "2018-03-04"},
		},
	})
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/parent/enrollments/{tenantSlug}/submit", rs.submitParentEnrollment)
	req := withClaims(
		httptest.NewRequest(http.MethodPost, "/parent/enrollments/testschule/submit", strings.NewReader(string(body))),
		7777,
	)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.True(t, requestSvc.called, "mapped parent submissions must reach RequestService before any student_guardian row exists")
	require.NotNil(t, requestSvc.got.GuardianAccountID)
	assert.Equal(t, int64(7777), *requestSvc.got.GuardianAccountID)
	assert.Equal(t, tenantID, requestSvc.got.TenantID)
	assert.Equal(t, "parent-late-token", requestSvc.got.LateInviteToken)
}

// TestSubmitParentEnrollment_BlocksRevokedGuardianPermission covers the
// #1663 permission gate: an account with a guardian link at the school
// whose links all lack parent_portal.enrollment.submit gets 403 and the
// submission never reaches the RequestService.
func TestSubmitParentEnrollment_BlocksRevokedGuardianPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	school := &platformModels.School{Name: "Testschule", Slug: "testschule"}
	school.ID = 1
	requestSvc := &parentSubmitRequestStub{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: false,
		}},
		RequestService: requestSvc,
		SchoolService:  &parentSubmitSchoolStub{school: school},
		db:             db,
	}

	body, err := json.Marshal(enrollmentAPI.SubmitEnrollmentRequest{
		PhaseID:           4242,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "anna@example.test",
		Children: []enrollmentAPI.SubmitChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "2018-03-04"},
		},
	})
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/parent/enrollments/{tenantSlug}/submit", rs.submitParentEnrollment)
	req := withClaims(
		httptest.NewRequest(http.MethodPost, "/parent/enrollments/testschule/submit", strings.NewReader(string(body))),
		7778,
	)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, requestSvc.called, "revoked submit permission must never reach RequestService")
}

// TestSubmitParentEnrollment_StampsSubmitEligibility covers the #1663
// eligibility plumbing: a guardian with the submit permission reaches
// the RequestService with GuardianSubmitEligible=true so linked_parents
// phases accept the submission.
func TestSubmitParentEnrollment_StampsSubmitEligibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	school := &platformModels.School{Name: "Testschule", Slug: "testschule"}
	school.ID = 1
	requestSvc := &parentSubmitRequestStub{}
	rs := &Resource{
		ParentService: &fakeParentService{submitStatus: &parentModels.GuardianSubmitStatus{
			Linked:              true,
			HasGuardianLink:     true,
			HasSubmitPermission: true,
		}},
		RequestService: requestSvc,
		SchoolService:  &parentSubmitSchoolStub{school: school},
		db:             db,
	}

	body, err := json.Marshal(enrollmentAPI.SubmitEnrollmentRequest{
		PhaseID:           4242,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "anna@example.test",
		Children: []enrollmentAPI.SubmitChildRequest{
			{FirstName: "Lara", LastName: "Beispiel", DateOfBirth: "2018-03-04"},
		},
	})
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/parent/enrollments/{tenantSlug}/submit", rs.submitParentEnrollment)
	req := withClaims(
		httptest.NewRequest(http.MethodPost, "/parent/enrollments/testschule/submit", strings.NewReader(string(body))),
		7779,
	)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.True(t, requestSvc.called)
	assert.True(t, requestSvc.got.GuardianSubmitEligible,
		"submit permission must be forwarded as GuardianSubmitEligible")
}

// --- toChildResponse -----------------------------------------------------

func TestToChildResponse_StringifiesIDs(t *testing.T) {
	now := timezone.NewDate(2026, 9, 1)
	in := &parentModels.ChildSummary{
		StudentID:    12345,
		TenantID:     6789,
		FirstName:    "Lara",
		LastName:     "Beispiel",
		SchoolClass:  "1a",
		Status:       "active",
		EnrolledFrom: &now,
		SchoolName:   "OGS Sonnenschule",
		SchoolSlug:   "sonnenschule",
	}
	out := toChildResponse(in)
	assert.Equal(t, "12345", out.StudentID, "int64 IDs are stringified per CLAUDE.md rule 4")
	assert.Equal(t, "6789", out.TenantID)
	assert.Equal(t, "Lara", out.FirstName)
	assert.Equal(t, "1a", out.SchoolClass)
	assert.Equal(t, "active", out.Status)
	require.NotNil(t, out.EnrolledFrom)
	assert.Equal(t, now, *out.EnrolledFrom)
	assert.Nil(t, out.EnrolledUntil, "nil pointers pass through unchanged")
}

// --- staffShortName ------------------------------------------------------

func TestStaffShortName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Anna Müller", "Anna M."},
		{"Sabine Schneider", "Sabine S."},
		{"Anna-Lena Öztürk", "Anna-Lena Ö."}, // rune-based initial, not a broken byte
		{"Anna von Berg", "Anna B."},         // multi-token: first + LAST initial
		{"Olivia", "Olivia"},                 // single token unchanged
		{"OGS-Team", "OGS-Team"},             // resolveStaffName fallback unchanged
		{"", ""},                             // empty stays empty
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.want, staffShortName(tc.in), "staffShortName(%q)", tc.in)
	}
}

// --- toMessageResponses staff-name masking -------------------------------

func TestToMessageResponses_StaffNameMaskedUnlessVisible(t *testing.T) {
	counterpart := "OGS Sonnenschule"
	messages := []*usersModels.ParentMessage{
		{
			// Frozen visible=false (older reply / school opted out): stays anonymous.
			SenderKind:       usersModels.ParentMessageSenderStaff,
			SenderName:       "Sabine Schneider",
			StaffNameVisible: false,
			Body:             "hidden",
		},
		{
			// Frozen visible=true (reply written while the setting was on): shows the
			// person as first name + last initial, never the full surname.
			SenderKind:       usersModels.ParentMessageSenderStaff,
			SenderName:       "Sabine Schneider",
			StaffNameVisible: true,
			Body:             "shown",
		},
		{
			// Guardian messages are never masked and never short-formed.
			SenderKind:       usersModels.ParentMessageSenderGuardian,
			SenderName:       "Olivia Berg",
			StaffNameVisible: false,
			Body:             "parent",
		},
	}

	out := toMessageResponses(messages, counterpart)
	require.Len(t, out, 3)
	assert.Equal(t, counterpart, out[0].SenderName, "masked staff reply collapses to the OGS label")
	assert.Equal(t, "Sabine S.", out[1].SenderName, "visible staff reply shows first name + last initial")
	assert.Equal(t, "Olivia Berg", out[2].SenderName, "guardian name passes through unchanged")
}
