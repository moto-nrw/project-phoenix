package services

import "testing"

func TestEnforceInstanceTimePolicy(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		appEnv string
		want   bool
	}{
		{appEnv: "development", want: false},
		{appEnv: "test", want: false},
		{appEnv: "staging", want: true},
		{appEnv: "production", want: true},
	} {
		t.Run(tt.appEnv, func(t *testing.T) {
			t.Parallel()
			if got := enforceInstanceTimePolicy(tt.appEnv); got != tt.want {
				t.Errorf("enforceInstanceTimePolicy(%q) = %t, want %t", tt.appEnv, got, tt.want)
			}
		})
	}
}
