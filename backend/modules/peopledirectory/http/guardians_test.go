package users_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	usersHTTP "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGuardianDirectory answers the guardian reads and records the writes
// the adapter issues; the students and persons it needs come from the same
// fake.
type fakeGuardianDirectory struct {
	peopledirectory.Capability
	guardians    map[int64]peopledirectory.Guardian
	students     map[int64]peopledirectory.Student
	studentLinks map[int64][]peopledirectory.GuardianWithLink
	guardianKids map[int64][]peopledirectory.StudentWithLink
	impact       peopledirectory.GuardianDeleteImpact
	evaluate     error
	deleteErr    error
	deleted      []peopledirectory.GuardianDelete
	added        []peopledirectory.NewStudentGuardian
	exportRows   []peopledirectory.GuardianPaymentRow
	exportCalls  int
}

func (f *fakeGuardianDirectory) FindGuardian(_ context.Context, id int64) (peopledirectory.Guardian, error) {
	guardian, ok := f.guardians[id]
	if !ok {
		return peopledirectory.Guardian{}, peopledirectory.ErrGuardianNotFound
	}
	return guardian, nil
}

func (f *fakeGuardianDirectory) ListStudentsByID(_ context.Context, ids []int64) ([]peopledirectory.Student, error) {
	result := []peopledirectory.Student{}
	for _, id := range ids {
		if student, ok := f.students[id]; ok {
			result = append(result, student)
		}
	}
	return result, nil
}

func (f *fakeGuardianDirectory) ListStudentGuardians(_ context.Context, studentID int64) ([]peopledirectory.GuardianWithLink, error) {
	return f.studentLinks[studentID], nil
}

func (f *fakeGuardianDirectory) ListGuardianStudents(_ context.Context, guardianID int64) ([]peopledirectory.StudentWithLink, error) {
	return f.guardianKids[guardianID], nil
}

func (f *fakeGuardianDirectory) GuardianDeleteImpact(context.Context, int64) (peopledirectory.GuardianDeleteImpact, error) {
	return f.impact, nil
}

func (f *fakeGuardianDirectory) EvaluateGuardianDelete(_ context.Context, _ int64, force, isAdmin bool) (bool, error) {
	if f.evaluate != nil {
		return true, f.evaluate
	}
	return len(f.impact.LinkIDs) > 0, nil
}

func (f *fakeGuardianDirectory) DeleteGuardian(_ context.Context, input peopledirectory.GuardianDelete) error {
	f.deleted = append(f.deleted, input)
	return f.deleteErr
}

func (f *fakeGuardianDirectory) AddGuardiansToStudent(_ context.Context, _ int64, guardians []peopledirectory.NewStudentGuardian) error {
	f.added = append(f.added, guardians...)
	return nil
}

func (f *fakeGuardianDirectory) ListPaymentExportRows(context.Context, peopledirectory.GuardianPaymentActor, string) ([]peopledirectory.GuardianPaymentRow, error) {
	f.exportCalls++
	return f.exportRows, nil
}

type guardianHarness struct {
	directory   *fakeGuardianDirectory
	router      chi.Router
	permitted   map[string]bool
	actorPerms  map[string]bool
	admin       bool
	staff       bool
	actorID     int64
	exposeToken bool
	rollbacks   int
	observed    []string
	rendered    []string
}

func newGuardianHarness(t *testing.T, directory *fakeGuardianDirectory) *guardianHarness {
	t.Helper()
	h := &guardianHarness{directory: directory, permitted: map[string]bool{}, actorPerms: map[string]bool{}, actorID: 42, staff: true}
	statusOf := map[usersHTTP.FailureKind]int{
		usersHTTP.FailureInvalidRequest: http.StatusBadRequest, usersHTTP.FailureUnauthorized: http.StatusUnauthorized,
		usersHTTP.FailureForbidden: http.StatusForbidden, usersHTTP.FailureNotFound: http.StatusNotFound,
		usersHTTP.FailureConflict: http.StatusConflict, usersHTTP.FailureInternal: http.StatusInternalServerError,
	}
	resource := usersHTTP.NewGuardianResource(directory, usersHTTP.GuardianRuntime{
		Protected: func(router chi.Router, register func(chi.Router, usersHTTP.Middleware)) {
			register(router, func(next http.Handler) http.Handler { return next })
		},
		Permission: func(permission string) usersHTTP.Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !h.permitted[permission] {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					next.ServeHTTP(w, r)
				})
			}
		},
		ParsePagination: func(*http.Request) (int, int) { return 1, 20 },
		Success: func(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
			payload, _ := json.Marshal(data)
			render.Status(r, status)
			_ = render.Render(w, r, response{Status: "success", Data: payload, Message: message})
		},
		SuccessPaginated: func(w http.ResponseWriter, r *http.Request, status int, data any, pagination usersHTTP.Pagination, message string) {
			payload, _ := json.Marshal(data)
			render.Status(r, status)
			_ = render.Render(w, r, response{Status: "success", Data: payload, Message: message})
		},
		Failure: func(w http.ResponseWriter, r *http.Request, kind usersHTTP.FailureKind, err error) {
			h.rendered = append(h.rendered, string(kind))
			_ = render.Render(w, r, errorResponse{StatusCode: statusOf[kind], Status: "error", Kind: string(kind), Error: err.Error()})
		},
		ObserveResponse: func(status int, code string) { h.observed = append(h.observed, http.StatusText(status)+":"+code) },
		ActorID:         func(*http.Request) int64 { return h.actorID },
		ActorRole:       func(*http.Request) string { return "admin" },
		HasPermission:   func(_ *http.Request, permission string) bool { return h.actorPerms[permission] },
		IsAdmin:         func(*http.Request) bool { return h.admin },
		IsVerifiedStaff: func(context.Context) bool { return h.staff },
		ExposeInvitationToken: func(*http.Request) bool {
			return h.exposeToken
		},
		MarkRollback: func(context.Context) { h.rollbacks++ },
		SendInvitation: func(_ context.Context, guardianID, actorID int64) (usersHTTP.GuardianInvitation, error) {
			return usersHTTP.GuardianInvitation{ID: 5, GuardianProfileID: guardianID, EmailSent: true, Token: "secret-token"}, nil
		},
		ListPendingInvitations: func(context.Context) ([]usersHTTP.PendingGuardianInvitation, error) { return nil, nil },
		InviteGuardianToStudent: func(context.Context, usersHTTP.GuardianInvite) (usersHTTP.GuardianInviteResult, error) {
			return usersHTTP.GuardianInviteResult{}, errors.New("managed contact")
		},
		InviteFailureKind:          func(error) usersHTTP.FailureKind { return usersHTTP.FailureForbidden },
		ListPendingApprovals:       func(context.Context) ([]usersHTTP.GuardianPendingApproval, error) { return nil, nil },
		PendingInvitationStudentID: func(context.Context, int64) (int64, error) { return 0, errors.New("unknown invitation") },
		ApproveInvitation:          func(context.Context, int64, int64) error { return nil },
		RejectInvitation:           func(context.Context, int64, int64) error { return nil },
		RenderPaymentExport: func(rows []peopledirectory.GuardianPaymentRow, format string) (usersHTTP.ExportFile, error) {
			return usersHTTP.ExportFile{ContentType: "application/" + format, Filename: "bankverbindungen." + format, Data: []byte("file")}, nil
		},
		Log: slog.Default(),
	})
	h.router = chi.NewRouter()
	h.router.Mount("/guardians", resource.Router())
	return h
}

func (h *guardianHarness) do(t *testing.T, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	reader := bytes.NewReader(nil)
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	return recorder
}

func TestGuardianRoutesAreGatedByPermission(t *testing.T) {
	t.Parallel()
	h := newGuardianHarness(t, &fakeGuardianDirectory{})

	assert.Equal(t, http.StatusForbidden, h.do(t, http.MethodGet, "/guardians/", nil).Code)
	assert.Equal(t, http.StatusForbidden, h.do(t, http.MethodGet, "/guardians/payment-overview", nil).Code)
	h.permitted["users:read"] = true
	assert.Equal(t, http.StatusBadRequest, h.do(t, http.MethodGet, "/guardians/abc", nil).Code)
	assert.Equal(t, http.StatusNotFound, h.do(t, http.MethodGet, "/guardians/7", nil).Code)
	assert.Contains(t, h.observed, "Not Found:not_found")
}

func TestDeleteGuardianRendersAudienceSpecificConflicts(t *testing.T) {
	t.Parallel()
	directory := &fakeGuardianDirectory{
		guardians:    map[int64]peopledirectory.Guardian{7: {ID: 7}},
		guardianKids: map[int64][]peopledirectory.StudentWithLink{7: {{Student: peopledirectory.Student{ID: 1}}}},
		evaluate:     &peopledirectory.GuardianStillLinkedError{StudentNames: []string{"Anna Müller", "Ben Müller"}},
	}
	h := newGuardianHarness(t, directory)
	h.permitted["users:delete"] = true

	recorder := h.do(t, http.MethodDelete, "/guardians/7", nil)
	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Noch mit Kindern verknüpft")
	assert.NotContains(t, recorder.Body.String(), "Anna", "staff must not learn the names of children they may not supervise")

	h.admin = true
	recorder = h.do(t, http.MethodDelete, "/guardians/7", nil)
	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "2 Kindern")
	assert.Contains(t, recorder.Body.String(), "Anna Müller, Ben Müller")
	assert.Empty(t, directory.deleted, "a refused delete never reaches the owner")

	directory.evaluate = nil
	directory.impact = peopledirectory.GuardianDeleteImpact{LinkIDs: []int64{11}, StudentNames: []string{"Anna Müller"}}
	recorder = h.do(t, http.MethodDelete, "/guardians/7?force=true&expected_link_ids=11", nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, directory.deleted, 1)
	assert.Equal(t, peopledirectory.GuardianDelete{GuardianID: 7, ActorAccountID: 42, WithLinks: true, ExpectedLinkIDs: []int64{11}}, directory.deleted[0])

	directory.deleteErr = peopledirectory.ErrGuardianDeletePreviewChanged
	recorder = h.do(t, http.MethodDelete, "/guardians/7?force=true&expected_link_ids=11", nil)
	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Vorschau")
	assert.Equal(t, 1, h.rollbacks, "a stale preview rolls the request back")
}

func TestStudentGuardiansHidePayerWithoutFinancialPermission(t *testing.T) {
	t.Parallel()
	directory := &fakeGuardianDirectory{
		students: map[int64]peopledirectory.Student{3: {ID: 3, Status: "active"}, 4: {ID: 4, Status: peopledirectory.StudentStatusAlumnus}},
		studentLinks: map[int64][]peopledirectory.GuardianWithLink{3: {{
			Guardian: peopledirectory.Guardian{ID: 7, HasAccount: true},
			Link:     peopledirectory.GuardianLink{ID: 9, IsPayer: true, GuardianRole: "pickup_only"},
		}}},
	}
	h := newGuardianHarness(t, directory)
	h.permitted["users:read"] = true

	recorder := h.do(t, http.MethodGet, "/guardians/students/3/guardians", nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), `"is_payer":false`)
	assert.Contains(t, recorder.Body.String(), `"account_status":"active_no_access"`, "an account without portal access on this child is not active for it")

	h.actorPerms["guardians:financial"] = true
	recorder = h.do(t, http.MethodGet, "/guardians/students/3/guardians", nil)
	assert.Contains(t, recorder.Body.String(), `"is_payer":true`)

	assert.Equal(t, http.StatusNotFound, h.do(t, http.MethodGet, "/guardians/students/4/guardians", nil).Code, "a graduated child is not found")
}

func TestBatchCreateRequiresCreatePermissionOnlyForNewProfiles(t *testing.T) {
	t.Parallel()
	directory := &fakeGuardianDirectory{students: map[int64]peopledirectory.Student{3: {ID: 3, Status: "active"}}}
	h := newGuardianHarness(t, directory)
	h.permitted["users:update"] = true
	var existing int64 = 5

	recorder := h.do(t, http.MethodPost, "/guardians/students/3/guardians/batch", map[string]any{
		"guardians": []map[string]any{{"first_name": "New", "relationship_type": "parent", "emergency_priority": 1}},
	})
	assert.Equal(t, http.StatusForbidden, recorder.Code, "creating a profile needs users:create")
	assert.Empty(t, directory.added)

	recorder = h.do(t, http.MethodPost, "/guardians/students/3/guardians/batch", map[string]any{
		"guardians": []map[string]any{{"guardian_profile_id": existing, "relationship_type": "parent", "emergency_priority": 1}},
	})
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	require.Len(t, directory.added, 1)
	require.NotNil(t, directory.added[0].GuardianProfileID)
	assert.Equal(t, existing, *directory.added[0].GuardianProfileID)

	h.staff = false
	assert.Equal(t, http.StatusForbidden, h.do(t, http.MethodPost, "/guardians/students/3/guardians/batch", map[string]any{
		"guardians": []map[string]any{{"guardian_profile_id": existing, "relationship_type": "parent", "emergency_priority": 1}},
	}).Code, "an account without a staff record may not change a child's guardians")
}

func TestPaymentExportRejectsUnknownFormatBeforeLoadingRows(t *testing.T) {
	t.Parallel()
	directory := &fakeGuardianDirectory{exportRows: []peopledirectory.GuardianPaymentRow{{StudentID: 1}}}
	h := newGuardianHarness(t, directory)
	h.permitted["guardians:financial"] = true

	recorder := h.do(t, http.MethodPost, "/guardians/payment-overview/export", map[string]any{"format": "csv"})
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, 0, directory.exportCalls, "a refused format must not load (and access-log) the rows")

	recorder = h.do(t, http.MethodPost, "/guardians/payment-overview/export", map[string]any{"format": "XLSX"})
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/xlsx", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "bankverbindungen.xlsx")
	assert.Equal(t, 1, directory.exportCalls)

	h.actorID = 0
	assert.Equal(t, http.StatusUnauthorized, h.do(t, http.MethodPost, "/guardians/payment-overview/export", map[string]any{"format": "pdf"}).Code)
}

func TestInvitationTokenIsOnlyExposedWhenTheRootAllowsIt(t *testing.T) {
	t.Parallel()
	h := newGuardianHarness(t, &fakeGuardianDirectory{})
	h.permitted["users:create"] = true

	recorder := h.do(t, http.MethodPost, "/guardians/7/invite", nil)
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	assert.NotContains(t, recorder.Body.String(), "secret-token")

	h.exposeToken = true
	recorder = h.do(t, http.MethodPost, "/guardians/7/invite", nil)
	assert.Contains(t, recorder.Body.String(), "secret-token")

	h.actorID = 0
	assert.Equal(t, http.StatusUnauthorized, h.do(t, http.MethodPost, "/guardians/7/invite", nil).Code)
}

func TestInviteFailuresUseTheRootClassification(t *testing.T) {
	t.Parallel()
	directory := &fakeGuardianDirectory{students: map[int64]peopledirectory.Student{3: {ID: 3, Status: "active"}}}
	h := newGuardianHarness(t, directory)
	h.permitted["users:create"] = true
	h.permitted["users:update"] = true

	recorder := h.do(t, http.MethodPost, "/guardians/students/3/invite", map[string]any{"email": "x@example.test"})
	assert.Equal(t, http.StatusForbidden, recorder.Code, "the root classified the failure as forbidden")
	assert.Equal(t, http.StatusBadRequest, h.do(t, http.MethodPost, "/guardians/invitations/9/approve", nil).Code, "an unknown invitation is bad input")
}
