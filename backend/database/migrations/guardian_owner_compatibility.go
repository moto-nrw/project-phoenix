package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

// installGuardianOwnerCompatibility runs inside the final-delta lock and
// transaction. It gives the three owner tables the database behaviour
// users.students_guardians had, moves every dependent object onto them, and
// turns the old table into a rollback-only mirror of the joined owners. The
// mirror exists for previous-image rollback only: no current provider reads or
// writes it, and #2757 removes it after the rollback window.
func installGuardianOwnerCompatibility(ctx context.Context, tx bun.Tx) error {
	// Deleting a guardian must not silently unlink its children (#819): the
	// copy window's CASCADE goes back to RESTRICT now that the relationship
	// table is authoritative.
	if err := setGuardianRelationshipDeleteAction(ctx, tx, "RESTRICT"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, guardianOwnerTriggers); err != nil {
		return fmt.Errorf("install guardian owner triggers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianPermissionGrantInvalidation); err != nil {
		return fmt.Errorf("install guardian permission grant invalidation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianAccountBinding); err != nil {
		return fmt.Errorf("install guardian account binding: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianGrantForeignKeys); err != nil {
		return fmt.Errorf("install guardian grant foreign keys: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianCompatibilityCounter); err != nil {
		return fmt.Errorf("install guardian compatibility counter: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianRelationshipMirror); err != nil {
		return fmt.Errorf("install guardian relationship mirror: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianPickupMirror); err != nil {
		return fmt.Errorf("install guardian pickup permission mirror: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianAccessMirror); err != nil {
		return fmt.Errorf("install guardian guardian access mirror: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianCompatibilityRouting); err != nil {
		return fmt.Errorf("install guardian previous-image routing: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianPushSubscriptionView); err != nil {
		return fmt.Errorf("install guardian push subscription view: %w", err)
	}
	if _, err := tx.ExecContext(ctx, guardianCompatibilityComments); err != nil {
		return fmt.Errorf("install guardian comments and grants: %w", err)
	}
	return nil
}

// guardianOwnerTriggers gives each owner table the updated_at maintenance
// users.students_guardians had. They are created after the final delta on
// purpose: the copy carries the source timestamps and a bumping trigger would
// have rewritten them out from under the checksum.
const guardianOwnerTriggers = `
	CREATE TRIGGER update_student_guardian_relationships_updated_at
		BEFORE UPDATE ON users.student_guardian_relationships
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	CREATE TRIGGER update_student_guardian_pickup_permissions_updated_at
		BEFORE UPDATE ON users.student_guardian_pickup_permissions
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	CREATE TRIGGER update_guardian_student_access_updated_at
		BEFORE UPDATE ON auth.guardian_student_access
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();`

// guardianPermissionGrantInvalidation moves the two grant invalidations of
// 1.15.363 and 1.15.365 from the old table onto the Identity row that now
// holds the parents-portal permissions. A recorded consent or meal
// participation grant stops counting the moment the permission it relied on is
// withdrawn, exactly as before; a previous-image write reaches these triggers
// through the routing below. The old functions stay with the mirror until
// #2757 removes both.
const guardianPermissionGrantInvalidation = `
	CREATE OR REPLACE FUNCTION meta.invalidate_parent_student_consent_access_grant()
	RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, meta AS $function$
	BEGIN
		DELETE FROM meta.parent_student_consent_permission_grants
		WHERE student_guardian_id = OLD.relationship_id;
		RETURN NEW;
	END
	$function$;
	CREATE OR REPLACE FUNCTION meta.invalidate_meal_participation_access_grant()
	RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, meta AS $function$
	BEGIN
		DELETE FROM meta.meal_participation_permission_grants
		WHERE student_guardian_id = OLD.relationship_id;
		RETURN NEW;
	END
	$function$;
	CREATE TRIGGER invalidate_parent_student_consent_permission_grant
		AFTER UPDATE OF permissions ON auth.guardian_student_access
		FOR EACH ROW
		WHEN ((OLD.permissions -> 'parent_portal.consent.manage') IS DISTINCT FROM (NEW.permissions -> 'parent_portal.consent.manage'))
		EXECUTE FUNCTION meta.invalidate_parent_student_consent_access_grant();
	CREATE TRIGGER invalidate_meal_participation_permission_grant
		AFTER UPDATE OF permissions ON auth.guardian_student_access
		FOR EACH ROW
		WHEN ((OLD.permissions -> 'parent_portal.meal_participation.manage') IS DISTINCT FROM (NEW.permissions -> 'parent_portal.meal_participation.manage'))
		EXECUTE FUNCTION meta.invalidate_meal_participation_access_grant();
	DROP TRIGGER invalidate_parent_student_consent_permission_grant ON users.students_guardians;
	DROP TRIGGER invalidate_meal_participation_permission_grant ON users.students_guardians;`

// guardianAccountBinding keeps the account an access row is bound to equal to
// the guardian profile's account. The binding is Identity's copy of a fact the
// guardian profile records: linking, moving or dropping a portal account is a
// write on users.guardian_profiles, by either image, and every access row of
// that guardian follows it in the same statement. The creating command binds
// the account the profile names at that moment.
const guardianAccountBinding = `
	CREATE OR REPLACE FUNCTION auth.bind_guardian_student_access_account()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	BEGIN
		UPDATE auth.guardian_student_access AS a SET account_id = NEW.account_id
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = NEW.tenant_id AND r.guardian_profile_id = NEW.id
		  AND a.tenant_id = r.tenant_id AND a.relationship_id = r.id
		  AND a.account_id IS DISTINCT FROM NEW.account_id;
		RETURN NULL;
	END
	$function$;
	CREATE TRIGGER guardian_profiles_bind_student_access
		AFTER UPDATE OF account_id ON users.guardian_profiles
		FOR EACH ROW WHEN (OLD.account_id IS DISTINCT FROM NEW.account_id)
		EXECUTE FUNCTION auth.bind_guardian_student_access_account();`

// guardianGrantForeignKeys moves the two foreign keys that name a link from
// the old table onto the relationship. The copy preserved the link id, so each
// grant keeps the number it already holds. They are added NOT VALID so the
// switch does not scan the grant tables under the write lock;
// ValidateGuardianOwnerForeignKeys confirms the existing rows afterwards. The
// guard fails the migration when a link foreign key exists that this list does
// not move.
const guardianGrantForeignKeys = `
	ALTER TABLE meta.parent_student_consent_permission_grants
		DROP CONSTRAINT parent_student_consent_permission_gran_student_guardian_id_fkey;
	ALTER TABLE meta.parent_student_consent_permission_grants
		ADD CONSTRAINT parent_student_consent_permission_gran_student_guardian_id_fkey
		FOREIGN KEY (student_guardian_id) REFERENCES users.student_guardian_relationships(id) ON DELETE CASCADE NOT VALID;
	ALTER TABLE meta.meal_participation_permission_grants
		DROP CONSTRAINT meal_participation_permission_grants_student_guardian_id_fkey;
	ALTER TABLE meta.meal_participation_permission_grants
		ADD CONSTRAINT meal_participation_permission_grants_student_guardian_id_fkey
		FOREIGN KEY (student_guardian_id) REFERENCES users.student_guardian_relationships(id) ON DELETE CASCADE NOT VALID;
	DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM pg_constraint
			WHERE confrelid = 'users.students_guardians'::regclass AND contype = 'f') THEN
			RAISE EXCEPTION 'a users.students_guardians foreign key was added after this migration was written; repoint it here before Cutover';
		END IF;
	END $$;`

// guardianCompatibilityCounter counts the rows the previous image writes into
// the mirror and the routing trigger copies into the owners. Reads of a base
// table cannot be counted; a previous image is observed through the writes it
// routes and through the deployment itself.
const guardianCompatibilityCounter = `
	CREATE SEQUENCE users.students_guardians_compatibility_writes;
	COMMENT ON SEQUENCE users.students_guardians_compatibility_writes IS
		'Rows the previous image wrote into the rollback-only users.students_guardians mirror and the routing trigger copied into the owner tables (#2756). Must trend to zero before #2757.';
	GRANT USAGE ON SEQUENCE users.students_guardians_compatibility_writes TO phoenix_tenant;
	GRANT USAGE, SELECT ON SEQUENCE users.students_guardians_compatibility_writes TO phoenix_admin;`

// The mirror and the routing triggers call each other's tables, so each marks
// its own writes with a transaction-local setting the other one skips:
// users.guardian_compat_mirror while a mirror trigger writes the old table,
// users.guardian_compat_routing while the routing trigger writes the owners.
// Unlike an equality guard, the marker also covers a relationship inserted
// before its pickup and access rows exist: routing that half-written mirror
// row would create the owner rows the owner commands insert next.

// guardianRelationshipMirror copies every owner write of a relationship onto
// the mirror row with the same id. A new relationship gets a mirror row with
// the old column defaults for the Care Plan and Identity halves; their own
// inserts, which follow in the same unit of work, fill them in.
const guardianRelationshipMirror = `
	CREATE OR REPLACE FUNCTION users.mirror_student_guardian_relationship()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	BEGIN
		IF coalesce(current_setting('users.guardian_compat_routing', true), '') = 'on' THEN
			RETURN NULL;
		END IF;
		PERFORM set_config('users.guardian_compat_mirror', 'on', true);
		IF TG_OP = 'DELETE' THEN
			DELETE FROM users.students_guardians WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
		ELSIF TG_OP = 'INSERT' THEN
			INSERT INTO users.students_guardians
				(id, tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
				 is_primary, is_emergency_contact, emergency_priority, is_payer, created_at, updated_at)
			VALUES (NEW.id, NEW.tenant_id, NEW.student_id, NEW.guardian_profile_id, NEW.relationship_type, NEW.guardian_role,
				NEW.is_primary, NEW.is_emergency_contact, NEW.emergency_priority, NEW.is_payer, NEW.created_at, NEW.updated_at);
		ELSE
			UPDATE users.students_guardians AS sg SET
				student_id = NEW.student_id, guardian_profile_id = NEW.guardian_profile_id,
				relationship_type = NEW.relationship_type, guardian_role = NEW.guardian_role,
				is_primary = NEW.is_primary, is_emergency_contact = NEW.is_emergency_contact,
				emergency_priority = NEW.emergency_priority, is_payer = NEW.is_payer
			WHERE sg.tenant_id = OLD.tenant_id AND sg.id = OLD.id
			  AND ROW(sg.student_id, sg.guardian_profile_id, sg.relationship_type, sg.guardian_role,
			          sg.is_primary, sg.is_emergency_contact, sg.emergency_priority, sg.is_payer)
			      IS DISTINCT FROM ROW(NEW.student_id, NEW.guardian_profile_id, NEW.relationship_type, NEW.guardian_role,
			          NEW.is_primary, NEW.is_emergency_contact, NEW.emergency_priority, NEW.is_payer);
		END IF;
		PERFORM set_config('users.guardian_compat_mirror', 'off', true);
		RETURN NULL;
	END
	$function$;
	CREATE TRIGGER student_guardian_relationships_mirror
		AFTER INSERT OR UPDATE OR DELETE ON users.student_guardian_relationships
		FOR EACH ROW EXECUTE FUNCTION users.mirror_student_guardian_relationship();`

// guardianPickupMirror copies Care Plan's pickup permission onto the mirror.
// A pickup row only disappears with its relationship, whose own mirror
// trigger removes the mirror row.
const guardianPickupMirror = `
	CREATE OR REPLACE FUNCTION users.mirror_student_guardian_pickup_permission()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	BEGIN
		IF TG_OP = 'DELETE' OR coalesce(current_setting('users.guardian_compat_routing', true), '') = 'on' THEN
			RETURN NULL;
		END IF;
		PERFORM set_config('users.guardian_compat_mirror', 'on', true);
		UPDATE users.students_guardians AS sg SET can_pickup = NEW.can_pickup, pickup_notes = NEW.pickup_notes
		WHERE sg.tenant_id = NEW.tenant_id AND sg.id = NEW.relationship_id
		  AND ROW(sg.can_pickup, sg.pickup_notes) IS DISTINCT FROM ROW(NEW.can_pickup, NEW.pickup_notes);
		PERFORM set_config('users.guardian_compat_mirror', 'off', true);
		RETURN NULL;
	END
	$function$;
	CREATE TRIGGER student_guardian_pickup_permissions_mirror
		AFTER INSERT OR UPDATE OR DELETE ON users.student_guardian_pickup_permissions
		FOR EACH ROW EXECUTE FUNCTION users.mirror_student_guardian_pickup_permission();`

// guardianAccessMirror copies Identity's parents-portal permissions onto the
// mirror. The account binding has no column in the old shape.
const guardianAccessMirror = `
	CREATE OR REPLACE FUNCTION auth.mirror_guardian_student_access()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	BEGIN
		IF TG_OP = 'DELETE' OR coalesce(current_setting('users.guardian_compat_routing', true), '') = 'on' THEN
			RETURN NULL;
		END IF;
		PERFORM set_config('users.guardian_compat_mirror', 'on', true);
		UPDATE users.students_guardians AS sg SET permissions = NEW.permissions
		WHERE sg.tenant_id = NEW.tenant_id AND sg.id = NEW.relationship_id
		  AND sg.permissions IS DISTINCT FROM NEW.permissions;
		PERFORM set_config('users.guardian_compat_mirror', 'off', true);
		RETURN NULL;
	END
	$function$;
	CREATE TRIGGER guardian_student_access_mirror
		AFTER INSERT OR UPDATE OR DELETE ON auth.guardian_student_access
		FOR EACH ROW EXECUTE FUNCTION auth.mirror_guardian_student_access();`

// guardianCompatibilityRouting sends a previous-image write of the old table
// to the owners of its columns and counts it. The old single-primary trigger
// stays on the mirror, so a previous image that promotes a guardian first
// demotes the others through nested mirror updates; each of them is routed
// before the promotion reaches the relationship's partial unique index. A
// student deletion cascades into the mirror and the relationships separately;
// that cascade is not a previous-image statement and routes nothing.
// The mirror's id default moves to the relationship sequence, so a
// previous-image insert draws the same ids the owners allocate.
const guardianCompatibilityRouting = `
	CREATE OR REPLACE FUNCTION users.route_students_guardians_compatibility()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	BEGIN
		IF coalesce(current_setting('users.guardian_compat_mirror', true), '') = 'on' THEN
			RETURN NULL;
		END IF;
		-- The mirror's student foreign key cascades after the child row is
		-- gone, beside the relationship's own cascade. That delete is not a
		-- previous-image statement; the relationship follows its child anyway.
		IF TG_OP = 'DELETE' AND NOT EXISTS (
			SELECT 1 FROM users.student_profiles AS s WHERE s.tenant_id = OLD.tenant_id AND s.id = OLD.student_id) THEN
			RETURN NULL;
		END IF;
		IF TG_OP = 'UPDATE' AND (NEW.id, NEW.tenant_id) IS DISTINCT FROM (OLD.id, OLD.tenant_id) THEN
			RAISE EXCEPTION 'student guardian identity is immutable' USING ERRCODE = '23514';
		END IF;
		PERFORM nextval('users.students_guardians_compatibility_writes');
		PERFORM set_config('users.guardian_compat_routing', 'on', true);
		IF TG_OP = 'DELETE' THEN
			-- The pickup permission and the access row follow through their
			-- foreign-key cascade.
			DELETE FROM users.student_guardian_relationships WHERE tenant_id = OLD.tenant_id AND id = OLD.id;
		ELSE
			INSERT INTO users.student_guardian_relationships AS r
				(id, tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
				 is_primary, is_emergency_contact, emergency_priority, is_payer, created_at, updated_at)
			VALUES (NEW.id, NEW.tenant_id, NEW.student_id, NEW.guardian_profile_id, NEW.relationship_type, NEW.guardian_role,
				NEW.is_primary, NEW.is_emergency_contact, NEW.emergency_priority, NEW.is_payer, NEW.created_at, NEW.updated_at)
			ON CONFLICT (id) DO UPDATE SET
				student_id = EXCLUDED.student_id, guardian_profile_id = EXCLUDED.guardian_profile_id,
				relationship_type = EXCLUDED.relationship_type, guardian_role = EXCLUDED.guardian_role,
				is_primary = EXCLUDED.is_primary, is_emergency_contact = EXCLUDED.is_emergency_contact,
				emergency_priority = EXCLUDED.emergency_priority, is_payer = EXCLUDED.is_payer
			WHERE ROW(r.student_id, r.guardian_profile_id, r.relationship_type, r.guardian_role,
			          r.is_primary, r.is_emergency_contact, r.emergency_priority, r.is_payer)
			      IS DISTINCT FROM ROW(EXCLUDED.student_id, EXCLUDED.guardian_profile_id, EXCLUDED.relationship_type,
			          EXCLUDED.guardian_role, EXCLUDED.is_primary, EXCLUDED.is_emergency_contact,
			          EXCLUDED.emergency_priority, EXCLUDED.is_payer);
			INSERT INTO users.student_guardian_pickup_permissions AS p
				(tenant_id, relationship_id, can_pickup, pickup_notes, created_at, updated_at)
			VALUES (NEW.tenant_id, NEW.id, NEW.can_pickup, NEW.pickup_notes, NEW.created_at, NEW.updated_at)
			ON CONFLICT (tenant_id, relationship_id) DO UPDATE SET
				can_pickup = EXCLUDED.can_pickup, pickup_notes = EXCLUDED.pickup_notes
			WHERE ROW(p.can_pickup, p.pickup_notes) IS DISTINCT FROM ROW(EXCLUDED.can_pickup, EXCLUDED.pickup_notes);
			INSERT INTO auth.guardian_student_access AS a
				(tenant_id, relationship_id, account_id, permissions, created_at, updated_at)
			SELECT NEW.tenant_id, NEW.id, g.account_id, NEW.permissions, NEW.created_at, NEW.updated_at
			FROM users.guardian_profiles AS g
			WHERE g.tenant_id = NEW.tenant_id AND g.id = NEW.guardian_profile_id
			ON CONFLICT (tenant_id, relationship_id) DO UPDATE SET
				account_id = EXCLUDED.account_id, permissions = EXCLUDED.permissions
			WHERE ROW(a.account_id, a.permissions) IS DISTINCT FROM ROW(EXCLUDED.account_id, EXCLUDED.permissions);
		END IF;
		PERFORM set_config('users.guardian_compat_routing', 'off', true);
		RETURN NULL;
	END
	$function$;
	CREATE TRIGGER students_guardians_route_compatibility
		AFTER INSERT OR UPDATE OR DELETE ON users.students_guardians
		FOR EACH ROW EXECUTE FUNCTION users.route_students_guardians_compatibility();
	ALTER TABLE users.students_guardians
		ALTER COLUMN id SET DEFAULT nextval('users.student_guardian_relationships_id_seq');`

// guardianPushSubscriptionView is the 1.15.360 view with its one link read
// (the children a guardian account may hear about) moved onto the owners. It
// would otherwise keep reading the mirror.
const guardianPushSubscriptionView = `
	CREATE OR REPLACE VIEW platform.delivery_push_subscriptions
	WITH (security_invoker = true, security_barrier = true) AS
	SELECT
		"push_subscription".*,
		COALESCE("account".active, FALSE) AS account_active,
		COALESCE("account_tenant".status = 'active', FALSE) AS tenant_active,
		EXISTS (
			SELECT 1
			FROM auth.account_roles AS "staff_account_role"
			INNER JOIN auth.roles AS "staff_role" ON "staff_role".id = "staff_account_role".role_id
			WHERE "staff_account_role".account_id = "push_subscription".account_id
				AND "staff_account_role".tenant_id = "push_subscription".tenant_id
				AND LOWER("staff_role".name) <> 'guardian'
		) AS has_staff_role,
		EXISTS (
			SELECT 1
			FROM auth.account_roles AS "school_account_role"
			INNER JOIN auth.roles AS "school_role" ON "school_role".id = "school_account_role".role_id
			WHERE "school_account_role".account_id = "push_subscription".account_id
				AND "school_account_role".tenant_id = "push_subscription".tenant_id
				AND "school_role".is_system
				AND LOWER(BTRIM("school_role".name)) = 'lehrkraft'
		) AS has_school_role,
		EXISTS (
			SELECT 1
			FROM auth.account_roles AS "guardian_account_role"
			INNER JOIN auth.roles AS "guardian_role" ON "guardian_role".id = "guardian_account_role".role_id
			WHERE "guardian_account_role".account_id = "push_subscription".account_id
				AND "guardian_account_role".tenant_id = "push_subscription".tenant_id
				AND LOWER("guardian_role".name) = 'guardian'
		) AS has_guardian_role,
		(
			EXISTS (
				SELECT 1
				FROM auth.account_roles AS "admin_account_role"
				INNER JOIN auth.roles AS "admin_role" ON "admin_role".id = "admin_account_role".role_id
				LEFT JOIN auth.role_permissions AS "role_permission" ON "role_permission".role_id = "admin_account_role".role_id
				LEFT JOIN auth.permissions AS "permission" ON "permission".id = "role_permission".permission_id
				WHERE "admin_account_role".account_id = "push_subscription".account_id
					AND "admin_account_role".tenant_id = "push_subscription".tenant_id
					AND (
						LOWER("admin_role".name) = 'admin'
						OR ("permission".resource = 'admin' AND "permission".action = '*')
						OR ("permission".resource = '*' AND "permission".action = '*')
					)
			) OR EXISTS (
				SELECT 1
				FROM auth.account_permissions AS "account_permission"
				INNER JOIN auth.permissions AS "permission" ON "permission".id = "account_permission".permission_id
				WHERE "account_permission".account_id = "push_subscription".account_id
					AND "account_permission".tenant_id = "push_subscription".tenant_id
					AND "account_permission".granted
					AND (
						("permission".resource = 'admin' AND "permission".action = '*')
						OR ("permission".resource = '*' AND "permission".action = '*')
					)
			)
		) AS effective_admin,
		COALESCE(ARRAY(
			SELECT "child_link".student_id
			FROM users.student_guardian_relationships AS "child_link"
			INNER JOIN auth.guardian_student_access AS "child_access"
				ON "child_access".tenant_id = "child_link".tenant_id
				AND "child_access".relationship_id = "child_link".id
			INNER JOIN users.guardian_profiles AS "child_guardian"
				ON "child_guardian".id = "child_link".guardian_profile_id
			WHERE "child_link".tenant_id = "push_subscription".tenant_id
				AND "child_guardian".account_id = "push_subscription".account_id
				AND "child_access".permissions @> '{"parent_portal.access": true}'::jsonb
		), '{}'::BIGINT[]) AS guardian_student_ids,
		CASE
			WHEN "push_subscription".token_family_id <> '' THEN EXISTS (
				SELECT 1 FROM auth.tokens AS "token"
				WHERE "token".account_id = "push_subscription".account_id
					AND "token".family_id = "push_subscription".token_family_id
					AND "token".rotated_at IS NULL AND "token".expiry > NOW()
			)
			ELSE EXISTS (
				SELECT 1 FROM auth.tokens AS "token"
				WHERE "token".account_id = "push_subscription".account_id
					AND "token".rotated_at IS NULL AND "token".expiry > NOW()
					AND (
						("push_subscription".portal = 'parent' AND "token".portal_scope IN ('parent', 'unknown', ''))
						OR ("push_subscription".portal = 'staff' AND "token".tenant_id = "push_subscription".tenant_id AND "token".portal_scope IN ('tenant', 'org', 'unknown', ''))
						OR ("push_subscription".portal = 'school' AND "token".tenant_id = "push_subscription".tenant_id AND "token".portal_scope IN ('school', 'unknown', ''))
					)
			)
		END AS has_live_token
	FROM iot.push_subscriptions AS "push_subscription"
	LEFT JOIN auth.accounts AS "account" ON "account".id = "push_subscription".account_id
	LEFT JOIN auth.account_tenants AS "account_tenant"
		ON "account_tenant".account_id = "push_subscription".account_id
		AND "account_tenant".tenant_id = "push_subscription".tenant_id;`

// guardianCompatibilityComments name the new authority for the operator who
// inspects the tables during the rollback window, and give the guardian
// invitation worker (phoenix_auth) the read of the child links it had on the
// old table.
const guardianCompatibilityComments = `
	COMMENT ON TABLE users.students_guardians IS
		'Rollback-only mirror of users.student_guardian_relationships, users.student_guardian_pickup_permissions and auth.guardian_student_access (#2756). Kept by triggers; nothing current reads or writes it. #2757 removes it.';
	COMMENT ON TABLE users.student_guardian_relationships IS
		'People-Directory-owned student-guardian relationship (#2756): type, role, primary, emergency contact and priority, payer. Authoritative since Cutover.';
	COMMENT ON TABLE users.student_guardian_pickup_permissions IS
		'Care-Plan-owned pickup permission of one relationship (#2756). Authoritative since Cutover.';
	COMMENT ON TABLE auth.guardian_student_access IS
		'Identity-owned parents-portal access of one relationship (#2756): account binding and permissions. Authoritative since Cutover.';
	GRANT SELECT ON users.student_guardian_relationships, users.student_guardian_pickup_permissions,
		auth.guardian_student_access TO phoenix_auth;`

// ValidateGuardianOwnerForeignKeys confirms the repointed grant constraints
// against the rows that already exist. It runs after the switch transaction:
// VALIDATE takes a share-update-exclusive lock on the grant table and a
// row-share lock on the relationships, so ordinary reads and writes continue
// while it scans. Validating an already valid constraint is a no-op, so an
// interrupted run is simply repeated by the next migrate.
func ValidateGuardianOwnerForeignKeys(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("guardian owner cutover: database is required")
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE meta.parent_student_consent_permission_grants
			VALIDATE CONSTRAINT parent_student_consent_permission_gran_student_guardian_id_fkey;
		ALTER TABLE meta.meal_participation_permission_grants
			VALIDATE CONSTRAINT meal_participation_permission_grants_student_guardian_id_fkey;
	`); err != nil {
		return fmt.Errorf("guardian owner cutover: validate repointed foreign keys: %w", err)
	}
	return nil
}
