package rooms

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return common.ErrorInvalidRequest(err)
}
