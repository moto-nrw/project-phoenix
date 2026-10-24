package peopledirectory

// GuardianRoleColumns are the relationship columns a guardian role change
// rewrites: the relationship type, the emergency contact and its priority,
// the pickup right, the role and its portal permissions. Every other column
// of the relationship keeps its stored value.
func GuardianRoleColumns() []string {
	return []string{"relationship_type", "is_emergency_contact", "can_pickup", "emergency_priority", "guardian_role", "permissions"}
}
