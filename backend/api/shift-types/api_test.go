// Integration tests for the shift-types handlers, focused on the optional
// Kategorie↔Schichtart sync added in the #1837 follow-up. The handlers are
// mounted behind a thin tenant-injecting middleware (mirroring the pattern in
// api/timetable/*_test.go), exercising create/update + the CategoryLinker
// composition end-to-end against the real activities service and DB.
package shifttypes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type shiftTypeTestSetup struct {
	res    *Resource
	db     *bun.DB
	router chi.Router
	ctx    context.Context
}

func buildShiftTypeSetup(t *testing.T) *shiftTypeTestSetup {
	t.Helper()
	db, svcs := testutil.SetupAPITest(t)

	res := NewResource(svcs.ShiftTypes, svcs.Activities, db, slog.Default())

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(tenant.WithTenantID(req.Context(), 1)))
		})
	})
	r.Post("/", res.create)
	r.Put("/{id}", res.update)

	return &shiftTypeTestSetup{res: res, db: db, router: r, ctx: testpkg.TenantContext(1)}
}

func (s *shiftTypeTestSetup) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *shiftTypeTestSetup) shiftTypeIDOf(t *testing.T, categoryID int64) *int64 {
	t.Helper()
	cat, err := repositories.NewFactory(s.db).ActivityCategory.FindByID(s.ctx, categoryID)
	require.NoError(t, err)
	return cat.ShiftTypeID
}

func decodeShiftTypeID(t *testing.T, w *httptest.ResponseRecorder) int64 {
	t.Helper()
	var env struct {
		Data ShiftTypeResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.NotZero(t, env.Data.ID)
	return env.Data.ID
}

func TestShiftType_CreateWithCategoryLinks(t *testing.T) {
	s := buildShiftTypeSetup(t)

	cat1 := testpkg.CreateTestActivityCategory(t, s.db, "st-create-1")
	cat2 := testpkg.CreateTestActivityCategory(t, s.db, "st-create-2")
	defer testpkg.CleanupActivityFixtures(t, s.db, cat1.ID, cat2.ID)

	w := s.do(t, http.MethodPost, "/", ShiftTypeRequest{
		Name:        fmt.Sprintf("Betreuung-%d", time.Now().UnixNano()),
		Color:       "#83CD2D",
		CategoryIDs: []int64{cat1.ID, cat2.ID},
	})
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	stID := decodeShiftTypeID(t, w)
	t.Cleanup(func() { _ = repositories.NewFactory(s.db).ShiftType.Delete(s.ctx, stID) })

	require.NotNil(t, s.shiftTypeIDOf(t, cat1.ID))
	assert.Equal(t, stID, *s.shiftTypeIDOf(t, cat1.ID))
	require.NotNil(t, s.shiftTypeIDOf(t, cat2.ID))
	assert.Equal(t, stID, *s.shiftTypeIDOf(t, cat2.ID))
}

func TestShiftType_UpdateSyncsAndOmittedLeavesUntouched(t *testing.T) {
	s := buildShiftTypeSetup(t)

	cat1 := testpkg.CreateTestActivityCategory(t, s.db, "st-upd-1")
	cat2 := testpkg.CreateTestActivityCategory(t, s.db, "st-upd-2")
	defer testpkg.CleanupActivityFixtures(t, s.db, cat1.ID, cat2.ID)

	name := fmt.Sprintf("Vorbereitung-%d", time.Now().UnixNano())
	cw := s.do(t, http.MethodPost, "/", ShiftTypeRequest{
		Name: name, Color: "#5080D8", CategoryIDs: []int64{cat1.ID, cat2.ID},
	})
	require.Equal(t, http.StatusCreated, cw.Code, "body=%s", cw.Body.String())
	stID := decodeShiftTypeID(t, cw)
	t.Cleanup(func() { _ = repositories.NewFactory(s.db).ShiftType.Delete(s.ctx, stID) })

	// Update narrowing the set to cat1 only -> cat2 is unlinked.
	active := true
	uw := s.do(t, http.MethodPut, fmt.Sprintf("/%d", stID), ShiftTypeRequest{
		Name: name, Color: "#5080D8", IsActive: &active, CategoryIDs: []int64{cat1.ID},
	})
	require.Equal(t, http.StatusOK, uw.Code, "body=%s", uw.Body.String())
	require.NotNil(t, s.shiftTypeIDOf(t, cat1.ID))
	assert.Equal(t, stID, *s.shiftTypeIDOf(t, cat1.ID))
	assert.Nil(t, s.shiftTypeIDOf(t, cat2.ID), "de-selected category unlinked on update")

	// Update WITHOUT category_ids (nil) must leave the mapping untouched.
	uw2 := s.do(t, http.MethodPut, fmt.Sprintf("/%d", stID), map[string]any{
		"name": name, "color": "#5080D8", "is_active": true,
	})
	require.Equal(t, http.StatusOK, uw2.Code, "body=%s", uw2.Body.String())
	require.NotNil(t, s.shiftTypeIDOf(t, cat1.ID), "omitted category_ids must not clear the mapping")
	assert.Equal(t, stID, *s.shiftTypeIDOf(t, cat1.ID))
}

func TestShiftType_UpdateRejectsUnknownCategoryIDs(t *testing.T) {
	s := buildShiftTypeSetup(t)

	cat1 := testpkg.CreateTestActivityCategory(t, s.db, "st-unknown-1")
	defer testpkg.CleanupActivityFixtures(t, s.db, cat1.ID)

	name := fmt.Sprintf("Betreuung-%d", time.Now().UnixNano())
	cw := s.do(t, http.MethodPost, "/", ShiftTypeRequest{
		Name: name, Color: "#83CD2D", CategoryIDs: []int64{cat1.ID},
	})
	require.Equal(t, http.StatusCreated, cw.Code, "body=%s", cw.Body.String())
	stID := decodeShiftTypeID(t, cw)
	t.Cleanup(func() { _ = repositories.NewFactory(s.db).ShiftType.Delete(s.ctx, stID) })
	require.NotNil(t, s.shiftTypeIDOf(t, cat1.ID))

	// An update carrying an unknown category id must be a 400 and must NOT clear
	// the existing mapping (#1837 review: validate before the destructive clear).
	active := true
	uw := s.do(t, http.MethodPut, fmt.Sprintf("/%d", stID), ShiftTypeRequest{
		Name: name, Color: "#83CD2D", IsActive: &active,
		CategoryIDs: []int64{cat1.ID, int64(999999999)},
	})
	require.Equal(t, http.StatusBadRequest, uw.Code, "body=%s", uw.Body.String())
	require.NotNil(t, s.shiftTypeIDOf(t, cat1.ID), "existing mapping must survive a rejected update")
	assert.Equal(t, stID, *s.shiftTypeIDOf(t, cat1.ID))
}
