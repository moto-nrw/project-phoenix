package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const pickupChangeTeamNoticesVersion = "1.15.957"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pickupChangeTeamNoticesVersion,
		Description: "Geänderte Abholzeit für einen Tag: Hinweis an das Team des Kindes mit Lesestand je Person (#6855)",
		DependsOn: []string{
			studentNoteTeamNoticesVersion,     // the school erasure functions this one extends (1.15.956)
			careScheduleChangeRequestsVersion, // the requests a notice points at
			createAccountTenantsVersion,       // the school membership of a recipient
		},
	})
	Migrations.MustRegister(pickupChangeTeamNoticesUp, pickupChangeTeamNoticesDown)
}

// pickupChangeTeamNoticesUpSQL stores who was told about a changed one-day
// pickup time of a child (#6855).
//
//   - One row per approved request of a family and staff member responsible
//     for the child. The row is the recipient's unread marker until they open
//     the child (read_at). A corrected decision sets it unread again.
//   - The row holds IDs, the day and two instants. The time and the state
//     stay on the request, so a correction reaches the marker through the
//     request; pickup_date lets the reads skip past days in the index.
//   - It goes with its request (ON DELETE CASCADE) and with the recipient's
//     membership of the school (auth.account_tenants): a person who leaves
//     the school leaves their markers behind, and the demo erasure, which
//     deletes the visitor's memberships, takes them along.
//   - The school erasure learns the table (school_erasure_* of 1.15.956 with
//     this table added, built below).
const pickupChangeTeamNoticesUpSQL = `
CREATE TABLE schedule.pickup_change_team_notices (
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	request_id BIGINT NOT NULL REFERENCES schedule.care_schedule_change_requests(id) ON DELETE CASCADE,
	recipient_account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
	pickup_date DATE NOT NULL,
	informed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	read_at TIMESTAMPTZ,
	PRIMARY KEY (request_id, recipient_account_id),
	CONSTRAINT fk_pickup_change_team_notices_recipient FOREIGN KEY (recipient_account_id, tenant_id)
		REFERENCES auth.account_tenants (account_id, tenant_id) ON DELETE CASCADE
);
-- The header of every staff page reads the reader's unread markers.
CREATE INDEX idx_pickup_change_team_notices_unread
	ON schedule.pickup_change_team_notices (tenant_id, recipient_account_id, pickup_date)
	WHERE read_at IS NULL;
COMMENT ON TABLE schedule.pickup_change_team_notices IS
	'Wem eine bestätigte Abholzeit-Änderung der Eltern für einen Tag gezeigt wurde, und ob die Person sie gelesen hat (#6855).';
GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.pickup_change_team_notices TO phoenix_tenant;
`

const pickupChangeTeamNoticesDownSQL = `DROP TABLE IF EXISTS schedule.pickup_change_team_notices;`

// pickupChangeTeamNoticesFingerprint is the incoming-FK fingerprint of the
// schema with this table: its foreign keys into planned tables
// (schedule.care_schedule_change_requests, auth.account_tenants) change it.
const pickupChangeTeamNoticesFingerprint = "ecc1fe36a69bf1565114644cd9d27d63869fd218ed76932a7f3c29509091342a"

const pickupChangeTeamNoticesTable = "schedule.pickup_change_team_notices"

// pickupChangeTeamNoticeErasureSQL returns the five school erasure functions
// of 1.15.956 with the markers added: planned, locked, scope-checked against
// their request, deleted first and checked empty afterwards.
func pickupChangeTeamNoticeErasureSQL() ([]string, error) {
	base, err := studentNoteTeamNoticeErasureSQL()
	if err != nil {
		return nil, err
	}
	planBase, preflightBase, residualBase, deleteBase, previewBase := base[0], base[1], base[2], base[3], base[4]
	plan, err := applyErasureEdits(planBase, erasureEdit{
		anchor: `  {"name": "schedule.care_schedule_change_requests","predicate": "r.tenant_id = $1"},` + "\n",
		before: `  {"name": "` + pickupChangeTeamNoticesTable + `","predicate": "r.tenant_id = $1"},` + "\n",
	})
	if err != nil {
		return nil, err
	}
	preflight, err := applyErasureEdits(preflightBase,
		erasureEdit{
			anchor: "    schedule.care_schedule_change_requests,\n",
			before: "    " + pickupChangeTeamNoticesTable + ",\n",
		},
		erasureEdit{
			anchor: `    SELECT 1 FROM schedule.care_schedule_change_requests AS c JOIN users.student_profiles AS p`,
			before: `    SELECT 1 FROM ` + pickupChangeTeamNoticesTable + ` AS c JOIN schedule.care_schedule_change_requests AS p ON c."request_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)` + "\n    UNION ALL\n",
		},
		erasureEdit{anchor: "'" + studentNoteTeamNoticesFingerprint + "'"},
	)
	if err != nil {
		return nil, err
	}
	preflight = strings.Replace(preflight, "'"+studentNoteTeamNoticesFingerprint+"'", "'"+pickupChangeTeamNoticesFingerprint+"'", 1)
	residual, err := applyErasureEdits(residualBase, erasureEdit{
		anchor: "    SELECT 1 FROM schedule.care_schedule_change_requests AS r WHERE r.tenant_id = p_school_id\n",
		before: "    SELECT 1 FROM " + pickupChangeTeamNoticesTable + " AS r WHERE r.tenant_id = p_school_id\n    UNION ALL\n",
	})
	if err != nil {
		return nil, err
	}
	deletion, err := applyErasureEdits(deleteBase, erasureEdit{
		anchor: "   v_child_keys := platform.school_erasure_freeze_child_keys(p_school_id);\n",
		after: "   SELECT count(*) INTO v_count FROM " + pickupChangeTeamNoticesTable + " AS r WHERE r.tenant_id = p_school_id;\n" +
			"   DELETE FROM " + pickupChangeTeamNoticesTable + " AS r WHERE r.tenant_id = p_school_id;\n" +
			"   v_counts := v_counts || jsonb_build_object('" + pickupChangeTeamNoticesTable + "', v_count);\n",
	})
	if err != nil {
		return nil, err
	}
	preview, err := applyErasureEdits(previewBase, erasureEdit{
		anchor: " v_counts := v_counts || jsonb_build_object('schedule.care_schedule_change_requests',",
		before: " v_counts := v_counts || jsonb_build_object('" + pickupChangeTeamNoticesTable + "',(SELECT count(*) FROM " + pickupChangeTeamNoticesTable + " AS r WHERE r.tenant_id = p_school));\n",
	})
	if err != nil {
		return nil, err
	}
	return []string{plan, preflight, residual, deletion, preview}, nil
}

func pickupChangeTeamNoticesUp(ctx context.Context, db *bun.DB) error {
	functions, err := pickupChangeTeamNoticeErasureSQL()
	if err != nil {
		return fmt.Errorf("build the school erasure with the pickup change notices: %w", err)
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, pickupChangeTeamNoticesUpSQL); err != nil {
			return fmt.Errorf("create pickup change team notices: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, pickupChangeTeamNoticesTable); err != nil {
			return err
		}
		if err := installDerivedSchoolErasureFunctions(ctx, tx, functions); err != nil {
			return fmt.Errorf("add the pickup change notices to the school erasure: %w", err)
		}
		return nil
	})
}

func pickupChangeTeamNoticesDown(ctx context.Context, db *bun.DB) error {
	functions, err := studentNoteTeamNoticeErasureSQL()
	if err != nil {
		return fmt.Errorf("rebuild the school erasure of 1.15.956: %w", err)
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := installDerivedSchoolErasureFunctions(ctx, tx, functions); err != nil {
			return fmt.Errorf("remove the pickup change notices from the school erasure: %w", err)
		}
		if _, err := tx.ExecContext(ctx, pickupChangeTeamNoticesDownSQL); err != nil {
			return fmt.Errorf("drop pickup change team notices: %w", err)
		}
		return nil
	})
}
