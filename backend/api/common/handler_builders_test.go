package common_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// route mounts h under a one-param chi route and fires a request at it, so
// chi.URLParam resolution works exactly like production.
func route(t *testing.T, method, pattern, path string, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Method(method, pattern, h)
	rec := httptest.NewRecorder()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(rec, req)
	return rec
}

func TestIDAction(t *testing.T) {
	t.Run("success responds 204 and passes the parsed id", func(t *testing.T) {
		var got int64
		h := common.IDAction("id", "invalid id", func(_ context.Context, id int64) error {
			got = id
			return nil
		}, common.ErrorInternalServer)

		rec := route(t, http.MethodPost, "/things/{id}", "/things/42", h, "")
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, int64(42), got)
	})

	t.Run("int-typed service functions work unchanged", func(t *testing.T) {
		var got int
		h := common.IDAction("id", "invalid id", func(_ context.Context, id int) error {
			got = id
			return nil
		}, common.ErrorInternalServer)

		rec := route(t, http.MethodPost, "/things/{id}", "/things/7", h, "")
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, 7, got)
	})

	t.Run("non-numeric id renders 400 with the invalid message", func(t *testing.T) {
		called := false
		h := common.IDAction("id", "invalid thing ID", func(_ context.Context, _ int64) error {
			called = true
			return nil
		}, common.ErrorInternalServer)

		rec := route(t, http.MethodPost, "/things/{id}", "/things/abc", h, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid thing ID")
		assert.False(t, called)
	})

	t.Run("service error renders through renderErr", func(t *testing.T) {
		h := common.IDAction("id", "invalid id", func(_ context.Context, _ int64) error {
			return errors.New("boom")
		}, common.ErrorConflict)

		rec := route(t, http.MethodPost, "/things/{id}", "/things/1", h, "")
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "boom")
	})
}

func TestTwoIDAction(t *testing.T) {
	t.Run("success passes both ids in order", func(t *testing.T) {
		var a, b int
		h := common.TwoIDAction("accountId", "invalid account ID", "roleId", "invalid role ID",
			func(_ context.Context, id1, id2 int) error {
				a, b = id1, id2
				return nil
			}, common.ErrorInternalServer)

		rec := route(t, http.MethodPost, "/accounts/{accountId}/roles/{roleId}", "/accounts/3/roles/9", h, "")
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, 3, a)
		assert.Equal(t, 9, b)
	})

	t.Run("second id invalid renders its own message", func(t *testing.T) {
		h := common.TwoIDAction("accountId", "invalid account ID", "roleId", "invalid role ID",
			func(_ context.Context, _, _ int) error { return nil }, common.ErrorInternalServer)

		rec := route(t, http.MethodPost, "/accounts/{accountId}/roles/{roleId}", "/accounts/3/roles/x", h, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid role ID")
	})
}

func TestIDFetch(t *testing.T) {
	t.Run("success wraps the payload in the standard envelope", func(t *testing.T) {
		type thing struct {
			Name string `json:"name"`
		}
		h := common.IDFetch("id", "invalid id", func(_ context.Context, id int64) (*thing, error) {
			return &thing{Name: "x"}, nil
		}, common.ErrorInternalServer, "Thing retrieved")

		rec := route(t, http.MethodGet, "/things/{id}", "/things/5", h, "")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"status":"success"`)
		assert.Contains(t, rec.Body.String(), `"name":"x"`)
		assert.Contains(t, rec.Body.String(), "Thing retrieved")
	})

	t.Run("fetch error renders through renderErr", func(t *testing.T) {
		h := common.IDFetch("id", "invalid id", func(_ context.Context, _ int64) (string, error) {
			return "", errors.New("gone")
		}, common.ErrorNotFound, "")

		rec := route(t, http.MethodGet, "/things/{id}", "/things/5", h, "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

type bindReq struct {
	Name string `json:"name"`
}

func (b *bindReq) Bind(_ *http.Request) error {
	if b.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func TestBindAction(t *testing.T) {
	newReq := func() *bindReq { return &bindReq{} }

	t.Run("success responds with the given status", func(t *testing.T) {
		h := common.BindAction(newReq, http.StatusCreated, func(_ context.Context, req *bindReq) (string, error) {
			return "made-" + req.Name, nil
		}, common.ErrorInternalServer, "Created")

		rec := route(t, http.MethodPost, "/things", "/things", h, `{"name":"a"}`)
		require.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "made-a")
	})

	t.Run("bind validation failure renders 400", func(t *testing.T) {
		h := common.BindAction(newReq, http.StatusCreated, func(_ context.Context, _ *bindReq) (string, error) {
			t.Fatal("service must not be called on bind failure")
			return "", nil
		}, common.ErrorInternalServer, "")

		rec := route(t, http.MethodPost, "/things", "/things", h, `{"name":""}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "name is required")
	})
}
