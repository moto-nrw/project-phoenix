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

	"github.com/stretchr/testify/assert"
)

// The Contract (#2763) removed the rollback-only mirror the presence cutover
// (#2762) had kept: the old execution columns of schedule.activity_instances,
// the old attendance columns of schedule.instance_students, their mirror and
// routing triggers and the write counter. Application code, fixtures and
// behavior tests alike read and write the execution and the attendance through
// Student Presence (active.activity_sessions, active.activity_session_attendance)
// or the tenant-safe presence projection. Historical migrations and the restore
// helpers keep the names for the frozen Expand/Backfill/Cutover contracts,
// which recreate the mirror inside disposable clones.
func TestPresenceStorageCallerInventory(t *testing.T) {
	t.Parallel()
	root := ".."
	exempt := map[string]bool{
		"test/presence_storage_cutover.go":               true, // historical restore helpers
		"test/presence_storage_caller_inventory_test.go": true, // this pattern
		"cmd/presence_backfill_test.go":                  true, // drives the refused backfill on a restored pre-cutover clone
	}
	files := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative == "database/migrations" || relative == "internal/architecture" || entry.Name() == "testdata" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || exempt[relative] {
			return nil
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, 0)
		if err != nil {
			return err
		}
		files++
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
			if match := presenceMirrorReference(value); match != "" {
				t.Errorf("%s: retired presence mirror reference %q", positions.Position(literal.Pos()), match)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	t.Logf("Inventory: %d Go files, zero retired presence mirror callers", files)
}

const (
	mirroredExecutionColumns  = `active_group_id|started_by|started_at|completed_at|completed_by|reopen_until|completion_snapshot`
	mirroredAttendanceColumns = `status|substatus|note|checked_in_at|checked_out_at|is_unplanned|not_scheduled|manual_status_at|student_status_day_id|pickup_exception_id`
)

var (
	presenceMirrorSQLComment = regexp.MustCompile(`(?m)^\s*--[^\n]*`)
	presenceMirrorQualified  = []*regexp.Regexp{
		// activity_instance.active_group_id, instance.started_at, ai.completed_at
		regexp.MustCompile(`\b(?:activity_instances?|instance|ai)\.(?:` + mirroredExecutionColumns + `)\b`),
		// instance_student.status, participant.checked_in_at
		regexp.MustCompile(`\b(?:instance_students?|participant|is)\.(?:` + mirroredAttendanceColumns + `)\b`),
		// The removed counter, mirror and routing objects.
		regexp.MustCompile(`\b(?:presence_compatibility_writes|mirror_activity_session(?:_attendance)?|route_activity_instance_compatibility|route_instance_student_compatibility|activity_sessions_mirror|activity_session_attendance_mirror|activity_instances_route_compatibility|instance_students_route_compatibility)\b`),
	}
	// A statement that names a plan table and one of its removed columns in
	// the same literal without qualifying it. Before the bare words are
	// inspected, references through the owner tables' aliases ("attendance",
	// "session") are stripped, but only in a literal that names the owner
	// table itself, so an old-table alias of the same name stays visible; the
	// retained planning status of the block (activity_instance.status) and
	// every other qualified status are stripped through any alias.
	presenceOwnerTable     = regexp.MustCompile(`\bactive\.activity_session(?:s|_attendance)\b`)
	presenceOwnerReference = regexp.MustCompile(`\b(?:attendance|session|activity_session|activity_session_attendance)\.\w+`)
	presenceQualifiedState = regexp.MustCompile(`\w+\.status\b`)
	presenceUnqualified    = []struct{ table, columns *regexp.Regexp }{
		{regexp.MustCompile(`\bschedule\.activity_instances\b`), regexp.MustCompile(`\b(?:` + mirroredExecutionColumns + `)\b`)},
		{regexp.MustCompile(`\bschedule\.instance_students\b`), regexp.MustCompile(`\b(?:` + mirroredAttendanceColumns + `)\b`)},
	}
)

// presenceMirrorReference returns the first reference to the removed mirror
// in one string literal, or "".
func presenceMirrorReference(value string) string {
	value = presenceMirrorSQLComment.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, `"`, "")
	for _, pattern := range presenceMirrorQualified {
		if match := pattern.FindString(value); match != "" {
			return match
		}
	}
	bare := value
	if presenceOwnerTable.MatchString(value) {
		bare = presenceOwnerReference.ReplaceAllString(bare, " ")
	}
	bare = presenceQualifiedState.ReplaceAllString(bare, " ")
	for _, pattern := range presenceUnqualified {
		if pattern.table.MatchString(value) {
			if match := pattern.columns.FindString(bare); match != "" {
				return match
			}
		}
	}
	return ""
}

func TestPresenceStorageCallerInventoryRecognizesMirrorNotOwners(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		`SELECT "activity_instance".active_group_id FROM schedule.activity_instances AS "activity_instance"`,
		`UPDATE schedule.instance_students SET checked_in_at = NOW() WHERE id = ?`,
		`SELECT instance_student.status FROM schedule.instance_students AS instance_student`,
		`INSERT INTO schedule.activity_instances (tenant_id, started_at) VALUES (?, ?)`,
		`SELECT coalesce(pg_sequence_last_value('active.presence_compatibility_writes'), 0)`,
		`DROP TRIGGER activity_sessions_mirror ON active.activity_sessions`,
	} {
		assert.NotEmpty(t, presenceMirrorReference(value), "missed mirror reference: %s", value)
	}
	for _, value := range []string{
		`SELECT "activity_instance".status FROM schedule.activity_instances AS "activity_instance" WHERE "activity_instance".status <> 'cancelled'`,
		`SELECT session.started_at FROM schedule.activity_instances AS instance JOIN active.activity_sessions AS session ON session.schedule_instance_id = instance.id`,
		`SELECT attendance.checked_in_at FROM schedule.instance_students AS participant JOIN active.activity_session_attendance AS attendance ON attendance.instance_student_id = participant.id`,
		`INSERT INTO active.activity_session_attendance (tenant_id, instance_student_id, status) VALUES (?, ?, ?)`,
		"-- schedule.instance_students.status is historical\nSELECT 42",
	} {
		assert.Empty(t, presenceMirrorReference(value), "not a mirror reference: %s", value)
	}
}
