package config

import (
	"context"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// SFTP target settings for the Export Transfer capability (#3050).
//
// The capability must not know which settings system stands behind it, and
// the composition root must not hardcode setting keys. Both are satisfied
// here: this file is the ONE place that maps the seven registry keys onto
// plain resolvers, next to the registry that defines them.

// SFTPExportResolvers reads the seven per-school SFTP values.
type SFTPExportResolvers struct {
	Enabled            func(context.Context) (bool, error)
	Host               func(context.Context) (string, error)
	Port               func(context.Context) (int, error)
	Username           func(context.Context) (string, error)
	Password           func(context.Context) (string, error)
	RemoteDirectory    func(context.Context) (string, error)
	HostKeyFingerprint func(context.Context) (string, error)
}

// SFTPExportKeys names the same settings, so an incomplete configuration can
// tell the school which fields are still empty using the keys the settings
// schema already carries labels for.
type SFTPExportKeys struct {
	Host               string
	Port               string
	Username           string
	Password           string
	RemoteDirectory    string
	HostKeyFingerprint string
}

// NewSFTPExportSettings binds the resolvers to a settings service.
//
// Resolution errors are passed through untouched. A settings store that
// cannot answer must not look like a school that has not filled in the form —
// the caller decides what to do, and both answers lead to no transfer, but
// only one of them is somebody's job to fix.
func NewSFTPExportSettings(settings SettingsService) (SFTPExportResolvers, SFTPExportKeys) {
	resolveString := func(key string) func(context.Context) (string, error) {
		return func(ctx context.Context) (string, error) { return settings.ResolveString(ctx, key) }
	}
	resolvers := SFTPExportResolvers{
		Enabled: func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, configModel.KeySFTPEnabled)
		},
		Port: func(ctx context.Context) (int, error) {
			return settings.ResolveInt(ctx, configModel.KeySFTPPort)
		},
		Host:               resolveString(configModel.KeySFTPHost),
		Username:           resolveString(configModel.KeySFTPUsername),
		Password:           resolveString(configModel.KeySFTPPassword),
		RemoteDirectory:    resolveString(configModel.KeySFTPRemoteDirectory),
		HostKeyFingerprint: resolveString(configModel.KeySFTPHostKeyFingerprint),
	}
	keys := SFTPExportKeys{
		Host:               configModel.KeySFTPHost,
		Port:               configModel.KeySFTPPort,
		Username:           configModel.KeySFTPUsername,
		Password:           configModel.KeySFTPPassword,
		RemoteDirectory:    configModel.KeySFTPRemoteDirectory,
		HostKeyFingerprint: configModel.KeySFTPHostKeyFingerprint,
	}
	return resolvers, keys
}
