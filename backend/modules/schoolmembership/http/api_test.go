package staff_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMembership answers the capability calls from small in-memory maps, so
// every handler path can be driven without a database.
type fakeMembership struct {
	schoolmembership.Capability
	staff    map[int64]schoolmembership.Staff
	teachers map[int64]schoolmembership.Teacher // keyed by staff ID
	listErr  error
}

func (f *fakeMembership) FindStaff(_ context.Context, id int64) (schoolmembership.Staff, error) {
	staff, ok := f.staff[id]
	if !ok {
		return schoolmembership.Staff{}, schoolmembership.ErrStaffNotFound
	}
	return staff, nil
}

func (f *fakeMembership) FindStaffByPerson(_ context.Context, personID int64) (schoolmembership.Staff, error) {
	for _, staff := range f.staff {
		if staff.PersonID == personID {
			return staff, nil
		}
	}
	return schoolmembership.Staff{}, schoolmembership.ErrStaffNotFound
}

func (f *fakeMembership) ListStaff(_ context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	wanted := map[int64]bool{}
	for _, id := range filter.IDs {
		wanted[id] = true
	}
	result := make([]schoolmembership.Staff, 0, len(f.staff))
	for _, staff := range f.staff {
		if len(wanted) > 0 && !wanted[staff.ID] {
			continue
		}
		result = append(result, staff)
	}
	sortByID(result)
	return result, nil
}

func (f *fakeMembership) FindTeacherByStaff(_ context.Context, staffID int64) (schoolmembership.Teacher, error) {
	teacher, ok := f.teachers[staffID]
	if !ok {
		return schoolmembership.Teacher{}, schoolmembership.ErrTeacherNotFound
	}
	return teacher, nil
}

func (f *fakeMembership) ListTeachers(_ context.Context, filter schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error) {
	wanted := map[int64]bool{}
	for _, id := range filter.StaffIDs {
		wanted[id] = true
	}
	result := make([]schoolmembership.Teacher, 0, len(f.teachers))
	for _, teacher := range f.teachers {
		if len(wanted) > 0 && !wanted[teacher.StaffID] {
			continue
		}
		result = append(result, teacher)
	}
	return result, nil
}

func sortByID(values []schoolmembership.Staff) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j].ID < values[j-1].ID; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

type response struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
}

func (response) Render(http.ResponseWriter, *http.Request) error { return nil }

type errorResponse struct {
	StatusCode int    `json:"-"`
	Status     string `json:"status"`
	Kind       string `json:"kind"`
	Error      string `json:"error"`
}

func (e errorResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.StatusCode)
	return nil
}

var failureStatus = map[staffHTTP.FailureKind]int{
	staffHTTP.FailureInvalidRequest: http.StatusBadRequest,
	staffHTTP.FailureUnauthorized:   http.StatusUnauthorized,
	staffHTTP.FailureForbidden:      http.StatusForbidden,
	staffHTTP.FailureNotFound:       http.StatusNotFound,
	staffHTTP.FailureConflict:       http.StatusConflict,
	staffHTTP.FailureInternal:       http.StatusInternalServerError,
}

// harness records every runtime call the adapter makes and serves the routes
// through a real chi router.
type harness struct {
	membership *fakeMembership
	router     chi.Router
	observed   []string
	txCalls    int

	permitted   map[string]bool
	permissions []string
	accountID   int64
	username    string

	persons     map[int64]staffHTTP.Person
	personErr   error
	accountRole map[int64]string
	emails      map[int64]string
	avatars     map[int64]string
	roles       map[int64]string // accountID -> role name for the ?role= filter
	present     []int64
	workStatus  map[int64]string
	absence     map[int64]string
	absenceLbl  map[int64]string

	groups        []staffHTTP.Group
	classes       []string
	setClasses    []string
	setClassActor int64
	classErr      error
	caregivers    []staffHTTP.StaffWithRoleResponse
	byRoles       []staffHTTP.StaffWithRoleResponse
	rolesAsked    []string

	created      staffHTTP.CreateStaffInput
	createResult staffHTTP.CreateStaffResult
	createErr    error
	updated      staffHTTP.UpdateStaffInput
	updateResult staffHTTP.UpdateStaffResult
	updateErr    error
	offboarded   int64
	offboardedBy string
	offboardErr  error
	grants       []bool
	grantedTo    []int64
	retryCalls   []string
	servedAvatar string

	pinHasPIN      bool
	pinLastChanged *time.Time
	pinStatusErr   error
	pinPreflight   error
	personsErr     error
	pinCurrent     *string
	pinNew         string
	pinUpdateErr   error
	personByAcct   map[int64]int64
}

func newHarness(t *testing.T, membership *fakeMembership) *harness {
	t.Helper()
	h := &harness{
		membership: membership, permitted: map[string]bool{},
		persons: map[int64]staffHTTP.Person{}, accountRole: map[int64]string{},
		emails: map[int64]string{}, avatars: map[int64]string{}, roles: map[int64]string{},
		workStatus: map[int64]string{}, absence: map[int64]string{}, absenceLbl: map[int64]string{},
		personByAcct: map[int64]int64{}, accountID: 7, username: "tester",
	}
	resource := staffHTTP.NewResource(membership, staffHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, staffHTTP.Middleware)) {
			register(router, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					h.txCalls++
					next.ServeHTTP(w, r)
				})
			})
		},
		Permission: func(required ...string) staffHTTP.Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					for _, permission := range required {
						if h.permitted[permission] {
							next.ServeHTTP(w, r)
							return
						}
					}
					w.WriteHeader(http.StatusForbidden)
				})
			}
		},
		Success: func(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
			payload, _ := json.Marshal(data)
			render.Status(r, status)
			_ = render.Render(w, r, response{Status: "success", Data: payload, Message: message})
		},
		Failure: func(w http.ResponseWriter, r *http.Request, kind staffHTTP.FailureKind, err error) {
			_ = render.Render(w, r, errorResponse{StatusCode: failureStatus[kind], Status: "error", Kind: string(kind), Error: err.Error()})
		},
		ObserveResponse: func(status int, code string) {
			h.observed = append(h.observed, http.StatusText(status)+":"+code)
		},
		ServeAvatar: func(w http.ResponseWriter, _ *http.Request, path string) {
			h.servedAvatar = path
			_, _ = io.WriteString(w, "image-bytes")
		},
		WriteFailure: func(w http.ResponseWriter, r *http.Request, err error) {
			_ = render.Render(w, r, errorResponse{StatusCode: http.StatusConflict, Status: "error", Kind: "write", Error: err.Error()})
		},
		SchoolClassFailure: func(w http.ResponseWriter, r *http.Request, err error) {
			_ = render.Render(w, r, errorResponse{StatusCode: http.StatusNotFound, Status: "error", Kind: "school_class", Error: err.Error()})
		},
		PINFailure: func(w http.ResponseWriter, r *http.Request, err error) {
			_ = render.Render(w, r, errorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Kind: "pin", Error: err.Error()})
		},
		Permissions:      func(context.Context) []string { return h.permissions },
		HasPermission:    matchPermission,
		CurrentAccountID: func(context.Context) int64 { return h.accountID },
		CurrentUsername:  func(context.Context) string { return h.username },
		Person: func(_ context.Context, id int64) (staffHTTP.Person, error) {
			if h.personErr != nil {
				return staffHTTP.Person{}, h.personErr
			}
			person, ok := h.persons[id]
			if !ok {
				return staffHTTP.Person{}, errors.New("person not found")
			}
			return person, nil
		},
		Persons: func(_ context.Context, ids []int64) ([]staffHTTP.Person, error) {
			if h.personsErr != nil {
				return nil, h.personsErr
			}
			result := make([]staffHTTP.Person, 0, len(ids))
			for _, id := range ids {
				if person, ok := h.persons[id]; ok {
					result = append(result, person)
				}
			}
			return result, nil
		},
		PersonIDByAccount: func(_ context.Context, accountID int64) (int64, bool, error) {
			personID, ok := h.personByAcct[accountID]
			return personID, ok, nil
		},
		PresentStaffIDs: func(context.Context) ([]int64, error) { return h.present, nil },
		WorkStatusMap:   func(context.Context) (map[int64]string, error) { return h.workStatus, nil },
		AbsenceMap:      func(context.Context) (map[int64]string, error) { return h.absence, nil },
		AbsenceLabelMap: func(context.Context) (map[int64]string, error) { return h.absenceLbl, nil },
		AccountRoles:    func(_ context.Context, ids []int64) (map[int64]string, error) { return pick(h.accountRole, ids), nil },
		AccountEmails:   func(_ context.Context, ids []int64) (map[int64]string, error) { return pick(h.emails, ids), nil },
		AccountAvatars:  func(_ context.Context, ids []int64) (map[int64]string, error) { return pick(h.avatars, ids), nil },
		AccountHasRole: func(_ context.Context, accountID int64, role string) bool {
			return h.roles[accountID] == role
		},
		GrantDefaultPermissions: func(_ context.Context, accountID int64, isTeacher bool) {
			h.grantedTo = append(h.grantedTo, accountID)
			h.grants = append(h.grants, isTeacher)
		},
		RetryQueuedDocumentCleanups: func(_ context.Context, source string) {
			h.retryCalls = append(h.retryCalls, source)
		},
		TeacherGroups: func(context.Context, int64) ([]staffHTTP.Group, error) { return h.groups, nil },
		SchoolClasses: func(context.Context, int64) ([]string, error) {
			if h.classErr != nil {
				return nil, h.classErr
			}
			return h.classes, nil
		},
		SetSchoolClasses: func(_ context.Context, _ int64, classes []string, actor int64) error {
			if h.classErr != nil {
				return h.classErr
			}
			h.setClasses = classes
			h.setClassActor = actor
			h.classes = classes
			return nil
		},
		ActiveCaregivers: func(context.Context) ([]staffHTTP.StaffWithRoleResponse, error) { return h.caregivers, nil },
		StaffByRoles: func(_ context.Context, roles []string) ([]staffHTTP.StaffWithRoleResponse, error) {
			h.rolesAsked = roles
			return h.byRoles, nil
		},
		CreateStaff: func(_ context.Context, input staffHTTP.CreateStaffInput) (staffHTTP.CreateStaffResult, error) {
			h.created = input
			return h.createResult, h.createErr
		},
		UpdateStaff: func(_ context.Context, input staffHTTP.UpdateStaffInput) (staffHTTP.UpdateStaffResult, error) {
			h.updated = input
			return h.updateResult, h.updateErr
		},
		Offboard: func(_ context.Context, staffID int64, username string) error {
			h.offboarded, h.offboardedBy = staffID, username
			return h.offboardErr
		},
		PINStatus: func(context.Context, int64) (bool, *time.Time, error) {
			return h.pinHasPIN, h.pinLastChanged, h.pinStatusErr
		},
		PINPreflight: func(context.Context, int64) error { return h.pinPreflight },
		UpdatePIN: func(_ context.Context, _ int64, current *string, newPIN string) error {
			h.pinCurrent, h.pinNew = current, newPIN
			return h.pinUpdateErr
		},
		Log: slog.New(slog.DiscardHandler),
	})
	h.router = chi.NewRouter()
	h.router.Mount("/staff", resource.Router())
	return h
}

// matchPermission is the wildcard-aware matcher the root supplies.
func matchPermission(required string, granted []string) bool {
	for _, permission := range granted {
		if permission == required {
			return true
		}
		if strings.HasSuffix(permission, ":*") && strings.HasPrefix(required, strings.TrimSuffix(permission, "*")) {
			return true
		}
	}
	return false
}

func pick(source map[int64]string, ids []int64) map[int64]string {
	result := make(map[int64]string, len(ids))
	for _, id := range ids {
		if value, ok := source[id]; ok {
			result[id] = value
		}
	}
	return result
}

// allow grants both the route gate and the field tiers for a permission.
func (h *harness) allow(permissions ...string) *harness {
	for _, permission := range permissions {
		h.permitted[permission] = true
	}
	h.permissions = append(h.permissions, permissions...)
	return h
}

func (h *harness) do(t *testing.T, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder) response {
	t.Helper()
	var body response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var body errorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func accountRef(id int64) *int64 { return &id }

// directoryFixture is one plain staff member (ID 1) and one teacher (ID 2).
func directoryFixture() *fakeMembership {
	employment := "full_time"
	return &fakeMembership{
		staff: map[int64]schoolmembership.Staff{
			1: {ID: 1, PersonID: 11, StaffNotes: "private note", EmploymentType: &employment},
			2: {ID: 2, PersonID: 12},
		},
		teachers: map[int64]schoolmembership.Teacher{
			2: {ID: 22, StaffID: 2, Specialization: " Musik ", Role: "Betreuung", Qualifications: "Erzieherin"},
		},
	}
}

func withPersons(h *harness) *harness {
	h.persons[11] = staffHTTP.Person{ID: 11, FirstName: "Ada", LastName: "Lovelace", TagID: "TAG-11", AccountID: accountRef(101)}
	h.persons[12] = staffHTTP.Person{ID: 12, FirstName: "Grace", LastName: "Hopper", TagID: "TAG-12", AccountID: accountRef(102)}
	h.emails[101] = "ada@example.com"
	h.accountRole[101] = "admin"
	h.avatars[102] = "/uploads/avatars/global/grace.png"
	return h
}

func TestListStaffAppliesFiltersAndTiers(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read", "staff:manage", "staff:stammdaten")
	h.present = []int64{1}
	h.workStatus[1] = "working"
	h.absence[2] = "sick"
	h.absenceLbl[2] = "Krank gemeldet"

	recorder := h.do(t, http.MethodGet, "/staff", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	body := decode(t, recorder)
	assert.Equal(t, "Staff members retrieved successfully", body.Message)
	payload := string(body.Data)
	assert.Contains(t, payload, `"person_id":"11"`, "person_id is a decimal string (#2222)")
	assert.Contains(t, payload, `"staff_notes":"private note"`)
	assert.Contains(t, payload, `"employment_type":"full_time"`)
	assert.Contains(t, payload, `"tag_id":"TAG-11"`)
	assert.Contains(t, payload, `"absence_type_label":"Krank gemeldet"`)
	assert.Contains(t, payload, `"was_present_today":true`)
	assert.Contains(t, payload, `"teacher_id":22`)
	assert.Contains(t, payload, `"specialization":"Musik"`, "specialization is trimmed")
	assert.Contains(t, payload, `"qualifications":"Erzieherin"`)
	assert.Contains(t, payload, `"email":"ada@example.com"`)
	assert.Equal(t, 1, h.txCalls)

	teachersOnly := h.do(t, http.MethodGet, "/staff?teachers_only=true", nil)
	require.Equal(t, http.StatusOK, teachersOnly.Code)
	assert.NotContains(t, string(decode(t, teachersOnly).Data), `"person_id":"11"`)

	byName := h.do(t, http.MethodGet, "/staff?first_name=gRa", nil)
	require.Equal(t, http.StatusOK, byName.Code)
	filtered := string(decode(t, byName).Data)
	assert.Contains(t, filtered, `"person_id":"12"`)
	assert.NotContains(t, filtered, `"person_id":"11"`)

	byLastName := h.do(t, http.MethodGet, "/staff?last_name=Nobody", nil)
	assert.Equal(t, "[]", string(decode(t, byLastName).Data))
}

func TestListStaffFiltersByAccountRole(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read")
	h.roles[101] = "teacher"

	recorder := h.do(t, http.MethodGet, "/staff?role=teacher", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	payload := string(decode(t, recorder).Data)
	assert.Contains(t, payload, `"person_id":"11"`)
	assert.NotContains(t, payload, `"person_id":"12"`, "a person whose account lacks the role is skipped")
}

func TestListStaffRedactsForTheDirectoryTier(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read")
	h.absence[2] = "sick"
	h.absenceLbl[2] = "Krank gemeldet"

	recorder := h.do(t, http.MethodGet, "/staff", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	payload := string(decode(t, recorder).Data)
	assert.NotContains(t, payload, "private note", "staff notes need staff:manage")
	assert.NotContains(t, payload, "employment_type", "personnel data needs staff:stammdaten or time_tracking:manage")
	assert.NotContains(t, payload, "TAG-11", "the NFC tag is personnel data")
	assert.NotContains(t, payload, "absence_type", "the absence reason and the school's own wording are personnel data")
	assert.NotContains(t, payload, "Erzieherin", "free-text qualifications are personnel data")
	assert.Contains(t, payload, `"specialization":"Musik"`, "the pedagogical labels stay visible")
}

func TestListStaffWithoutPermissionIsForbidden(t *testing.T) {
	t.Parallel()
	h := newHarness(t, directoryFixture())

	recorder := h.do(t, http.MethodGet, "/staff", nil)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Zero(t, h.txCalls)
}

func TestListStaffReportsCapabilityFailure(t *testing.T) {
	t.Parallel()
	membership := directoryFixture()
	membership.listErr = errors.New("storage down")
	h := newHarness(t, membership)
	h.allow("users:read")

	recorder := h.do(t, http.MethodGet, "/staff", nil)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, h.observed, "Internal Server Error:internal_error")
}

func TestListEndpointsFailWhenThePersonLookupFails(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read", "staff_documents:health")
	h.personsErr = errors.New("person store down")

	// The handlers this adapter replaces read staff and person in one join,
	// so a broken person lookup answered 500 instead of an empty list.
	for _, target := range []string{"/staff", "/staff/documents-directory", "/staff/available"} {
		recorder := h.do(t, http.MethodGet, target, nil)
		assert.Equal(t, http.StatusInternalServerError, recorder.Code, target)
		assert.Equal(t, "person store down", decodeError(t, recorder).Error, target)
	}
	assert.Contains(t, h.observed, "Internal Server Error:internal_error")
}

func TestGetStaffSplitsTeacherAndStaffBranch(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read")

	plain := h.do(t, http.MethodGet, "/staff/1", nil)
	require.Equal(t, http.StatusOK, plain.Code)
	assert.Equal(t, "Staff member retrieved successfully", decode(t, plain).Message)
	assert.NotContains(t, string(decode(t, plain).Data), "teacher_id")

	teacher := h.do(t, http.MethodGet, "/staff/2", nil)
	require.Equal(t, http.StatusOK, teacher.Code)
	assert.Equal(t, "Teacher retrieved successfully", decode(t, teacher).Message)
	assert.Contains(t, string(decode(t, teacher).Data), `"teacher_id":22`)

	missing := h.do(t, http.MethodGet, "/staff/99", nil)
	assert.Equal(t, http.StatusNotFound, missing.Code)
	assert.Equal(t, "staff member not found", decodeError(t, missing).Error)

	invalid := h.do(t, http.MethodGet, "/staff/abc", nil)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
	assert.Equal(t, "invalid staff ID", decodeError(t, invalid).Error)
	assert.Contains(t, h.observed, "Bad Request:invalid_parameters")
}

func TestMinimalProfilesReturnIdentityOnly(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("staff:financial")

	recorder := h.do(t, http.MethodGet, "/staff/financial-profile/1", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	body := decode(t, recorder)
	assert.Equal(t, "Financial staff profile retrieved successfully", body.Message)
	assert.JSONEq(t, `{"id":1,"name":"Ada Lovelace","firstName":"Ada","lastName":"Lovelace"}`, string(body.Data))

	documents := h.do(t, http.MethodGet, "/staff/documents-profile/1", nil)
	require.Equal(t, http.StatusOK, documents.Code)
	assert.Equal(t, "Document staff profile retrieved successfully", decode(t, documents).Message)
}

func TestDocumentDirectorySortsAndDrainsTheCleanupQueue(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("staff_documents:health")

	recorder := h.do(t, http.MethodGet, "/staff/documents-directory", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	body := decode(t, recorder)
	assert.Equal(t, "Document staff directory retrieved successfully", body.Message)
	assert.Equal(t, `[{"id":1,"name":"Ada Lovelace","firstName":"Ada","lastName":"Lovelace"},{"id":2,"name":"Grace Hopper","firstName":"Grace","lastName":"Hopper"}]`, string(body.Data))
	assert.Equal(t, []string{"directory"}, h.retryCalls)
}

func TestServeAvatarFallsBackToNotFound(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read")

	served := h.do(t, http.MethodGet, "/staff/2/avatar", nil)
	require.Equal(t, http.StatusOK, served.Code)
	assert.Equal(t, "/uploads/avatars/global/grace.png", h.servedAvatar)

	// Staff 1's account has no stored avatar.
	noAvatar := h.do(t, http.MethodGet, "/staff/1/avatar", nil)
	assert.Equal(t, http.StatusNotFound, noAvatar.Code)

	unknownStaff := h.do(t, http.MethodGet, "/staff/99/avatar", nil)
	assert.Equal(t, http.StatusNotFound, unknownStaff.Code)

	invalid := h.do(t, http.MethodGet, "/staff/abc/avatar", nil)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
}

func TestGetStaffGroupsAnswersEmptyForNonTeachers(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read")
	h.groups = []staffHTTP.Group{{ID: 5, Name: "Gruppe A"}}

	nonTeacher := h.do(t, http.MethodGet, "/staff/1/groups", nil)
	require.Equal(t, http.StatusOK, nonTeacher.Code)
	body := decode(t, nonTeacher)
	assert.Equal(t, "Staff member is not a teacher and has no assigned groups", body.Message)
	assert.Equal(t, "[]", string(body.Data))

	teacher := h.do(t, http.MethodGet, "/staff/2/groups", nil)
	require.Equal(t, http.StatusOK, teacher.Code)
	assert.Equal(t, `[{"id":5,"name":"Gruppe A"}]`, string(decode(t, teacher).Data))

	missing := h.do(t, http.MethodGet, "/staff/99/groups", nil)
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

func TestSchoolClassesRoundTripAndValidation(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read", "users:manage")
	h.classes = []string{"1a", "2b"}

	read := h.do(t, http.MethodGet, "/staff/2/school-classes", nil)
	require.Equal(t, http.StatusOK, read.Code)
	assert.Equal(t, "School classes retrieved successfully", decode(t, read).Message)
	assert.Contains(t, string(decode(t, read).Data), `"school_classes":["1a","2b"]`)

	written := h.do(t, http.MethodPut, "/staff/2/school-classes", map[string]any{"school_classes": []string{"3c"}})
	require.Equal(t, http.StatusOK, written.Code)
	assert.Equal(t, "School classes updated successfully", decode(t, written).Message)
	assert.Equal(t, []string{"3c"}, h.setClasses)
	assert.EqualValues(t, 7, h.setClassActor, "the audit actor is the authenticated account")

	missingField := h.do(t, http.MethodPut, "/staff/2/school-classes", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, missingField.Code)
	assert.Equal(t, "school_classes is required", decodeError(t, missingField).Error)

	h.accountID = 0
	unauthenticated := h.do(t, http.MethodPut, "/staff/2/school-classes", map[string]any{"school_classes": []string{"1a"}})
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	assert.Equal(t, "invalid token", decodeError(t, unauthenticated).Error)
}

func TestSchoolClassFailuresAreDelegatedToTheRoot(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read")
	h.classErr = errors.New("Klassenname darf nicht leer sein")

	recorder := h.do(t, http.MethodGet, "/staff/2/school-classes", nil)
	assert.Equal(t, http.StatusNotFound, recorder.Code, "the root's renderer decides the status")
	assert.Equal(t, "Klassenname darf nicht leer sein", decodeError(t, recorder).Error)
}

func TestAvailableStaffListsTeachersWithPersons(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read")

	recorder := h.do(t, http.MethodGet, "/staff/available", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	body := decode(t, recorder)
	assert.Equal(t, "Available staff members retrieved successfully", body.Message)
	assert.Contains(t, string(body.Data), `"teacher_id":22`)
	assert.NotContains(t, string(body.Data), `"person_id":"11"`, "only teachers are listed")
}

func TestStaffByRoleSplitsCaregiverPoolFromRoleLookup(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:read")
	h.caregivers = []staffHTTP.StaffWithRoleResponse{{ID: 2, PersonID: 12, TeacherID: 22, FullName: "Grace Hopper", IsActiveCaregiver: true}}
	h.byRoles = []staffHTTP.StaffWithRoleResponse{
		{ID: 1, PersonID: 11, FullName: "Ada Lovelace"},
		{ID: 1, PersonID: 11, FullName: "Ada Lovelace"},
	}

	pool := h.do(t, http.MethodGet, "/staff/by-role?role=user", nil)
	require.Equal(t, http.StatusOK, pool.Code)
	assert.Equal(t, "Active caregivers retrieved successfully", decode(t, pool).Message)
	assert.Contains(t, string(decode(t, pool).Data), `"is_active_caregiver":true`)
	assert.Nil(t, h.rolesAsked, "the caregiver pool never reaches the role lookup")

	byRole := h.do(t, http.MethodGet, "/staff/by-role?roles=teacher,%20admin", nil)
	require.Equal(t, http.StatusOK, byRole.Code)
	assert.Equal(t, "Staff members with role retrieved successfully", decode(t, byRole).Message)
	assert.Equal(t, []string{"teacher", "admin"}, h.rolesAsked)
	assert.Equal(t, 1, strings.Count(string(decode(t, byRole).Data), `"full_name":"Ada Lovelace"`), "one staff member matching several roles appears once")

	missing := h.do(t, http.MethodGet, "/staff/by-role", nil)
	assert.Equal(t, http.StatusBadRequest, missing.Code)
	assert.Equal(t, "role or roles parameter is required", decodeError(t, missing).Error)
}

func TestCreateStaffValidatesBodyAndGrantsPermissions(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:create")
	h.createResult = staffHTTP.CreateStaffResult{Staff: schoolmembership.Staff{ID: 3, PersonID: 11}}

	missingPerson := h.do(t, http.MethodPost, "/staff", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, missingPerson.Code)
	assert.Equal(t, "person ID is required", decodeError(t, missingPerson).Error)

	unknownPerson := h.do(t, http.MethodPost, "/staff", map[string]any{"person_id": 999})
	assert.Equal(t, http.StatusNotFound, unknownPerson.Code)
	assert.Equal(t, "person not found", decodeError(t, unknownPerson).Error)

	// person_id travels as a decimal string on the way in, too (#2222).
	created := h.do(t, http.MethodPost, "/staff", map[string]any{"person_id": "11", "staff_notes": "hired"})
	require.Equal(t, http.StatusCreated, created.Code)
	assert.Equal(t, "Staff member created successfully", decode(t, created).Message)
	assert.EqualValues(t, 11, h.created.PersonID)
	assert.Equal(t, "hired", h.created.StaffNotes)
	assert.Equal(t, []string{"users:create"}, h.created.ActorPermissions)
	assert.Equal(t, []int64{101}, h.grantedTo)
	assert.Equal(t, []bool{false}, h.grants)
}

func TestCreateStaffMapsTheTeacherOutcomes(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:create")
	h.createResult = staffHTTP.CreateStaffResult{
		Staff:   schoolmembership.Staff{ID: 3, PersonID: 11},
		Teacher: &schoolmembership.Teacher{ID: 33, StaffID: 3},
	}

	teacher := h.do(t, http.MethodPost, "/staff", map[string]any{"person_id": 11, "is_teacher": true})
	require.Equal(t, http.StatusCreated, teacher.Code)
	assert.Equal(t, "Teacher created successfully", decode(t, teacher).Message)
	assert.Equal(t, []bool{true}, h.grants)

	h.createResult = staffHTTP.CreateStaffResult{Staff: schoolmembership.Staff{ID: 3, PersonID: 11}, TeacherCreationFailed: true}
	h.grants, h.grantedTo = nil, nil
	failed := h.do(t, http.MethodPost, "/staff", map[string]any{"person_id": 11, "is_teacher": true})
	require.Equal(t, http.StatusCreated, failed.Code)
	assert.Equal(t, "Staff member created successfully, but failed to create teacher record", decode(t, failed).Message)
	assert.Empty(t, h.grantedTo, "a failed teacher record grants nothing")

	h.createErr = errors.New("staff record already in use")
	conflict := h.do(t, http.MethodPost, "/staff", map[string]any{"person_id": 11})
	assert.Equal(t, http.StatusConflict, conflict.Code, "the root's write renderer decides the status")
	assert.Equal(t, "staff record already in use", decodeError(t, conflict).Error)
}

func TestUpdateStaffGuardsPersonReassignment(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("staff:manage")
	h.updateResult = staffHTTP.UpdateStaffResult{Staff: schoolmembership.Staff{ID: 1, PersonID: 11}, Action: staffHTTP.TeacherActionNone}

	unchanged := h.do(t, http.MethodPut, "/staff/1", map[string]any{"person_id": "11", "staff_notes": "edited"})
	require.Equal(t, http.StatusOK, unchanged.Code)
	assert.Equal(t, "Staff member updated successfully", decode(t, unchanged).Message)
	assert.Equal(t, "edited", h.updated.StaffNotes)

	reassign := h.do(t, http.MethodPut, "/staff/1", map[string]any{"person_id": "12"})
	assert.Equal(t, http.StatusForbidden, reassign.Code)
	assert.Equal(t, "insufficient permission to reassign a staff record to another person", decodeError(t, reassign).Error)

	h.allow("users:manage")
	h.updateResult = staffHTTP.UpdateStaffResult{Staff: schoolmembership.Staff{ID: 1, PersonID: 12}, Action: staffHTTP.TeacherActionNone}
	allowed := h.do(t, http.MethodPut, "/staff/1", map[string]any{"person_id": "12"})
	require.Equal(t, http.StatusOK, allowed.Code)
	assert.EqualValues(t, 12, h.updated.PersonID)

	unknownPerson := h.do(t, http.MethodPut, "/staff/1", map[string]any{"person_id": "999"})
	assert.Equal(t, http.StatusNotFound, unknownPerson.Code)
	assert.Equal(t, "person not found", decodeError(t, unknownPerson).Error)

	missingStaff := h.do(t, http.MethodPut, "/staff/99", map[string]any{"person_id": "11"})
	assert.Equal(t, http.StatusNotFound, missingStaff.Code)
	assert.Equal(t, "staff member not found", decodeError(t, missingStaff).Error)
}

func TestUpdateStaffMessagesFollowTheTeacherAction(t *testing.T) {
	t.Parallel()
	cases := []struct {
		action  staffHTTP.TeacherAction
		teacher *schoolmembership.Teacher
		message string
	}{
		{staffHTTP.TeacherActionUpdated, &schoolmembership.Teacher{ID: 22, StaffID: 1}, "Teacher updated successfully"},
		{staffHTTP.TeacherActionCreated, &schoolmembership.Teacher{ID: 22, StaffID: 1}, "Teacher updated successfully"},
		{staffHTTP.TeacherActionExisting, &schoolmembership.Teacher{ID: 22, StaffID: 1}, "Teacher updated successfully"},
		{staffHTTP.TeacherActionUpdateFailed, nil, "Staff member updated successfully, but failed to update teacher record"},
		{staffHTTP.TeacherActionCreateFailed, nil, "Staff member updated successfully, but failed to create teacher record"},
		{staffHTTP.TeacherActionNone, nil, "Staff member updated successfully"},
	}
	for _, testCase := range cases {
		t.Run(string(testCase.action), func(t *testing.T) {
			t.Parallel()
			h := withPersons(newHarness(t, directoryFixture()))
			h.allow("staff:manage")
			h.updateResult = staffHTTP.UpdateStaffResult{
				Staff: schoolmembership.Staff{ID: 1, PersonID: 11}, Teacher: testCase.teacher, Action: testCase.action,
			}
			recorder := h.do(t, http.MethodPut, "/staff/1", map[string]any{"person_id": "11"})
			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, testCase.message, decode(t, recorder).Message)
		})
	}
}

func TestDeleteStaffOffboardsIdempotently(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.allow("users:delete")

	recorder := h.do(t, http.MethodDelete, "/staff/1", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "Staff member deleted successfully", decode(t, recorder).Message)
	assert.EqualValues(t, 1, h.offboarded)
	assert.Equal(t, "tester", h.offboardedBy)

	// An ID that no longer exists still answers 200 — offboarding never
	// looked the record up.
	unknown := h.do(t, http.MethodDelete, "/staff/999999", nil)
	assert.Equal(t, http.StatusOK, unknown.Code)

	invalid := h.do(t, http.MethodDelete, "/staff/abc", nil)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)

	h.offboardErr = errors.New("Personal kann nicht gelöscht werden")
	conflict := h.do(t, http.MethodDelete, "/staff/1", nil)
	assert.Equal(t, http.StatusConflict, conflict.Code)
}

func TestPINStatusRequiresAStaffAccount(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	changed := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	h.pinHasPIN, h.pinLastChanged = true, &changed
	h.personByAcct[7] = 11 // person 11 is staff 1

	recorder := h.do(t, http.MethodGet, "/staff/pin", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	body := decode(t, recorder)
	assert.Equal(t, "PIN status retrieved successfully", body.Message)
	assert.Contains(t, string(body.Data), `"has_pin":true`)
	assert.Contains(t, string(body.Data), `"last_changed"`)

	// An account whose person carries no staff record is refused.
	h.personByAcct[7] = 99
	refused := h.do(t, http.MethodGet, "/staff/pin", nil)
	assert.Equal(t, http.StatusForbidden, refused.Code)
	assert.Equal(t, "only staff members can access PIN settings", decodeError(t, refused).Error)

	// An account without a person at all is an administrator and passes.
	delete(h.personByAcct, 7)
	admin := h.do(t, http.MethodGet, "/staff/pin", nil)
	assert.Equal(t, http.StatusOK, admin.Code)

	h.accountID = 0
	unauthenticated := h.do(t, http.MethodGet, "/staff/pin", nil)
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	assert.Equal(t, "invalid token", decodeError(t, unauthenticated).Error)
}

func TestPINStatusReportsAnUnknownAccount(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.pinStatusErr = errors.New("no such account")

	recorder := h.do(t, http.MethodGet, "/staff/pin", nil)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "account not found", decodeError(t, recorder).Error)
}

func TestUpdatePINValidatesTheFourDigitFormat(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.personByAcct[7] = 11

	for _, testCase := range []struct{ body, message string }{
		{`{}`, "new PIN is required"},
		{`{"new_pin":"123"}`, "PIN must be exactly 4 digits"},
		{`{"new_pin":"12ab"}`, "PIN must contain only digits"},
	} {
		recorder := h.do(t, http.MethodPut, "/staff/pin", json.RawMessage(testCase.body))
		assert.Equal(t, http.StatusBadRequest, recorder.Code, testCase.body)
		assert.Equal(t, testCase.message, decodeError(t, recorder).Error)
	}

	current := "0000"
	accepted := h.do(t, http.MethodPut, "/staff/pin", map[string]any{"new_pin": "1234", "current_pin": current})
	require.Equal(t, http.StatusOK, accepted.Code)
	assert.Equal(t, "PIN updated successfully", decode(t, accepted).Message)
	assert.Contains(t, string(decode(t, accepted).Data), `"success":true`)
	assert.Equal(t, "1234", h.pinNew)
	require.NotNil(t, h.pinCurrent)
	assert.Equal(t, current, *h.pinCurrent)

	h.personByAcct[7] = 99
	refused := h.do(t, http.MethodPut, "/staff/pin", map[string]any{"new_pin": "1234"})
	assert.Equal(t, http.StatusForbidden, refused.Code)
	assert.Equal(t, "only staff members can manage PIN settings", decodeError(t, refused).Error)
}

func TestUpdatePINRunsThePreflightBeforeTheStaffCheck(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	// Account 7 points at a person with no staff record, so the staff-only
	// check would refuse it on its own.
	h.personByAcct[7] = 99
	h.pinPreflight = errors.New("account is temporarily locked due to failed PIN attempts")

	recorder := h.do(t, http.MethodPut, "/staff/pin", map[string]any{"new_pin": "1234"})

	// The lockout message wins over "only staff members can manage PIN
	// settings" — the order the pre-refactor handler had.
	assert.Equal(t, http.StatusUnauthorized, recorder.Code, "the root's PIN renderer decides the status")
	assert.Equal(t, "account is temporarily locked due to failed PIN attempts", decodeError(t, recorder).Error)
	assert.Empty(t, h.pinNew, "a failed preflight never reaches the update")
}

func TestUpdatePINDelegatesVerificationFailures(t *testing.T) {
	t.Parallel()
	h := withPersons(newHarness(t, directoryFixture()))
	h.personByAcct[7] = 11
	h.pinUpdateErr = errors.New("current PIN is incorrect")

	recorder := h.do(t, http.MethodPut, "/staff/pin", map[string]any{"new_pin": "1234"})
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, "current PIN is incorrect", decodeError(t, recorder).Error)
}

func TestNewResourceRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { staffHTTP.NewResource(nil, staffHTTP.Runtime{}) })
	assert.Panics(t, func() { staffHTTP.NewResource(&fakeMembership{}, staffHTTP.Runtime{}) })
}
