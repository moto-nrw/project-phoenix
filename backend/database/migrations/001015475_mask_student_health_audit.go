package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const maskStudentHealthAuditVersion = "1.15.475"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     maskStudentHealthAuditVersion,
		Description: "Remove health free text from student change history (#3840)",
		DependsOn:   []string{studentFieldEditsVersion},
	})
	Migrations.MustRegister(maskStudentHealthAuditUp, maskStudentHealthAuditDown)
}

// A one-time, privileged exception to append-only: keep the event and its
// attribution, but remove health free text across every tenant. One statement
// is atomic, and rerunning it produces the same masks. No RLS or ACL changes.
func maskStudentHealthAuditUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
  UPDATE audit.student_field_edits
  SET old_value = CASE WHEN COALESCE(old_value, '') = ''
                       THEN '' ELSE 'Angabe vorhanden' END,
      new_value = CASE WHEN COALESCE(new_value, '') = '' THEN ''
                       WHEN COALESCE(old_value, '') = '' THEN 'Angabe hinzugefügt'
                       ELSE 'Angabe geändert' END
  WHERE field_name = 'health_info'
 `)
	if err != nil {
		return fmt.Errorf("mask student health audit values: %w", err)
	}
	return nil
}

func maskStudentHealthAuditDown(_ context.Context, _ *bun.DB) error {
	return errors.New("student health audit redaction is irreversible; roll forward")
}
