package groups

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/education"
)

// ErrorRenderer renders an error to an HTTP response.
//
// User-facing branches render the inner sentinel directly so the JSON
// `error` field carries just the (German) message instead of EducationError's
// "education: {Op}: …" prefix. The 500 fallback keeps the wrapped eduErr so
// server logs still capture the operation context. Same pattern as
// api/rooms/errors.go.
func ErrorRenderer(err error) render.Renderer {
	if eduErr, ok := err.(*education.EducationError); ok {
		inner := eduErr.Unwrap()
		switch inner {
		case education.ErrGroupNotFound:
			return common.ErrorNotFound(inner)
		case education.ErrDuplicateGroup:
			return common.ErrorConflict(inner)
		case education.ErrRoomNotFound:
			return common.ErrorInvalidRequest(inner)
		case education.ErrTeacherNotFound:
			return common.ErrorInvalidRequest(inner)
		case education.ErrGroupTeacherNotFound:
			return common.ErrorNotFound(inner)
		case education.ErrGroupHasStudents:
			return common.ErrorConflict(inner)
		default:
			return common.ErrorInternalServer(eduErr)
		}
	}

	return common.ErrorInternalServer(err)
}
