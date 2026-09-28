package authorize

import (
	"context"
	"errors"
	"testing"
)

type actionScopeSettings map[string]string

func (s actionScopeSettings) ResolveString(_ context.Context, key string) (string, error) {
	if value, ok := s[key]; ok {
		return value, nil
	}
	return "", errors.New("settings unavailable")
}

func TestSchoolWideActionScope(t *testing.T) {
	t.Parallel()
	const actionKey = "operations.block_start_scope"
	tests := []struct {
		name               string
		action, visibility string
		want, wantErr      bool
	}{
		{name: "all staff with school-wide visibility", action: "all_staff", visibility: "all_staff", want: true},
		{name: "own action", action: "own", visibility: "all_staff"},
		{name: "unknown action", action: "unknown", visibility: "all_staff"},
		{name: "personal visibility", action: "all_staff", visibility: "own"},
		{name: "legacy personal visibility", action: "all_staff", visibility: "admins"},
		{name: "visibility failure", action: "all_staff", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			settings := actionScopeSettings{actionKey: tt.action}
			if tt.visibility != "" {
				settings[operationalOverviewScopeKey] = tt.visibility
			}
			got, err := SchoolWideActionScope(context.Background(), settings, actionKey)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("SchoolWideActionScope() = %v, %v; want %v, error %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
	if _, err := SchoolWideActionScope(context.Background(), actionScopeSettings{}, actionKey); err == nil {
		t.Fatal("an action scope lookup failure must not read as a denial")
	}
}
