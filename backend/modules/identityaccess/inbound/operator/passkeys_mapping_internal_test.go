package operator

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The passkey handler tests drive the routes from api/operator; the error
// mapping they cannot reach from there is pinned here (#3231).
func TestOperatorPasskeyErrorMapping(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	mapOperatorPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), ErrPasskeyNotFound)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	mapOperatorPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), errors.New("boom"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
