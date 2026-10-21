package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditRecords "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge"
	mergeCompose "github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge/compose"
	mergeHTTP "github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type staffMergeHarness struct {
	db         *bun.DB
	schoolID   int64
	operatorID int64
	router     chi.Router
}

type namesakeBeforeRetirementMemberships struct {
	schoolmembership.StaffPersonMerge
	db       *bun.DB
	schoolID int64
	first    string
	last     string
	inserted bool
}

type uncommittedNamesakePeople struct {
	peopledirectory.StaffPersonMerge
	db       *bun.DB
	schoolID int64
	first    string
	last     string
}

func (p uncommittedNamesakePeople) RetireMergedStaffPerson(ctx context.Context, schoolID, donorID, keeperID int64) error {
	tx, err := p.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.NewRaw(`INSERT INTO users.persons (tenant_id, first_name, last_name) VALUES (?, ?, ?)`, p.schoolID, p.first, p.last).Exec(context.Background()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return p.StaffPersonMerge.RetireMergedStaffPerson(ctx, schoolID, donorID, keeperID)
}

func (m *namesakeBeforeRetirementMemberships) ReassignStaffPerson(ctx context.Context, input schoolmembership.StaffPersonReassignment) error {
	if !m.inserted {
		_, err := m.db.NewRaw(`INSERT INTO users.persons (tenant_id, first_name, last_name) VALUES (?, ?, ?)`, m.schoolID, m.first, m.last).Exec(context.Background())
		if err != nil {
			return err
		}
		m.inserted = true
	}
	return m.StaffPersonMerge.ReassignStaffPerson(ctx, input)
}

func newStaffMergeHarness(t *testing.T) staffMergeHarness {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.OwnTenant(t)
	handler, err := newStaffPersonMergeRoutes(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Mount("/", handler)
	operator := testpkg.CreateTestOperator(t, db)
	return staffMergeHarness{db: db, schoolID: schoolID, operatorID: operator.ID, router: router}
}
func (h staffMergeHarness) request(t *testing.T, method string, schoolID int64, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, method, fmt.Sprintf("/%d", schoolID), body, testutil.WithClaims(t, sessiontoken.AppClaims{ID: int(h.operatorID), Scope: "platform"}))
	return testutil.ExecuteRequest(h.router, req)
}
func (h staffMergeHarness) preview(t *testing.T) []staffpersonmerge.Candidate {
	t.Helper()
	rr := h.request(t, http.MethodGet, h.schoolID, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var result struct {
		Data []staffpersonmerge.Candidate `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	return result.Data
}
func mergeSelection(candidates []staffpersonmerge.Candidate) staffpersonmerge.Request {
	request := staffpersonmerge.Request{SchoolConfirmed: true}
	for _, candidate := range candidates {
		request.Cases = append(request.Cases, staffpersonmerge.Selection{DonorID: candidate.Donor.Person.ID, KeeperID: candidate.Keeper.Person.ID, Revision: candidate.Revision})
	}
	return request
}
func (h staffMergeHarness) pair(t *testing.T, first string) (keeper, donor *testpkg.Staff) {
	t.Helper()
	keeper, _ = testpkg.CreateTestStaffWithAccount(t, h.db, first, "Doppelsatz")
	donor = testpkg.CreateTestStaff(t, h.db, first, "Doppelsatz")
	_, err := h.db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = '2026-09-02T12:00:00Z' WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	return keeper, donor
}

func TestStaffPersonMergeKeepsEmploymentHistoryAndAccount(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	keeper, donor := h.pair(t, "Anna")
	ctx := testpkg.Ctx(t)
	// The account-bearing person is older, as explicitly reported for school 2.
	_, err := h.db.NewRaw(`UPDATE users.persons SET created_at = '2026-04-14T12:00:00Z' WHERE tenant_id = ? AND id = ?`, h.schoolID, keeper.PersonID).Exec(ctx)
	require.NoError(t, err)
	_, err = h.db.NewRaw(`UPDATE users.persons SET birthday = '1980-02-03' WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.PersonID).Exec(ctx)
	require.NoError(t, err)
	_, err = h.db.NewRaw(`INSERT INTO users.staff_master_data (tenant_id,staff_id,address_city) VALUES (?,?,'Frühere Adresse')`, h.schoolID, donor.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = h.db.NewRaw(`INSERT INTO active.staff_vacation_quota (tenant_id,staff_id,year,entitled_days,carryover_days) VALUES (?,?,2026,30,4),(?,?,2026,20,0)`, h.schoolID, donor.ID, h.schoolID, keeper.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = h.db.NewRaw(`INSERT INTO active.work_sessions (tenant_id,staff_id,date,check_in_time,check_out_time,created_by) VALUES (?,?,'2026-08-03','2026-08-03T08:00:00Z','2026-08-03T12:00:00Z',?)`, h.schoolID, donor.ID, donor.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = h.db.NewRaw(`INSERT INTO active.staff_month_balance_snapshots (tenant_id,staff_id,year,month,closing_balance_minutes,closed_by) VALUES (?,?,2026,8,125,?)`, h.schoolID, donor.ID, donor.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = h.db.NewRaw(`INSERT INTO audit.personnel_number_changes (tenant_id,staff_id,changed_by,old_value,new_value) VALUES (?,?,?,'alt','historisch')`, h.schoolID, donor.ID, keeper.ID).Exec(ctx)
	require.NoError(t, err)
	before := staffMergeHistory(t, h.db, h.schoolID)
	candidates := h.preview(t)
	require.Len(t, candidates, 1)
	assert.Equal(t, keeper.PersonID, candidates[0].Keeper.Person.ID)
	assert.Equal(t, donor.ID, candidates[0].Donor.Employment.ID)
	rr := h.request(t, http.MethodPost, h.schoolID, mergeSelection(candidates))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Empty(t, h.preview(t), "a completed repair is not offered again")
	assert.Equal(t, before, staffMergeHistory(t, h.db, h.schoolID), "employment-owned history is byte-for-byte unchanged")
	var personID int64
	require.NoError(t, h.db.NewRaw(`SELECT person_id FROM users.staff_school_memberships WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.ID).Scan(ctx, &personID))
	assert.Equal(t, keeper.PersonID, personID)
	var retired bool
	require.NoError(t, h.db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.persons WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.PersonID).Scan(ctx, &retired))
	assert.True(t, retired)
	var birthday string
	require.NoError(t, h.db.NewRaw(`SELECT birthday::text FROM users.persons WHERE tenant_id = ? AND id = ?`, h.schoolID, keeper.PersonID).Scan(ctx, &birthday))
	assert.Equal(t, "1980-02-03", birthday)
	var audit struct {
		OperatorID int64
		Changes    json.RawMessage
	}
	require.NoError(t, h.db.NewRaw(`SELECT operator_id,changes FROM platform.operator_audit_log WHERE operator_id = ? AND resource_id = ? AND action = 'merge'`, h.operatorID, keeper.PersonID).Scan(context.Background(), &audit))
	assert.Equal(t, h.operatorID, audit.OperatorID)
	assert.JSONEq(t, fmt.Sprintf(`{"school_id":%d,"donor_person_id":%d,"keeper_person_id":%d,"donor_staff_id":%d,"keeper_staff_id":%d,"school_confirmed":true}`, h.schoolID, donor.PersonID, keeper.PersonID, donor.ID, keeper.ID), string(audit.Changes))
	rr = h.request(t, http.MethodPost, h.schoolID, mergeSelection(candidates))
	assert.Equal(t, http.StatusConflict, rr.Code, "a replay cannot apply the repair again")
}
func staffMergeHistory(t *testing.T, db *bun.DB, schoolID int64) string {
	t.Helper()
	var value string
	require.NoError(t, db.NewRaw(`SELECT json_build_object(
 'master_data',(SELECT json_agg(row_to_json(m) ORDER BY m.id) FROM users.staff_master_data m WHERE tenant_id = ?),
 'vacation',(SELECT json_agg(row_to_json(v) ORDER BY v.id) FROM active.staff_vacation_quota v WHERE tenant_id = ?),
 'sessions',(SELECT json_agg(row_to_json(w) ORDER BY w.id) FROM active.work_sessions w WHERE tenant_id = ?),
 'snapshots',(SELECT json_agg(row_to_json(s) ORDER BY s.id) FROM active.staff_month_balance_snapshots s WHERE tenant_id = ?),
 'audit',(SELECT json_agg(row_to_json(a) ORDER BY a.id) FROM audit.personnel_number_changes a WHERE tenant_id = ?))::text`, schoolID, schoolID, schoolID, schoolID, schoolID).Scan(testpkg.Ctx(t), &value))
	return value
}

func TestStaffPersonMergeNeverOffersOrMergesNamesakeChildren(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	first := testpkg.CreateTestStudent(t, h.db, "Kim", "Kind", "2a")
	second := testpkg.CreateTestStudent(t, h.db, "Kim", "Kind", "2b")
	assert.Empty(t, h.preview(t), "namesake children are different people")
	// Even anomalous staff records and a linked account must not turn children
	// into repair candidates. This satisfies every other candidate predicate.
	account := testpkg.CreateTestAccount(t, h.db, "child-merge@example.test")
	_, err := h.db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE tenant_id = ? AND id = ?`, account.ID, h.schoolID, first.PersonID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	testpkg.CreateTestStaffForPerson(t, h.db, first.PersonID)
	retired := testpkg.CreateTestStaffForPerson(t, h.db, second.PersonID)
	_, err = h.db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = NOW() WHERE tenant_id = ? AND id = ?`, h.schoolID, retired.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Empty(t, h.preview(t))
	rr := h.request(t, http.MethodPost, h.schoolID, staffpersonmerge.Request{SchoolConfirmed: true, Cases: []staffpersonmerge.Selection{{DonorID: second.PersonID, KeeperID: first.PersonID, Revision: "forged"}}})
	assert.Equal(t, http.StatusConflict, rr.Code)
	var live int
	require.NoError(t, h.db.NewRaw(`SELECT count(*) FROM users.persons WHERE tenant_id = ? AND id IN (?,?) AND deleted_at IS NULL`, h.schoolID, first.PersonID, second.PersonID).Scan(testpkg.Ctx(t), &live))
	assert.Equal(t, 2, live)
}

func TestStaffPersonMergeKeepsAccountWhenItsEmploymentIsRetired(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	keeper, donor := h.pair(t, "Umgekehrt")
	_, err := h.db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = CASE WHEN id = ? THEN NOW() ELSE NULL END WHERE tenant_id = ? AND id IN (?,?)`, keeper.ID, h.schoolID, keeper.ID, donor.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	candidates := h.preview(t)
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].Keeper.Employment.DeletedAt)
	rr := h.request(t, http.MethodPost, h.schoolID, mergeSelection(candidates))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var active int64
	require.NoError(t, h.db.NewRaw(`SELECT person_id FROM users.staff_school_memberships WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL`, h.schoolID, donor.ID).Scan(testpkg.Ctx(t), &active))
	assert.Equal(t, keeper.PersonID, active)
}

func TestStaffPersonMergeRejectsStaleAndUnconfirmedBatches(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"school not confirmed", "empty batch", "duplicate case", "changed person", "changed employment", "wrong school"} {
		t.Run(scenario, func(t *testing.T) {
			testpkg.OwnTenant(t)
			h := newStaffMergeHarness(t)
			_, donor := h.pair(t, "Erster")
			h.pair(t, "Zweiter")
			candidates := h.preview(t)
			require.Len(t, candidates, 2)
			input := mergeSelection(candidates)
			school := h.schoolID
			status := http.StatusConflict
			switch scenario {
			case "school not confirmed":
				input.SchoolConfirmed = false
				status = http.StatusBadRequest
			case "empty batch":
				input.Cases = nil
				status = http.StatusBadRequest
			case "duplicate case":
				input.Cases = append(input.Cases, input.Cases[0])
			case "changed person":
				_, err := h.db.NewRaw(`UPDATE users.persons SET birthday = '1981-01-01' WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.PersonID).Exec(testpkg.Ctx(t))
				require.NoError(t, err)
			case "changed employment":
				_, err := h.db.NewRaw(`UPDATE users.staff_school_memberships SET updated_at = updated_at + interval '1 second' WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.ID).Exec(testpkg.Ctx(t))
				require.NoError(t, err)
			case "wrong school":
				school, _ = testpkg.CreateTestTenant(t, h.db)
			}
			rr := h.request(t, http.MethodPost, school, input)
			assert.Equal(t, status, rr.Code, rr.Body.String())
			var live int
			require.NoError(t, h.db.NewRaw(`SELECT count(*) FROM users.persons WHERE tenant_id = ? AND deleted_at IS NULL`, h.schoolID).Scan(testpkg.Ctx(t), &live))
			assert.Equal(t, 4, live, "no selected case may be partially applied")
		})
	}
}

func TestStaffPersonMergeRejectsNamesakeAddedBeforeRetirement(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	_, donor := h.pair(t, "Späte")
	input := mergeSelection(h.preview(t))
	audit, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{DB: h.db})
	require.NoError(t, err)
	workflow, err := mergeCompose.New(
		peopleCompose.NewStaffPersonMerge(),
		&namesakeBeforeRetirementMemberships{
			StaffPersonMerge: membershipCompose.NewStaffPersonMerge(),
			db:               h.db,
			schoolID:         h.schoolID,
			first:            "Späte",
			last:             "Doppelsatz",
		},
		staffPersonMergeAudit{create: audit.OperatorAuditLog.Create},
		operatorIDFromSession,
	)
	require.NoError(t, err)
	h.router = chi.NewRouter()
	h.router.Mount("/", mergeHTTP.Router(workflow, TenantScopeMiddleware))

	rr := h.request(t, http.MethodPost, h.schoolID, input)
	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())

	var live int
	require.NoError(t, h.db.NewRaw(`SELECT count(*) FROM users.persons WHERE tenant_id = ? AND first_name = ? AND last_name = ? AND deleted_at IS NULL`, h.schoolID, "Späte", "Doppelsatz").Scan(testpkg.Ctx(t), &live))
	assert.Equal(t, 3, live)
	var donorPersonID int64
	require.NoError(t, h.db.NewRaw(`SELECT person_id FROM users.staff_school_memberships WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.ID).Scan(testpkg.Ctx(t), &donorPersonID))
	assert.Equal(t, donor.PersonID, donorPersonID, "the failed repair must roll back the employment reassignment")
	var audits int
	require.NoError(t, h.db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log WHERE operator_id = ? AND action = 'merge'`, h.operatorID).Scan(testpkg.Ctx(t), &audits))
	assert.Zero(t, audits)
}

func TestStaffPersonMergeRejectsNamesakeCommittedBeforeFinalCheck(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	_, donor := h.pair(t, "Gleichzeitig")
	input := mergeSelection(h.preview(t))
	audit, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{DB: h.db})
	require.NoError(t, err)
	workflow, err := mergeCompose.New(
		uncommittedNamesakePeople{StaffPersonMerge: peopleCompose.NewStaffPersonMerge(), db: h.db, schoolID: h.schoolID, first: "Gleichzeitig", last: "Doppelsatz"},
		membershipCompose.NewStaffPersonMerge(),
		staffPersonMergeAudit{create: audit.OperatorAuditLog.Create},
		operatorIDFromSession,
	)
	require.NoError(t, err)
	h.router = chi.NewRouter()
	h.router.Mount("/", mergeHTTP.Router(workflow, TenantScopeMiddleware))

	rr := h.request(t, http.MethodPost, h.schoolID, input)
	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())

	var live int
	require.NoError(t, h.db.NewRaw(`SELECT count(*) FROM users.persons WHERE tenant_id = ? AND first_name = ? AND last_name = ? AND deleted_at IS NULL`, h.schoolID, "Gleichzeitig", "Doppelsatz").Scan(testpkg.Ctx(t), &live))
	assert.Equal(t, 3, live)
	var donorPersonID int64
	require.NoError(t, h.db.NewRaw(`SELECT person_id FROM users.staff_school_memberships WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.ID).Scan(testpkg.Ctx(t), &donorPersonID))
	assert.Equal(t, donor.PersonID, donorPersonID)
	var audits int
	require.NoError(t, h.db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log WHERE operator_id = ? AND action = 'merge'`, h.operatorID).Scan(testpkg.Ctx(t), &audits))
	assert.Zero(t, audits)
}

func TestStaffPersonMergeRollsBackWholeBatchAndAuditOnFailure(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	h.pair(t, "Erster")
	h.pair(t, "Zweiter")
	candidates := h.preview(t)
	require.Len(t, candidates, 2)
	audit, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{DB: h.db})
	require.NoError(t, err)
	calls := 0
	workflow, err := mergeCompose.New(peopleCompose.NewStaffPersonMerge(), membershipCompose.NewStaffPersonMerge(), staffPersonMergeAudit{create: func(ctx context.Context, entry *auditRecords.OperatorAuditEntry) error {
		calls++
		if calls == 2 {
			return errors.New("audit unavailable")
		}
		return audit.OperatorAuditLog.Create(ctx, entry)
	}}, operatorIDFromSession)
	require.NoError(t, err)
	h.router = chi.NewRouter()
	h.router.Mount("/", mergeHTTP.Router(workflow, TenantScopeMiddleware))
	rr := h.request(t, http.MethodPost, h.schoolID, mergeSelection(candidates))
	assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	assert.Equal(t, 2, calls, "failure happened after both person mutations and the first audit write")
	assert.Equal(t, candidates, h.preview(t), "every person and employment change must roll back")
	var audits int
	require.NoError(t, h.db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log WHERE operator_id = ? AND action = 'merge'`, h.operatorID).Scan(testpkg.Ctx(t), &audits))
	assert.Zero(t, audits, "even the first successful audit append must roll back")
}

func TestStaffPersonMergeWithholdsAmbiguousGroups(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"three people", "two accounts", "no account", "both active", "both retired", "extra employment"} {
		t.Run(scenario, func(t *testing.T) {
			testpkg.OwnTenant(t)
			h := newStaffMergeHarness(t)
			keeper, donor := h.pair(t, "Prüfung")
			ctx := testpkg.Ctx(t)
			switch scenario {
			case "three people":
				testpkg.CreateTestPerson(t, h.db, "Prüfung", "Doppelsatz")
			case "two accounts":
				account := testpkg.CreateTestAccount(t, h.db, "second-merge@example.test")
				_, err := h.db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE tenant_id = ? AND id = ?`, account.ID, h.schoolID, donor.PersonID).Exec(ctx)
				require.NoError(t, err)
			case "no account":
				_, err := h.db.NewRaw(`UPDATE users.persons SET account_id = NULL WHERE tenant_id = ? AND id = ?`, h.schoolID, keeper.PersonID).Exec(ctx)
				require.NoError(t, err)
			case "both active":
				_, err := h.db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = NULL WHERE tenant_id = ? AND id = ?`, h.schoolID, donor.ID).Exec(ctx)
				require.NoError(t, err)
			case "both retired":
				_, err := h.db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = NOW() WHERE tenant_id = ? AND id = ?`, h.schoolID, keeper.ID).Exec(ctx)
				require.NoError(t, err)
			case "extra employment":
				testpkg.CreateTestStaffForPerson(t, h.db, donor.PersonID)
			}
			assert.Empty(t, h.preview(t))
		})
	}
}

func TestStaffPersonMergeSerializesConcurrentConfirmation(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	h.pair(t, "Parallel")
	input := mergeSelection(h.preview(t))
	start := make(chan struct{})
	statuses := make(chan int, 2)
	for range 2 {
		go func() { <-start; statuses <- h.request(t, http.MethodPost, h.schoolID, input).Code }()
	}
	close(start)
	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, []int{<-statuses, <-statuses})
	var audits int
	require.NoError(t, h.db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log WHERE operator_id = ? AND action = 'merge'`, h.operatorID).Scan(testpkg.Ctx(t), &audits))
	assert.Equal(t, 1, audits)
}

func TestStaffPersonMergePreviewUsesTwoBulkReads(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	for _, name := range []string{"Anna", "Ben", "Cem"} {
		h.pair(t, name)
	}
	counter := testpkg.CaptureQueriesForContext(t, h.db)
	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/%d", h.schoolID), nil, testutil.WithClaims(t, sessiontoken.AppClaims{ID: int(h.operatorID), Scope: "platform"}))
	rr := testutil.ExecuteRequest(h.router, req.WithContext(counter.Context(req.Context())))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	testpkg.AssertQueryBudget(t, "api.operator.staff_person_merge.preview", counter.Selects(""))
}
