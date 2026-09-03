package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type observationLog struct {
	mu   sync.Mutex
	seen []Observation
}

func (l *observationLog) record(observation Observation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, observation)
}

func buildModule(t *testing.T, db *bun.DB, observe ...func(Observation)) *careplan.Module {
	t.Helper()
	observer := func(Observation) {}
	if len(observe) > 0 {
		observer = observe[0]
	}
	module, err := New(Dependencies{
		DB: db, Observe: observer, AmbientDB: func(context.Context) bun.IDB { return db },
		StudentLock: func(context.Context, int64) error { return nil }, StudentNotFound: errors.New("student not found"),
	})
	require.NoError(t, err)
	return module
}

func tenantContext(t *testing.T, db *bun.DB, tenantID int64) context.Context {
	t.Helper()
	testpkg.EnsureTestTenant(t, db, tenantID)
	return tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
}

func offeringFields(phaseID int64, name string) careplan.CareOfferingFields {
	return careplan.CareOfferingFields{
		PhaseID: phaseID, Name: name, DaysOfWeekMode: "fixed",
		AvailableDays: []string{"mon", "tue"}, AutoAddGradeLevels: []int{},
		IsActive: true, CountsAsCare: true, SelectionRule: "optional",
	}
}

func createOffering(t *testing.T, ctx context.Context, module *careplan.Module, fields careplan.CareOfferingFields) careplan.CareOffering {
	t.Helper()
	offering, err := module.CreateCareOffering(ctx, careplan.CreateCareOffering{CareOfferingFields: fields})
	require.NoError(t, err)
	return offering
}

func TestCareOfferingLifecycleAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)

	trigger := createOffering(t, ctx, module, offeringFields(phase.ID, "Frühbetreuung"))
	fields := offeringFields(phase.ID, "Ganztag")
	fields.AutoAddTriggerOfferingIDs = []int64{trigger.ID, trigger.ID}
	created := createOffering(t, ctx, module, fields)
	require.Equal(t, []int64{trigger.ID}, created.AutoAddTriggerOfferingIDs)

	found, err := module.FindCareOffering(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, []int64{trigger.ID}, found.AutoAddTriggerOfferingIDs)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	otherCtx := tenantContext(t, db, otherTenantID)
	_, err = module.FindCareOffering(otherCtx, created.ID)
	require.ErrorIs(t, err, careplan.ErrCareOfferingNotFound)
	listed, err := module.ListCareOfferings(otherCtx, careplan.CareOfferingFilter{})
	require.NoError(t, err)
	assert.Empty(t, listed)
	require.ErrorIs(t, module.DeleteCareOffering(otherCtx, created.ID), careplan.ErrCareOfferingNotFound)
}

func TestCareOfferingCompoundWritesRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	stable := createOffering(t, ctx, module, offeringFields(phase.ID, "Stabil"))

	otherTenantID := testpkg.UniqueTestTenantID(t)
	otherCtx := tenantContext(t, db, otherTenantID)
	foreignPhaseID := insertPhase(t, db, otherCtx, otherTenantID, "Fremd")
	foreign := createOffering(t, otherCtx, module, offeringFields(foreignPhaseID, "Fremdauslöser"))

	failedCreate := offeringFields(phase.ID, "Rollback Create")
	failedCreate.AutoAddTriggerOfferingIDs = []int64{foreign.ID}
	_, err := module.CreateCareOffering(ctx, careplan.CreateCareOffering{CareOfferingFields: failedCreate})
	require.Error(t, err)
	rows, listErr := module.ListCareOfferings(ctx, careplan.CareOfferingFilter{})
	require.NoError(t, listErr)
	require.Len(t, rows, 1, "the offering insert must roll back when its trigger insert fails")

	retry := offeringFields(phase.ID, "Rollback Create")
	retry.AutoAddTriggerOfferingIDs = []int64{stable.ID}
	created := createOffering(t, ctx, module, retry)
	require.Equal(t, []int64{stable.ID}, created.AutoAddTriggerOfferingIDs)

	update := offeringFields(phase.ID, "Must Roll Back")
	update.AutoAddTriggerOfferingIDs = []int64{foreign.ID}
	_, err = module.UpdateCareOffering(ctx, careplan.UpdateCareOffering{ID: created.ID, CareOfferingFields: update})
	require.Error(t, err)
	afterFailure, findErr := module.FindCareOffering(ctx, created.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "Rollback Create", afterFailure.Name)
	assert.Equal(t, []int64{stable.ID}, afterFailure.AutoAddTriggerOfferingIDs)

	update.Name = "Retried"
	update.AutoAddTriggerOfferingIDs = nil
	updated, err := module.UpdateCareOffering(ctx, careplan.UpdateCareOffering{ID: created.ID, CareOfferingFields: update})
	require.NoError(t, err)
	assert.Equal(t, "Retried", updated.Name)
	assert.Empty(t, updated.AutoAddTriggerOfferingIDs)

	log.mu.Lock()
	defer log.mu.Unlock()
	require.NotEmpty(t, log.seen)
	foundFailure := false
	for _, observation := range log.seen {
		if observation.Operation == "create_care_offering" && observation.Err != nil {
			foundFailure = true
			assert.Positive(t, observation.Stats.Queries)
		}
	}
	assert.True(t, foundFailure, "the failed compound write must be observable under its stable operation")
}

func TestOfferingChangeDuplicateAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	studentID, childID, accountID := insertOfferingChangeFixture(t, db, ctx, testpkg.Tenant(t))
	input := careplan.OfferingChangeRequest{
		StudentID: studentID, RequestChildID: childID, SubmittedBy: accountID,
		Payload: json.RawMessage(`{"offerings":[]}`), EffectiveFrom: "2030-09-01", Status: "pending",
	}
	created, err := module.CreateOfferingChange(ctx, input)
	require.NoError(t, err)
	_, err = module.CreateOfferingChange(ctx, input)
	require.ErrorIs(t, err, careplan.ErrOfferingChangeAlreadyOpen)
	assert.Equal(t, "already_pending", careplan.ErrorCode(err))

	otherTenantID := testpkg.UniqueTestTenantID(t)
	otherCtx := tenantContext(t, db, otherTenantID)
	_, err = module.FindOfferingChange(otherCtx, created.ID, false)
	require.ErrorIs(t, err, careplan.ErrOfferingChangeNotFound)
	foreign, err := module.ListOfferingChanges(otherCtx, careplan.OfferingChangeFilter{})
	require.NoError(t, err)
	assert.Empty(t, foreign)
	require.ErrorIs(t, module.DecideOfferingChange(otherCtx, careplan.DecideOfferingChange{ID: created.ID, Status: "rejected"}), careplan.ErrOfferingChangeNotPending)
}

func insertPhase(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64, name string) int64 {
	t.Helper()
	var id int64
	err := tenant.WithinTenant(ctx, mustTenantID(t, tenantID), func(txCtx context.Context) error {
		return transactionDB(t, txCtx).NewRaw(`INSERT INTO enrollment.phases (tenant_id, name, service_start_date, service_end_date) VALUES (?, ?, '2030-08-01', '2031-07-31') RETURNING id`, tenantID, name).Scan(txCtx, &id)
	})
	require.NoError(t, err)
	return id
}

func insertOfferingChangeFixture(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64) (int64, int64, int64) {
	t.Helper()
	student := testpkg.CreateTestStudent(t, db, "Care", "Change", "2a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("care-change-%d@example.test", testpkg.UniqueSuffix()))
	var childID int64
	err := tenant.WithinTenant(ctx, mustTenantID(t, tenantID), func(txCtx context.Context) error {
		tx := transactionDB(t, txCtx)
		var phaseID, requestID int64
		if err := tx.NewRaw(`INSERT INTO enrollment.phases (tenant_id, name, service_start_date, service_end_date) VALUES (?, ?, '2030-08-01', '2031-07-31') RETURNING id`, tenantID, fmt.Sprintf("Care change %d", testpkg.UniqueSuffix())).Scan(txCtx, &phaseID); err != nil {
			return err
		}
		if err := tx.NewRaw(`INSERT INTO enrollment.requests (tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token) VALUES (?, ?, 'Care', 'Guardian', ?, ?) RETURNING id`, tenantID, phaseID, account.Email, fmt.Sprintf("care-change-%d", testpkg.UniqueSuffix())).Scan(txCtx, &requestID); err != nil {
			return err
		}
		return tx.NewRaw(`INSERT INTO enrollment.request_children (tenant_id, request_id, first_name, last_name, date_of_birth, status, created_student_id) VALUES (?, ?, 'Care', 'Change', '2020-01-01', 'approved', ?) RETURNING id`, tenantID, requestID, student.ID).Scan(txCtx, &childID)
	})
	require.NoError(t, err)
	return student.ID, childID, account.ID
}

func transactionDB(t *testing.T, ctx context.Context) bun.IDB {
	t.Helper()
	raw, ok := tenant.TransactionFromContext(ctx)
	require.True(t, ok)
	switch tx := raw.(type) {
	case bun.Tx:
		return tx
	case *bun.Tx:
		return tx
	default:
		t.Fatalf("unexpected transaction %T", raw)
		return nil
	}
}

func mustTenantID(t *testing.T, value int64) tenant.TenantID {
	t.Helper()
	id, err := tenant.NewTenantID(value)
	require.NoError(t, err)
	return id
}
