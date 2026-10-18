package defaults

import config "github.com/moto-nrw/project-phoenix/modules/settings/records"

var pickupReviewsDefinitions = []config.Definition{
	{
		Key:             config.KeyRequirePickupOfferingReview,
		Label:           "Abgleich fester Abholzeiten mit dem Angebot",
		Description:     "Passt eine neue feste Abholzeit nicht zum gebuchten Betreuungsangebot, wählt das Team ein anderes Angebot oder eine Ausnahme.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "betreuungszeiten",
		SortOrder:       1,
		AccessPolicy:    config.AccessShared,
	},
}
