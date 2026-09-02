package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPhaseExpiryRepository_ListSnapshots_CountsWholeCohortAtFirstAffectedDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	phase := makeValidPhase(uniquePhaseName("expiry-whole-cohort"))
	phase.ServiceStartDate = timezone.NewDate(2026, 8, 1)
	phase.ServiceEndDate = timezone.NewDate(2027, 1, 29)
	request := makeRequest(0, uniqueToken("expiry-whole-cohort"), "cohort@example.test")

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := enrollmentRepo.NewPhaseRepository(db).Create(ctx, phase); err != nil {
			return err
		}
		request.PhaseID = phase.ID
		return enrollmentRepo.NewRequestRepository(db).Create(ctx, request)
	}))

	mondayStudent := testpkg.CreateTestStudent(t, db, "Monday", "Child", "2a")
	fridayStudent := testpkg.CreateTestStudent(t, db, "Friday", "Child", "2a")
	validUntil := phase.ServiceEndDate.AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := usersRepo.NewStudentRepository(db)
		for _, student := range []*usersModels.Student{mondayStudent, fridayStudent} {
			if err := studentRepo.SetEnrollmentWindowByID(
				ctx, student.ID, phase.ServiceStartDate, usersModels.StudentStatusActive,
			); err != nil {
				return err
			}
		}
		childRepo := enrollmentRepo.NewRequestChildRepository(db)
		offeringRepo := enrollmentRepo.NewCareOfferingRepository(db)
		linkRepo := enrollmentRepo.NewRequestChildOfferingRepository(db)
		for _, fixture := range []struct {
			student *usersModels.Student
			day     string
		}{
			{student: mondayStudent, day: "mon"},
			{student: fridayStudent, day: "fri"},
		} {
			child := makeChild(request.ID, fixture.day, "Child")
			child.Status = enrollmentModels.ChildStatusApproved
			child.CreatedStudentID = &fixture.student.ID
			if err := childRepo.Create(ctx, child); err != nil {
				return err
			}
			offering := makeOffering(phase.ID, uniqueOfferingName("cohort-"+fixture.day))
			offering.AvailableDays = []string{fixture.day}
			if err := offeringRepo.Create(ctx, offering); err != nil {
				return err
			}
			if err := linkRepo.Create(ctx, &enrollmentModels.RequestChildOffering{
				RequestChildID: child.ID,
				CareOfferingID: offering.ID,
				ValidUntil:     &validUntil,
			}); err != nil {
				return err
			}
		}
		return nil
	}))

	var snapshots []*enrollmentModels.PhaseExpirySnapshot
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		snapshots, err = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return err
	}))
	require.Len(t, snapshots, 1)
	assert.Equal(t, timezone.NewDate(2027, 2, 1), snapshots[0].FirstAffectedDate)
	assert.Equal(t, 2, snapshots[0].AffectedChildren,
		"the Monday warning must immediately count children booked later in the same week")
	assert.Equal(t, 2, snapshots[0].UnresolvedChildren)

	futureStart := phase.ServiceEndDate.AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := usersRepo.NewStudentRepository(db)
		if err := studentRepo.SetEnrollmentWindowByID(
			ctx, mondayStudent.ID, phase.ServiceEndDate, usersModels.StudentStatusPending,
		); err != nil {
			return err
		}
		return studentRepo.SetEnrollmentWindowByID(
			ctx, fridayStudent.ID, futureStart, usersModels.StudentStatusPending,
		)
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		snapshots, err = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return err
	}))
	require.Len(t, snapshots, 1)
	assert.Equal(t, 2, snapshots[0].AffectedChildren,
		"a pending rollover student must keep the source booking warning visible")

	enrollmentEndedBeforePhaseEnd := phase.ServiceEndDate.AddDays(-1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := usersRepo.NewStudentRepository(db)
		if err := studentRepo.SetEnrollmentWindowByID(
			ctx, fridayStudent.ID, phase.ServiceEndDate, usersModels.StudentStatusPending,
		); err != nil {
			return err
		}
		_, err := studentRepo.SetEnrolledUntilByIDs(
			ctx, []int64{fridayStudent.ID}, &enrollmentEndedBeforePhaseEnd,
		)
		return err
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		snapshots, err = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return err
	}))
	require.Len(t, snapshots, 1)
	assert.Equal(t, 1, snapshots[0].AffectedChildren,
		"pending students ending before the source phase end must not trigger a warning")
}

func TestPhaseExpiryRepository_ListSnapshots_FindsMondayAfterFridayForNonCareOffering(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	phase := makeValidPhase(uniquePhaseName("expiry-source"))
	phase.ServiceStartDate = timezone.NewDate(2026, 8, 1)
	phase.ServiceEndDate = timezone.NewDate(2027, 1, 29)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentRepo.NewPhaseRepository(db).Create(ctx, phase)
	}))

	request := makeRequest(phase.ID, uniqueToken("expiry-source"), "expiry@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentRepo.NewRequestRepository(db).Create(ctx, request)
	}))

	student := testpkg.CreateTestStudent(t, db, "Expiry", "Student", "2a")
	lastCareDay := phase.ServiceEndDate
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := usersRepo.NewStudentRepository(db)
		if err := studentRepo.SetEnrollmentWindowByID(
			ctx, student.ID, phase.ServiceStartDate, usersModels.StudentStatusActive,
		); err != nil {
			return err
		}
		_, err := studentRepo.SetEnrolledUntilByIDs(ctx, []int64{student.ID}, &lastCareDay)
		return err
	}))
	child := makeChild(request.ID, "Expiry", "Student")
	child.Status = enrollmentModels.ChildStatusApproved
	child.CreatedStudentID = &student.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentRepo.NewRequestChildRepository(db).Create(ctx, child)
	}))

	offering := makeOffering(phase.ID, uniqueOfferingName("lunch"))
	offering.AvailableDays = []string{"mon"}
	offering.IncludesLunch = true
	offering.CountsAsCare = false
	offering.CountsAsCareSet = true
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentRepo.NewCareOfferingRepository(db).Create(ctx, offering)
	}))

	validFrom := phase.ServiceStartDate
	validUntil := phase.ServiceEndDate.AddDays(1)
	link := &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: offering.ID,
		ValidFrom:      &validFrom,
		ValidUntil:     &validUntil,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentRepo.NewRequestChildOfferingRepository(db).Create(ctx, link)
	}))
	secondOffering := makeOffering(phase.ID, uniqueOfferingName("second-offering"))
	secondOffering.AvailableDays = []string{"tue"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentRepo.NewCareOfferingRepository(db).Create(ctx, secondOffering)
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentRepo.NewRequestChildOfferingRepository(db).Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: child.ID,
			CareOfferingID: secondOffering.ID,
		})
	}))

	var snapshots []*enrollmentModels.PhaseExpirySnapshot
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return listErr
	})
	require.NoError(t, err)
	require.Len(t, snapshots, 1)

	snapshot := snapshots[0]
	assert.Equal(t, phase.ID, snapshot.SourcePhaseID)
	assert.Equal(t, timezone.NewDate(2027, 2, 1), snapshot.FirstAffectedDate)
	assert.Equal(t, 1, snapshot.AffectedChildren, "multiple offerings must not count one child twice")
	assert.Equal(t, 1, snapshot.UnresolvedChildren)
	assert.Nil(t, snapshot.SuccessorPhaseID)

	studentRepo := usersRepo.NewStudentRepository(db)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return studentRepo.UpdateStatus(ctx, student.ID, usersModels.StudentStatusPending)
	}))
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return listErr
	})
	require.NoError(t, err)
	require.Len(t, snapshots, 1,
		"a pending child whose enrollment already covers the phase end still needs a successor booking")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return studentRepo.UpdateStatus(ctx, student.ID, usersModels.StudentStatusActive)
	}))

	childRepo := enrollmentRepo.NewRequestChildRepository(db)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.UpdateStatus(ctx, child.ID, enrollmentModels.ChildStatusSubmitted, nil, 0)
	}))
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return listErr
	})
	require.NoError(t, err)
	assert.Empty(t, snapshots, "a non-approved request child must not trigger a warning")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.UpdateStatus(ctx, child.ID, enrollmentModels.ChildStatusApproved, nil, 0)
	}))
	offering.IsActive = false
	secondOffering.IsActive = false
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		offeringRepo := enrollmentRepo.NewCareOfferingRepository(db)
		if updateErr := offeringRepo.Update(ctx, offering); updateErr != nil {
			return updateErr
		}
		return offeringRepo.Update(ctx, secondOffering)
	}))
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return listErr
	})
	require.NoError(t, err)
	assert.Empty(t, snapshots, "inactive offerings must not trigger a warning")
	offering.IsActive = true
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentRepo.NewCareOfferingRepository(db).Update(ctx, offering)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return usersRepo.NewStudentRepository(db).UpdateStatus(ctx, student.ID, usersModels.StudentStatusInactive)
	}))
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return listErr
	})
	require.NoError(t, err)
	assert.Empty(t, snapshots, "an already inactive data corpse must not trigger a future warning")

	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryRepository(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 2, 1),
			timezone.NewDate(2027, 3, 3),
		)
		return listErr
	})
	require.NoError(t, err)
	require.Len(t, snapshots, 1, "phase-driven inactivation must keep the overdue warning visible")
	assert.Equal(t, 1, snapshots[0].AffectedChildren)
}

// legacyStudentDirectory serves the enrollment port from the legacy student
// repository, so these tests stay inside the enrollment module's allowed
// imports while the report reads students through the owner port (#2662).
type legacyStudentDirectory struct {
	students usersModels.StudentRepository
}

func (d legacyStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]enrollmentRepo.DirectoryStudent, error) {
	byID, err := d.students.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentRepo.DirectoryStudent, 0, len(byID))
	for _, student := range byID {
		result = append(result, toDirectoryStudent(student))
	}
	return result, nil
}

func (d legacyStudentDirectory) ListEnrolledStudents(ctx context.Context) ([]enrollmentRepo.DirectoryStudent, error) {
	students, err := d.students.List(ctx, map[string]any{})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		if student.IsAlumnus() {
			continue
		}
		result = append(result, toDirectoryStudent(student))
	}
	return result, nil
}

func toDirectoryStudent(student *usersModels.Student) enrollmentRepo.DirectoryStudent {
	row := enrollmentRepo.DirectoryStudent{
		ID: student.ID, SchoolClass: student.SchoolClass, Status: string(student.Status), Alumnus: student.IsAlumnus(),
	}
	if student.EnrolledFrom != nil {
		row.EnrolledFrom = student.EnrolledFrom.String()
	}
	if student.EnrolledUntil != nil {
		row.EnrolledUntil = student.EnrolledUntil.String()
	}
	return row
}

// newPhaseExpiryRepository returns the phase-expiry repository with the
// student port bound, as the service graph composes it.
func newPhaseExpiryRepository(t *testing.T, db *bun.DB) enrollmentModels.PhaseExpiryRepository {
	t.Helper()
	repo := enrollmentRepo.NewPhaseExpiryRepository(db).(*enrollmentRepo.PhaseExpiryRepository)
	repo.BindStudentDirectory(legacyStudentDirectory{students: usersRepo.NewStudentRepository(db)})
	return repo
}
