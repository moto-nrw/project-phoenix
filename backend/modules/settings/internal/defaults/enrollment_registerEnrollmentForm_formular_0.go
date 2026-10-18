package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var enrollment_registerEnrollmentForm_formular_0Definitions = []config.Definition{
	{
		Key:             config.KeyEnrollmentCollectGradeLevel,
		Label:           "Klassenstufe abfragen",
		Description:     "Eltern wählen die zukünftige Klassenstufe ihres Kindes (1-13) im Formular. Die konkrete Klasse weist die Schulleitung später zu.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "formular",
		SortOrder:       20,
		DependsOn:       dependsOnEnabled,
	},
	{
		Key:             config.KeyEnrollmentCollectSchoolClass,
		Label:           "Konkrete Klasse abfragen (ab Klasse 2)",
		Description:     "Zusätzlich zur Klassenstufe wird ab der 2. Klasse die konkrete Klasse (z.B. 2a) abgefragt. Die auswählbaren Klassen und ob die Angabe verpflichtend ist, werden je Anmeldephase festgelegt. Für die 1. Klasse wird weiterhin nur die Klassenstufe erfasst.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "formular",
		SortOrder:       21,
		DependsOn: &config.Dependency{
			Key:       config.KeyEnrollmentCollectGradeLevel,
			Condition: "eq",
			Value:     true,
		},
	},
	{
		Key:             config.KeyEnrollmentAllowSubmissionEdit,
		Label:           "Bearbeitung durch Eltern erlauben",
		Description:     "Eltern können ihre Einreichung über den Status-Link bearbeiten, solange noch keine Entscheidung getroffen wurde.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "formular",
		SortOrder:       21,
		DependsOn:       dependsOnEnabled,
	},
}
