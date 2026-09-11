package groups

import (
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/education"
)

// errorRules map education-service sentinels to HTTP responses.
//
// Rendered through common.UnwrapRenderer: user-facing branches render the
// inner sentinel directly so the JSON `error` field carries just the
// (German) message instead of EducationError's "education: {Op}: …" prefix.
// The 500 fallback keeps the wrapped error so server logs still capture the
// operation context. Same pattern as api/rooms/errors.go.
var errorRules = []common.ErrorRule{
	{Target: education.ErrGroupNotFound, Render: common.ErrorNotFound},
	{Target: education.ErrDuplicateGroup, Render: common.ErrorConflict},
	{Target: education.ErrRoomNotFound, Render: common.ErrorInvalidRequest},
	{Target: education.ErrTeacherNotFound, Render: common.ErrorInvalidRequest},
	{Target: education.ErrGroupTeacherNotFound, Render: common.ErrorNotFound},
	{Target: education.ErrGroupHasStudents, Render: common.ErrorConflict},
	{Target: education.ErrGroupHasHandover, Render: common.ErrorConflict},
}

// ErrorRenderer renders an error to an HTTP response.
var ErrorRenderer = common.UnwrapRenderer[*education.EducationError](errorRules, common.ErrorInternalServer)
