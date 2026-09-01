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
	if sqlUpdateBelongsToLockClause(query, matches[0][0]) {
		t.Fatal("statement UPDATE must remain a table write")
	}
	if !sqlUpdateBelongsToLockClause(query, matches[1][0]) {
		t.Fatal("FOR UPDATE must not be treated as a write to table SKIP")
	}
}
