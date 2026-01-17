package repositories_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	factory := repositories.NewFactory(db)
	require.NotNil(t, factory)

	// Verify auth repositories are initialized
	t.Run("auth repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Account)
		assert.NotNil(t, factory.AccountParent)
		assert.NotNil(t, factory.Role)
		assert.NotNil(t, factory.Permission)
		assert.NotNil(t, factory.RolePermission)
		assert.NotNil(t, factory.AccountRole)
		assert.NotNil(t, factory.AccountPermission)
		assert.NotNil(t, factory.Token)
		assert.NotNil(t, factory.PasswordResetToken)
		assert.NotNil(t, factory.PasswordResetRateLimit)
		assert.NotNil(t, factory.InvitationToken)
		assert.NotNil(t, factory.GuardianInvitation)
	})

	// Verify users repositories are initialized
	t.Run("users repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Person)
		assert.NotNil(t, factory.RFIDCard)
		assert.NotNil(t, factory.Staff)
		assert.NotNil(t, factory.Student)
		assert.NotNil(t, factory.Teacher)
		assert.NotNil(t, factory.Guest)
		assert.NotNil(t, factory.Profile)
		assert.NotNil(t, factory.PersonGuardian)
		assert.NotNil(t, factory.StudentGuardian)
		assert.NotNil(t, factory.GuardianProfile)
		assert.NotNil(t, factory.PrivacyConsent)
	})

	// Verify facilities repositories are initialized
	t.Run("facilities repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Room)
	})

	// Verify education repositories are initialized
	t.Run("education repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Group)
		assert.NotNil(t, factory.GroupTeacher)
		assert.NotNil(t, factory.GroupSubstitution)
	})

	// Verify schedule repositories are initialized
	t.Run("schedule repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Dateframe)
		assert.NotNil(t, factory.Timeframe)
		assert.NotNil(t, factory.RecurrenceRule)
	})

	// Verify activities repositories are initialized
	t.Run("activities repositories", func(t *testing.T) {
		assert.NotNil(t, factory.ActivityGroup)
		assert.NotNil(t, factory.ActivityCategory)
		assert.NotNil(t, factory.ActivitySchedule)
		assert.NotNil(t, factory.ActivitySupervisor)
		assert.NotNil(t, factory.StudentEnrollment)
	})

	// Verify active repositories are initialized
	t.Run("active repositories", func(t *testing.T) {
		assert.NotNil(t, factory.ActiveGroup)
		assert.NotNil(t, factory.ActiveVisit)
		assert.NotNil(t, factory.GroupSupervisor)
		assert.NotNil(t, factory.CombinedGroup)
		assert.NotNil(t, factory.GroupMapping)
		assert.NotNil(t, factory.Attendance)
	})

	// Verify feedback repositories are initialized
	t.Run("feedback repositories", func(t *testing.T) {
		assert.NotNil(t, factory.FeedbackEntry)
	})

	// Verify IoT repositories are initialized
	t.Run("iot repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Device)
	})

	// Verify config repositories are initialized
	t.Run("config repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Setting)
	})

	// Verify audit repositories are initialized
	t.Run("audit repositories", func(t *testing.T) {
		assert.NotNil(t, factory.DataDeletion)
		assert.NotNil(t, factory.AuthEvent)
		assert.NotNil(t, factory.DataImport)
	})
}
