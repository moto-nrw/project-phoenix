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

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type reviewRecordingBroadcaster struct {
	events []realtime.Event
}

func (b *reviewRecordingBroadcaster) BroadcastToGroup(_ int64, _ string, _ realtime.Event) error {
	return nil
}

func (b *reviewRecordingBroadcaster) BroadcastToTenant(_ int64, event realtime.Event) error {
	b.events = append(b.events, event)
	return nil
}

func (b *reviewRecordingBroadcaster) BroadcastToAll(_ realtime.Event) error {
	return nil
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

func TestMasterDataReview_ApproveAppliesNameChange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	var decided *userModels.StudentDataChangeRequest
	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		d, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		decided = d
		return e
	})
	require.NoError(t, err)
	assert.Equal(t, userModels.DataChangeStatusApproved, decided.Status)
	require.NotNil(t, decided.AppliedAt)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Maximilian", person.FirstName)
}

func TestMasterDataReview_ApproveAppliesOtherPersonFields(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	lastName := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)
	birthday := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "birthday", `null`, `"2017-12-24"`)

	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
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

func TestMasterDataReview_ConcurrentPersonFieldApprovalsDoNotOverwrite(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	firstName := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)
	lastName := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, row := range []*userModels.StudentDataChangeRequest{firstName, lastName} {
		wg.Add(1)
		go func(idx int, req *userModels.StudentDataChangeRequest) {
			defer wg.Done()
			errs[idx] = tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
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
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	approvals := []bool{true, false}
	for i, approve := range approvals {
		wg.Add(1)
		go func(idx int, approve bool) {
			defer wg.Done()
			errs[idx] = tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
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
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, nil)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)
	insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)

	var items []*userService.MasterDataReviewItem
	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var e error
		items, e = svc.ListPending(txCtx)
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
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		items, e := svc.ListPending(txCtx)
		require.NoError(t, e)
		assert.Empty(t, items)

		_, e = svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: 0, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewNotFound)

	err = tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: 999_999_999, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewNotFound)
}

func TestMasterDataReview_ApproveAppliesDepartureModes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetDeparture, "allowed_departure_modes", `{}`, `{"mon":["bus"],"wed":["pickup"]}`)

	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true, ReviewedBy: chain.AccountID})
		return e
	})
	require.NoError(t, err)

	student, err := repos.Student.FindByID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, []userModels.DepartureMode{userModels.DepartureBus}, student.AllowedDepartureModes[userModels.PickupDayMonday])
	assert.Equal(t, []userModels.DepartureMode{userModels.DeparturePickup}, student.AllowedDepartureModes[userModels.PickupDayWednesday])
}

func TestMasterDataReview_StalePersonApprovalConflicts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	person.FirstName = "StaffEdit"
	require.NoError(t, repos.Person.Update(context.Background(), person))

	err = tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewStaleValue)

	person, err = repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "StaffEdit", person.FirstName)
}

func TestMasterDataReview_StaleDepartureApprovalConflicts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetDeparture, "allowed_departure_modes", `{}`, `{"mon":["bus"]}`)

	student, err := repos.Student.FindByID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	student.AllowedDepartureModes = userModels.AllowedDepartureModes{
		userModels.PickupDayTuesday: []userModels.DepartureMode{userModels.DeparturePickup},
	}
	require.NoError(t, repos.Student.Update(context.Background(), student))

	err = tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
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
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	broadcaster := &reviewRecordingBroadcaster{}
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default(), broadcaster)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true, ReviewedBy: chain.AccountID})
		assert.Empty(t, broadcaster.events, "broadcast must wait until the transaction commits")
		return e
	})
	require.NoError(t, err)
	require.Len(t, broadcaster.events, 1)
	assert.Equal(t, realtime.EventStudentUpdated, broadcaster.events[0].Type)
}

func TestMasterDataReview_RejectLeavesRecordUnchanged(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: false, Reason: "Bitte Nachweis"})
		return e
	})
	require.NoError(t, err)

	person, err := repos.Person.FindByID(context.Background(), chain.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Felix", person.FirstName, "rejected change must not touch the record")
}

func TestMasterDataReview_DecideNonPendingRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	row := insertPendingChange(t, db, repos, chain, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	// First decision approves; the second must fail as not-pending.
	err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	require.NoError(t, err)

	err = tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
		return e
	})
	assert.ErrorIs(t, err, userService.ErrReviewNotPending)
}

func TestMasterDataReview_ApproveInvalidRowsRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()
	repos := repositories.NewFactory(db)
	svc := userService.NewMasterDataReviewService(repos.StudentDataChangeRequest, repos.Student, repos.Person, slog.Default())

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
			defer testpkg.CleanupParentGuardianChain(t, db, chain)

			row := insertPendingChange(t, db, repos, chain, tt.target, tt.field, `null`, tt.value)
			err := tenant.WithTenantTx(context.Background(), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
				_, e := svc.Decide(txCtx, userService.MasterDataReviewDecideInput{RequestID: row.ID, Approve: true})
				return e
			})
			assert.ErrorIs(t, err, tt.want)
		})
	}
}
