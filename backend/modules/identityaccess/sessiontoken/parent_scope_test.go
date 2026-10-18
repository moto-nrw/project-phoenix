package sessiontoken

import "testing"

func TestAppClaims_IsParentScope(t *testing.T) {
	t.Parallel()

	for scope, want := range map[string]bool{"parent": true, "": false, "school": false, "platform": false, "tenant": false} {
		c := &AppClaims{Scope: scope}
		if got := c.IsParentScope(); got != want {
			t.Errorf("IsParentScope() with scope %q = %v, want %v", scope, got, want)
		}
	}
}
