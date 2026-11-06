package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The request queues' name search finds "Gómez" for "gomez" (#4949).
func TestRequestQueueStudentNameSearchIgnoresAccents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	wanted := testpkg.CreateTestStudent(t, db, "Zoë", "Gómez", "1a")
	testpkg.CreateTestStudent(t, db, "Kim", "Becker", "1a")

	for _, search := range []string{"zoe gomez", "GÓMEZ"} {
		var ids []int64
		query := db.NewSelect().
			TableExpr(`(SELECT id AS student_id, tenant_id FROM users.student_profiles) AS "request"`).
			ColumnExpr(`"request".student_id`).
			Where(`"request".tenant_id = ?`, testpkg.Tenant(t))
		require.NoError(t, filterRequestQueueStudentName(testpkg.StudentDirectory{}, query, "request", search).Scan(ctx, &ids), search)
		assert.Equal(t, []int64{wanted.ID}, ids, search)
	}
}

// A fullwidth ％ folds to "%"; the search escapes after folding, so it
// matches literally instead of every student (#5791).
func TestRequestQueueStudentNameSearchTreatsFullwidthLikeCharactersLiterally(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	testpkg.CreateTestStudent(t, db, "Zoë", "Gómez", "1a")

	for _, search := range []string{"％", "＿", "＼"} {
		var ids []int64
		query := db.NewSelect().
			TableExpr(`(SELECT id AS student_id, tenant_id FROM users.student_profiles) AS "request"`).
			ColumnExpr(`"request".student_id`).
			Where(`"request".tenant_id = ?`, testpkg.Tenant(t))
		require.NoError(t, filterRequestQueueStudentName(testpkg.StudentDirectory{}, query, "request", search).Scan(ctx, &ids), search)
		assert.Empty(t, ids, search)
	}
}

// The withdrawal queue's search finds "Özdemir" for "ozdem" (#4949).
func TestWithdrawalSearchIgnoresAccents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	store := NewDirectoryStore(New(func(context.Context) (bun.IDB, int64, error) {
		return db, testpkg.Tenant(t), nil
	}), testpkg.StudentDirectory{})
	actor := testpkg.CreateTestAccount(t, db, fmt.Sprintf("withdrawal-search-%d@example.com", testpkg.UniqueSuffix()))
	wanted := testpkg.CreateTestStudent(t, db, "Zoë", "Özdemir", "2b")
	other := testpkg.CreateTestStudent(t, db, "Kim", "Becker", "2b")
	for _, student := range []*testpkg.Student{wanted, other} {
		testpkg.CreateTestCareWithdrawalCompletion(t, db, student.ID, actor.ID, calendar.NewDate(2026, 9, 1))
	}

	for _, search := range []string{"ozdem", "ZOE"} {
		rows, total, _, err := store.ListWithdrawals(ctx, careplan.WithdrawalStatePending, careplan.WithdrawalListFilter{Search: search, Page: 1, PageSize: 10})
		require.NoError(t, err, search)
		assert.Equal(t, 1, total, search)
		require.Len(t, rows, 1, search)
		require.NotNil(t, rows[0].StudentID, search)
		assert.Equal(t, wanted.ID, *rows[0].StudentID, search)
	}
}

// The withdrawal queue's search treats LIKE characters literally, also the
// fullwidth ones unaccent turns into them (#5791).
func TestWithdrawalSearchTreatsLikeCharactersLiterally(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	store := NewDirectoryStore(New(func(context.Context) (bun.IDB, int64, error) {
		return db, testpkg.Tenant(t), nil
	}), testpkg.StudentDirectory{})
	actor := testpkg.CreateTestAccount(t, db, fmt.Sprintf("withdrawal-like-%d@example.com", testpkg.UniqueSuffix()))
	student := testpkg.CreateTestStudent(t, db, "Zoë", "Özdemir", "2b")
	testpkg.CreateTestCareWithdrawalCompletion(t, db, student.ID, actor.ID, calendar.NewDate(2026, 9, 1))

	for _, search := range []string{"％", "＿", "%", "_"} {
		rows, total, _, err := store.ListWithdrawals(ctx, careplan.WithdrawalStatePending, careplan.WithdrawalListFilter{Search: search, Page: 1, PageSize: 10})
		require.NoError(t, err, search)
		assert.Zero(t, total, search)
		assert.Empty(t, rows, search)
	}
}
