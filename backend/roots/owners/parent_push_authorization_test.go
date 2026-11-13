package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	contractcompose "github.com/moto-ogs/project-phoenix/modules/carecontract/compose"
	deliverycompose "github.com/moto-ogs/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	mail "github.com/moto-ogs/project-phoenix/modules/delivery/mail"
	deliveryruntime "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	organizationcompose "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	billingcompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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
	related                     string
	db                          *bun.DB
	ctx                         context.Context
	tenantID                    int64
	organization, run, document int64
	runtime                     *deliverycompose.Runtime
	worker                      *deliveryruntime.Worker
	transport                   *documentPushTransport
	observations                []string
}

func newBillingPushFixture(t *testing.T) (*billingPushFixture, testpkg.ParentChain) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.OwnTenant(t))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	f := &billingPushFixture{tenantID: chain.TenantID, related: parentbilling.PushRelatedBillingDocument, db: db, ctx: testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db)), transport: &documentPushTransport{}}
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(f.ctx, &f.organization))
	testpkg.OwnTraegerContributionRules(t, db, f.organization)
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id,service,holiday_period_id,period_name,service_from,service_until,status,released_at,issued_on,document_count,total_cents)
 VALUES (?,'holiday',8,'Test','2026-10-12','2026-10-23','released',NOW(),'2026-10-01',1,4500) RETURNING id`, f.organization).Scan(f.ctx, &f.run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id,run_id,service,number,number_year,number_sequence,issued_on,holiday_period_id,holiday_registration_id,recipient_first_name,recipient_last_name,recipient_email,service_from,service_until,total_cents)
 VALUES (?,?,'holiday','TEST-1',0,1,'2026-10-01',8,100,'Test','Family',?,'2026-10-12','2026-10-16',4500) RETURNING id`, f.organization, f.run, chain.Email).Scan(f.ctx, &f.document))
	_, err := db.ExecContext(f.ctx, `INSERT INTO parent_billing.run_letters (organization_id,run_id,payment_due_on) VALUES (?,?,'2026-10-15')`, f.organization, f.run)
	require.NoError(t, err)
	_, err = db.ExecContext(f.ctx, `INSERT INTO parent_billing.document_letters (organization_id,document_id,run_id,filename,pdf) VALUES (?,?,?,'test.pdf',?)`, f.organization, f.document, f.run, []byte("%PDF-test"))
	require.NoError(t, err)
	_, err = db.ExecContext(f.ctx, `INSERT INTO parent_billing.document_mailings (organization_id,run_id,document_id,delivery_id,recipient_email) VALUES (?,?,?,?,?)`, f.organization, f.run, f.document, f.document+900000, chain.Email)
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
		mail.Email{}, nil, DeliveryRuntime{Runtime: f.runtime}, nil, nil, nil, cancelledModules{}, nil, f.transport, nil)
	require.NoError(t, err)
	return f, chain
}

// fanout persists device snapshots through the same durable Web Push channel
// as the parent document notifier. No transport is invoked until RunOnce.
func (f *billingPushFixture) fanout(t *testing.T, chain testpkg.ParentChain, n int) []int64 {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := f.db.ExecContext(f.ctx, `INSERT INTO iot.push_subscriptions (tenant_id,account_id,portal,endpoint,p256dh,auth) VALUES (?,?,'parent',?,'p256dh-key','auth-key')`, chain.TenantID, chain.AccountID, fmt.Sprintf("https://fcm.googleapis.com/fcm/send/test-%d-%d", chain.AccountID, i))
		require.NoError(t, err)
	}
	channel, err := deliverycompose.NewTenantDurableWebPushChannel(testpkg.TenantRuntime(t, f.db), f.db, deliverycompose.NewPushSubscriptionRepository(f.db), deliverycompose.VAPIDConfig{PublicKey: "pub", PrivateKey: "priv", Subscriber: "mailto:test@example.org"}, deliverycompose.NewDurablePushOutbox(f.runtime.Module), slog.Default())
	require.NoError(t, err)
	durable, ok := channel.(interface {
		DeliverDurably(context.Context, deliverycompose.NotificationEvent) (int, error)
	})
	require.True(t, ok)
	require.NoError(t, (ParentPortalRuntime{}).WithinTenant(f.ctx, chain.TenantID, func(ctx context.Context) error {
		count, err := durable.DeliverDurably(ctx, deliverycompose.NotificationEvent{Type: parentbilling.PushKindBillingDocument, IdempotencyKey: fmt.Sprintf("%s:%d", f.related, f.document), RelatedType: f.related, RelatedID: f.document,
			Audience: deliverycompose.NotificationAudience{TenantID: chain.TenantID, Scope: "guardian", GuardianAccountIDs: []int64{chain.AccountID}, StudentIDs: []int64{chain.StudentID}}, Title: "Document", Body: "New document", DeepLink: "/documents"})
		require.Equal(t, n, count)
		return err
	}))
	var ids []int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM platform.push_outbox WHERE tenant_id = ? ORDER BY id`, chain.TenantID).Scan(f.ctx, &ids))
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
	require.NoError(t, f.db.NewRaw(`SELECT status FROM platform.push_outbox WHERE tenant_id = ? ORDER BY id`, f.tenantID).Scan(f.ctx, &states))
	return states
}

func TestBillingDevicePushRechecksCurrentObjectAndExactRelationship(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, scenario := range []string{"valid", "right revoked", "portal access revoked", "account inactive", "cancelled invoice", "letter removed", "recipient changed", "retry after error"} {
		t.Run(scenario, func(t *testing.T) {
			f, chain := newBillingPushFixture(t)
			f.fanout(t, chain, 2)
			if scenario == "retry after error" {
				f.transport.err = errors.New("temporary push transport failure")
				f.runWorker(t)
				assert.Equal(t, []string{"pending", "pending"}, f.states(t))
				assert.Equal(t, 2, f.transport.calls)
				f.transport.err = nil
				_, err := f.db.ExecContext(f.ctx, `UPDATE platform.push_outbox SET next_retry_at = 'epoch'::timestamptz WHERE tenant_id = ?`, chain.TenantID)
				require.NoError(t, err)
			}
			switch scenario {
			case "right revoked", "retry after error":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.guardian_student_access SET permissions = permissions - 'parent_portal.enrollments.view' WHERE tenant_id = ? AND relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?)`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
				require.NoError(t, err)
				// A right on another child does not authorize this persisted audience.
				other := testpkg.CreateTestStudentForTenant(t, f.db, chain.TenantID, "Other", "Child", "2a")
				testpkg.CreateTestStudentGuardianLinkForTenant(t, f.db, chain.TenantID, other.ID, chain.GuardianProfileID, "primary_guardian")
			case "portal access revoked":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.guardian_student_access SET permissions = permissions - 'parent_portal.access' WHERE tenant_id = ?`, chain.TenantID)
				require.NoError(t, err)
			case "account inactive":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.accounts SET active = false WHERE id = ?`, chain.AccountID)
				require.NoError(t, err)
			case "cancelled invoice":
				_, err := f.db.ExecContext(f.ctx, `INSERT INTO parent_billing.documents (organization_id,run_id,service,number,number_year,number_sequence,issued_on,holiday_period_id,holiday_registration_id,recipient_first_name,recipient_last_name,recipient_email,service_from,service_until,total_cents,kind,reference_document_id,reason)
     SELECT organization_id,run_id,service,'TEST-2',number_year,2,issued_on,holiday_period_id,holiday_registration_id,recipient_first_name,recipient_last_name,recipient_email,service_from,service_until,-total_cents,'cancellation',id,'Test cancellation' FROM parent_billing.documents WHERE id = ?`, f.document)
				require.NoError(t, err)
			case "letter removed":
				_, err := f.db.ExecContext(f.ctx, `DELETE FROM parent_billing.document_letters WHERE document_id = ?`, f.document)
				require.NoError(t, err)
			case "recipient changed":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.accounts SET email = ? WHERE id = ?`, fmt.Sprintf("changed-%d@example.org", chain.AccountID), chain.AccountID)
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
			assert.NotContains(t, logs, chain.Email)
		})
	}
}

// Scope checks execute under the administrative test connection, with RLS
// bypassed: the owner's explicit organization predicate must still deny.
func TestBillingPushObjectScopeWithoutRLS(t *testing.T) {
	t.Parallel()
	f, chain := newBillingPushFixture(t)
	foreign := testpkg.CreateTestOrganization(t, f.db, &testpkg.Organization{Name: "Foreign push", Slug: fmt.Sprintf("foreign-push-%d", chain.AccountID)}).ID

	owner := billingcompose.NewParentDocuments()
	check := []parentbilling.DocumentPushCandidate{{Key: 1, DocumentID: f.document, Email: chain.Email}}
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
	contractID, _ := f.contract(t, chain)
	_, releaseID, groupID := f.release(t, chain)
	sibling := testpkg.CreateTestStudent(t, f.db, "Sibling", "Scope", "1a")
	school := testpkg.CreateTestSchoolInOrganization(t, f.db, f.organization, "Another push school")
	var otherGroup int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO education.groups (tenant_id,name) VALUES (?,'Other group') RETURNING id`, chain.TenantID).Scan(f.ctx, &otherGroup))
	contractOwner := contractcompose.NewInbox(nil)
	releaseOwner := organizationcompose.NewSharedTraegerDocuments()
	for _, organization := range []int64{f.organization, foreign} {
		require.NoError(t, (ParentPortalRuntime{}).WithinTraeger(f.ctx, organization, func(ctx context.Context) error {
			require.NoError(t, testpkg.WithoutRequestRole(ctx))
			contracts, err := contractOwner.ValidContractPushes(ctx, organization, []carecontract.DocumentPushCandidate{
				{Key: 1, ContractID: contractID, Email: chain.Email, SchoolID: chain.TenantID, StudentIDs: []int64{chain.StudentID}},
				{Key: 2, ContractID: contractID, Email: chain.Email, SchoolID: school, StudentIDs: []int64{chain.StudentID}},
				{Key: 3, ContractID: contractID, Email: chain.Email, SchoolID: chain.TenantID, StudentIDs: []int64{sibling.ID}},
			})
			require.NoError(t, err)
			releases, err := releaseOwner.ValidReleasePushes(ctx, organization, []organizationtenancy.ReleasePushCandidate{
				{Key: 1, ReleaseID: releaseID, SchoolID: chain.TenantID, GroupIDs: []int64{groupID}},
				{Key: 2, ReleaseID: releaseID, SchoolID: school, GroupIDs: []int64{groupID}},
				{Key: 3, ReleaseID: releaseID, SchoolID: chain.TenantID, GroupIDs: []int64{otherGroup}},
			})
			require.NoError(t, err)
			if organization == f.organization {
				assert.Equal(t, map[int64]bool{1: true}, contracts)
				assert.Equal(t, map[int64]bool{1: true}, releases)
			} else {
				assert.Empty(t, contracts)
				assert.Empty(t, releases)
			}
			return nil
		}))
	}

}

// contract fixes the event to the first mailed version, as contract sending
// does. The test does not replace any policy or owner with an authorization mock.
func (f *billingPushFixture) contract(t *testing.T, chain testpkg.ParentChain) (contractID, versionID int64) {
	t.Helper()
	testpkg.OwnTraegerCareContracts(t, f.db, f.organization)
	require.NoError(t, f.db.NewRaw(`INSERT INTO care_contract.contracts (organization_id,school_id,request_child_id,student_id,status,sent_at)
 VALUES (?,?,?,?,'versendet',NOW()) RETURNING id`, f.organization, chain.TenantID, testpkg.UniqueTestTenantID(t), chain.StudentID).Scan(f.ctx, &contractID))
	require.NoError(t, f.db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id,contract_id,version_number,effective_from,child_first_name,child_last_name,school_name,term_kind,notice_months,notice_to,special_termination,special_termination_note,statute_required,statute_title,statute_url,contract_partner)
 VALUES (?,?,1,'2027-08-01','Test','Child','School','open_ended',1,'month_end',false,'',false,'','','') RETURNING id`, f.organization, contractID).Scan(f.ctx, &versionID))
	var documentID int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO care_contract.contract_documents (organization_id,contract_id,contract_version_id,template_version,filename,pdf)
 VALUES (?,?,?,0,'contract.pdf',?) RETURNING id`, f.organization, contractID, versionID, []byte("%PDF-contract")).Scan(f.ctx, &documentID))
	_, err := f.db.ExecContext(f.ctx, `INSERT INTO care_contract.contract_mailings (organization_id,contract_id,contract_document_id,outbox_id,recipient_email)
 VALUES (?,?,?,?,?)`, f.organization, contractID, documentID, testpkg.UniqueTestTenantID(t), chain.Email)
	require.NoError(t, err)
	return contractID, versionID
}

func (f *billingPushFixture) release(t *testing.T, chain testpkg.ParentChain) (documentID, releaseID, groupID int64) {
	t.Helper()
	testpkg.OwnTraegerDocuments(t, f.db, f.organization)
	require.NoError(t, f.db.NewRaw(`INSERT INTO education.groups (tenant_id,name) VALUES (?,'Push group') RETURNING id`, chain.TenantID).Scan(f.ctx, &groupID))
	_, err := f.db.ExecContext(f.ctx, `UPDATE users.student_school_memberships SET group_id=? WHERE tenant_id=? AND student_profile_id=?`, groupID, chain.TenantID, chain.StudentID)
	require.NoError(t, err)
	require.NoError(t, f.db.NewRaw(`INSERT INTO platform.traeger_documents (organization_id,title,file_name,content_type,size_bytes,content)
 VALUES (?,'Document','brief.pdf','application/pdf',8,'%PDF-1.4'::bytea) RETURNING id`, f.organization).Scan(f.ctx, &documentID))
	require.NoError(t, f.db.NewRaw(`INSERT INTO platform.traeger_document_releases (organization_id,document_id,audience,school_id,group_id,group_name)
 VALUES (?,?,'parents',?,?,'Push group') RETURNING id`, f.organization, documentID, chain.TenantID, groupID).Scan(f.ctx, &releaseID))
	return documentID, releaseID, groupID
}

func TestContractDevicePushRechecksVersionAndExactChild(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, scenario := range []string{"valid", "withdrawn", "right revoked", "replaced version", "newer version", "retry withdrawn"} {
		t.Run(scenario, func(t *testing.T) {
			f, chain := newBillingPushFixture(t)
			contractID, versionID := f.contract(t, chain)
			f.document = contractID
			f.related = carecontract.PushRelatedContractDocument
			f.fanout(t, chain, 2)
			if scenario == "retry withdrawn" {
				f.transport.err = errors.New("temporary push failure")
				f.runWorker(t)
				require.Equal(t, 2, f.transport.calls)
				f.transport.err = nil
				_, err := f.db.ExecContext(f.ctx, `UPDATE platform.push_outbox SET next_retry_at='epoch'::timestamptz WHERE tenant_id=?`, chain.TenantID)
				require.NoError(t, err)
			}
			switch scenario {
			case "withdrawn", "retry withdrawn":
				_, err := f.db.ExecContext(f.ctx, `UPDATE care_contract.contracts SET status='widerrufen',concluded_at=NOW(),conclusion_method='papier' WHERE id=?`, contractID)
				require.NoError(t, err)
			case "right revoked":
				_, err := f.db.ExecContext(f.ctx, `UPDATE auth.guardian_student_access SET permissions=permissions-'parent_portal.enrollments.view' WHERE tenant_id=? AND relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE student_id=?)`, chain.TenantID, chain.StudentID)
				require.NoError(t, err)
				sibling := testpkg.CreateTestStudent(t, f.db, "Sibling", "Child", "1a")
				testpkg.CreateTestStudentGuardianLink(t, f.db, sibling.ID, chain.GuardianProfileID, "primary_guardian")
			case "replaced version":
				_, err := f.db.ExecContext(f.ctx, `UPDATE care_contract.contract_versions SET status='wirksam' WHERE id=?`, versionID)
				require.NoError(t, err)
				_, err = f.db.ExecContext(f.ctx, `UPDATE care_contract.contract_versions SET status='ersetzt' WHERE id=?`, versionID)
				require.NoError(t, err)
			case "newer version":
				var nextVersion int64
				err := f.db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id,contract_id,version_number,effective_from,status,child_first_name,child_last_name,school_name,term_kind,notice_months,notice_to,special_termination,special_termination_note,statute_required,statute_title,statute_url,contract_partner)
     VALUES (?,?,2,'2027-09-01','wirksam','Test','Child','School','open_ended',1,'month_end',false,'',false,'','','') RETURNING id`, f.organization, contractID).Scan(f.ctx, &nextVersion)
				require.NoError(t, err)
				var nextDocument int64
				require.NoError(t, f.db.NewRaw(`INSERT INTO care_contract.contract_documents (organization_id,contract_id,contract_version_id,template_version,filename,pdf) VALUES (?,?,?,0,'new-contract.pdf',?) RETURNING id`, f.organization, contractID, nextVersion, []byte("%PDF-new")).Scan(f.ctx, &nextDocument))
				_, err = f.db.ExecContext(f.ctx, `INSERT INTO care_contract.contract_mailings (organization_id,contract_id,contract_document_id,outbox_id,recipient_email) VALUES (?,?,?,?,?)`, f.organization, contractID, nextDocument, testpkg.UniqueTestTenantID(t), chain.Email)
				require.NoError(t, err)
			}
			counter := testpkg.CaptureQueriesForContext(t, f.db)
			f.ctx = counter.Context(f.ctx)
			f.runWorker(t)
			if scenario == "valid" {
				assert.Equal(t, []string{"sent", "sent"}, f.states(t))
				assert.Equal(t, 2, f.transport.calls)
				assert.Len(t, counter.Selects("care_contract.contracts"), 1)
				return
			}
			assert.Equal(t, []string{"cancelled", "cancelled"}, f.states(t))
			expected := 0
			if scenario == "retry withdrawn" {
				expected = 2
			}
			assert.Equal(t, expected, f.transport.calls)
			f.runWorker(t)
			assert.Equal(t, expected, f.transport.calls, "terminal cancellation")
		})
	}
}

func TestReleasedDocumentDevicePushRechecksReleaseAndCurrentAudience(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, scenario := range []string{"valid access only", "revoked", "revoked and released again", "deleted", "group changed", "access revoked", "retry revoked"} {
		t.Run(scenario, func(t *testing.T) {
			f, chain := newBillingPushFixture(t)
			documentID, releaseID, _ := f.release(t, chain)
			// Released documents require access, not the enrollment viewing right.
			_, err := f.db.ExecContext(f.ctx, `UPDATE auth.guardian_student_access SET permissions='{"parent_portal.access":true}'::jsonb WHERE tenant_id=?`, chain.TenantID)
			require.NoError(t, err)
			f.document = releaseID
			f.related = organizationtenancy.PushRelatedTraegerDocumentRelease
			f.fanout(t, chain, 2)
			if scenario == "retry revoked" {
				f.transport.err = errors.New("temporary push failure")
				f.runWorker(t)
				require.Equal(t, 2, f.transport.calls)
				f.transport.err = nil
				_, err = f.db.ExecContext(f.ctx, `UPDATE platform.push_outbox SET next_retry_at='epoch'::timestamptz WHERE tenant_id=?`, chain.TenantID)
				require.NoError(t, err)
			}
			switch scenario {
			case "revoked", "revoked and released again", "retry revoked":
				_, err = f.db.ExecContext(f.ctx, `UPDATE platform.traeger_document_releases SET revoked_at=NOW() WHERE id=?`, releaseID)
				require.NoError(t, err)
				if scenario == "revoked and released again" {
					_, err = f.db.ExecContext(f.ctx, `INSERT INTO platform.traeger_document_releases (organization_id,document_id,audience,school_id) VALUES (?,?,'parents',?)`, f.organization, documentID, chain.TenantID)
					require.NoError(t, err)
				}
			case "deleted":
				_, err = f.db.ExecContext(f.ctx, `UPDATE platform.traeger_documents SET deleted_at=NOW(),content=NULL WHERE id=?`, documentID)
				require.NoError(t, err)
			case "group changed":
				_, err = f.db.ExecContext(f.ctx, `UPDATE users.student_school_memberships SET group_id=NULL WHERE tenant_id=? AND student_profile_id=?`, chain.TenantID, chain.StudentID)
				require.NoError(t, err)
			case "access revoked":
				_, err = f.db.ExecContext(f.ctx, `UPDATE auth.guardian_student_access SET permissions='{}'::jsonb WHERE tenant_id=?`, chain.TenantID)
				require.NoError(t, err)
			}
			counter := testpkg.CaptureQueriesForContext(t, f.db)
			f.ctx = counter.Context(f.ctx)
			f.runWorker(t)
			if scenario == "valid access only" {
				assert.Equal(t, []string{"sent", "sent"}, f.states(t))
				assert.Equal(t, 2, f.transport.calls)
				assert.Len(t, counter.Selects("platform.traeger_document_releases"), 1)
				return
			}
			assert.Equal(t, []string{"cancelled", "cancelled"}, f.states(t))
			expected := 0
			if scenario == "retry revoked" {
				expected = 2
			}
			assert.Equal(t, expected, f.transport.calls)
			f.runWorker(t)
			assert.Equal(t, expected, f.transport.calls, "revoking the original release is terminal even after releasing again")
		})
	}
}
