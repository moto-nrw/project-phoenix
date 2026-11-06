package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Thresholds of the hints on the Träger dashboard (#4675): the Träger-Büro
// decides from which value a school shows up under „Auffälligkeiten“. Only
// the Träger sets them; 0 switches a hint off. The defaults are a starting
// point, not a legal care ratio: one staff member for more than 20 present
// children, a tenth of the children sick, one child waiting for a decision.
func init() {
	registerDefinitions(traegerDashboardDefinitions)
}

var traegerDashboardDefinitions = []config.Definition{
	{
		Key:             config.KeyTraegerAlertChildrenPerStaff,
		Levels:          config.LevelTraeger,
		Label:           "Höchstens so viele Kinder je Betreuungskraft",
		Description:     "Kommen auf eine anwesende Betreuungskraft mehr Kinder, zeigt das Dashboard „Zu wenig Personal“. Das gilt auch, wenn Kinder da sind, aber keine Betreuungskraft. 0 schaltet den Hinweis aus.",
		Type:            config.FieldNumber,
		Default:         20,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "dashboard",
		Category:        "auffaelligkeiten",
		SortOrder:       10,
		Validation:      config.Range(0, 100),
	},
	{
		Key:             config.KeyTraegerAlertSickPercent,
		Levels:          config.LevelTraeger,
		Label:           "Viele Krankmeldungen ab (Prozent)",
		Description:     "Ist heute mindestens dieser Anteil der Kinder einer Schule krank gemeldet, zeigt das Dashboard „Viele Krankmeldungen“. 0 schaltet den Hinweis aus.",
		Type:            config.FieldNumber,
		Default:         10,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "dashboard",
		Category:        "auffaelligkeiten",
		SortOrder:       20,
		Validation:      config.Range(0, 100),
	},
	{
		Key:             config.KeyTraegerAlertOpenEnrollments,
		Levels:          config.LevelTraeger,
		Label:           "Offene Anmeldungen ab",
		Description:     "Warten an einer Schule mindestens so viele Kinder aus der Online-Anmeldung auf eine Entscheidung, zeigt das Dashboard „Offene Anmeldungen“. 0 schaltet den Hinweis aus.",
		Type:            config.FieldNumber,
		Default:         1,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "dashboard",
		Category:        "auffaelligkeiten",
		SortOrder:       30,
		Validation:      config.Range(0, 1000),
	},
}
