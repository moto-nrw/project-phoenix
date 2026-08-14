package parent_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSubmitCareExceptionWithReasonPersistsReason(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	date := timezone.TodayDate().AddDays(1)
	result, err := svc.SubmitCareExceptionWithReason(
		context.Background(),
		chain.AccountID,
		chain.StudentID,
		date,
		wallClock(14, 30),
		nil,
		"  Arzttermin  ",
	)
	require.NoError(t, err)
	require.NotNil(t, result.Reason)
	assert.Equal(t, "Arzttermin", *result.Reason)

	var reason string
	require.NoError(t, db.NewSelect().
		Column("reason").
		TableExpr("schedule.student_pickup_exceptions").
		Where("student_id = ?", chain.StudentID).
		Where("exception_date = ?", date).
		Scan(context.Background(), &reason))
	assert.Equal(t, "Arzttermin", reason)
}

func TestSubmitCareExceptionWithReasonValidatesInput(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	date := timezone.TodayDate().AddDays(1)
	_, err := svc.SubmitCareExceptionWithReason(
		context.Background(),
		chain.AccountID,
		chain.StudentID,
		date,
		wallClock(14, 30),
		nil,
		"   ",
	)
	assert.ErrorIs(t, err, parentService.ErrCareExceptionReasonRequired)

	_, err = svc.SubmitCareExceptionWithReason(
		context.Background(),
		chain.AccountID,
		chain.StudentID,
		date,
		wallClock(14, 30),
		nil,
		strings.Repeat("a", 256),
	)
	assert.ErrorIs(t, err, parentService.ErrCareExceptionReasonTooLong)
}
