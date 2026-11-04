package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// 1.15.720 (#4801): school_erasure_delete freezes the keys of the tables that
// reach the school only through a parent here, before the parents are deleted,
// and hands them on to school_erasure_assert_empty. The statements used to be
// at the start of school_erasure_delete; they live in their own function so
// that file stays under the 800-line limit (decided by the user in #5109).
// Statements, order, transaction and privileges are unchanged: the function
// runs as the caller (SECURITY INVOKER) inside the erasure transaction, is
// VOLATILE like its caller, and no role may call it directly.
func installParentConferenceErasureChildKeys(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, parentConferenceErasureChildKeysSQL)
	return err
}

const parentConferenceErasureChildKeysSQL = `CREATE OR REPLACE FUNCTION platform.school_erasure_freeze_child_keys(p_school_id BIGINT)
 RETURNS JSONB LANGUAGE plpgsql VOLATILE SECURITY INVOKER SET search_path = pg_catalog
 AS $body$
 DECLARE
  v_child_keys JSONB := '{}'::jsonb;
  v_keys JSONB;
 BEGIN
   -- Freeze indirect keys before their parents disappear.
   SELECT COALESCE(jsonb_agg(r."role_id"), '[]'::jsonb) INTO v_keys FROM auth.role_permissions AS r WHERE r.role_id IN (SELECT p0.id FROM auth.roles AS p0 WHERE p0.tenant_id = p_school_id);
   v_child_keys := v_child_keys || jsonb_build_object('auth.role_permissions', v_keys);
   SELECT COALESCE(jsonb_agg(r."model_id"), '[]'::jsonb) INTO v_keys FROM config.work_time_model_entries AS r WHERE r.model_id IN (SELECT p0.id FROM config.work_time_models AS p0 WHERE p0.tenant_id = p_school_id);
   v_child_keys := v_child_keys || jsonb_build_object('config.work_time_model_entries', v_keys);
   SELECT COALESCE(jsonb_agg(r."student_guardian_id"), '[]'::jsonb) INTO v_keys FROM meta.meal_participation_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id);
   v_child_keys := v_child_keys || jsonb_build_object('meta.meal_participation_permission_grants', v_keys);
   SELECT COALESCE(jsonb_agg(r."student_guardian_id"), '[]'::jsonb) INTO v_keys FROM meta.parent_student_consent_permission_grants AS r WHERE r.student_guardian_id IN (SELECT p0.id FROM users.student_guardian_relationships AS p0 WHERE p0.tenant_id = p_school_id);
   v_child_keys := v_child_keys || jsonb_build_object('meta.parent_student_consent_permission_grants', v_keys);
   RETURN v_child_keys;
 END $body$;
 REVOKE ALL ON FUNCTION platform.school_erasure_freeze_child_keys(BIGINT) FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_demo, phoenix_school_erasure;
`
