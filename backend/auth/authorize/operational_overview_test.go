package authorize

import (
	"context"
	"errors"
	"testing"
)

type overviewSettings struct {
	scope string
	err   error
}

func (s overviewSettings) ResolveString(context.Context, string) (string, error) {
	return s.scope, s.err
}

type overviewStaff struct{ present bool }

func (s overviewStaff) HasCurrentStaff(context.Context) (bool, error) { return s.present, nil }

func TestOperationalOverview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, scope              string
		staff, assignment, admin bool
		want                     bool
	}{
		{"own denies", OverviewScopeOwn, true, false, false, false},
		{"own allows admin", OverviewScopeOwn, false, false, true, true},
		{"admins allows admin", OverviewScopeAdmins, false, false, true, true},
		{"admins denies staff", OverviewScopeAdmins, true, false, false, false},
		{"all staff allows staff", OverviewScopeAllStaff, true, false, false, true},
		{"all staff denies nonstaff", OverviewScopeAllStaff, false, false, false, false},
		{"school stays assignment-bound", OverviewScopeAllStaff, true, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasOperationalOverview(context.Background(), overviewSettings{scope: tt.scope}, overviewStaff{tt.staff}, tt.assignment, tt.admin)
			if err != nil || got != tt.want {
				t.Fatalf("HasOperationalOverview() = %v, %v; want %v, nil", got, err, tt.want)
			}
			resolved, err := HasOperationalOverviewForResolvedStaff(context.Background(), overviewSettings{scope: tt.scope}, tt.assignment, tt.admin, tt.staff)
			if err != nil || resolved != tt.want {
				t.Fatalf("HasOperationalOverviewForResolvedStaff() = %v, %v; want %v, nil", resolved, err, tt.want)
			}
		})
	}
}

func TestOperationalOverviewSettingsErrorFailsClosed(t *testing.T) {
	t.Parallel()
	_, err := HasOperationalOverview(context.Background(), overviewSettings{err: errors.New("db")}, overviewStaff{true}, false, true)
	if err == nil {
		t.Fatal("settings error must propagate")
	}
}

func TestOperationalOverviewMissingSettingsFailsClosed(t *testing.T) {
	t.Parallel()
	got, err := HasOperationalOverview(context.Background(), nil, overviewStaff{true}, false, true)
	if err != nil || got {
		t.Fatalf("HasOperationalOverview() = %v, %v; want false, nil", got, err)
	}
}
