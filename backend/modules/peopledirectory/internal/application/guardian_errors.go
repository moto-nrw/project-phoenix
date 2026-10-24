package application

import (
	"errors"
)

// The guardian service's error vocabulary, moved with it from services/users
// (#3755). The model-free errors of the people services live in
// userscontract (#3750). The messages are diagnostic detail; the client shows
// the catalog text selected by the code.
var (
	// errGuardianServiceRolePresetsUnbound refuses a role assignment while the
	// composition root has not bound the role presets; nothing falls back to
	// a role of its own.
	errGuardianServiceRolePresetsUnbound = errors.New("guardian role presets are not wired; refusing role assignment")
)
