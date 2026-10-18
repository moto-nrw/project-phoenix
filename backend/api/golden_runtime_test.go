package api

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/stretchr/testify/require"
)

// goldenRuntime is the production server root a contract test drives: the
// assembled handler, its router and the pool the root runs over.
type goldenRuntime struct {
	handler http.Handler
	router  chi.Router
	db      transactionruntime.Database
}

// newGoldenRuntime reads the server root of a production runtime.
func newGoldenRuntime(t *testing.T, production *Runtime) *goldenRuntime {
	t.Helper()
	handler, ok := production.Handler().(*serveHandler)
	require.True(t, ok)
	return &goldenRuntime{handler: handler, router: handler.router, db: production.db}
}

// ServeHTTP routes a request through the assembled handler.
func (api *goldenRuntime) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.handler.ServeHTTP(w, r)
}

// Handler returns the fully assembled production HTTP graph the suites drive.
func (runtime *Runtime) Handler() http.Handler {
	return runtime.handler
}
