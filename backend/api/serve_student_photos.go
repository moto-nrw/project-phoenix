package api

import (
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
)

// buildServeStudentPhotoRuntime binds photo deletion to the server's local uploads.
func buildServeStudentPhotoRuntime(db transactionruntime.Database, env owners.RootEnvironment, settings settingsCompose.SettingsService, hub *realtime.Hub, audit owners.AuditOwner) owners.StudentPhotoRuntime {
	unlinker := peopledirectory.NewPhotoUnlinker(env.Logger.With("component", "student-photo-unlinker"), "public", nativeUploads.UploadFiles{})
	return owners.BuildStudentPhotoRuntime(db, env, settings, hub, audit, unlinker, actingAccountID)
}
