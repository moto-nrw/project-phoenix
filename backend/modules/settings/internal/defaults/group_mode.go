package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var groupModeDefinitions = []config.Definition{
	{
		Key:   config.KeyGroupMode,
		Label: "Arbeit mit festen Gruppen",
		Description: "Legt fest, ob Kinder im Alltag festen OGS-Gruppen zugeordnet sind oder ob alle berechtigten Mitarbeitenden mit allen Kindern arbeiten. " +
			"Diese Einstellung beschreibt nur die Organisation. Wer welche Räume in der Aktuellen Aufsicht sieht, steht unter \"Sicht auf alle Räume\".",
		Type:            config.FieldSelect,
		Default:         config.GroupModeFixedGroups,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "organisation",
		SortOrder:       1,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Feste Gruppen", Value: config.GroupModeFixedGroups},
				{Label: "Offene Betreuung ohne feste Gruppen", Value: config.GroupModeOpenCare},
			},
		},
	},
}
