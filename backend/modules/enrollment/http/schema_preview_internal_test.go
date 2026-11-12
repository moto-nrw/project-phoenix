package enrollmenthttp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

type previewErrorRequestService struct {
	RequestService
	err error
}

func (s previewErrorRequestService) LoadPhasePreviewCareOfferings(context.Context, int64) (*capability.PublicFormBootstrapData, error) {
	return nil, s.err
}

func TestSchemaPreviewBootstrapErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		schemaErr error
		phaseErr  error
		listErr   error
		status    int
		code      string
	}{
		{name: "missing schema", schemaErr: capability.ErrFormSchemaNotFound, status: http.StatusNotFound, code: "enrollment.schema_not_found"},
		{name: "missing phase", phaseErr: fmt.Errorf("preview: %w", capability.ErrPhaseNotFound), status: http.StatusNotFound, code: "enrollment.phase_not_found"},
		{name: "phase database failure", phaseErr: errors.New("database unavailable"), status: http.StatusInternalServerError},
		{name: "phase list failure", listErr: fmt.Errorf("list phases: %w", sql.ErrNoRows), status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs := &Resource{
				FormSchemaService: &mockFormSchemaService{getByIDErr: tc.schemaErr},
				PhaseService:      &mockPhaseService{listErr: tc.listErr},
				RequestService:    previewErrorRequestService{err: tc.phaseErr},
			}
			router := chi.NewRouter()
			router.Get("/schema/preview", rs.getSchemaPreviewBootstrap)
			path := "/schema/preview?base=1&phaseId=42"
			if tc.schemaErr != nil {
				path = "/schema/preview?schemaId=42"
			} else if tc.listErr != nil {
				path = "/schema/preview?base=1"
			}
			w := executeSchemaJSON(t, router, http.MethodGet, path, nil)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.code != "" {
				var problem struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
				require.Equal(t, tc.code, problem.Code)
			}
		})
	}
}
