package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const (
	mealOrderErasureVersion     = "1.15.496"
	mealOrderErasureDescription = "Mandantenlöschung: Bestellungen, Anmeldeänderungen, Caterer-Einladungen und Belieferungen einer Schule (#3873, #3837)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealOrderErasureVersion,
		Description: mealOrderErasureDescription,
		DependsOn:   []string{mealCaterersVersion, "1.15.490"},
	})
	Migrations.MustRegister(mealOrderErasureUp, mealOrderErasureDown)
}

// mealOrderErasureTables are the school-scoped tables of the meal ordering
// (#3873) in deletion order: an order's changes and lines before its days,
// its days before the order. The caterer business and its accounts are
// shared across schools and stay (ADR 0077, ADR 0092).
var mealOrderErasureTables = []string{
	"schedule.meal_order_changes",
	"schedule.meal_order_lines",
	"schedule.meal_order_days",
	"schedule.meal_orders",
	"schedule.meal_registration_changes",
	"auth.caterer_invitations",
	"auth.caterer_schools",
}

// mealOrderErasureFingerprint is the incoming-FK fingerprint of the plan with
// these tables: they add their edges to platform.schools, to
// users.student_profiles and among themselves.
const mealOrderErasureFingerprint = "58970c245c962589fb86d888a586a3fb5b48aba33f86d341b50ba4ec55b8d253"

// The installed erasure plan is a frozen projection of the data lifecycle
// catalog (ADR 0092): a new school-scoped table needs a migration updating
// it. This one installs the 1.15.481 and 1.15.490 functions again with the
// meal ordering tables added, each tenant-scoped by tenant_id; Down installs
// them unchanged.
func mealOrderErasureUp(ctx context.Context, db *bun.DB) error {
	statements, err := mealOrderErasureStatements()
	if err != nil {
		return err
	}
	return execMealOrderErasure(ctx, db, statements)
}

func mealOrderErasureDown(ctx context.Context, db *bun.DB) error {
	preview, err := schoolErasurePreviewSQL()
	if err != nil {
		return err
	}
	return execMealOrderErasure(ctx, db, []string{schoolErasurePlanSQL, schoolErasurePreflightSQL, schoolErasureResidualSQL, schoolErasureDeleteSQL, preview})
}

func execMealOrderErasure(ctx context.Context, db *bun.DB, statements []string) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("install the school erasure with the meal orders: %w", err)
			}
		}
		return nil
	})
}

// mealOrderErasureAnchor is the plan table the new tables are placed before.
const mealOrderErasureAnchor = "schedule.meal_participation_overrides"

func mealOrderErasureStatements() ([]string, error) {
	preview, err := schoolErasurePreviewSQL()
	if err != nil {
		return nil, err
	}
	lines := func(format string) string {
		var out strings.Builder
		for _, table := range mealOrderErasureTables {
			out.WriteString(strings.ReplaceAll(format, "%t", table))
		}
		return out.String()
	}
	type insertion struct {
		name, source, anchor, lines string
	}
	insertions := []insertion{
		{"plan", schoolErasurePlanSQL, `  {"name": "` + mealOrderErasureAnchor + `"`, lines(`  {"name": "%t","predicate": "r.tenant_id = $1"},` + "\n")},
		{"preflight", schoolErasurePreflightSQL, "    " + mealOrderErasureAnchor + ",\n", lines("    %t,\n")},
		{"residual", schoolErasureResidualSQL, "    SELECT 1 FROM " + mealOrderErasureAnchor + " AS r", lines("    SELECT 1 FROM %t AS r WHERE r.tenant_id = p_school_id\n    UNION ALL\n")},
		{"delete", schoolErasureDeleteSQL, "   SELECT count(*) INTO v_count FROM " + mealOrderErasureAnchor + " AS r", lines(
			"   SELECT count(*) INTO v_count FROM %t AS r WHERE r.tenant_id = p_school_id;\n" +
				"   DELETE FROM %t AS r WHERE r.tenant_id = p_school_id;\n" +
				"   v_counts := v_counts || jsonb_build_object('%t', v_count);\n")},
		{"preview", preview, " v_counts := v_counts || jsonb_build_object('" + mealOrderErasureAnchor + "'", lines(
			" v_counts := v_counts || jsonb_build_object('%t',(SELECT count(*) FROM %t AS r WHERE r.tenant_id = p_school));\n")},
	}
	statements := make([]string, 0, len(insertions))
	for _, step := range insertions {
		if strings.Count(step.source, step.anchor) != 1 {
			return nil, fmt.Errorf("school erasure %s: anchor %q is not unique", step.name, mealOrderErasureAnchor)
		}
		statements = append(statements, strings.Replace(step.source, step.anchor, step.lines+step.anchor, 1))
	}
	const frozen = "965e62ae91b59194532b9156d076734507fa84e1c90dd8d5b5dc93d16572d510"
	if strings.Count(statements[1], frozen) != 1 {
		return nil, fmt.Errorf("school erasure preflight: fingerprint %s not found", frozen)
	}
	statements[1] = strings.Replace(statements[1], frozen, mealOrderErasureFingerprint, 1)
	return statements, nil
}

// schoolErasurePreviewSQL is the preview function of 1.15.490 as a
// replacement of the installed one.
func schoolErasurePreviewSQL() (string, error) {
	const start, end = "CREATE FUNCTION platform.school_erasure_preview(", "END $body$;"
	from := strings.Index(schoolErasureFilesSQL, start)
	if from < 0 {
		return "", fmt.Errorf("school erasure preview not found")
	}
	length := strings.Index(schoolErasureFilesSQL[from:], end)
	if length < 0 {
		return "", fmt.Errorf("school erasure preview has no end")
	}
	return "CREATE OR REPLACE " + strings.TrimPrefix(schoolErasureFilesSQL[from:from+length+len(end)], "CREATE "), nil
}
