package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/slotlists"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSlotListsService struct {
	optionsResult *slotlists.OptionsResult
	optionsErr    error
	buildResult   *slotlists.Result
	buildErr      error
	renderFile    listexport.File
	renderErr     error
	buildCalled   bool
	lastDate      timezone.Date
	lastParams    slotlists.Params
	lastFormat    listexport.Format
}

func (m *mockSlotListsService) BuildList(_ context.Context, params slotlists.Params) (*slotlists.Result, error) {
	m.buildCalled = true
	m.lastParams = params
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	return m.buildResult, nil
}

func (m *mockSlotListsService) ListOptions(_ context.Context, date timezone.Date) (*slotlists.OptionsResult, error) {
	m.lastDate = date
	if m.optionsErr != nil {
		return nil, m.optionsErr
	}
	return m.optionsResult, nil
}

func (m *mockSlotListsService) RenderList(_ context.Context, params slotlists.Params, format listexport.Format) (listexport.File, error) {
	m.lastParams = params
	m.lastFormat = format
	if m.renderErr != nil {
		return listexport.File{}, m.renderErr
	}
	return m.renderFile, nil
}

func slotListTestRouter(handler http.HandlerFunc, path string) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post(path, handler)
	return r
}

func slotListPost(t *testing.T, router chi.Router, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestPreviewSlotListParsesExplicitEmptySlotSelection(t *testing.T) {
	mock := &mockSlotListsService{
		buildResult: &slotlists.Result{
			Date:   "2030-09-04",
			Target: slotlists.TargetSlots,
			Source: slotlists.SourceReconciliation,
			Rows:   []slotlists.Row{},
		},
	}
	rs := NewResource(Dependencies{SlotListsService: mock})
	router := slotListTestRouter(rs.previewSlotList, "/preview")

	w := slotListPost(t, router, "/preview", map[string]any{
		"date":         "2030-09-04",
		"target":       "slots",
		"source":       "reconciliation",
		"instance_ids": []string{},
	})

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, mock.lastParams.InstanceIDsSet, "explicit [] must not be treated like omitted instance_ids")
	assert.Empty(t, mock.lastParams.InstanceIDs)
	assert.Equal(t, slotlists.TargetSlots, mock.lastParams.Target)
	assert.Equal(t, slotlists.SourceReconciliation, mock.lastParams.Source)
}

func TestPreviewSlotListParsesStringIDs(t *testing.T) {
	mock := &mockSlotListsService{
		buildResult: &slotlists.Result{
			Date:   "2030-09-04",
			Target: slotlists.TargetSlots,
			Source: slotlists.SourcePlanned,
			Rows:   []slotlists.Row{},
		},
	}
	rs := NewResource(Dependencies{SlotListsService: mock})
	router := slotListTestRouter(rs.previewSlotList, "/preview")

	w := slotListPost(t, router, "/preview", map[string]any{
		"date":         "2030-09-04",
		"target":       "slots",
		"source":       "planned",
		"instance_ids": []string{"9007199254740993"},
		"group_ids":    []string{"9223372036854775807"},
	})

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []int64{9007199254740993}, mock.lastParams.InstanceIDs)
	assert.Equal(t, []int64{9223372036854775807}, mock.lastParams.GroupIDs)
}

func TestPreviewSlotListRejectsInvalidGroupingForTarget(t *testing.T) {
	mock := &mockSlotListsService{}
	rs := NewResource(Dependencies{SlotListsService: mock})
	router := slotListTestRouter(rs.previewSlotList, "/preview")

	w := slotListPost(t, router, "/preview", map[string]any{
		"date":          "2030-09-04",
		"target":        "pickup_cohort",
		"pickup_cohort": "short_day",
		"source":        "planned",
		"group_by":      "slot",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, mock.buildCalled, "invalid request must not call the service")
}

func TestExportSlotListDefaultsToPDFAndStreamsFile(t *testing.T) {
	mock := &mockSlotListsService{
		renderFile: listexport.File{
			Data:        []byte("%PDF test"),
			ContentType: "application/pdf",
			Filename:    "tagesliste-plan-angebote-2030-09-04.pdf",
		},
	}
	rs := NewResource(Dependencies{SlotListsService: mock})
	router := slotListTestRouter(rs.exportSlotList, "/export")

	w := slotListPost(t, router, "/export", map[string]any{
		"date":   "2030-09-04",
		"target": "slots",
		"source": "planned",
	})

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, listexport.FormatPDF, mock.lastFormat)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "tagesliste-plan-angebote-2030-09-04.pdf")
	assert.Equal(t, "%PDF test", w.Body.String())
}

func TestExportSlotListReportsRenderErrors(t *testing.T) {
	mock := &mockSlotListsService{renderErr: errors.New("renderer failed")}
	rs := NewResource(Dependencies{SlotListsService: mock})
	router := slotListTestRouter(rs.exportSlotList, "/export")

	w := slotListPost(t, router, "/export", map[string]any{
		"date":   "2030-09-04",
		"target": "slots",
		"source": "planned",
		"format": "xlsx",
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, listexport.FormatXLSX, mock.lastFormat)
}

func TestExportSlotListMapsDriftToConflict(t *testing.T) {
	mock := &mockSlotListsService{renderErr: slotlists.ErrListDrifted}
	rs := NewResource(Dependencies{SlotListsService: mock})
	router := slotListTestRouter(rs.exportSlotList, "/export")

	w := slotListPost(t, router, "/export", map[string]any{
		"date":               "2030-09-04",
		"target":             "slots",
		"source":             "planned",
		"expected_signature": "stale-hash",
	})

	// A drift refusal must surface as 409 (client refreshes and re-checks), not
	// a 500 that reads as a server fault (#1565 review pass 2).
	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "seit der Vorschau geändert")
}

func TestExportSlotListForwardsExpectedSignature(t *testing.T) {
	mock := &mockSlotListsService{
		renderFile: listexport.File{
			Data:        []byte("%PDF test"),
			ContentType: "application/pdf",
			Filename:    "tagesliste.pdf",
		},
	}
	rs := NewResource(Dependencies{SlotListsService: mock})
	router := slotListTestRouter(rs.exportSlotList, "/export")

	w := slotListPost(t, router, "/export", map[string]any{
		"date":               "2030-09-04",
		"target":             "slots",
		"source":             "planned",
		"expected_signature": "abc123",
	})

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "abc123", mock.lastParams.ExpectedSignature)
}
