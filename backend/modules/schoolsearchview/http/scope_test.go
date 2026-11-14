package schoolsearchviewhttp

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	"github.com/stretchr/testify/require"
)

type staffCheck struct {
	staff bool
	err   error
}

func (s staffCheck) HasCurrentStaff(context.Context) (bool, error) { return s.staff, s.err }

func scopeFor(t *testing.T, staff staffCheck, granted ...string) schoolsearchview.Scope {
	t.Helper()
	resource := &Resource{staff: staff}
	scope, err := resource.scope(context.WithValue(context.Background(), sessiontoken.CtxPermissions, granted))
	require.NoError(t, err)
	return scope
}

func TestScopeOpensEachKindWithThePermissionOfItsPage(t *testing.T) {
	t.Parallel()
	staff := staffCheck{staff: true}
	require.Equal(t, schoolsearchview.Scope{Children: true, ChildDetails: true, Contacts: true, Staff: true},
		scopeFor(t, staff, permissions.UsersRead), "users:read opens children, their details, contacts and the staff list")
	require.Equal(t, schoolsearchview.Scope{Children: true},
		scopeFor(t, staff, permissions.StudentsRead), "the guest reads names, never contact data or answers")
	require.Equal(t, schoolsearchview.Scope{Rooms: true},
		scopeFor(t, staff, permissions.RoomsRead))
	require.Equal(t, schoolsearchview.Scope{},
		scopeFor(t, staff, permissions.GroupsRead), "a group hit leads to the child list, so it needs a child read")
	require.Equal(t, schoolsearchview.Scope{Children: true, Groups: true},
		scopeFor(t, staff, permissions.StudentsRead, permissions.GroupsRead))
	require.Equal(t, schoolsearchview.Scope{Staff: true},
		scopeFor(t, staff, permissions.StaffStammdaten))
	require.Equal(t, schoolsearchview.Scope{Children: true, ChildDetails: true, Contacts: true, Staff: true, Rooms: true, Groups: true},
		scopeFor(t, staffCheck{}, permissions.AdminWildcard), "an administrator reads everything without a staff record")
}

func TestScopeClosesChildDataToAccountsWithoutAStaffRecord(t *testing.T) {
	t.Parallel()
	require.Equal(t, schoolsearchview.Scope{Staff: true}, scopeFor(t, staffCheck{}, permissions.UsersRead))
	resource := &Resource{staff: staffCheck{err: errors.New("lookup failed")}}
	_, err := resource.scope(context.WithValue(context.Background(), sessiontoken.CtxPermissions, []string{permissions.UsersRead}))
	require.Error(t, err, "an unknown staff check never widens the scope")
}
