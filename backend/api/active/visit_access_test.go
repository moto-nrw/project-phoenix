package active

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
)

func TestRequireVisitViewUsesPrincipalAndPreservesDenialContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		principal *permissions.Principal
		param     string
		want      int
	}{
		{name: "missing principal", param: "1", want: http.StatusUnauthorized},
		{name: "malformed visit ID", principal: visitTestPrincipal(t), param: "bad", want: http.StatusForbidden},
		{name: "admin role bypass", principal: visitAdminRolePrincipal(t), param: "1", want: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/active/visits/"+tt.param, nil)
			route := chi.NewRouteContext()
			route.URLParams.Add("id", tt.param)
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, route)
			if tt.principal != nil {
				ctx = permissions.WithPrincipal(ctx, *tt.principal)
			}
			recorder := httptest.NewRecorder()
			(&Resource{}).requireVisitView(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
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
		{name: "student owns visit", resource: visitResource(visitFixture{studentForPerson: 10}), want: http.StatusNoContent},
		{name: "unrelated caller", resource: visitResource(visitFixture{}), want: http.StatusForbidden},
		{name: "visit lookup fails", resource: visitResource(visitFixture{visitErr: internalErr}), want: http.StatusInternalServerError},
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

func TestVisitReadAllPreservesLegacyBroadGrants(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		roles       []string
		permissions []string
		admin       bool
		want        bool
	}{
		{name: "admin role", roles: []string{"admin"}, want: true},
		{name: "admin role remains canonical", roles: []string{"ADMIN"}},
		{name: "signed admin claim", admin: true, want: true},
		{name: "admin wildcard", permissions: []string{"admin:*"}, want: true},
		{name: "visits read", permissions: []string{permissions.VisitsRead}, want: true},
		{name: "visits manage", permissions: []string{permissions.VisitsManage}, want: true},
		{name: "resource wildcard was not a broad grant", permissions: []string{"visits:*"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
				AccountID: 1, TenantID: 2, Roles: tt.roles, Permissions: tt.permissions, Admin: tt.admin,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := visitReadAll(principal); got != tt.want {
				t.Fatalf("visitReadAll() = %v, want %v", got, tt.want)
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

type visitActiveService struct {
	activeSvc.Service
	fixture visitFixture
}

func (s visitActiveService) GetVisit(context.Context, int64) (*activeModel.Visit, error) {
	return &activeModel.Visit{StudentID: 10}, s.fixture.visitErr
}

type visitPersonService struct {
	userSvc.PersonService
	fixture visitFixture
}

func (s visitPersonService) FindByAccountID(context.Context, int64) (*userModel.Person, error) {
	return &userModel.Person{Model: base.Model{ID: 1}}, nil
}

func (s visitPersonService) GetStudentByPersonID(context.Context, int64) (*userModel.Student, error) {
	if s.fixture.studentForPerson == 0 {
		return nil, nil
	}
	return &userModel.Student{Model: base.Model{ID: s.fixture.studentForPerson}}, nil
}

func (s visitPersonService) GetStaffByPersonID(context.Context, int64) (*userModel.Staff, error) {
	return nil, nil
}

func visitResource(fixture visitFixture) *Resource {
	return &Resource{
		ActiveService: visitActiveService{fixture: fixture}, PersonService: visitPersonService{fixture: fixture},
	}
}

func executeVisitView(t *testing.T, resource *Resource, principal permissions.Principal, param string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/active/visits/"+param, nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", param)
	ctx := permissions.WithPrincipal(context.WithValue(req.Context(), chi.RouteCtxKey, route), principal)
	recorder := httptest.NewRecorder()
	resource.requireVisitView(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, req.WithContext(ctx))
	return recorder
}
