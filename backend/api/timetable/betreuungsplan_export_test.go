package timetable

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Betreuungsplan export route calls the public plan export capability
// (#2706). These tests pin the wire contract of that call over a capability
// fake: the default template, the parameter refusal, the internal-variant
// gate and the file headers.

type fakePlanExport struct {
	params planexport.Params
	file   listexport.File
	err    error
}

func (f *fakePlanExport) ExportDienstplan(context.Context, planexport.Params) (listexport.File, error) {
	return listexport.File{}, errors.New("not the care plan")
}

func (f *fakePlanExport) ExportBetreuungsplan(_ context.Context, params planexport.Params) (listexport.File, error) {
	f.params = params
	return f.file, f.err
}

func exportRequest(t *testing.T, body map[string]any, perms []string) (*fakePlanExport, int, http.Header, string) {
	t.Helper()
	fake := &fakePlanExport{file: listexport.File{Data: []byte("%PDF-1.7"), ContentType: "application/pdf", Filename: "Betreuungsplan 2026-07-27.pdf"}}
	rs := &Resource{Dependencies: Dependencies{PlanExportService: fake}}
	router := setupTestRouter(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), jwt.CtxPermissions, perms)
		rs.exportBetreuungsplan(w, r.WithContext(ctx))
	}, http.MethodPost, false)
	recorder := executeRequest(router, http.MethodPost, "/", body)
	return fake, recorder.Code, recorder.Header(), recorder.Body.String()
}

func TestBetreuungsplanExportDefaultsTheTemplateAndStreamsTheFile(t *testing.T) {
	t.Parallel()

	fake, code, header, body := exportRequest(t, map[string]any{"from": "2026-07-27", "to": "2026-07-31"}, []string{permissions.SchedulesRead})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, planexport.TemplateByOffering, fake.params.Template)
	assert.Equal(t, planexport.VariantNotice, fake.params.Variant)
	assert.Equal(t, listexport.FormatPDF, fake.params.Format)
	assert.Equal(t, planexport.Date("2026-07-27"), fake.params.From)
	assert.Equal(t, "application/pdf", header.Get("Content-Type"))
	assert.Equal(t, `attachment; filename="Betreuungsplan 2026-07-27.pdf"`, header.Get("Content-Disposition"))
	assert.Equal(t, "8", header.Get("Content-Length"))
	assert.Equal(t, "%PDF-1.7", body)
}

func TestBetreuungsplanExportRefusesBadParameters(t *testing.T) {
	t.Parallel()

	fake, code, _, _ := exportRequest(t, map[string]any{"from": "27.07.2026", "to": "2026-07-31"}, []string{permissions.SchedulesRead})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Empty(t, fake.params.From, "an unparsable day never reaches the capability")

	fake, code, _, _ = exportRequest(t, map[string]any{"from": "2026-07-27", "to": "2026-07-31", "format": "docx"}, []string{permissions.SchedulesRead})
	assert.Equal(t, http.StatusOK, code, "the handler defers format validation to the capability")
	assert.Equal(t, listexport.FormatDOCX, fake.params.Format)
}

func TestBetreuungsplanExportMapsCapabilityErrors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		err  error
		code int
	}{
		"invalid params": {planexport.ErrRangeTooLarge, http.StatusBadRequest},
		"renderer down":  {errors.New("boom"), http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := &fakePlanExport{err: tc.err}
			rs := &Resource{Dependencies: Dependencies{PlanExportService: fake}}
			router := setupTestRouter(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), jwt.CtxPermissions, []string{permissions.SchedulesRead})
				rs.exportBetreuungsplan(w, r.WithContext(ctx))
			}, http.MethodPost, false)
			recorder := executeRequest(router, http.MethodPost, "/", map[string]any{"from": "2026-07-27", "to": "2026-07-31"})
			assert.Equal(t, tc.code, recorder.Code)
		})
	}
}

func TestBetreuungsplanExportGatesTheInternalVariant(t *testing.T) {
	t.Parallel()

	body := map[string]any{"from": "2026-07-27", "to": "2026-07-31", "variant": "intern"}
	fake, code, _, _ := exportRequest(t, body, []string{permissions.SchedulesRead})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Empty(t, fake.params.Variant, "the internal sheet is not rendered for a reader")

	fake, code, _, _ = exportRequest(t, body, []string{permissions.SchedulesRead, permissions.SchedulesManage})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, planexport.VariantInternal, fake.params.Variant)
}

func TestBetreuungsplanExportRequiresTheWiredCapability(t *testing.T) {
	t.Parallel()

	rs := &Resource{}
	router := setupTestRouter(rs.exportBetreuungsplan, http.MethodPost, false)
	recorder := executeRequest(router, http.MethodPost, "/", map[string]any{"from": "2026-07-27", "to": "2026-07-31"})
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}
