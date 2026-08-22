package platform

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validOperatorRefreshToken() *OperatorRefreshToken {
	return &OperatorRefreshToken{
		Model: base.Model{
			ID:        91,
			CreatedAt: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.January, 2, 4, 5, 6, 0, time.UTC),
		},
		OperatorID: 77,
		Token:      "opaque-refresh-handle",
		Expiry:     time.Date(2026, time.January, 3, 3, 4, 5, 0, time.UTC),
		FamilyID:   "refresh-family",
		Generation: 2,
	}
}

func TestOperatorRefreshToken_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*OperatorRefreshToken)
		wantErr string
	}{
		{
			name: "valid",
		},
		{
			name: "requires operator id",
			mutate: func(token *OperatorRefreshToken) {
				token.OperatorID = 0
			},
			wantErr: "operator ID is required",
		},
		{
			name: "requires token",
			mutate: func(token *OperatorRefreshToken) {
				token.Token = ""
			},
			wantErr: "token value is required",
		},
		{
			name: "requires expiry",
			mutate: func(token *OperatorRefreshToken) {
				token.Expiry = time.Time{}
			},
			wantErr: "expiry is required",
		},
		{
			name: "requires family id",
			mutate: func(token *OperatorRefreshToken) {
				token.FamilyID = ""
			},
			wantErr: "family ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := validOperatorRefreshToken()
			if tt.mutate != nil {
				tt.mutate(token)
			}

			err := token.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOperatorRefreshToken_EntityAccessors(t *testing.T) {
	t.Parallel()

	token := validOperatorRefreshToken()

	assert.Equal(t, token.ID, token.GetID())
	assert.Equal(t, token.CreatedAt, token.GetCreatedAt())
	assert.Equal(t, token.UpdatedAt, token.GetUpdatedAt())
}
