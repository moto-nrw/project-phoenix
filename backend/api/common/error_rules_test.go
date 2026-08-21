package common_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
)

var (
	errRuleNotFound = errors.New("thing not found")
	errRuleConflict = errors.New("thing conflicts")
)

// fakeDomainError mirrors the domain wrapper shape (EducationError,
// FacilitiesError, ...) used by the typed-unwrap packages.
type fakeDomainError struct {
	Op  string
	Err error
}

func (e *fakeDomainError) Error() string { return fmt.Sprintf("fake: %s: %v", e.Op, e.Err) }
func (e *fakeDomainError) Unwrap() error { return e.Err }

func renderStatus(t *testing.T, renderer render.Renderer) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	require.NoError(t, render.Render(rec, req, renderer))
	return rec.Code, rec.Body.String()
}

func TestRenderWithRules(t *testing.T) {
	t.Parallel()

	rules := []common.ErrorRule{
		{Target: errRuleNotFound, Render: common.ErrorNotFound},
		{Target: errRuleConflict, Render: common.ErrorConflict},
	}

	t.Run("first matching rule wins", func(t *testing.T) {
		code, _ := renderStatus(t, common.RenderWithRules(errRuleNotFound, rules, common.ErrorInternalServer))
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("wrapped sentinel still matches via errors.Is", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", errRuleConflict)
		code, body := renderStatus(t, common.RenderWithRules(wrapped, rules, common.ErrorInternalServer))
		assert.Equal(t, http.StatusConflict, code)
		// RenderWithRules renders the FULL error, wrapper prefix included.
		assert.Contains(t, body, "outer: thing conflicts")
	})

	t.Run("no match falls through to fallback", func(t *testing.T) {
		code, _ := renderStatus(t, common.RenderWithRules(errors.New("unknown"), rules, common.ErrorInternalServer))
		assert.Equal(t, http.StatusInternalServerError, code)
	})
}

func TestRulesRenderer(t *testing.T) {
	t.Parallel()

	renderer := common.RulesRenderer([]common.ErrorRule{
		{Target: errRuleNotFound, Render: common.ErrorNotFound},
	}, common.ErrorInternalServer)

	code, _ := renderStatus(t, renderer(errRuleNotFound))
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = renderStatus(t, renderer(errors.New("other")))
	assert.Equal(t, http.StatusInternalServerError, code)
}

func TestUnwrapRenderer(t *testing.T) {
	t.Parallel()

	renderer := common.UnwrapRenderer[*fakeDomainError]([]common.ErrorRule{
		{Target: errRuleNotFound, Render: common.ErrorNotFound},
		{Target: errRuleConflict, Render: common.ErrorConflict},
	}, common.ErrorInternalServer)

	t.Run("renders the inner sentinel without the wrapper prefix", func(t *testing.T) {
		code, body := renderStatus(t, renderer(&fakeDomainError{Op: "create", Err: errRuleNotFound}))
		assert.Equal(t, http.StatusNotFound, code)
		assert.Contains(t, body, "thing not found")
		assert.NotContains(t, body, "fake: create")
	})

	t.Run("unmatched inner error falls to fallback with the FULL error", func(t *testing.T) {
		code, body := renderStatus(t, renderer(&fakeDomainError{Op: "create", Err: errors.New("weird")}))
		assert.Equal(t, http.StatusInternalServerError, code)
		assert.Contains(t, body, "fake: create: weird")
	})

	t.Run("non-domain error falls to fallback", func(t *testing.T) {
		code, _ := renderStatus(t, renderer(errors.New("plain")))
		assert.Equal(t, http.StatusInternalServerError, code)
	})

	t.Run("domain error nested inside another wrapper still unwraps", func(t *testing.T) {
		nested := fmt.Errorf("outer: %w", &fakeDomainError{Op: "get", Err: errRuleConflict})
		code, body := renderStatus(t, renderer(nested))
		assert.Equal(t, http.StatusConflict, code)
		assert.Contains(t, body, "thing conflicts")
	})
}
