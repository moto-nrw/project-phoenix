package users

import (
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The stored role of a student-guardian relationship, classified for
// the Identity & Access relative access flow (#3225): a full guardian keeps
// portal access, a social worker is a school-managed contact the invite flow
// never upgrades, everything else is a restrictive contact. The composition
// root binds these to the Identity & Access guardian directory seam.

// IsFullGuardianRole reports whether the stored relationship role is one of
// the full guardian presets.
func IsFullGuardianRole(role string) bool {
	return authorize.IsFullGuardianRole(role)
}

// IsSocialWorkerGuardianRole reports whether the stored relationship role is
// the school-managed social worker preset.
func IsSocialWorkerGuardianRole(role string) bool {
	return authorize.NormalizeGuardianRole(role) == authorize.GuardianRoleSocialWorker
}

// PromoteStudentGuardianLink upgrades the relationship to legal guardian
// with the server-derived parent-portal permission set (#2172).
func PromoteStudentGuardianLink(link *userModels.StudentGuardian) {
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRoleLegalGuardian)
}
