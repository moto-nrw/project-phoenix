package defaults

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// School lesson end times (#3372). The team maintains them per school and
// picks one as an arrival time ("nach der 5. Stunde"); the form copies the
// clock time, nothing references the lesson afterwards.
func init() {
	for period := 1; period <= config.SchoolPeriodCount; period++ {
		config.Register(config.Definition{
			Key:             config.SchoolPeriodEndKey(period),
			Label:           fmt.Sprintf("Ende der %d. Stunde", period),
			Description:     fmt.Sprintf("Das Team kann diese Uhrzeit bei der Ankunft als „%d. Stunde“ wählen. Leer bedeutet: nicht wählbar.", period),
			Type:            config.FieldTime,
			Default:         "",
			ReadPermission:  "config:read",
			WritePermission: "config:update",
			Tab:             "operations",
			Category:        "schulstunden",
			SortOrder:       period,
			AccessPolicy:    config.AccessShared,
		})
	}
}
