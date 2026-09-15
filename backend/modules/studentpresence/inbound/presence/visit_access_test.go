package presence

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func TestRequireVisitViewUsesPrincipalAndPreservesDenialContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		principal *permissions.Principal
		param     string
		want      int
	}{
		{name: "missing principal", param: "1", want: testutil.StatusUnauthorized},
		{name: "malformed visit ID", principal: visitTestPrincipal(t), param: "bad", want: testutil.StatusForbidden},
		{name: "admin role bypass", principal: visitAdminRolePrincipal(t), param: "1", want: testutil.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(testutil.MethodGet, "/active/visits/"+tt.param, nil)
			ctx := testutil.WithURLParams(req, "id", tt.param).Context()
			if tt.principal != nil {
				ctx = permissions.WithPrincipal(ctx, *tt.principal)
			}
			recorder := httptest.NewRecorder()
			(resourceForTest(Resource{})).requireVisitView(testutil.HandlerFunc(func(w testutil.ResponseWriter, _ *testutil.Request) {
				w.WriteHeader(testutil.StatusNoContent)
			})).ServeHTTP(recorder, req.WithContext(ctx))
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
		})
	}
}

func TestRequireVisitViewMapsRelationshipsAndErrors(t *testing.T) {
	t.Parallel()
	internalErr := errors.New("private database detail")
	tests := []struct {
		name     string
		resource *Resource
		want     int
	}{
		{name: "student owns visit", resource: visitResource(visitFixture{studentForPerson: 10}), want: testutil.StatusNoContent},
		{name: "unrelated caller", resource: visitResource(visitFixture{}), want: testutil.StatusForbidden},
		{name: "visit lookup fails", resource: visitResource(visitFixture{visitErr: internalErr}), want: testutil.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := executeVisitView(t, tt.resource, *visitTestPrincipal(t), "1")
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tt.want, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), internalErr.Error()) {
				t.Fatalf("response leaked internal error: %s", recorder.Body.String())
			}
		})
	}
}

func visitTestPrincipal(t *testing.T) *permissions.Principal {
	t.Helper()
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: 1, TenantID: 2})
	if err != nil {
		t.Fatal(err)
	}
	return &principal
}

func visitAdminRolePrincipal(t *testing.T) *permissions.Principal {
	t.Helper()
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: 1, TenantID: 2, Roles: []string{"admin"}})
	if err != nil {
		t.Fatal(err)
	}
	return &principal
}

type visitFixture struct {
	studentForPerson int64
	visitErr         error
}

type visitPresenceQueries struct {
	PresenceQueries
	fixture visitFixture
}

func (s visitPresenceQueries) FindVisit(context.Context, int64) (*studentpresence.Visit, error) {
	return &studentpresence.Visit{StudentID: 10}, s.fixture.visitErr
}

type visitPersonService struct {
	People
	fixture visitFixture
}

func (s visitPersonService) FindByAccountID(context.Context, int64) (*PersonIdentity, error) {
	return &PersonIdentity{ID: 1}, nil
}

func (s visitPersonService) GetStudentByPersonID(context.Context, int64) (*StudentIdentity, error) {
	if s.fixture.studentForPerson == 0 {
		return nil, nil
	}
	return &StudentIdentity{ID: s.fixture.studentForPerson}, nil
}

func (s visitPersonService) GetStaffByPersonID(context.Context, int64) (*StaffIdentity, error) {
	return nil, nil
}

func visitResource(fixture visitFixture) *Resource {
	return resourceForTest(Resource{
		Presence: visitPresenceQueries{fixture: fixture}, PersonService: visitPersonService{fixture: fixture},
	})
}

func executeVisitView(t *testing.T, resource *Resource, principal permissions.Principal, param string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(testutil.MethodGet, "/active/visits/"+param, nil)
	ctx := permissions.WithPrincipal(testutil.WithURLParams(req, "id", param).Context(), principal)
	recorder := httptest.NewRecorder()
	resource.requireVisitView(testutil.HandlerFunc(func(w testutil.ResponseWriter, _ *testutil.Request) {
		w.WriteHeader(testutil.StatusNoContent)
	})).ServeHTTP(recorder, req.WithContext(ctx))
	return recorder
}
