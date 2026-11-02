// Router-level tests for the history of the Träger-Büro and the mail to the
// person concerned (#5474): the history shows invitations, role changes and
// removals of the own Träger only, newest first and page by page; a role
// without "Konten verwalten" gets 403; a role change and a removal queue
// exactly one mail to that person in the unit of the change.
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type traegerHistoryAnswer struct {
	Entries []struct {
		ID          int64    `json:"id"`
		Action      string   `json:"action"`
		ActorEmail  string   `json:"actor_email"`
		TargetEmail string   `json:"target_email"`
		RolesBefore []string `json:"roles_before"`
		RolesAfter  []string `json:"roles_after"`
	} `json:"entries"`
	HasMore bool `json:"has_more"`
}

func traegerHistory(t *testing.T, router http.Handler, token string, before int64) traegerHistoryAnswer {
	t.Helper()
	path := "/traeger/accounts/history"
	if before > 0 {
		path += fmt.Sprintf("?before=%d", before)
	}
	rec := traegerCall(t, router, http.MethodGet, path, token, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[traegerHistoryAnswer](t, rec.Body.Bytes())
}

// traegerChangeMail is a notice queued in Delivery's outbox.
type traegerChangeMail struct {
	Recipient   string `bun:"recipient"`
	Template    string `bun:"template"`
	Subject     string `bun:"subject"`
	ActorEmail  string `bun:"actor_email"`
	RolesBefore string `bun:"roles_before"`
	RolesAfter  string `bun:"roles_after"`
	Removed     bool   `bun:"removed"`
	Self        bool   `bun:"self"`
}

const traegerChangeMailQueued = `FROM platform.email_outbox WHERE tenant_id IS NULL AND kind = ? AND payload->>'Type' = 'traeger_account_change' AND (payload->>'ReferenceID')::bigint = ?`

func traegerChangeMails(t *testing.T, db bun.IDB, accountID int64) []traegerChangeMail {
	t.Helper()
	var mails []traegerChangeMail
	require.NoError(t, db.NewRaw(`SELECT payload->'Message'->'To'->>'Address' AS recipient, payload->'Message'->>'Template' AS template,
		payload->'Message'->>'Subject' AS subject, payload->'Message'->'Content'->>'ActorEmail' AS actor_email,
		payload->'Message'->'Content'->>'RolesBefore' AS roles_before, payload->'Message'->'Content'->>'RolesAfter' AS roles_after,
		(payload->'Message'->'Content'->>'Removed')::boolean AS removed, (payload->'Message'->'Content'->>'Self')::boolean AS self `+
		traegerChangeMailQueued+` ORDER BY id`, owners.TransactionalMailKind, accountID).Scan(context.Background(), &mails))
	return mails
}

func TestTraegerAccountHistoryShowsTheChangesOfTheOwnTraeger(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	own := traegerBuero(t, db, "Träger Verlauf")
	foreign := traegerBuero(t, db, "Träger Verlauf Fremd")
	leader, session := traegerMember(t, db, router, own, identityaccess.TraegerRoleLeadership)
	colleague, _ := traegerMember(t, db, router, own, identityaccess.TraegerRoleReadOnly)
	_, foreignSession := traegerMember(t, db, router, foreign, identityaccess.TraegerRoleLeadership)
	address := fmt.Sprintf("verlauf-%d@test.local", testpkg.UniqueTestTenantID(t))
	traegerInvite(t, db, router, session.AccessToken, address, identityaccess.TraegerRoleBilling)
	invitation := traegerBueroList(t, router, session.AccessToken).Invitations[0].ID
	rec := traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/invitations/%d", invitation), session.AccessToken, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", colleague.ID), session.AccessToken,
		map[string]any{"roles": []string{identityaccess.TraegerRolePersonnel, identityaccess.TraegerRoleReadOnly}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/%d", colleague.ID), session.AccessToken, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	traegerInvite(t, db, router, foreignSession.AccessToken, fmt.Sprintf("fremd-verlauf-%d@test.local", testpkg.UniqueTestTenantID(t)), identityaccess.TraegerRoleReadOnly)

	history := traegerHistory(t, router, session.AccessToken, 0)
	require.Len(t, history.Entries, 4, "the reads of the member list are no change")
	assert.False(t, history.HasMore)
	actions := make([]string, 0, len(history.Entries))
	for _, entry := range history.Entries {
		actions = append(actions, entry.Action)
		assert.Equal(t, leader.Email, entry.ActorEmail, "the history names who acted")
	}
	assert.Equal(t, []string{"member_removed", "roles_changed", "invitation_revoked", "invitation_created"}, actions, "newest first")
	removal, change, created := history.Entries[0], history.Entries[1], history.Entries[3]
	assert.Equal(t, colleague.Email, removal.TargetEmail)
	assert.Equal(t, []string{identityaccess.TraegerRolePersonnel, identityaccess.TraegerRoleReadOnly}, removal.RolesBefore)
	assert.Empty(t, removal.RolesAfter)
	assert.Equal(t, colleague.Email, change.TargetEmail)
	assert.Equal(t, []string{identityaccess.TraegerRoleReadOnly}, change.RolesBefore, "the history shows the roles before")
	assert.Equal(t, []string{identityaccess.TraegerRolePersonnel, identityaccess.TraegerRoleReadOnly}, change.RolesAfter, "and after")
	assert.Equal(t, address, created.TargetEmail)
	assert.Equal(t, []string{identityaccess.TraegerRoleBilling}, created.RolesAfter)

	foreignHistory := traegerHistory(t, router, foreignSession.AccessToken, 0)
	require.Len(t, foreignHistory.Entries, 1, "no Träger sees the history of another")
	assert.NotEqual(t, address, foreignHistory.Entries[0].TargetEmail)
}

func TestTraegerAccountHistoryRequiresTheAccountsRight(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Verlauf Rechte")
	traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleLeadership)
	for _, role := range []string{identityaccess.TraegerRoleAdministration, identityaccess.TraegerRoleReadOnly} {
		_, session := traegerMember(t, db, router, organizationID, role)
		rec := traegerCall(t, router, http.MethodGet, "/traeger/accounts/history", session.AccessToken, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s gets 403", role)
	}
	rec := traegerCall(t, router, http.MethodGet, "/traeger/accounts/history", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTraegerAccountHistoryIsPagedAndFlat(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Verlauf Seiten")
	leader, session := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleLeadership)
	second, _ := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleLeadership)
	ledger := func(actor int64, count int) {
		for range count {
			_, err := db.ExecContext(context.Background(), `INSERT INTO audit.traeger_account_changes (organization_id, actor_account_id, action, target_email, roles_after)
				VALUES (?, ?, 'invitation_created', 'seite@test.local', '{read_only}'), (?, ?, 'members_listed', NULL, '{}')`,
				organizationID, actor, organizationID, actor)
			require.NoError(t, err)
		}
	}
	read := func(before int64) ([]string, traegerHistoryAnswer) {
		counter := testpkg.CaptureQueriesForContext(t, db)
		path := "/traeger/accounts/history"
		if before > 0 {
			path += fmt.Sprintf("?before=%d", before)
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+session.AccessToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects(""), decodeSchoolPortalData[traegerHistoryAnswer](t, rec.Body.Bytes())
	}

	ledger(leader.ID, 3)
	few, _ := read(0)
	ledger(second.ID, identityaccess.TraegerAccountHistoryPageSize)
	many, first := read(0)
	assert.Len(t, many, len(few), "the history reads the same statements for any number of entries and people")
	querybudgets.AssertQueryBudget(t, "api.traeger.accounts.history", few)

	require.Len(t, first.Entries, identityaccess.TraegerAccountHistoryPageSize)
	assert.True(t, first.HasMore)
	assert.Equal(t, second.Email, first.Entries[0].ActorEmail)
	_, rest := read(first.Entries[len(first.Entries)-1].ID)
	require.Len(t, rest.Entries, 3, "the next page continues before the last entry")
	assert.False(t, rest.HasMore)
	assert.Equal(t, leader.Email, rest.Entries[0].ActorEmail)
	assert.Less(t, rest.Entries[0].ID, first.Entries[len(first.Entries)-1].ID)

	rec := traegerCall(t, router, http.MethodGet, "/traeger/accounts/history?before=x", session.AccessToken, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTraegerRoleChangeAndRemovalMailThePersonConcerned(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Benachrichtigung")
	leader, session := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleLeadership)
	colleague, _ := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleReadOnly)
	other, _ := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleLeadership)

	traegerInvite(t, db, router, session.AccessToken, fmt.Sprintf("keine-mitteilung-%d@test.local", testpkg.UniqueTestTenantID(t)), identityaccess.TraegerRoleReadOnly)
	rec := traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", colleague.ID), session.AccessToken,
		map[string]any{"roles": []string{identityaccess.TraegerRoleReadOnly}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, traegerChangeMails(t, db, colleague.ID), "the same roles again change nothing and mail nothing")

	rec = traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", colleague.ID), session.AccessToken,
		map[string]any{"roles": []string{identityaccess.TraegerRoleBilling, identityaccess.TraegerRoleReadOnly}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	mails := traegerChangeMails(t, db, colleague.ID)
	require.Len(t, mails, 1, "a role change queues exactly one mail")
	assert.Equal(t, colleague.Email, mails[0].Recipient, "to the person concerned")
	assert.Equal(t, "traeger-account-change.html", mails[0].Template)
	assert.Equal(t, leader.Email, mails[0].ActorEmail, "naming who changed the roles")
	assert.Equal(t, "Nur lesen", mails[0].RolesBefore)
	assert.Equal(t, "Abrechnung und Nur lesen", mails[0].RolesAfter)
	assert.False(t, mails[0].Removed)
	assert.False(t, mails[0].Self)
	assert.Contains(t, mails[0].Subject, "Träger Benachrichtigung")
	rec = traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", colleague.ID), session.AccessToken,
		map[string]any{"roles": []string{identityaccess.TraegerRoleReadOnly, identityaccess.TraegerRoleBilling}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, traegerChangeMails(t, db, colleague.ID), 1, "the same roles in another order change nothing and mail nothing")

	rec = traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/%d", colleague.ID), session.AccessToken, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	mails = traegerChangeMails(t, db, colleague.ID)
	require.Len(t, mails, 2, "a removal queues exactly one more mail")
	assert.True(t, mails[1].Removed)
	assert.Equal(t, "Abrechnung und Nur lesen", mails[1].RolesBefore)

	// A person who changes their own roles is mailed as well: a taken-over
	// account would do exactly that.
	rec = traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", leader.ID), session.AccessToken,
		map[string]any{"roles": []string{identityaccess.TraegerRoleAdministration, identityaccess.TraegerRoleLeadership}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	own := traegerChangeMails(t, db, leader.ID)
	require.Len(t, own, 1)
	assert.Equal(t, leader.Email, own[0].Recipient)
	assert.True(t, own[0].Self)
	assert.Empty(t, traegerChangeMails(t, db, other.ID), "nobody else is mailed")
}

// The mail commits with the change or not at all.
func TestTraegerRoleChangeMailRollsBackWithTheChange(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Rücknahme")
	leader, _ := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleLeadership)
	colleague, _ := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleReadOnly)
	actor := identityaccess.TraegerActor{OrganizationID: organizationID, AccountID: leader.ID}
	errAbort := errors.New("abort after the change")

	for name, change := range map[string]func(context.Context) error{
		"roles": func(ctx context.Context) error {
			_, err := identity.ChangeTraegerRoles(ctx, actor, colleague.ID, []string{identityaccess.TraegerRolePersonnel})
			return err
		},
		"removal": func(ctx context.Context) error { return identity.RemoveTraegerMember(ctx, actor, colleague.ID) },
	} {
		err := tenant.WithinAdmin(tenant.WithUnitOfWork(context.Background(), unit), func(adminCtx context.Context) error {
			require.NoError(t, change(adminCtx), name)
			transaction, ok := tenant.TransactionFromContext(adminCtx)
			require.True(t, ok)
			var tx bun.IDB
			switch value := transaction.(type) {
			case bun.Tx:
				tx = value
			case *bun.Tx:
				tx = *value
			default:
				t.Fatalf("unexpected transaction %T", transaction)
			}
			require.Len(t, traegerChangeMails(t, tx, colleague.ID), 1, "%s: the change queued its mail in its unit", name)
			return errAbort
		})
		require.ErrorIs(t, err, errAbort)
		assert.Empty(t, traegerChangeMails(t, db, colleague.ID), "%s: the rolled-back change leaves no mail", name)
	}
	var roles []string
	require.NoError(t, db.NewRaw(`SELECT role FROM auth.organization_account_roles WHERE organization_id = ? AND account_id = ?`, organizationID, colleague.ID).
		Scan(context.Background(), &roles))
	assert.Equal(t, []string{identityaccess.TraegerRoleReadOnly}, roles, "nor a change")
	assert.Empty(t, traegerLedger(t, db, organizationID), "nor a ledger entry")
}
