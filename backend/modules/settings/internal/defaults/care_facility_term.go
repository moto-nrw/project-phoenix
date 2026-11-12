package defaults

import (
	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

// The term for the school's care in fixed parent texts (#6042). Schools in
// Lower Saxony call it "GTS" (Ganztagsschule), others keep "OGS". The texts
// use the term with a feminine article ("die OGS prüft"), which the
// description tells the school.
func init() {
	registerDefinitions(careFacilityTermDefinitions)
}

// careFacilityTermPattern admits a plain name: letters, digits, spaces and
// the punctuation of school names. Markup, placeholders, line breaks and
// leading or trailing spaces never pass.
var careFacilityTermPattern = `^[\p{L}\p{N}(](?:[\p{L}\p{N} ./()-]*[\p{L}\p{N}.)])?$`

var careFacilityTermMaxLength = 40

var careFacilityTermDefinitions = []config.Definition{
	{
		Key:         config.KeyEnrollmentCareFacilityTerm,
		Label:       "Name der Betreuung in Texten für Eltern",
		Description: "So heißt Ihre Betreuung in den festen Texten der Online-Anmeldung, der Eltern-App und in den E-Mails dazu. Zum Beispiel: „Die OGS prüft Ihre Angaben.“ Passt keine Auswahl, wählen Sie „Eigener Name“. Der Name muss zu „die“ passen. Ihre eigenen Texte im Formular ändern sich nicht.",
		Type:        config.FieldText,
		Default:     "OGS",
		Options: &config.SelectOptions{Static: []config.SelectOption{
			{Label: "OGS", Value: "OGS"},
			{Label: "GTS", Value: "GTS"},
		}},
		Validation: &config.ValidationRules{
			Pattern:   &careFacilityTermPattern,
			MaxLength: &careFacilityTermMaxLength,
		},
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "enrollment",
		Category:        "allgemein",
		SortOrder:       2,
	},
}
