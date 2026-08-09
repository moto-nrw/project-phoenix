package active

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mixed-scope bulk status writes must fail closed before any row lands.
// Regression for the class-trip bulk partial-commit path under outer withTx.
func TestBulkCreateForDates_RejectsUnauthorizedWithoutPartialWrites(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	service := NewStudentStatusDayService(repoFactory.StudentStatusDay)
	studentService := users.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent, repoFactory.StudentCompanion, nil)

	allowed := testpkg.CreateTestStudent(t, db, "BulkStatusAllowed", "Student", "BSA1")
	denied := testpkg.CreateTestStudent(t, db, "BulkStatusDenied", "Student", "BSD1")
	defer testpkg.CleanupActivityFixtures(t, db, allowed.ID, denied.ID)

	ctx := testpkg.TenantContext(1)
	dates := []timezone.Date{timezone.NewDate(2026, 5, 12)}
	err := service.BulkCreateForDates(ctx, StatusDayWriteContext{
		DB:             db,
		TenantID:       1,
		StudentService: studentService,
		Authorize: func(_ context.Context, student *userModels.Student) bool {
			return student.ID == allowed.ID
		},
		AfterCommit: func(int64) {},
	}, []int64{allowed.ID, denied.ID}, activeModels.StudentStatusDayClassTrip, "Klassenfahrt", dates)

	require.ErrorIs(t, err, ErrStudentStatusDayReassigned)

	for _, studentID := range []int64{allowed.ID, denied.ID} {
		rows, findErr := service.GetActiveByStudentAndDateRange(ctx, studentID, dates[0], dates[0])
		require.NoError(t, findErr)
		assert.Empty(t, rows, "authorization failure must not leave status days for student %d", studentID)
	}
}
