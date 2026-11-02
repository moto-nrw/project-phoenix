package architecture

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestPreparedStatementsKeepStaticTableOwnershipAndFailClosed(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "go.mod"), "module example.test/prepared\n\ngo 1.25\n")
	writeFile(t, filepath.Join(project, "value.go"), `package prepared
import ("context"; "database/sql")
func Notify(ctx context.Context, db *sql.DB, payload string) {
 stmt, _ := db.PrepareContext(ctx, "SELECT pg_notify($1, $2)")
 stmt.ExecContext(ctx, "channel", payload)
}
func Owned(ctx context.Context, db *sql.DB) {
 stmt, _ := db.PrepareContext(ctx, "SELECT id FROM alpha.records")
 stmt.QueryContext(ctx)
}
func Foreign(ctx context.Context, db *sql.DB) {
 stmt, _ := db.PrepareContext(ctx, "DELETE FROM beta.records WHERE id = $1")
 stmt.ExecContext(ctx, 123)
}
func Dynamic(ctx context.Context, db *sql.DB, query string) {
 stmt, _ := db.PrepareContext(ctx, query)
 stmt.ExecContext(ctx)
}
func Reassigned(ctx context.Context, db *sql.DB) {
 stmt, _ := db.PrepareContext(ctx, "SELECT 1")
 stmt, _ = db.PrepareContext(ctx, "DELETE FROM beta.records")
 stmt.ExecContext(ctx)
}
func RangeReassigned(ctx context.Context, db *sql.DB, statements []*sql.Stmt) {
 stmt, _ := db.PrepareContext(ctx, "SELECT 1")
 for _, stmt = range statements { stmt.ExecContext(ctx) }
}
func Parameter(ctx context.Context, stmt *sql.Stmt) { stmt.ExecContext(ctx) }
func ParameterRebound(ctx context.Context, db *sql.DB, stmt *sql.Stmt) {
 stmt.ExecContext(ctx)
 stmt, _ = db.PrepareContext(ctx, "SELECT 1")
}
func InitializedThenRebound(ctx context.Context, db *sql.DB, unknown *sql.Stmt) {
 var stmt = unknown
 stmt.ExecContext(ctx)
 stmt, _ = db.PrepareContext(ctx, "SELECT 1")
}
func Escaped(ctx context.Context, db *sql.DB) {
 stmt, _ := db.PrepareContext(ctx, "SELECT 1")
 ptr := &stmt
 *ptr, _ = db.PrepareContext(ctx, "DELETE FROM beta.records")
 stmt.ExecContext(ctx)
}
`)
	policy := &Policy{
		ModulePath: "example.test/prepared", Build: Build{GOOS: "linux", GOARCH: "amd64"},
		Packages:    []Package{{Path: ".", Owner: "alpha", Role: "postgres"}},
		DataObjects: []DataObject{{Name: "alpha.records", WriteOwner: "alpha"}, {Name: "beta.records", WriteOwner: "beta"}},
	}
	violations, err := analyzeSemantics(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, violation := range violations {
		for _, location := range violation.Locations {
			got = append(got, violation.Rule+"|"+location.Declaration)
		}
	}
	slices.Sort(got)
	want := []string{"tables.foreign-read|Foreign", "tables.foreign-write|Foreign", "tables.unresolved|Dynamic", "tables.unresolved|Escaped", "tables.unresolved|InitializedThenRebound", "tables.unresolved|Parameter", "tables.unresolved|ParameterRebound", "tables.unresolved|RangeReassigned", "tables.unresolved|Reassigned"}
	if !slices.Equal(got, want) {
		t.Fatalf("prepared statement findings = %v, want %v", got, want)
	}
}
