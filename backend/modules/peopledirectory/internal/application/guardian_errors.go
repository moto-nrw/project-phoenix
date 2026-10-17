package application

import (
	"errors"
)

// The guardian service's error vocabulary, moved with it from services/users
// (#3755). The model-free errors of the people services live in
// userscontract (#3750). Do not change the messages: they are rendered
// verbatim by the HTTP layer.
var (
	// errGuardianServiceRolePresetsUnbound refuses a role assignment while the
	// composition root has not bound the role presets; nothing falls back to
	// a role of its own.
	errGuardianServiceRolePresetsUnbound = errors.New("guardian role presets are not wired; refusing role assignment")
)
