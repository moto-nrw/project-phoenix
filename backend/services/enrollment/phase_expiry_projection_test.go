package enrollment_test

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	peopleTest "github.com/moto-nrw/project-phoenix/modules/peopledirectory/peopletest"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"

	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPhaseExpiryProjection_ListSnapshots_CountsWholeCohortAtFirstAffectedDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	phase := makeOwnerEligibilityPhase(uniquePhaseName("expiry-whole-cohort"))
	phase.ServiceStartDate = capability.Date(timezone.NewDate(2026, 8, 1))
	phase.ServiceEndDate = capability.Date(timezone.NewDate(2027, 1, 29))
	request := makeOwnerRequest(0, uniqueToken("expiry-whole-cohort"), "cohort@example.test")

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertPhase(ctx, phase); err != nil {
			return err
		}
		request.PhaseID = phase.ID
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertRequest(ctx, request)
	}))

	mondayStudent := testpkg.CreateTestStudent(t, db, "Monday", "Child", "2a")
	fridayStudent := testpkg.CreateTestStudent(t, db, "Friday", "Child", "2a")
	validUntil := timezone.Date(phase.ServiceEndDate).AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := newExpiryStudentFixture(t, db)
		for _, student := range []*usersModels.Student{mondayStudent, fridayStudent} {
			if err := studentRepo.SetEnrollmentWindowByID(
				ctx, student.ID, timezone.Date(phase.ServiceStartDate), usersModels.StudentStatusActive,
			); err != nil {
				return err
			}
		}
		childRepo := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant)
		offeringRepo := carePlanTest.NewCareOfferingRepository(t, db)
		linkRepo := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant)
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
			if err := childRepo.InsertChild(ctx, child); err != nil {
				return err
			}
			offering := makeOffering(phase.ID, uniqueOfferingName("cohort-"+fixture.day))
			offering.AvailableDays = []string{fixture.day}
			if err := offeringRepo.Create(ctx, offering); err != nil {
				return err
			}
			if err := linkRepo.InsertRequestChildOffering(ctx, &capability.RequestChildOffering{
				RequestChildID: child.ID,
				CareOfferingID: offering.ID,
				ValidUntil:     offeringDatePointer(validUntil),
			}); err != nil {
				return err
			}
		}
		return nil
	}))

	var snapshots []*capability.PhaseExpirySnapshot
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		snapshots, err = newPhaseExpiryProjection(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return err
	}))
	require.Len(t, snapshots, 1)
	assert.Equal(t, capability.Date("2027-02-01"), snapshots[0].FirstAffectedDate)
	assert.Equal(t, 2, snapshots[0].AffectedChildren,
		"the Monday warning must immediately count children booked later in the same week")
	assert.Equal(t, 2, snapshots[0].UnresolvedChildren)

	futureStart := timezone.Date(phase.ServiceEndDate).AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := newExpiryStudentFixture(t, db)
		if err := studentRepo.SetEnrollmentWindowByID(
			ctx, mondayStudent.ID, timezone.Date(phase.ServiceEndDate), usersModels.StudentStatusPending,
		); err != nil {
			return err
		}
		return studentRepo.SetEnrollmentWindowByID(
			ctx, fridayStudent.ID, futureStart, usersModels.StudentStatusPending,
		)
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		snapshots, err = newPhaseExpiryProjection(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return err
	}))
	require.Len(t, snapshots, 1)
	assert.Equal(t, 2, snapshots[0].AffectedChildren,
		"a pending rollover student must keep the source booking warning visible")

	enrollmentEndedBeforePhaseEnd := timezone.Date(phase.ServiceEndDate).AddDays(-1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := newExpiryStudentFixture(t, db)
		if err := studentRepo.SetEnrollmentWindowByID(
			ctx, fridayStudent.ID, timezone.Date(phase.ServiceEndDate), usersModels.StudentStatusPending,
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
		snapshots, err = newPhaseExpiryProjection(t, db).ListSnapshots(
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

func TestPhaseExpiryProjection_ListSnapshots_FindsMondayAfterFridayForNonCareOffering(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	phase := makeOwnerEligibilityPhase(uniquePhaseName("expiry-source"))
	phase.ServiceStartDate = capability.Date(timezone.NewDate(2026, 8, 1))
	phase.ServiceEndDate = capability.Date(timezone.NewDate(2027, 1, 29))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertPhase(ctx, phase)
	}))

	request := makeOwnerRequest(phase.ID, uniqueToken("expiry-source"), "expiry@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertRequest(ctx, request)
	}))

	student := testpkg.CreateTestStudent(t, db, "Expiry", "Student", "2a")
	lastCareDay := timezone.Date(phase.ServiceEndDate)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := newExpiryStudentFixture(t, db)
		if err := studentRepo.SetEnrollmentWindowByID(
			ctx, student.ID, timezone.Date(phase.ServiceStartDate), usersModels.StudentStatusActive,
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
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertChild(ctx, child)
	}))

	offering := makeOffering(phase.ID, uniqueOfferingName("lunch"))
	offering.AvailableDays = []string{"mon"}
	offering.IncludesLunch = true
	offering.CountsAsCare = false
	offering.CountsAsCareSet = true
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return carePlanTest.NewCareOfferingRepository(t, db).Create(ctx, offering)
	}))

	validFrom := timezone.Date(phase.ServiceStartDate)
	validUntil := timezone.Date(phase.ServiceEndDate).AddDays(1)
	link := &capability.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: offering.ID,
		ValidFrom:      offeringDatePointer(validFrom),
		ValidUntil:     offeringDatePointer(validUntil),
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertRequestChildOffering(ctx, link)
	}))
	secondOffering := makeOffering(phase.ID, uniqueOfferingName("second-offering"))
	secondOffering.AvailableDays = []string{"tue"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return carePlanTest.NewCareOfferingRepository(t, db).Create(ctx, secondOffering)
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertRequestChildOffering(ctx, &capability.RequestChildOffering{
			RequestChildID: child.ID,
			CareOfferingID: secondOffering.ID,
		})
	}))

	var snapshots []*capability.PhaseExpirySnapshot
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryProjection(t, db).ListSnapshots(
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
	assert.Equal(t, capability.Date("2027-02-01"), snapshot.FirstAffectedDate)
	assert.Equal(t, 1, snapshot.AffectedChildren, "multiple offerings must not count one child twice")
	assert.Equal(t, 1, snapshot.UnresolvedChildren)
	assert.Nil(t, snapshot.SuccessorPhaseID)

	studentRepo := newExpiryStudentFixture(t, db)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return studentRepo.UpdateStatus(ctx, student.ID, usersModels.StudentStatusPending)
	}))
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryProjection(t, db).ListSnapshots(
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

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).UpdateChildStatus(ctx, child.ID, enrollmentModels.ChildStatusSubmitted, nil, 0)
	}))
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryProjection(t, db).ListSnapshots(
			ctx,
			timezone.NewDate(2027, 1, 2),
			timezone.NewDate(2027, 2, 1),
		)
		return listErr
	})
	require.NoError(t, err)
	assert.Empty(t, snapshots, "a non-approved request child must not trigger a warning")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).UpdateChildStatus(ctx, child.ID, enrollmentModels.ChildStatusApproved, nil, 0)
	}))
	offering.IsActive = false
	secondOffering.IsActive = false
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		offeringRepo := carePlanTest.NewCareOfferingRepository(t, db)
		if updateErr := offeringRepo.Update(ctx, offering); updateErr != nil {
			return updateErr
		}
		return offeringRepo.Update(ctx, secondOffering)
	}))
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryProjection(t, db).ListSnapshots(
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
		return carePlanTest.NewCareOfferingRepository(t, db).Update(ctx, offering)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return newExpiryStudentFixture(t, db).UpdateStatus(ctx, student.ID, usersModels.StudentStatusInactive)
	}))
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var listErr error
		snapshots, listErr = newPhaseExpiryProjection(t, db).ListSnapshots(
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
		snapshots, listErr = newPhaseExpiryProjection(t, db).ListSnapshots(
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

type expiryStudentDirectory struct{ query peopleTest.StudentQuery }

func (d expiryStudentDirectory) ListEnrolledStudents(ctx context.Context) ([]enrollmentService.PhaseExpiryStudent, error) {
	students, err := d.query.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentService.PhaseExpiryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, enrollmentService.PhaseExpiryStudent{ID: student.ID, Status: student.Status, EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil})
	}
	return result, nil
}

type phaseExpiryCareOfferingDirectory struct{ query careplan.Query }

func (d phaseExpiryCareOfferingDirectory) ListCareOfferings(ctx context.Context) ([]enrollmentService.PhaseExpiryOffering, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentService.PhaseExpiryOffering, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentService.PhaseExpiryOffering{ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID, DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive})
	}
	return result, nil
}

// Exercise the production projection with the real owners.
func newPhaseExpiryProjection(t *testing.T, db *bun.DB) enrollmentService.PhaseExpirySnapshots {
	t.Helper()
	owner := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant)
	students, err := peopleTest.NewStudentQuery(db)
	require.NoError(t, err)
	return enrollmentService.NewPhaseExpiryProjection(owner, expiryStudentDirectory{students}, phaseExpiryCareOfferingDirectory{carePlanTest.NewCarePlan(t, db)}, owner)
}

// Fixture-only lifecycle setup: the report tests do not exercise lifecycle commands.
type expiryStudentFixture struct {
	t  *testing.T
	db *bun.DB
}

func newExpiryStudentFixture(t *testing.T, db *bun.DB) expiryStudentFixture {
	t.Helper()
	return expiryStudentFixture{t, db}
}

func (f expiryStudentFixture) set(ctx context.Context, ids []int64, values map[string]any) (count int64, err error) {
	err = testpkg.WithTenantTx(f.t, ctx, f.db, testpkg.Tenant(f.t), func(ctx context.Context, tx bun.Tx) error {
		query := tx.NewUpdate().TableExpr("users.students").Where("tenant_id = ?", testpkg.Tenant(f.t)).Where("id IN (?)", bun.List(ids))
		for column, value := range values {
			query = query.Set("? = ?", bun.Ident(column), value)
		}
		result, err := query.Exec(ctx)
		if err != nil {
			return err
		}
		count, err = result.RowsAffected()
		return err
	})
	return count, err
}

func (f expiryStudentFixture) SetEnrollmentWindowByID(ctx context.Context, id int64, from timezone.Date, status usersModels.StudentStatus) error {
	count, err := f.set(ctx, []int64{id}, map[string]any{"enrolled_from": from, "enrolled_until": nil, "status": string(status)})
	if err == nil && count != 1 {
		return fmt.Errorf("student fixture: expected one row, got %d", count)
	}
	return err
}

func (f expiryStudentFixture) SetEnrolledUntilByIDs(ctx context.Context, ids []int64, until *timezone.Date) (int64, error) {
	return f.set(ctx, ids, map[string]any{"enrolled_until": until})
}

func (f expiryStudentFixture) UpdateStatus(ctx context.Context, id int64, status usersModels.StudentStatus) error {
	count, err := f.set(ctx, []int64{id}, map[string]any{"status": string(status)})
	if err == nil && count != 1 {
		return fmt.Errorf("student fixture: expected one row, got %d", count)
	}
	return err
}
