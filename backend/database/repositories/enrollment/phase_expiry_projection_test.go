package enrollment_test

import (
	"context"
	"encoding/json"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
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
		if err := enrollmentCompose.New().InsertPhase(ctx, phase); err != nil {
			return err
		}
		request.PhaseID = phase.ID
		return enrollmentCompose.New().InsertRequest(ctx, request)
	}))

	mondayStudent := testpkg.CreateTestStudent(t, db, "Monday", "Child", "2a")
	fridayStudent := testpkg.CreateTestStudent(t, db, "Friday", "Child", "2a")
	validUntil := timezone.Date(phase.ServiceEndDate).AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := usersRepo.NewStudentRepository(db)
		for _, student := range []*usersModels.Student{mondayStudent, fridayStudent} {
			if err := studentRepo.SetEnrollmentWindowByID(
				ctx, student.ID, timezone.Date(phase.ServiceStartDate), usersModels.StudentStatusActive,
			); err != nil {
				return err
			}
		}
		childRepo := enrollmentCompose.New()
		offeringRepo := carePlanTest.NewCareOfferingRepository(t, db)
		linkRepo := enrollmentCompose.New()
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
		studentRepo := usersRepo.NewStudentRepository(db)
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
		studentRepo := usersRepo.NewStudentRepository(db)
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
		return enrollmentCompose.New().InsertPhase(ctx, phase)
	}))

	request := makeOwnerRequest(phase.ID, uniqueToken("expiry-source"), "expiry@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().InsertRequest(ctx, request)
	}))

	student := testpkg.CreateTestStudent(t, db, "Expiry", "Student", "2a")
	lastCareDay := timezone.Date(phase.ServiceEndDate)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		studentRepo := usersRepo.NewStudentRepository(db)
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
		return enrollmentCompose.New().InsertChild(ctx, child)
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
		return enrollmentCompose.New().InsertRequestChildOffering(ctx, link)
	}))
	secondOffering := makeOffering(phase.ID, uniqueOfferingName("second-offering"))
	secondOffering.AvailableDays = []string{"tue"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return carePlanTest.NewCareOfferingRepository(t, db).Create(ctx, secondOffering)
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().InsertRequestChildOffering(ctx, &capability.RequestChildOffering{
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

	studentRepo := usersRepo.NewStudentRepository(db)
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
		return enrollmentCompose.New().UpdateChildStatus(ctx, child.ID, enrollmentModels.ChildStatusSubmitted, nil, 0)
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
		return enrollmentCompose.New().UpdateChildStatus(ctx, child.ID, enrollmentModels.ChildStatusApproved, nil, 0)
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
		return usersRepo.NewStudentRepository(db).UpdateStatus(ctx, student.ID, usersModels.StudentStatusInactive)
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

type legacyStudentDirectory struct {
	students usersModels.StudentRepository
}

func (d legacyStudentDirectory) ListEnrolledStudents(ctx context.Context) ([]expiryStudent, error) {
	students, err := d.students.List(ctx, map[string]any{})
	if err != nil {
		return nil, err
	}
	result := make([]expiryStudent, 0, len(students))
	for _, student := range students {
		if student.IsAlumnus() {
			continue
		}
		result = append(result, toDirectoryStudent(student))
	}
	return result, nil
}

func toDirectoryStudent(student *usersModels.Student) expiryStudent {
	row := expiryStudent{
		ID: student.ID, Status: string(student.Status),
	}
	if student.EnrolledFrom != nil {
		row.EnrolledFrom = student.EnrolledFrom.String()
	}
	if student.EnrolledUntil != nil {
		row.EnrolledUntil = student.EnrolledUntil.String()
	}
	return row
}

type phaseExpiryCareOfferingDirectory struct{ query careplan.Query }

func (d phaseExpiryCareOfferingDirectory) ListCareOfferings(ctx context.Context) ([]expiryOffering, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]expiryOffering, 0, len(values))
	for _, value := range values {
		result = append(result, expiryOffering{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive,
		})
	}
	return result, nil
}

// newPhaseExpiryProjection returns the phase-expiry repository with the
// student port bound, as the service graph composes it.

type expiryOffering struct {
	ID             int64    `json:"id"`
	TenantID       int64    `json:"tenant_id"`
	PhaseID        int64    `json:"phase_id"`
	DaysOfWeekMode string   `json:"days_of_week_mode"`
	AvailableDays  []string `json:"available_days"`
	IsActive       bool     `json:"is_active"`
}

type expiryFixtureProjection struct {
	owner     *capability.Module
	students  legacyStudentDirectory
	offerings phaseExpiryCareOfferingDirectory
}

func newPhaseExpiryProjection(t *testing.T, db *bun.DB) expiryFixtureProjection {
	t.Helper()
	return expiryFixtureProjection{enrollmentCompose.New(), legacyStudentDirectory{usersRepo.NewStudentRepository(db)}, phaseExpiryCareOfferingDirectory{carePlanTest.NewCarePlan(t, db)}}
}

func (p expiryFixtureProjection) ListSnapshots(ctx context.Context, asOf, through timezone.Date) ([]*capability.PhaseExpirySnapshot, error) {
	students, err := p.students.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	offerings, err := p.offerings.ListCareOfferings(ctx)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(offerings)
	if err != nil {
		return nil, err
	}
	input := capability.PhaseExpiryInput{AsOf: capability.Date(asOf), WarningThrough: capability.Date(through), OfferingsJSON: string(encoded)}
	for _, student := range students {
		input.StudentIDs = append(input.StudentIDs, student.ID)
		input.StudentStatuses = append(input.StudentStatuses, student.Status)
		input.EnrolledFrom = append(input.EnrolledFrom, student.EnrolledFrom)
		input.EnrolledUntil = append(input.EnrolledUntil, student.EnrolledUntil)
	}
	return p.owner.PhaseExpirySnapshots(ctx, input)
}

type expiryStudent struct {
	ID            int64
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}
