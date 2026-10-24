package api

import (
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/realtime"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// The People Directory, School Membership and Workforce glue of the root
// takes built owners. The roots below build those owners from the database
// for the serving root, the CLI roots and the test graphs (#4007). Each
// composes an owner of another module than the one it serves, so no single
// owner composition can host it (ADR 0053 point 1).

// NewStaffMembershipRuntime composes the staff membership closures over the
// service graph and the root's Identity & Access module, with the
// offboarding workflow over db and an unobserved Workforce employment owner
// (#4007).
func NewStaffMembershipRuntime(users peoplePorts.PersonDirectory, education schoolstructure.GroupManagement, staffDay workforceCompose.StaffDayStatus, hub *realtime.Hub, db transactionruntime.Database, identity identityaccess.Owner, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
	offboarding, err := NewStaffOffboardingOwners(db, peopleCompose.MustNewReads(db.Pool(), studentDirectoryView.NewReader()), owners.MembershipStaffEmployment(workforceCompose.MustNewStaffEmployment(db.Pool())), logger)
	if err != nil {
		panic(err)
	}
	return bindStaffMembershipRuntime(users, education, staffDay, hub, offboarding, identity, logger, hooks)
}
