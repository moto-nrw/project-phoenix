package api

import (
	"context"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// This file binds the five surfaces the People Directory photo lifecycle needs
// beyond its own rows (#3349): the tenant feature flag, the caller's access,
// the acting account, the stored-file cleanup with the live refresh, and the
// Audit Platform consent trail. They only exist once the HTTP layer is up,
// which is why the owner takes them through a late binder.

// StudentPhotoRuntimeDependencies are the surfaces the photo lifecycle needs
// from the other owners.
type StudentPhotoRuntimeDependencies struct {
	Settings    peoplePorts.PhotoSettings
	Broadcaster owners.PhotoBroadcaster
	Unlinker    peoplePorts.PhotoUnlinker
	Consents    owners.StudentPhotoConsentRecorder
	Logger      *slog.Logger
	// ActingAccount reads the acting account of the request a granted
	// consent is recorded for; nil records none.
	ActingAccount func(context.Context) int64
}

// StudentPhotoFeatureToggle is the one photo lifecycle operation the settings
// side effects call when the feature flag changes.
type StudentPhotoFeatureToggle interface {
	HandleFeatureToggle(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)
}
