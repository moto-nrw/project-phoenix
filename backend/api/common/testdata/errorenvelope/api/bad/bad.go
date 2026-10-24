package bad

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
)

const unknownCode = "care.not_registered"

func refuse(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "nope", http.StatusBadRequest)
	http.NotFound(w, r)
}

// legacyBody sets an error status in Render but keeps the old members.
type legacyBody struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (b *legacyBody) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusConflict)
	return nil
}

// messageBody embeds the shared envelope but adds a message text.
type messageBody struct {
	*common.ErrResponse
	Message string `json:"message"`
}

// problem has the envelope's shape; its literal names an unknown code.
type problem struct {
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

var unknown = problem{Status: http.StatusConflict, Detail: "x", Code: unknownCode}

func legacyMap() map[string]any {
	return map[string]any{"status": "error", "error": "x"}
}

// rowFailure is the old envelope without a Render method.
type rowFailure struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// rowResult is a per-row result inside a success body, not an error body.
type rowResult struct {
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

// shared embeds the envelope and adds an extension member.
type shared struct {
	*common.ErrResponse
	Conflicts []string `json:"conflicts"`
}

func messageMap() map[string]any {
	return map[string]any{"status": http.StatusConflict, "message": "x"}
}

func codelessMap() map[string]any {
	return map[string]any{"status": http.StatusConflict, "detail": "x"}
}

func codelessStatusMap() map[string]any {
	return map[string]any{"status": http.StatusForbidden}
}

// successMap is a success envelope, not an error body.
func successMap() map[string]any {
	return map[string]any{"status": http.StatusOK, "message": "ok"}
}

func rawErrorWrite(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("standby"))
}

// statusOnly writes an error status without a body; the middleware answers it.
func statusOnly(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}

// titledProblem is an error body by its title and status but has no code.
type titledProblem struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}
