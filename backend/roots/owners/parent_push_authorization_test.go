package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	deliverycompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	mail "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	deliveryruntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	billingcompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type documentPushTransport struct {
	calls int
	err   error
}

func (p *documentPushTransport) SendPush(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	p.calls++
	return delivery.ProviderResult{StatusCode: 201}, p.err
}

type documentPushDisplays struct{}

func (documentPushDisplays) ResolveGuardianDisplays(context.Context, []int64) ([]delivery.GuardianDisplay, error) {
	return nil, nil
}

type billingPushFixture struct {
	db                          *bun.DB
	ctx                         context.Context
	chain                       testpkg.ParentChain
	organization, run, document int64
	runtime                     *deliverycompose.Runtime
	worker                      *deliveryruntime.Worker
	transport                   *documentPushTransport
	observations                []string
}

func newBillingPushFixture(t *testing.T) *billingPushFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	f := &billingPushFixture{db: db, ctx: testpkg.WithPackageTenantRuntime(testpkg.OwnCtx(t)), chain: testpkg.CreateTestParentGuardianChain(t, db), transport: &documentPushTransport{}}
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, f.chain.TenantID).Scan(f.ctx, &f.organization))
	testpkg.OwnTraegerContributionRules(t, db, f.organization)
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id,service,holiday_period_id,period_name,service_from,service_until,status,released_at,issued_on,document_count,total_cents)
 VALUES (?,'holiday',8,'Test','2026-10-12','2026-10-23','released',NOW(),'2026-10-01',1,4500) RETURNING id`, f.organization).Scan(f.ctx, &f.run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id,run_id,service,number,number_year,number_sequence,issued_on,holiday_period_id,holiday_registration_id,recipient_first_name,recipient_last_name,recipient_email,service_from,service_until,total_cents)
 VALUES (?,?,'holiday','TEST-1',0,1,'2026-10-01',8,100,'Test','Family',?,'2026-10-12','2026-10-16',4500) RETURNING id`, f.organization, f.run, f.chain.Email).Scan(f.ctx, &f.document))
	_, err := db.ExecContext(f.ctx, `INSERT INTO parent_billing.document_letters (organization_id,document_id,run_id,filename,pdf) VALUES (?,?,?,'test.pdf',?)`, f.organization, f.document, f.run, []byte("%PDF-test"))
	require.NoError(t, err)
	_, err = db.ExecContext(f.ctx, `INSERT INTO parent_billing.document_mailings (organization_id,run_id,document_id,delivery_id,recipient_email) VALUES (?,?,?,?,?)`, f.organization, f.run, f.document, f.document+900000, f.chain.Email)
	require.NoError(t, err)
	f.runtime, err = deliverycompose.NewModule(deliverycompose.ModuleDependencies{DB: db, People: documentPushDisplays{}, Observe: func(o delivery.Observation) {
		text := o.Operation
		if o.Err != nil {
			text += ":" + o.Err.Error()
		}
		f.observations = append(f.observations, text)
	}})
	require.NoError(t, err)
	f.worker, err = buildDeliveryWorker(testpkg.NewDatabase(t, db), RootEnvironment{Logger: slog.Default()}, nil,
		mail.Email{}, nil, DeliveryRuntime{Runtime: f.runtime}, nil, nil, nil, cancelledModules{}, nil, f.transport)
	require.NoError(t, err)
	return f
}

// fanout persists device snapshots through the same durable Web Push channel
// as the parent document notifier. No transport is invoked until RunOnce.
func (f *billingPushFixture) fanout(t *testing.T, n int) []int64 {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := f.db.ExecContext(f.ctx, `INSERT INTO iot.push_subscriptions (tenant_id,account_id,portal,endpoint,p256dh,auth) VALUES (?,?,'parent',?,'p256dh-key','auth-key')`, f.chain.TenantID, f.chain.AccountID, fmt.Sprintf("https://fcm.googleapis.com/fcm/send/test-%d-%d", f.chain.AccountID, i))
		require.NoError(t, err)
	}
	channel, err := deliverycompose.NewTenantDurableWebPushChannel(testpkg.TenantRuntime(t, f.db), f.db, deliverycompose.NewPushSubscriptionRepository(f.db), deliverycompose.VAPIDConfig{PublicKey: "pub", PrivateKey: "priv", Subscriber: "mailto:test@example.org"}, deliverycompose.NewDurablePushOutbox(f.runtime.Module), slog.Default())
	require.NoError(t, err)
	durable, ok := channel.(interface {
		DeliverDurably(context.Context, deliverycompose.NotificationEvent) (int, error)
	})
	require.True(t, ok)
	require.NoError(t, (ParentPortalRuntime{}).WithinTenant(f.ctx, f.chain.TenantID, func(ctx context.Context) error {
		count, err := durable.DeliverDurably(ctx, deliverycompose.NotificationEvent{Type: parentbilling.PushKindBillingDocument, IdempotencyKey: fmt.Sprintf("document:%d", f.document), RelatedType: parentbilling.PushRelatedBillingDocument, RelatedID: f.document,
			Audience: deliverycompose.NotificationAudience{TenantID: f.chain.TenantID, Scope: "guardian", GuardianAccountIDs: []int64{f.chain.AccountID}, StudentIDs: []int64{f.chain.StudentID}}, Title: "Document", Body: "New document", DeepLink: "/documents"})
		require.Equal(t, n, count)
		return err
	}))
	var ids []int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM platform.push_outbox WHERE tenant_id = ? ORDER BY id`, f.chain.TenantID).Scan(f.ctx, &ids))
	require.Len(t, ids, n)
	assert.Zero(t, f.transport.calls)
	return ids
}

func (f *billingPushFixture) runWorker(t *testing.T) {
	t.Helper()
	_, err := f.worker.RunOnce(f.ctx, 100, 6)
	require.NoError(t, err)
}
func (f *billingPushFixture) states(t *testing.T) []string {
	t.Helper()
	var states []string
	require.NoError(t, f.db.NewRaw(`SELECT status FROM platform.push_outbox WHERE tenant_id = ? ORDER BY id`, f.chain.TenantID).Scan(f.ctx, &states))
	return states
}

func TestBillingDevicePushRechecksCurrentObjectAndExactRelationship(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"valid", "right revoked", "portal access revoked", "account inactive", "cancelled invoice", "letter removed", "recipient changed", "retry after error"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			f := newBillingPushFixture(t)
			f.fanout(t, 2)
			if scenario == "retry after error" {
				f.transport.err = errors.New("temporary push transport failure")
				f.runWorker(t)
				assert.Equal(t, []string{"pending", "pending"}, f.states(t))
				assert.Equal(t, 2, f.transport.calls)
				f.transport.err = nil
				_, err := f.db.ExecContext(f.ctx, `UPDATE platform.push_outbox SET next_attempt_at = NOW() WHERE tenant_id = ?`, f.chain.TenantID)
				require.NoError(t, err)
			}
			switch scenario {
			case "right revoked", "retry after error":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.guardian_student_access SET permissions = permissions - 'parent_portal.enrollments.view' WHERE tenant_id = ? AND relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?)`, f.chain.TenantID, f.chain.StudentID, f.chain.GuardianProfileID)
				require.NoError(t, err)
				// A right on another child does not authorize this persisted audience.
				other := testpkg.CreateTestStudentForTenant(t, f.db, f.chain.TenantID, "Other", "Child", "2a")
				testpkg.CreateTestStudentGuardianLinkForTenant(t, f.db, f.chain.TenantID, other.ID, f.chain.GuardianProfileID, "primary_guardian")
			case "portal access revoked":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.guardian_student_access SET permissions = permissions - 'parent_portal.access' WHERE tenant_id = ?`, f.chain.TenantID)
				require.NoError(t, err)
			case "account inactive":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.accounts SET active = false WHERE id = ?`, f.chain.AccountID)
				require.NoError(t, err)
			case "cancelled invoice":
				_, err := f.db.ExecContext(f.ctx, `INSERT INTO parent_billing.documents (organization_id,run_id,service,number,number_year,number_sequence,issued_on,holiday_period_id,holiday_registration_id,recipient_first_name,recipient_last_name,recipient_email,service_from,service_until,total_cents,kind,reference_document_id,reason)
     SELECT organization_id,run_id,service,'TEST-2',number_year,2,issued_on,holiday_period_id,holiday_registration_id,recipient_first_name,recipient_last_name,recipient_email,service_from,service_until,-total_cents,'cancellation',id,'Test cancellation' FROM parent_billing.documents WHERE id = ?`, f.document)
				require.NoError(t, err)
			case "letter removed":
				_, err := f.db.ExecContext(f.ctx, `DELETE FROM parent_billing.document_letters WHERE document_id = ?`, f.document)
				require.NoError(t, err)
			case "recipient changed":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.accounts SET email = ? WHERE id = ?`, fmt.Sprintf("changed-%d@example.org", f.chain.AccountID), f.chain.AccountID)
				require.NoError(t, err)
			}
			counter := testpkg.CaptureQueriesForContext(t, f.db)
			f.ctx = counter.Context(f.ctx)
			f.runWorker(t)
			if scenario == "valid" {
				assert.Len(t, counter.Selects("parent_billing.documents"), 1, "one object read for all devices")
			}
			if scenario == "valid" {
				assert.Equal(t, []string{"sent", "sent"}, f.states(t))
				assert.Equal(t, 2, f.transport.calls)
				return
			}
			assert.Equal(t, []string{"cancelled", "cancelled"}, f.states(t))
			expected := 0
			if scenario == "retry after error" {
				expected = 2
			}
			assert.Equal(t, expected, f.transport.calls)
			f.runWorker(t)
			assert.Equal(t, expected, f.transport.calls, "cancelled is terminal")
			logs := strings.Join(f.observations, "\n")
			assert.Contains(t, logs, "cancelled")
			assert.NotContains(t, logs, f.chain.Email)
		})
	}
}

// Scope checks execute under the administrative test connection, with RLS
// bypassed: the owner's explicit organization predicate must still deny.
func TestBillingPushObjectScopeWithoutRLS(t *testing.T) {
	t.Parallel()
	f := newBillingPushFixture(t)
	foreign := testpkg.CreateTestOrganization(t, f.db, &testpkg.Organization{Name: "Foreign push", Slug: fmt.Sprintf("foreign-push-%d", f.chain.AccountID)}).ID

	owner := billingcompose.NewDocumentPushValidity()
	check := []parentbilling.DocumentPushCandidate{{Key: 1, DocumentID: f.document, Email: f.chain.Email}}
	require.NoError(t, (ParentPortalRuntime{}).WithinTraeger(f.ctx, foreign, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, ok := tenant.TransactionFromContext(ctx)
		require.True(t, ok)
		dbtx, ok := tx.(bun.Tx)
		require.True(t, ok)
		var bypasses bool
		require.NoError(t, dbtx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses)
		allowed, err := owner.ValidDocumentPushes(ctx, foreign, check)
		require.NoError(t, err)
		assert.Empty(t, allowed, "an explicit object ID never crosses the Träger boundary")
		return nil
	}))
	require.NoError(t, (ParentPortalRuntime{}).WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		allowed, err := owner.ValidDocumentPushes(ctx, f.organization, check)
		require.NoError(t, err)
		assert.Equal(t, map[int64]bool{1: true}, allowed)
		return nil
	}))
}
