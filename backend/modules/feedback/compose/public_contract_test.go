package compose_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicFeedbackContractPreservesNormalizedEntry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Contract", "1a")
	settings, err := compose.NewBoundSettings(func(context.Context) (bool, error) { return true, nil }, func(context.Context) (int, error) { return 90, nil })
	require.NoError(t, err)
	module, err := compose.New(compose.Dependencies{DB: db, Settings: settings, Today: func() feedback.Date { return "2026-08-31" }, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	entry, err := module.Submit(testpkg.Ctx(t), feedback.CreateEntry{Value: " positive ", Day: "2026-08-31", Time: "12:34:56", StudentID: student.ID})
	require.NoError(t, err)
	found, err := module.LookupEntry(testpkg.Ctx(t), entry.ID)
	require.NoError(t, err)
	require.Equal(t, feedback.ValuePositive, found.Value)
	require.Equal(t, feedback.Date("2026-08-31"), found.Day)
	require.Equal(t, "12:34:56", found.Time)
}
