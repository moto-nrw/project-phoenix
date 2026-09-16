package auth

import (
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// GuardianRoleClass classifies a stored students_guardians role for the
// Identity & Access relative access flow (#3225): a full guardian keeps
// portal access, a social worker is a school-managed contact the invite
// flow never upgrades, everything else is a restrictive contact.
type GuardianRoleClass int

const (
	GuardianRoleRestricted GuardianRoleClass = iota
	GuardianRoleFull
	GuardianRoleSocialWorker
)

// ClassifyGuardianRole applies the shared guardian role policy.
func ClassifyGuardianRole(role string) GuardianRoleClass {
	if authorize.IsFullGuardianRole(role) {
		return GuardianRoleFull
	}
	if authorize.NormalizeGuardianRole(role) == authorize.GuardianRoleSocialWorker {
		return GuardianRoleSocialWorker
	}
	return GuardianRoleRestricted
}

// PromoteStudentGuardianLink upgrades the relationship to legal guardian
// with the server-derived parent-portal permission set (#2172).
func PromoteStudentGuardianLink(link *userModels.StudentGuardian) {
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRoleLegalGuardian)
}
