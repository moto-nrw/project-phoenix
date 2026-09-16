package architecture

import "testing"

func TestSQLUpdateLockClauseIsNotTreatedAsTableWrite(t *testing.T) {
	t.Parallel()
	query := "UPDATE platform.email_outbox SET status = 'claimed' WHERE id IN " +
		"(SELECT id FROM platform.email_outbox FOR UPDATE SKIP LOCKED)"
	matches := writeTablePattern.FindAllStringSubmatchIndex(query, -1)
	if len(matches) != 2 {
		t.Fatalf("expected the SQL regex to see the statement and lock clause, got %d matches", len(matches))
	}
	if sqlUpdateIsNotStatementTarget(query, matches[0][0]) {
		t.Fatal("statement UPDATE must remain a table write")
	}
	if !sqlUpdateIsNotStatementTarget(query, matches[1][0]) {
		t.Fatal("FOR UPDATE must not be treated as a write to table SKIP")
	}
}

func TestSQLConflictActionUpdateIsNotTreatedAsTableWrite(t *testing.T) {
	t.Parallel()
	query := "INSERT INTO users.care_withdrawal_completions (tenant_id, student_id) " +
		"SELECT tenant_id, student_id FROM users.care_withdrawal_completions WHERE id = ? " +
		"ON CONFLICT (tenant_id, student_id) WHERE state = 'pending' DO UPDATE SET updated_at = EXCLUDED.updated_at"
	matches := writeTablePattern.FindAllStringSubmatchIndex(query, -1)
	if len(matches) != 2 {
		t.Fatalf("expected the SQL regex to see the INSERT and the conflict action, got %d matches", len(matches))
	}
	if sqlUpdateIsNotStatementTarget(query, matches[0][0]) {
		t.Fatal("INSERT INTO must remain a table write")
	}
	if !sqlUpdateIsNotStatementTarget(query, matches[1][0]) {
		t.Fatal("DO UPDATE must not be treated as a write to table SET")
	}
}

func TestSQLSystemCatalogReadsAreNeitherOwnedNorUnresolved(t *testing.T) {
	t.Parallel()
	analyzer := semanticAnalyzer{
		packages:    map[string]Package{"example.com/repo": {Owner: "people-directory", Role: "postgres"}},
		dataObjects: map[string]DataObject{"users.students": {Name: "users.students", WriteOwner: "people-directory"}},
		dataSchemas: map[string]struct{}{"users": {}},
	}
	query := `SELECT a.attname FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN information_schema.columns ic ON ic.column_name = a.attname
		WHERE c.relname = 'students'`
	violations := analyzer.sqlStringViolations("example.com/repo", "people-directory", "VerifySchema", "NewRaw", query, true)
	if len(violations) != 0 {
		t.Fatalf("system catalog reads must produce no finding, got %+v", violations)
	}

	unknown := `SELECT id FROM mystery.table`
	violations = analyzer.sqlStringViolations("example.com/repo", "people-directory", "Lookup", "NewRaw", unknown, true)
	if len(violations) != 1 || violations[0].Rule != "tables.unresolved" {
		t.Fatalf("a read of an unknown schema must stay unresolved, got %+v", violations)
	}
}
