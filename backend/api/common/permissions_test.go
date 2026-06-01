package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasAdminPermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		want        bool
	}{
		{
			name:        "admin wildcard grants access",
			permissions: []string{"rooms:read", "admin:*"},
			want:        true,
		},
		{
			name:        "global wildcard grants access",
			permissions: []string{"*:*"},
			want:        true,
		},
		{
			name:        "resource permissions are not admin permissions",
			permissions: []string{"rooms:read", "students:read"},
			want:        false,
		},
		{
			name:        "empty permissions do not grant access",
			permissions: nil,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasAdminPermissions(tt.permissions))
		})
	}
}
