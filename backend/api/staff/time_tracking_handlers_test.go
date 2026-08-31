package staff

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

func TestAdminAbsenceErrorRules_ClassifyAllowanceErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		err  error
		want int
	}{
		{"invalid allowance", activeSvc.ErrAbsenceTypeAllowanceInvalid, http.StatusBadRequest},
		{"exceeded allowance", fmt.Errorf("booking: %w", activeSvc.ErrAbsenceTypeAllowanceExceeded), http.StatusConflict},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/staff/1/absences", nil)
			rec := httptest.NewRecorder()
			renderer := common.RenderWithRules(tt.err, adminAbsenceErrorRules, common.ErrorInternalServer)
			require.NoError(t, render.Render(rec, req, renderer))
			assert.Equal(t, tt.want, rec.Code)
		})
	}
}
