package importpkg

import (
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// LegacyReads are the retained repositories the Data Import still reads
// after the owner cutover (#2708). None of them is written: the RFID column
// resolves a card, the group and room columns their records, and the staff
// row type the tenant's roles, the importer's account and the school name
// for the invitation e-mail. Naming them here keeps the composition root out
// of the model packages; each import stays tracked in
// architecture/legacy.jsonl under its own migration issue.
type LegacyReads struct {
	RFIDCard        authModels.RFIDCardRepository
	InvitationToken authModels.InvitationTokenRepository
	Account         authModels.AccountRepository
	AccountTenant   authModels.AccountTenantRepository
	Role            authModels.RoleRepository
	Permission      authModels.PermissionRepository
	School          platformModels.SchoolRepository
	Groups          education.GroupRepository
	Rooms           facilities.RoomRepository
}
