package users

import (
	"github.com/moto-nrw/project-phoenix/api/common"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// errorRules map users-service sentinels to HTTP responses. Matched via
// errors.Is against the full error and rendered with the wrapper text
// (this package never stripped the "users error during {Op}: …" prefix).
var errorRules = []common.ErrorRule{
	{Target: usersSvc.ErrPersonNotFound, Render: common.ErrorNotFound},
	{Target: usersSvc.ErrAccountNotFound, Render: common.ErrorNotFound},
	{Target: usersSvc.ErrRFIDCardNotFound, Render: common.ErrorNotFound},
	{Target: usersSvc.ErrAccountAlreadyLinked, Render: common.ErrorConflict},
	{Target: usersSvc.ErrRFIDCardAlreadyLinked, Render: common.ErrorConflict},
	{Target: usersSvc.ErrPersonIdentifierRequired, Render: common.ErrorInvalidRequest},
}

// ErrorRenderer renders an error to an HTTP response.
var ErrorRenderer = common.RulesRenderer(errorRules, common.ErrorInternalServer)
