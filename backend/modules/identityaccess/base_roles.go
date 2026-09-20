package identityaccess

// ValidBaseRoles returns the privilege tiers accepted by the role API.
// Keep these values aligned with the database constraint and frontend labels.
func ValidBaseRoles() []string {
	return []string{BaseRoleAdmin, BaseRoleUser, BaseRoleGuardian}
}
