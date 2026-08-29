package reminders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	permissions2 "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	remindersService "github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// captureService records the scope it was asked to compute so the test can
// assert how the handler resolved admin-vs-caregiver scope.
type captureService struct {
	gotScope remindersService.Scope
	called   bool
}

func (c *captureService) Compute(_ context.Context, scope remindersService.Scope) (*remindersService.Result, error) {
	c.called = true
	c.gotScope = scope
	return &remindersService.Result{Enabled: true}, nil
}

// stubUserContext satisfies UserContextService by embedding the interface and
// records whether GetCurrentStaff was consulted. For the admin paths it must
// never be called; for the caregiver path it returns the staff row.
type stubUserContext struct {
	usercontext.UserContextService
	staff  *usersModels.Staff
	err    error
	called bool
}

func (s *stubUserContext) GetCurrentStaff(context.Context) (*usersModels.Staff, error) {
	s.called = true
	return s.staff, s.err
}

func newRequestWithAuth(claims jwt.AppClaims, permissions []string) *http.Request {
	claims.ID = 1
	claims.TenantID = 2
	claims.Permissions = permissions
	ctx := context.Background()
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	principal, err := permissions2.NewPrincipal(permissions2.PrincipalInput{
		AccountID: 1, TenantID: 2, Roles: claims.Roles, Permissions: permissions, Admin: claims.IsAdmin,
	})
	if err != nil {
		panic(err)
	}
	ctx = permissions2.WithPrincipal(ctx, principal)
	return httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
}

// TestListRemindersAdminScope pins the reviewer's finding (PR #1799): admin
// scope must be derived from the effective wildcard permissions, not from the
// literal "admin" role name (claims.IsAdmin). A wildcard admin / service
// account without a staff row must get the admin view, not a 403 or a
// caregiver-scoped list.
func TestListRemindersAdminScope(t *testing.T) {
	t.Parallel()

	staff := &usersModels.Staff{}
	staff.ID = 42

	tests := []struct {
		name            string
		claims          jwt.AppClaims
		permissions     []string
		staff           *usersModels.Staff
		staffErr        error
		wantStatus      int
		wantAdminScope  bool
		wantStaffID     int64
		wantStaffLookup bool
	}{
		{
			name:            "wildcard admin without staff row gets admin scope",
			claims:          jwt.AppClaims{IsAdmin: false},
			permissions:     []string{"admin:*"},
			staff:           nil,
			staffErr:        usercontext.ErrUserNotLinkedToStaff,
			wantStatus:      http.StatusOK,
			wantAdminScope:  true,
			wantStaffLookup: false,
		},
		{
			name:            "full-access wildcard also gets admin scope",
			claims:          jwt.AppClaims{IsAdmin: false},
			permissions:     []string{"*:*"},
			staffErr:        usercontext.ErrUserNotLinkedToPerson,
			wantStatus:      http.StatusOK,
			wantAdminScope:  true,
			wantStaffLookup: false,
		},
		{
			name:            "literal admin still gets admin scope",
			claims:          jwt.AppClaims{IsAdmin: true},
			permissions:     []string{"users:read"},
			wantStatus:      http.StatusOK,
			wantAdminScope:  true,
			wantStaffLookup: false,
		},
		{
			name:            "caregiver is scoped to their staff id",
			claims:          jwt.AppClaims{IsAdmin: false},
			permissions:     []string{"users:read"},
			staff:           staff,
			wantStatus:      http.StatusOK,
			wantAdminScope:  false,
			wantStaffID:     42,
			wantStaffLookup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &captureService{}
			uc := &stubUserContext{staff: tt.staff, err: tt.staffErr}
			rs := &Resource{RemindersService: svc, UserContext: uc}

			rec := httptest.NewRecorder()
			rs.listReminders(rec, newRequestWithAuth(tt.claims, tt.permissions))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if !svc.called {
				t.Fatalf("Compute was not called")
			}
			if svc.gotScope.IsAdmin != tt.wantAdminScope {
				t.Errorf("scope.IsAdmin = %v, want %v", svc.gotScope.IsAdmin, tt.wantAdminScope)
			}
			if svc.gotScope.StaffID != tt.wantStaffID {
				t.Errorf("scope.StaffID = %d, want %d", svc.gotScope.StaffID, tt.wantStaffID)
			}
			if uc.called != tt.wantStaffLookup {
				t.Errorf("GetCurrentStaff called = %v, want %v", uc.called, tt.wantStaffLookup)
			}
		})
	}
}

// TestListRemindersCaregiverWithoutStaffForbidden keeps the existing contract:
// a non-admin, non-wildcard account with no staff row is a 403, not a 500.
func TestListRemindersCaregiverWithoutStaffForbidden(t *testing.T) {
	t.Parallel()

	svc := &captureService{}
	uc := &stubUserContext{err: usercontext.ErrUserNotLinkedToPerson}
	rs := &Resource{RemindersService: svc, UserContext: uc}

	rec := httptest.NewRecorder()
	rs.listReminders(rec, newRequestWithAuth(jwt.AppClaims{IsAdmin: false}, []string{"users:read"}))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if svc.called {
		t.Errorf("Compute must not run when the caller is not authorized")
	}
}
