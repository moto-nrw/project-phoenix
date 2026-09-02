package timetracking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// notFoundSentinel mirrors models/base.ErrNotFound by its marker method.
type notFoundSentinel struct{}

func (notFoundSentinel) Error() string       { return "repository: not found" }
func (notFoundSentinel) RepositoryNotFound() {}

func TestRequireBalanceAdjustmentStaffClassifiesLookupErrors(t *testing.T) {
	t.Parallel()

	// The repository not-found result is what database/repositories/base
	// TranslateNotFound produces: the persistence-neutral sentinel joined
	// onto sql.ErrNoRows, wrapped by the domain error. This package may
	// import neither models/base nor database/sql, so the shape is rebuilt
	// with the RepositoryNotFound marker common.IsNotFound matches.
	notFound := fmt.Errorf("find by id: %w", errors.Join(
		notFoundSentinel{},
		errors.New("sql: no rows in result set"),
	))

	tests := []struct {
		name       string
		lookupErr  error
		wantStatus int
	}{
		{
			name:       "missing staff",
			lookupErr:  notFound,
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
			resource := &StaffAdminResource{
				logger: slog.Default(),
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
