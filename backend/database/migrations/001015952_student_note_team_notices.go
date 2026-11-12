package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const studentNoteTeamNoticesVersion = "1.15.952"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentNoteTeamNoticesVersion,
		Description: "Kindernotizen: Speichern und das Team informieren, mit Lesestand je Person (#6854)",
		DependsOn: []string{
			demoVisitorErasureFamilyDebtorNumbersVersion, // the newest migration on development (1.15.951)
			studentNotesVersion,                          // the notes a notice points at
			createAccountTenantsVersion,                  // the school membership of a recipient
			holidayClosureEventsVersion,                  // the school erasure functions this one extends
		},
	})
	Migrations.MustRegister(studentNoteTeamNoticesUp, studentNoteTeamNoticesDown)
}

// studentNoteTeamNoticesUpSQL stores who was told about a child's note (#6854).
//
//   - One row per note and recipient. The author chose "Speichern und Team
//     informieren"; the row is the recipient's unread marker until they open
//     the child's notes (read_at). Telling the team again about a corrected
//     note sets the row back to unread.
//   - The row holds IDs and two instants only. The wording stays on the note,
//     so a correction, a narrower audience or a deletion reaches the marker
//     through the note itself: the reads join the live note.
//   - It goes with its note (ON DELETE CASCADE) and with the recipient's
//     membership of the school (auth.account_tenants): a person who leaves
//     the school leaves their markers behind, and the demo erasure, which
//     deletes the visitor's memberships, takes them along.
//   - The school erasure learns the table (school_erasure_* of 1.15.942 with
//     this table added, built below).
const studentNoteTeamNoticesUpSQL = `
CREATE TABLE users.student_note_team_notices (
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	note_id BIGINT NOT NULL REFERENCES users.student_notes(id) ON DELETE CASCADE,
	recipient_account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
	informed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	read_at TIMESTAMPTZ,
	PRIMARY KEY (note_id, recipient_account_id),
	CONSTRAINT fk_student_note_team_notices_recipient FOREIGN KEY (recipient_account_id, tenant_id)
		REFERENCES auth.account_tenants (account_id, tenant_id) ON DELETE CASCADE
);
-- The header of every staff page counts the reader's unread markers.
CREATE INDEX idx_student_note_team_notices_unread
	ON users.student_note_team_notices (tenant_id, recipient_account_id)
	WHERE read_at IS NULL;
COMMENT ON TABLE users.student_note_team_notices IS
	'Wem eine Kindernotiz mit „Speichern und Team informieren“ gezeigt wurde, und ob die Person sie gelesen hat (#6854).';
GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_note_team_notices TO phoenix_tenant;
`

const studentNoteTeamNoticesDownSQL = `DROP TABLE IF EXISTS users.student_note_team_notices;`

// studentNoteTeamNoticesFingerprint is the incoming-FK fingerprint of the
// schema with this table: its foreign keys into planned tables
// (users.student_notes, auth.account_tenants) change it; the one to
// auth.accounts is outside the plan.
const studentNoteTeamNoticesFingerprint = "1da8b31d9ec59db2f12f6c9fbde8d92cd68de2ade8bea5eb739706774e664749"

// holidayClosureErasureFingerprint is the fingerprint the preflight of
// 1.15.942 checks.
const holidayClosureErasureFingerprint = "f002c755b764f223644a2655347a8b03aeccc50acedc65bdd73c6facef1e6c9a"

// erasureEdit inserts text at one anchor of an erasure function. The anchor
// must occur exactly once, so a changed base function fails the migration
// (and TestStudentNoteTeamNoticeErasureExtendsTheErasureOf942) instead of
// silently skipping the table.
type erasureEdit struct {
	anchor string
	before string // inserted in front of the anchor
	after  string // inserted behind the anchor
}

func applyErasureEdits(base string, edits ...erasureEdit) (string, error) {
	out := base
	for _, edit := range edits {
		if count := strings.Count(out, edit.anchor); count != 1 {
			return "", fmt.Errorf("erasure anchor %q occurs %d times, want 1", edit.anchor, count)
		}
		out = strings.Replace(out, edit.anchor, edit.before+edit.anchor+edit.after, 1)
	}
	return out, nil
}

const studentNoteTeamNoticesTable = "users.student_note_team_notices"

// studentNoteTeamNoticeErasureSQL returns the five school erasure functions
// of 1.15.942 with the notices added: planned, locked, scope-checked against
// their note, deleted first and checked empty afterwards.
func studentNoteTeamNoticeErasureSQL() ([]string, error) {
	plan, err := applyErasureEdits(holidayClosureErasurePlanSQL, erasureEdit{
		anchor: `  {"name": "users.student_notes","predicate": "r.tenant_id = $1"},` + "\n",
		before: `  {"name": "` + studentNoteTeamNoticesTable + `","predicate": "r.tenant_id = $1"},` + "\n",
	})
	if err != nil {
		return nil, err
	}
	preflight, err := applyErasureEdits(holidayClosureErasurePreflightSQL,
		erasureEdit{
			anchor: "    users.student_notes,\n",
			before: "    " + studentNoteTeamNoticesTable + ",\n",
		},
		erasureEdit{
			anchor: `    SELECT 1 FROM users.student_notes AS c JOIN activities.groups AS p`,
			before: `    SELECT 1 FROM ` + studentNoteTeamNoticesTable + ` AS c JOIN users.student_notes AS p ON c."note_id" = p."id" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)` + "\n    UNION ALL\n",
		},
		erasureEdit{
			anchor: "'" + holidayClosureErasureFingerprint + "'",
			before: "",
		},
	)
	if err != nil {
		return nil, err
	}
	preflight = strings.Replace(preflight, "'"+holidayClosureErasureFingerprint+"'", "'"+studentNoteTeamNoticesFingerprint+"'", 1)
	residual, err := applyErasureEdits(holidayClosureErasureResidualSQL, erasureEdit{
		anchor: "    SELECT 1 FROM users.student_notes AS r WHERE r.tenant_id = p_school_id\n",
		before: "    SELECT 1 FROM " + studentNoteTeamNoticesTable + " AS r WHERE r.tenant_id = p_school_id\n    UNION ALL\n",
	})
	if err != nil {
		return nil, err
	}
	deletion, err := applyErasureEdits(holidayClosureErasureDeleteSQL, erasureEdit{
		anchor: "   v_child_keys := platform.school_erasure_freeze_child_keys(p_school_id);\n",
		after: "   SELECT count(*) INTO v_count FROM " + studentNoteTeamNoticesTable + " AS r WHERE r.tenant_id = p_school_id;\n" +
			"   DELETE FROM " + studentNoteTeamNoticesTable + " AS r WHERE r.tenant_id = p_school_id;\n" +
			"   v_counts := v_counts || jsonb_build_object('" + studentNoteTeamNoticesTable + "', v_count);\n",
	})
	if err != nil {
		return nil, err
	}
	preview, err := applyErasureEdits(holidayClosureErasurePreviewSQL, erasureEdit{
		anchor: " v_counts := v_counts || jsonb_build_object('users.student_notes',",
		before: " v_counts := v_counts || jsonb_build_object('" + studentNoteTeamNoticesTable + "',(SELECT count(*) FROM " + studentNoteTeamNoticesTable + " AS r WHERE r.tenant_id = p_school));\n",
	})
	if err != nil {
		return nil, err
	}
	return []string{plan, preflight, residual, deletion, preview}, nil
}

func studentNoteTeamNoticesUp(ctx context.Context, db *bun.DB) error {
	functions, err := studentNoteTeamNoticeErasureSQL()
	if err != nil {
		return fmt.Errorf("build the school erasure with the note notices: %w", err)
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, studentNoteTeamNoticesUpSQL); err != nil {
			return fmt.Errorf("create student note team notices: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, studentNoteTeamNoticesTable); err != nil {
			return err
		}
		if err := installDerivedSchoolErasureFunctions(ctx, tx, functions); err != nil {
			return fmt.Errorf("add the note notices to the school erasure: %w", err)
		}
		return nil
	})
}

func studentNoteTeamNoticesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installHolidayClosureErasurePlan, installHolidayClosureErasurePreflight,
			installHolidayClosureErasureResidual, installHolidayClosureErasureDelete,
			installHolidayClosureErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("remove the note notices from the school erasure: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, studentNoteTeamNoticesDownSQL); err != nil {
			return fmt.Errorf("drop student note team notices: %w", err)
		}
		return nil
	})
}
