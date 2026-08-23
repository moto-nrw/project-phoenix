package users_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// reviewNotesSettings stubs the parentmessaging settings resolver so the emitter
// treats messaging as enabled (or not) for the decision-pill tests.
type reviewNotesSettings struct{ enabled bool }

func (s reviewNotesSettings) ResolveBoolForTenant(context.Context, int64, string) (bool, error) {
	return s.enabled, nil
}

// authorizedCtx stamps admin permissions so the per-child write gate in
// ListPending/Decide short-circuits. These tests exercise decide LOGIC (apply,
// staleness, concurrency), not authorization; the scope gate itself is covered
// by TestMasterDataReview_ScopedToSupervisedChildren.
func authorizedCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, jwt.CtxPermissions, []string{"admin:*"})
}

func insertPendingChange(t *testing.T, db *bun.DB, repos *repositories.Factory, c testpkg.ParentChain, target, field string, oldVal, newVal string) *userModels.StudentDataChangeRequest {
	t.Helper()
	row := &userModels.StudentDataChangeRequest{
		StudentID:   c.StudentID,
		SubmittedBy: c.AccountID,
		Target:      target,
		FieldKey:    field,
		OldValue:    json.RawMessage(oldVal),
		NewValue:    json.RawMessage(newVal),
		Status:      userModels.DataChangeStatusPending,
	}
	row.SetTenantID(c.TenantID)
	err := tenant.WithTenantTx(context.Background(), db, c.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return repos.StudentDataChangeRequest.Create(txCtx, row)
	})
	require.NoError(t, err)
	return row
}

// TestMasterDataReview_ScopedToWritableChildren proves the per-child write
// gate: a caller with users:update but neither admin permissions nor a staff
// record (the service is wired without a user context here, as a guest or
// guardian account resolves) cannot see the request in the scoped queue and
// cannot decide it — while the admin path sees and decides the same request.
func TestMasterDataReview_ScopedToWritableChildren(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	denyBase := context.WithValue(context.Background(), jwt.CtxPermissions, []string{"users:update"})
	err := tenant.WithTenantTx(denyBase, db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, _, e := svc.ListPending(txCtx, modelBase.RequestQueueFilters{})
		require.NoError(t, e)
		assert.Empty(t, items, "a caller who cannot write the child must not see its request in the queue")
		_, e = svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewForbidden)

	err = tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, _, e := svc.ListPending(txCtx, modelBase.RequestQueueFilters{})
		require.NoError(t, e)
		require.Len(t, items, 1)
		_, e = svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: false, Reason: "closing", ReviewedBy: chain.AccountID})
		return e
	})
	require.NoError(t, err)
}

func TestMasterDataReview_ApproveAppliesNameChange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	var decided *userService.MasterDataReviewItem
	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		d, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		decided = d
		return e
	})
	require.NoError(t, err)
	assert.Equal(t, userModels.DataChangeStatusApproved, decided.Request.Status)
	require.NotNil(t, decided.Request.AppliedAt)
	assert.Equal(t, "Maximilian", decided.FirstName)
	assert.Equal(t, "Schneider", decided.LastName)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Maximilian", person.FirstName)
}

func TestMasterDataReview_ApproveAppliesOtherPersonFields(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	lastName := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)
	birthday := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "birthday", `null`, `"2017-12-24"`)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		if _, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: lastName.ID, Approve: true}); e != nil {
			return e
		}
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: birthday.ID, Approve: true})
		return e
	})
	require.NoError(t, err)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Müller", person.LastName)
	require.NotNil(t, person.Birthday)
	assert.Equal(t, "2017-12-24", person.Birthday.String())
}

func TestMasterDataReview_ApproveAppliesSchoolClass(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	audit := userService.NewStudentAuditService(repos.StudentFieldEdit, slog.Default())
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, audit, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetStudent, "school_class", `"1a"`, `"2b"`)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{
			RequestID:  row.ID,
			Approve:    true,
			ReviewedBy: chain.AccountID,
		})
		return decideErr
	})
	require.NoError(t, err)

	student, err := repos.Student.FindByID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, "2b", student.SchoolClass)
}

func TestMasterDataReview_ConcurrentPersonFieldApprovalsDoNotOverwrite(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	firstName := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)
	lastName := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, row := range []*userModels.StudentDataChangeRequest{firstName, lastName} {
		wg.Add(1)
		go func(idx int, req *userModels.StudentDataChangeRequest) {
			defer wg.Done()
			errs[idx] = tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
				_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: req.ID, Approve: true})
				return e
			})
		}(i, row)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Max", person.FirstName)
	assert.Equal(t, "Müller", person.LastName)
}

func TestMasterDataReview_ConcurrentDecisionsKeepStatusAndRecordConsistent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	approvals := []bool{true, false}
	for i, approve := range approvals {
		wg.Add(1)
		go func(idx int, approve bool) {
			defer wg.Done()
			errs[idx] = tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
				_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: approve})
				return e
			})
		}(i, approve)
	}
	wg.Wait()

	successes := 0
	conflicts := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, userService.ErrReviewNotPending):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent decision error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	ctx := tenant.WithTenantID(context.Background(), chain.TenantID)
	decided, err := repos.StudentDataChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	switch decided.Status {
	case userModels.DataChangeStatusApproved:
		assert.Equal(t, "Max", person.FirstName)
	case userModels.DataChangeStatusRejected:
		assert.Equal(t, "Felix", person.FirstName)
	default:
		t.Fatalf("unexpected final status %q", decided.Status)
	}
}

func TestMasterDataReview_ListPendingEnrichesStudentNames(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, nil)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)
	insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)

	var items []*userService.MasterDataReviewItem
	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var e error
		items, _, e = svc.ListPending(txCtx, modelBase.RequestQueueFilters{})
		return e
	})
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		assert.Equal(t, "Felix", item.FirstName)
		assert.Equal(t, "Schneider", item.LastName)
		assert.Equal(t, chain.StudentID, item.Request.StudentID)
	}
}

func TestMasterDataReview_ListPendingEmptyAndInvalidRequestID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, _, e := svc.ListPending(txCtx, modelBase.RequestQueueFilters{})
		require.NoError(t, e)
		assert.Empty(t, items)

		_, e = svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: 0, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewNotFound)

	err = tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: 999_999_999, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewNotFound)
}

func TestMasterDataReview_ApproveAppliesDepartureModes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	audit := userService.NewStudentAuditService(repos.StudentFieldEdit, slog.Default())
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, audit, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetDeparture, "allowed_departure_modes", `{}`, `{"mon":["bus"],"wed":["pickup"]}`)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true, ReviewedBy: chain.AccountID})
		return e
	})
	require.NoError(t, err)

	student, err := repos.Student.FindByID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, []userModels.DepartureMode{userModels.DepartureBus}, student.AllowedDepartureModes[userModels.PickupDayMonday])
	assert.Equal(t, []userModels.DepartureMode{userModels.DeparturePickup}, student.AllowedDepartureModes[userModels.PickupDayWednesday])

	history, err := audit.GetChangeHistory(tenant.WithTenantID(context.Background(), chain.TenantID), chain.StudentID)
	require.NoError(t, err)
	require.NotEmpty(t, history)
	var departureEditFound bool
	for _, edit := range history {
		if edit.FieldName == "departure_days" {
			departureEditFound = true
			assert.Equal(t, chain.AccountID, edit.EditedBy)
		}
	}
	assert.True(t, departureEditFound)
}

func TestMasterDataReview_StalePersonApprovalConflicts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	person.FirstName = "StaffEdit"
	require.NoError(t, repos.Person.Update(context.Background(), person))

	err = tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewStaleValue)

	person, err = repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "StaffEdit", person.FirstName)
}

func TestMasterDataReview_StaleDepartureApprovalConflicts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetDeparture, "allowed_departure_modes", `{}`, `{"mon":["bus"]}`)

	student, err := repos.Student.FindByID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	student.AllowedDepartureModes = userModels.AllowedDepartureModes{
		userModels.PickupDayTuesday: []userModels.DepartureMode{userModels.DeparturePickup},
	}
	require.NoError(t, repos.Student.Update(context.Background(), student))

	err = tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewStaleValue)

	student, err = repos.Student.FindByID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, []userModels.DepartureMode{userModels.DeparturePickup}, student.AllowedDepartureModes[userModels.PickupDayTuesday])
	assert.Empty(t, student.AllowedDepartureModes[userModels.PickupDayMonday])
}

func TestMasterDataReview_ApprovalBroadcastsStudentUpdatedAfterCommit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default(), broadcaster)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true, ReviewedBy: chain.AccountID})
		assert.Empty(t, broadcaster.CallsByMethod("tenant"), "broadcast must wait until the transaction commits")
		return e
	})
	require.NoError(t, err)
	tenantCalls := broadcaster.CallsByMethod("tenant")
	require.Len(t, tenantCalls, 1)
	assert.Equal(t, realtime.EventStudentUpdated, tenantCalls[0].Event.Type)
}

func TestMasterDataReview_RejectLeavesRecordUnchanged(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: false, Reason: "Bitte Nachweis"})
		return e
	})
	require.NoError(t, err)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Felix", person.FirstName, "rejected change must not touch the record")
}

func TestMasterDataReview_DecideNonPendingRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	// First decision approves; the second must fail as not-pending.
	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	require.NoError(t, err)

	err = tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewNotPending)
}

func TestMasterDataReview_ApproveInvalidRowsRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default())

	tests := []struct {
		name   string
		target string
		field  string
		value  string
		want   error
	}{
		{name: "invalid target", target: userModels.DataChangeTargetStudent, field: "health_info", value: `"x"`, want: userService.ErrReviewInvalidTarget},
		{name: "invalid person value", target: userModels.DataChangeTargetPerson, field: "first_name", value: `123`, want: userService.ErrReviewInvalidValue},
		{name: "invalid birthday", target: userModels.DataChangeTargetPerson, field: "birthday", value: `"bad-date"`, want: userService.ErrReviewInvalidValue},
		{name: "invalid departure field", target: userModels.DataChangeTargetDeparture, field: "pickup_status", value: `{}`, want: userService.ErrReviewInvalidTarget},
		{name: "invalid departure value", target: userModels.DataChangeTargetDeparture, field: "allowed_departure_modes", value: `123`, want: userService.ErrReviewInvalidValue},
		{name: "unknown departure mode", target: userModels.DataChangeTargetDeparture, field: "allowed_departure_modes", value: `{"mon":["spaceship"]}`, want: userService.ErrReviewInvalidValue},
		{name: "unsupported accompanied mode", target: userModels.DataChangeTargetDeparture, field: "allowed_departure_modes", value: `{"mon":["accompanied"]}`, want: userService.ErrReviewInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := testpkg.CreateTestParentGuardianChain(t, db)

			row := insertPendingChange(t, db, repos, chain, tt.target, tt.field, `null`, tt.value)
			err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
				_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
				return e
			})
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

// findRequestStatusPill returns the request_status pill written into the
// (student, guardian) thread by the emitter after a master-data decision, or
// nil when none was written. Runs inside the tenant tx so RLS admits the rows.
func findRequestStatusPill(t *testing.T, db *bun.DB, repos *repositories.Factory, c testpkg.ParentChain) *userModels.ParentMessage {
	t.Helper()
	var pill *userModels.ParentMessage
	err := tenant.WithTenantTx(context.Background(), db, c.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		thread, ferr := repos.ParentMessageThread.FindByStudentGuardian(txCtx, c.StudentID, c.AccountID)
		if ferr != nil || thread == nil {
			return ferr
		}
		msgs, merr := repos.ParentMessage.ListByThread(txCtx, thread.ID, 50)
		if merr != nil {
			return merr
		}
		for _, m := range msgs {
			if m.EventType == userModels.ParentMessageEventRequestStatus {
				pill = m
			}
		}
		return nil
	})
	require.NoError(t, err)
	return pill
}

// TestMasterDataReview_ApproveEmitsDecisionPill wires a REAL emitter (not the
// nil the logic-only tests use) so the after-commit decision pill actually
// lands: approving a master-data change must drop a "bestätigt" request_status
// pill, stamped staff/master_data, into the submitting guardian's thread.
func TestMasterDataReview_ApproveEmitsDecisionPill(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	broadcaster := testpkg.NewRecordingBroadcaster()
	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
		reviewNotesSettings{enabled: true}, broadcaster, slog.Default())
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, emitter, nil, slog.Default(), broadcaster)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true, ReviewedBy: chain.AccountID})
		return e
	})
	require.NoError(t, err)

	pill := findRequestStatusPill(t, db, repos, chain)
	require.NotNil(t, pill, "an approved master-data request must drop a decision pill")
	assert.Equal(t, userModels.ParentMessageRequestStatusDone, pill.RequestStatus)
	assert.Equal(t, "Anfrage bestätigt, Stammdaten übernommen", pill.Body)
	assert.Equal(t, userModels.ParentMessageSenderStaff, pill.EventActorKind)
}

// TestMasterDataReview_RejectEmitsPillWithReason covers the rejection branch of
// the decision pill: a rejected request carries the "abgelehnt: <reason>" body
// and the rejected status, never the applied one.
func TestMasterDataReview_RejectEmitsPillWithReason(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	broadcaster := testpkg.NewRecordingBroadcaster()
	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
		reviewNotesSettings{enabled: true}, broadcaster, slog.Default())
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, emitter, nil, slog.Default(), broadcaster)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)

	err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: false, Reason: "Nachweis fehlt", ReviewedBy: chain.AccountID})
		return e
	})
	require.NoError(t, err)

	pill := findRequestStatusPill(t, db, repos, chain)
	require.NotNil(t, pill, "a rejected request must drop a decision pill")
	assert.Equal(t, userModels.ParentMessageRequestStatusRejected, pill.RequestStatus)
	assert.Equal(t, "Anfrage abgelehnt: Nachweis fehlt", pill.Body)

	// The rejection must NOT have touched the live record.
	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Schneider", person.LastName)
}

// TestMasterDataReview_CompanionEventOnlyOnEffectiveChange pins WHO an approval
// wakes: student_companions_changed makes every open "läuft mit" editor in the
// school mark itself stale and refuse to save, so it may only fire when the
// approval actually reconciled a link. The request's TARGET is not that
// question — a departure approval that drops no link (because the child has
// none, or because the affected weekdays keep allowing "Anderes Kind") must stay
// silent, or routine parent requests cost unrelated staff their unsaved edits.
func TestMasterDataReview_CompanionEventOnlyOnEffectiveChange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	bc := testpkg.NewRecordingBroadcaster()
	svc := userService.NewMasterDataReviewServiceWithAudit(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil, nil, nil, slog.Default(), bc)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	approve := func(t *testing.T, row *userModels.StudentDataChangeRequest) {
		t.Helper()
		bc.Reset()
		err := tenant.WithTenantTx(authorizedCtx(context.Background()), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true, ReviewedBy: chain.AccountID})
			return e
		})
		require.NoError(t, err)
	}

	t.Run("a departure approval without links stays silent", func(t *testing.T) {
		row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetDeparture,
			"allowed_departure_modes", `{}`, `{"mon":["bus"]}`)
		approve(t, row)
		assert.False(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"a plan change that trims no link changed no Laufgemeinschaft to announce")
		assert.True(t, bc.HasEventType(realtime.EventStudentUpdated),
			"the ordinary student_updated invalidation still has to fire")
	})

	t.Run("a departure approval that drops a link announces it", func(t *testing.T) {
		linkCompanionOnTuesday(t, db, repos, chain)

		// The approval is refused as stale unless it names the plan it starts
		// from, so read the live one the linking left behind.
		student, err := repos.Student.FindByID(testpkg.TenantContext(chain.TenantID), chain.StudentID)
		require.NoError(t, err)
		current, err := json.Marshal(student.AllowedDepartureModes)
		require.NoError(t, err)
		narrowed := userModels.AllowedDepartureModes{}
		for day, allowed := range student.AllowedDepartureModes {
			narrowed[day] = allowed
		}
		narrowed[userModels.PickupDayTuesday] = []userModels.DepartureMode{userModels.DepartureBus}
		requested, err := json.Marshal(narrowed)
		require.NoError(t, err)

		row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetDeparture,
			"allowed_departure_modes", string(current), string(requested))
		approve(t, row)
		assert.True(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"Tuesday no longer allows 'Anderes Kind', so the link is gone from the partner's card too")
	})
}

// linkCompanionOnTuesday gives the chain's child a Laufgemeinschaft partner on
// Tuesday and returns the partner's id. Both children keep their own free-text
// "mit wem" note, so dropping the link later does not strand the partner (a
// different rule, with its own test).
func linkCompanionOnTuesday(t *testing.T, db *bun.DB, repos *repositories.Factory, chain testpkg.ParentChain) int64 {
	t.Helper()
	ctx := testpkg.TenantContext(chain.TenantID)

	partner := testpkg.CreateTestStudent(t, db, "ReviewCompanion", "Partner", "1a")
	t.Cleanup(func() {
		_, err := db.NewDelete().
			TableExpr("users.student_companions").
			Where("student_low_id IN (?, ?) OR student_high_id IN (?, ?)",
				chain.StudentID, partner.ID, chain.StudentID, partner.ID).
			Exec(context.Background())
		if err != nil {
			t.Logf("Warning: failed to cleanup users.student_companions: %v", err)
		}
	})

	for _, studentID := range []int64{partner.ID, chain.StudentID} {
		student, err := repos.Student.FindByID(ctx, studentID)
		require.NoError(t, err)
		modes := userModels.AllowedDepartureModes{}
		for day, allowed := range student.AllowedDepartureModes {
			modes[day] = allowed
		}
		modes[userModels.PickupDayTuesday] = []userModels.DepartureMode{userModels.DepartureAccompanied}
		student.AllowedDepartureModes = modes
		student.DepartureDays = nil
		student.BusDays = nil
		student.PickupDays = nil
		note := "Nachbarskind"
		student.DepartureCompanionNote = &note
		require.NoError(t, repos.Student.Update(ctx, student))
	}

	edge, err := userModels.NewStudentCompanion(chain.StudentID, partner.ID, 2)
	require.NoError(t, err)
	require.NoError(t, repos.StudentCompanion.ReplaceForStudent(ctx, chain.StudentID, []*userModels.StudentCompanion{edge}))
	return partner.ID
}
