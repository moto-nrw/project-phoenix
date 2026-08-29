package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// File storage settings (#2596). Who may manage folders is a permission
// (files:manage); whether the rest of the team may add files is a school
// decision, so it is a setting. The default is off: a new school starts with a
// storage the leadership curates, and opens it deliberately.
func init() {
	config.Register(config.Definition{
		Key:             config.KeyFilesStaffUploadEnabled,
		Label:           "Team darf Dateien hochladen",
		Description:     "Wenn diese Einstellung eingeschaltet ist, darf auch das Team Dateien hochladen. Mitarbeitende können eigene Dateien wieder löschen. Ordner verwaltet weiterhin die Leitung.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "dateien",
		SortOrder:       1,
	})

	config.Register(config.Definition{
		Key:             config.KeyFilesMaxStorageMB,
		Label:           "Speicherplatz für Dateien (MB)",
		Description:     "Höchstgröße aller Dateien in der Dateiablage zusammen. Ist der Platz voll, müssen erst Dateien gelöscht werden.",
		Type:            config.FieldNumber,
		Default:         1024,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "dateien",
		SortOrder:       2,
		Validation:      config.Range(100, 51200),
	})
}
