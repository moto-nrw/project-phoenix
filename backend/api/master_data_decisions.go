package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

// The Stammdaten decision and the cross-kind parent-request commands are
// Care Plan's (#3354). The adapters below bind their consumer-owned ports to
// the People Directory rows and change history, the realtime hub, the parent
// chat and the identity-owned review scope; they decide nothing.

var _ carePlanCompose.MasterDataRecords = owners.MasterDataRecords{}
