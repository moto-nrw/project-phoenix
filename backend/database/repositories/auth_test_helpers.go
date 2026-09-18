package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/uptrace/bun"
)

// NewAuthTestRepositories supplies the compatibility carrier required by the
// auth constructors. Only auth, invitation and identity dependencies are built;
// API tests receive the resulting auth capabilities, never this carrier.
func NewAuthTestRepositories(db *bun.DB, command auditModels.Command) (*Factory, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return nil, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	r := &Factory{
		db: db, Account: members.Account, AccountTenant: members.AccountTenant, Person: members.Person,
		Staff: members.Staff, Teacher: members.Teacher, GroupTeacher: members.GroupTeacher, ClassTeacher: members.ClassTeacher,
		AccountParent: authRepo.NewAccountParentRepository(db),
		AccountRole:   authRepo.NewAccountRoleRepository(db), AccountPermission: authRepo.NewAccountPermissionRepository(db),
		Role: authRepo.NewRoleRepository(db), RolePermission: authRepo.NewRolePermissionRepository(db),
		Permission: authRepo.NewPermissionRepository(db),
		RFIDCard:   authRepo.NewRFIDCardRepository(db), Student: NewStudentRepository(db),
		InvitationToken: authRepo.NewInvitationTokenRepository(db),
		GuardianProfile: NewGuardianProfileRepository(db), StudentGuardian: NewStudentGuardianRepository(db),
		ParentEnrollmentRequest: parentRepo.NewEnrollmentRequestRepository(carePlanLegacy.NewParentRuntime(db), enrollmentCompose.New(), identityAccountDirectory{accounts: newIdentityAccess(db, nil)}),
		MFACredential:           authRepo.NewMFACredentialRepository(db), MFAEmailChallenge: authRepo.NewMFAEmailChallengeRepository(db),
		MFATrustedDevice: authRepo.NewMFATrustedDeviceRepository(db), MFAOverride: authRepo.NewMFAOverrideRepository(db),
		PushSubscription: deliveryCompose.NewPushSubscriptionRepository(db),
		AuthEvent:        authEventCommand{auditRepo.NewAuthEventRepository(newTestAuditRuntime(db)), command},
		// The operator second factor appends to the operator ledger
		// (#3331), so the composed test module needs it bound.
		OperatorAuditLog: newOperatorAuditLog(newTestAuditRuntime(db)),
	}
	r.BindOrganizationTenancy(organizations)
	return r, nil
}
