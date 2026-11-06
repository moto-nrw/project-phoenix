package users

// The INSERT consumes the UPDATE's result, so the guardian row is locked and
// its retention mark is cleared before a relationship can be inserted. Keep
// the unconditional UPDATE even for an existing pair: re-linking still clears
// retention and runs the guardian's update trigger.
//
// Read account_id from RETURNING, not guardian_profiles: after waiting for a
// concurrent guardian update, the statement snapshot can still hold the old
// account while UPDATE RETURNING supplies the version we actually locked.
const nonPrimaryRelationshipPrefix = `WITH linked_guardian AS (
	UPDATE users.guardian_profiles SET childless_since = NULL
	WHERE tenant_id = ?0 AND id = ?2
	RETURNING account_id
), inserted AS (
	INSERT INTO users.student_guardian_relationships
		(tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
		 is_primary, is_emergency_contact, emergency_priority, is_payer)
	SELECT ?0, ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8 FROM linked_guardian WHERE TRUE
`

// No guardian yields no rows (not found); an existing pair yields one row
// with id zero (no-op). In both cases only a new link receives owner halves.
const nonPrimaryRelationshipSuffix = `
	RETURNING id, created_at, updated_at
)
SELECT COALESCE(inserted.id, 0) AS id, inserted.created_at, inserted.updated_at, linked_guardian.account_id
FROM linked_guardian LEFT JOIN inserted ON TRUE`

const insertNonPrimaryRelationshipSQL = nonPrimaryRelationshipPrefix + nonPrimaryRelationshipSuffix

const insertNonPrimaryRelationshipIfAbsentSQL = nonPrimaryRelationshipPrefix +
	`ON CONFLICT (tenant_id, student_id, guardian_profile_id) DO NOTHING` + nonPrimaryRelationshipSuffix

func relationshipInsertStatement(primary, ifAbsent bool) string {
	switch {
	case !primary && ifAbsent:
		return insertNonPrimaryRelationshipIfAbsentSQL
	case !primary:
		return insertNonPrimaryRelationshipSQL
	case ifAbsent:
		return insertRelationshipIfAbsentSQL
	default:
		return insertRelationshipSQL
	}
}
