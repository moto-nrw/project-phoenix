package staff

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireBalanceAdjustmentStaffClassifiesLookupErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		lookupErr  error
		wantStatus int
	}{
		{
			name: "missing staff",
			lookupErr: &modelBase.DatabaseError{
				Op:  "find by id",
				Err: sql.ErrNoRows,
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "database failure",
			lookupErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := &Resource{
				PersonService: &userstest.PersonServiceMock{
					GetStaffByIDFn: func(context.Context, int64) (*userModels.Staff, error) {
						return nil, tt.lookupErr
					},
				},
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/staff/42/time-tracking/adjustments", nil)

			found := resource.requireBalanceAdjustmentStaff(recorder, request, 42)

			require.False(t, found)
			assert.Equal(t, tt.wantStatus, recorder.Code)
		})
	}
}
