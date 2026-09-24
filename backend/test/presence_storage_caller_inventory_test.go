package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The presence cutover (#2762) left the old execution columns of
// schedule.activity_instances and the old attendance columns of
// schedule.instance_students in place as a rollback-only mirror kept by
// triggers. Current application providers read and write those facts through
// Student Presence (active.activity_sessions, active.activity_session_attendance)
// or the tenant-safe presence projection, never through the mirrored columns
// and never through the routing counter. Migrations own the compatibility SQL;
// test-support packages build fixtures and are not providers.
func TestPresenceStorageCallerInventory(t *testing.T) {
	t.Parallel()
	mirroredExecution := `active_group_id|started_by|started_at|completed_at|completed_by|reopen_until|completion_snapshot`
	mirroredAttendance := `status|substatus|note|checked_in_at|checked_out_at|is_unplanned|not_scheduled|manual_status_at|student_status_day_id|pickup_exception_id`
	qualified := []*regexp.Regexp{
		// "activity_instance".active_group_id, activity_instance.started_at, ai.completed_at
		regexp.MustCompile(`\b(?:activity_instances?|instance|ai)"?\.(?:` + mirroredExecution + `)\b`),
		// "instance_student".status, instance_student.checked_in_at
		regexp.MustCompile(`\b(?:instance_students?|participant|is)"?\.(?:` + mirroredAttendance + `)\b`),
		regexp.MustCompile(`\bpresence_compatibility_writes\b`),
	}
	// A statement that names the old table and one of its mirrored columns
	// in the same literal without qualifying it. Qualified references were
	// judged above. Before the bare words are inspected, references through
	// the owner tables' aliases ("attendance", "session") are stripped, but
	// only in a literal that names the owner table itself, so an old-table
	// alias of the same name stays visible; the retained planning status of
	// the block ("activity_instance".status) is stripped through any alias.
	ownerTable := regexp.MustCompile(`\bactive\.activity_session(?:s|_attendance)\b`)
	ownerReference := regexp.MustCompile(`\b(?:attendance|session)"?\.\w+`)
	planningStatus := regexp.MustCompile(`\w+"?\.status\b`)
	unqualified := []struct {
		table   *regexp.Regexp
		columns *regexp.Regexp
	}{
		{regexp.MustCompile(`\bschedule\.activity_instances\b`), regexp.MustCompile(`\b(?:` + mirroredExecution + `)\b`)},
		{regexp.MustCompile(`\bschedule\.instance_students\b`), regexp.MustCompile(`\b(?:` + mirroredAttendance + `)\b`)},
	}
	for _, root := range []string{"api", "services", "modules", "workflows", "database/repositories", "seed"} {
		err := filepath.WalkDir(filepath.Join("..", root), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				// Test-support packages (timetabletest, ...)
				// build fixtures for the retained tests and are not providers.
				if strings.HasSuffix(entry.Name(), "test") && path != filepath.Join("..", root) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Errorf("%s: invalid string: %v", positions.Position(literal.Pos()), err)
					return true
				}
				for _, pattern := range qualified {
					if match := pattern.FindString(value); match != "" {
						t.Errorf("%s: application references the rollback-only presence mirror: %q", positions.Position(literal.Pos()), match)
					}
				}
				bare := planningStatus.ReplaceAllString(value, " ")
				if ownerTable.MatchString(value) {
					bare = ownerReference.ReplaceAllString(bare, " ")
				}
				for _, pattern := range unqualified {
					if pattern.table.MatchString(value) {
						if match := pattern.columns.FindString(bare); match != "" {
							t.Errorf("%s: statement on %s names the mirrored column %q", positions.Position(literal.Pos()), pattern.table.String(), match)
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("inventory %s: %v", root, err)
		}
	}
}
