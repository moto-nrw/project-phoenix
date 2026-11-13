package owners

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	deliverycompose "github.com/moto-ogs/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/delivery/mail"
	peoplecompose "github.com/moto-ogs/project-phoenix/modules/peopledirectory/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Decision C keeps the race, but its resulting pending entry must reach a
// terminal state through the real dispatcher and then the worker's retention.
func TestOrphanReleaseAccountPushStartsRetentionWhenWorkerMarksSent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, context.Background(), db)
	database := testpkg.NewDatabase(t, db)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Orphan release", Slug: fmt.Sprintf("orphan-release-%d", testpkg.Tenant(t)),
	}).ID
	testpkg.OwnTraegerDocuments(t, db, org)
	school := testpkg.CreateTestSchoolInOrganization(t, db, org, "Erased demo school")
	account := testpkg.CreateTestAccount(t, db, "erased-release@example.org").ID
	var document int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.traeger_documents
		(organization_id, title, file_name, content_type, size_bytes, content)
		VALUES (?, 'Retention', 'test.pdf', 'application/pdf', 8, '%PDF-1.4'::bytea) RETURNING id`, org).Scan(ctx, &document))
	var releases []int64
	for _, scope := range []any{school, nil} {
		var release int64
		require.NoError(t, db.NewRaw(`INSERT INTO platform.traeger_document_releases
			(organization_id, document_id, audience, school_id)
			VALUES (?, ?, 'parents', ?) RETURNING id`, org, document, scope).Scan(ctx, &release))
		releases = append(releases, release)
	}
	testpkg.SoftDeleteTestSchool(t, db, school)
	_, err := db.ExecContext(ctx, `UPDATE auth.accounts SET email = 'demo-besuch-' || id || '@example.invalid',
		username = NULL, active = FALSE, password_hash = NULL, personal_data_erased_at = NOW() WHERE id = ?`, account)
	require.NoError(t, err)

	runtime, err := deliverycompose.NewModule(deliverycompose.ModuleDependencies{
		DB: db, People: documentPushDisplays{}, Observe: func(delivery.Observation) {},
	})
	require.NoError(t, err)
	people, err := NewPeopleDirectory(database)
	require.NoError(t, err)
	identity := NewPeopleIdentityAccess(database, nil)
	organizations, err := NewOrganizationTenancy(database)
	require.NoError(t, err)
	notices := BuildParentDocumentNotices(database, RootEnvironment{Logger: slog.Default()},
		ComposeParentPortalReads(database, people, identity, nil), organizations, identity,
		PeopleRows{Repositories: peoplecompose.Repositories{
			Students: NewStudentRepository(database, nil, nil), GuardianProfiles: NewGuardianProfileRepository(database),
		}}, DeliveryNotifications{}, runtime.Module)
	transport := &documentPushTransport{}
	worker, err := buildDeliveryWorker(database, RootEnvironment{Logger: slog.Default()}, nil,
		mail.Email{}, nil, DeliveryRuntime{Runtime: runtime}, nil, nil, nil, cancelledModules{}, notices, transport, nil)
	require.NoError(t, err)
	for _, release := range releases {
		require.NoError(t, (ParentPortalRuntime{}).WithinTraeger(ctx, org, func(txCtx context.Context) error {
			return (ParentReleaseFanOut{Delivery: runtime.Module}).EnqueueReleaseAccounts(txCtx, org, release, []int64{account})
		}))
	}
	var states []string
	require.NoError(t, db.NewRaw(`SELECT status FROM platform.push_outbox WHERE organization_id = ? ORDER BY id`, org).Scan(ctx, &states))
	require.Equal(t, []string{"pending", "pending"}, states)

	queries := testpkg.CaptureQueriesForContext(t, db)
	ctx = queries.Context(ctx)
	started := time.Now()
	processed, err := worker.RunOnce(ctx, 25, 6)
	require.NoError(t, err)
	require.Equal(t, 2, processed)
	var sent []time.Time
	require.NoError(t, db.NewRaw(`SELECT sent_at FROM platform.push_outbox
		WHERE organization_id = ? AND status = 'sent' ORDER BY id`, org).Scan(ctx, &sent))
	require.Len(t, sent, 2, "the real route and NotifyTraegerDocumentReleaseAccount finish even without children")
	assert.NotEmpty(t, queries.Selects("auth.active_school_ids_for_account"), "the dispatcher actually resolves the orphan's children")
	for _, at := range sent {
		assert.WithinRange(t, at, started, time.Now())
	}
	assert.Zero(t, transport.calls, "an orphan has no device recipient")

	// Old enqueue/update timestamps must not start this retention before sent_at.
	_, err = db.ExecContext(ctx, `UPDATE platform.push_outbox SET created_at = NOW() - INTERVAL '31 days',
		updated_at = NOW() - INTERVAL '31 days' WHERE organization_id = ?`, org)
	require.NoError(t, err)
	_, err = worker.RunOnce(ctx, 25, 6)
	require.NoError(t, err)
	var count int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.push_outbox WHERE organization_id = ?`, org).Scan(ctx, &count))
	require.Equal(t, 2, count, "fresh sent_at keeps both entries")
	_, err = db.ExecContext(ctx, `UPDATE platform.push_outbox SET sent_at = NOW() - INTERVAL '31 days'
		WHERE organization_id = ?`, org)
	require.NoError(t, err)
	_, err = worker.RunOnce(ctx, 25, 6)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.push_outbox WHERE organization_id = ?`, org).Scan(ctx, &count))
	assert.Zero(t, count, "the worker removes both orphan entries after their sent_at retention expires")
}
