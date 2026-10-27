package contract

import "context"

// StudentClassTeachers is one reported child's class as entered and the
// accounts of the Lehrkräfte assigned to it (education.class_teachers), for
// the moto schule notice of an absence (#4792).
type StudentClassTeachers struct {
	SchoolClass string
	AccountIDs  []int64
}

// SchoolClassReach names the class of each reported child and its assigned
// Lehrkräfte. The root binds it over People Directory, School Membership and
// the staff accounts. A child without a class is missing from the result;
// SchoolClass is trimmed and never empty.
type SchoolClassReach interface {
	StudentClassTeachers(ctx context.Context, studentIDs []int64) (map[int64]StudentClassTeachers, error)
}

// SchoolClassRecipients are the collaborators of the moto schule notice of
// an absence. A zero value (no Reach) keeps the absence notice OGS-only.
type SchoolClassRecipients struct {
	Reach SchoolClassReach
	// Consent is the single consent enforcement point: tenant gate and the
	// person's decision.
	Consent interface {
		FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
	}
	// Subscribers lists the accounts that switched the type on. It is the
	// cheap first question (nobody, no lookup at all) and the source of the
	// Schulverwaltung, narrowed by Accounts and Permissions.
	Subscribers interface {
		ListOptedIn(ctx context.Context, notificationType string) ([]int64, error)
	}
	Accounts interface {
		ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error)
		FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error)
	}
	// Permissions matches granted permission names under the Security
	// Runtime's wildcard rules.
	Permissions func(required string, granted []string) bool
}
