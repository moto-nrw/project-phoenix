package services

import (
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
)

// SFTPExportSettings hands the composition root the SFTP resolvers and their
// setting keys for the Export Transfer capability (#3050).
//
// It exists so the root can wire that module without naming the settings
// package: the key strings stay next to their registry definitions, and the
// root only forwards the binding.
func (f *Factory) SFTPExportSettings() (settingsCompose.SFTPExportResolvers, settingsCompose.SFTPExportKeys) {
	return settingsCompose.NewSFTPExportSettings(f.Settings)
}
