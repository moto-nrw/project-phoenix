package usercontext

import (
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// errorRules map usercontext-service sentinels to HTTP responses. Matched
// via errors.Is against the full error and rendered with the wrapper text.
var errorRules = []common.ErrorRule{
	{Target: usercontext.ErrUserNotAuthenticated, Render: common.ErrorUnauthorized},
	{Target: usercontext.ErrUserNotAuthorized, Render: common.ErrorForbidden},
	{Target: usercontext.ErrUserNotFound, Render: common.ErrorNotFound},
	{Target: usercontext.ErrUserNotLinkedToPerson, Render: common.ErrorNotFound},
	{Target: usercontext.ErrUserNotLinkedToStaff, Render: common.ErrorNotFound},
	{Target: usercontext.ErrUserNotLinkedToTeacher, Render: common.ErrorNotFound},
	{Target: usercontext.ErrGroupNotFound, Render: common.ErrorNotFound},
	{Target: usercontext.ErrNoActiveGroups, Render: common.ErrorNotFound},
	{Target: usercontext.ErrInvalidOperation, Render: common.ErrorInvalidRequest},
}

// ErrorRenderer maps usercontext service errors to HTTP error responses.
var ErrorRenderer = common.RulesRenderer(errorRules, common.ErrorInternalServer)
