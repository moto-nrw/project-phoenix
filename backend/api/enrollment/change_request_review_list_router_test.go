package enrollment_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Anmeldungsänderungen in the request module (#2435), pinned at the HTTP seam:
// the assembled router with the real middleware chain, the real service and
// the real keyset SQL. Until this endpoint existed, a decided change request
// left the surface for good — the history is the whole point.

type reviewListEnv struct {
	db        *bun.DB
	router    http.Handler
	tenantID  int64
	requestID int64
	childIDs  []int64
	accountID int64
	token     string
}

func setupReviewListTest(t *testing.T) *reviewListEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	settings := stubTakeoverSettings{}

	var accountID int64
	require.NoError(t, db.NewSelect().
		TableExpr("auth.accounts").
		ColumnExpr("id").
		OrderExpr("id").
		Limit(1).
		Scan(ctx, &accountID))
	reviewer := &usersModels.Person{
		FirstName: "Rita",
		LastName:  "Ruecksicht",
		AccountID: &accountID,
	}
	reviewer.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().Model(reviewer).ModelTableExpr("users.persons").Scan(ctx))

	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular "+t.Name(), []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, accountID)
	require.NoError(t, err)

	phase := &enrollmentModels.Phase{
		Name:             "review-list-" + t.Name(),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2026, 9, 1),
		ServiceEndDate:   timezone.NewDate(2027, 7, 31),
		IsActive:         true,
		FormSchemaID:     &schema.ID,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.SetTenantID(tenantID)
	require.NoError(t, repos.Phase.Create(ctx, phase))

	requestSvc := enrollmentService.NewRequestService(enrollmentService.RequestServiceConfig{
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestGuardianRepo:      repos.RequestGuardian,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		FormSchemaRepo:           repos.FormSchema,
		PhaseRepo:                repos.Phase,
		SchoolRepo:               repos.School,
		RateLimitRepo:            repos.SubmissionRateLimit,
		OutboxEnqueuer:           discardingOutbox{},
		Settings:                 settings,
		FrontendURL:              "http://localhost:3000",
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       db,
		Logger:                   slog.Default(),
	})
	changeRequestSvc := enrollmentService.NewChangeRequestService(enrollmentService.ChangeRequestServiceConfig{
		ChangeRequestRepo:        repos.ChangeRequest,
		MessageRepo:              repos.ChangeRequestMessage,
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestGuardianRepo:      repos.RequestGuardian,
		LateInviteRepo:           repos.LateInvite,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		FormSchemaRepo:           repos.FormSchema,
		PhaseRepo:                repos.Phase,
		SchoolRepo:               repos.School,
		GuardianProfileRepo:      repos.GuardianProfile,
		GuardianPhoneRepo:        repos.GuardianPhoneNumber,
		PersonRepo:               repos.Person,
		StudentRepo:              repos.Student,
		GuardianAuthorizer:       repos.StudentGuardian,
		Settings:                 settings,
		OutboxEnqueuer:           discardingOutbox{},
		FrontendURL:              "http://localhost:3000",
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       db,
		Logger:                   slog.Default(),
	})

	resource := enrollmentAPI.NewResource(
		nil, nil, requestSvc, nil, nil, nil, nil, nil, changeRequestSvc,
		nil, nil, nil, nil, db,
	)

	submitted, err := requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          tenantID,
		PhaseID:           phase.ID,
		GuardianFirstName: "Annegret",
		GuardianLastName:  "Zoffenbach",
		GuardianEmail:     "review-list@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           false,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Quirina",
				LastName:         "Zoffenbach",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(1),
			},
			{
				FirstName:        "Xander",
				LastName:         "Zoffenbach",
				DateOfBirth:      timezone.NewDate(2019, 6, 2),
				TargetGradeLevel: testpkg.Int16Ptr(1),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 2)

	env := &reviewListEnv{
		db:        db,
		router:    testpkg.TenantRuntimeMiddleware(t, db)(resource.Router()),
		tenantID:  tenantID,
		requestID: submitted.Request.ID,
		childIDs:  []int64{submitted.Children[0].ID, submitted.Children[1].ID},
		accountID: accountID,
	}
	claims := jwt.AppClaims{
		ID:          int(accountID),
		Sub:         "review-list@example.com",
		Username:    "reviewer",
		FirstName:   "Rita",
		LastName:    "Ruecksicht",
		Roles:       []string{"admin"},
		Permissions: []string{"config:manage", "config:read"},
		TenantID:    tenantID,
	}
	env.token = testutil.MintTestJWT(t, claims)

	return env
}

// insertChangeRequest writes one row with controlled instants so the keyset
// order is deterministic. childID pins the affected child; 0 leaves it open.
func (env *reviewListEnv) insertChangeRequest(
	t *testing.T,
	status string,
	occurredAt time.Time,
	childID int64,
	decided bool,
) *enrollmentModels.ChangeRequest {
	t.Helper()
	row := &enrollmentModels.ChangeRequest{
		RequestID:        env.requestID,
		Origin:           enrollmentModels.ChangeRequestOriginParent,
		Status:           status,
		BaseSnapshot:     map[string]any{"guardian_first_name": "Annegret"},
		ProposedSnapshot: map[string]any{"guardian_first_name": "Anne"},
		Diff:             map[string]any{"changed": []string{"guardian_first_name"}},
	}
	if childID > 0 {
		row.RequestChildID = &childID
	}
	if decided {
		row.ReviewedByAccountID = &env.accountID
		row.ReviewedAt = &occurredAt
		row.CreatedAt = occurredAt.Add(-time.Hour)
		row.UpdatedAt = occurredAt
	} else {
		row.CreatedAt = occurredAt
		row.UpdatedAt = occurredAt
	}
	row.SetTenantID(env.tenantID)
	_, err := env.db.NewInsert().
		Model(row).
		ModelTableExpr("enrollment.change_requests").
		Exec(testpkg.TenantContext(env.tenantID))
	require.NoError(t, err)
	return row
}

type reviewListEnvelope struct {
	Data struct {
		Items []struct {
			RequestType string    `json:"request_type"`
			OccurredAt  time.Time `json:"occurred_at"`
			Data        struct {
				ID         string   `json:"id"`
				Status     string   `json:"status"`
				ChildNames []string `json:"child_names"`
				Children   []struct {
					CaseID    string  `json:"case_id"`
					StudentID *string `json:"student_id"`
					Name      string  `json:"name"`
				} `json:"children"`
				GuardianName     string         `json:"guardian_name"`
				DecidedByName    string         `json:"decided_by_name"`
				BaseSnapshot     map[string]any `json:"base_snapshot"`
				ProposedSnapshot map[string]any `json:"proposed_snapshot"`
				Diff             map[string]any `json:"diff"`
				DecidedAt        *string        `json:"decided_at"`
			} `json:"data"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	} `json:"data"`
}

func (env *reviewListEnv) getPath(t *testing.T, path string, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func (env *reviewListEnv) get(t *testing.T, query string, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/change-requests/list?"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func (env *reviewListEnv) fetch(t *testing.T, query string) reviewListEnvelope {
	t.Helper()
	rec := env.get(t, query, env.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out reviewListEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out
}

func idsOf(page reviewListEnvelope) []string {
	ids := make([]string, 0, len(page.Data.Items))
	for _, item := range page.Data.Items {
		ids = append(ids, item.Data.ID)
	}
	return ids
}

func idOf(row *enrollmentModels.ChangeRequest) string {
	return strconv.FormatInt(row.ID, 10)
}

func TestChangeRequestReviewList_OpenShowsUndecidedNewestFirst(t *testing.T) {
	t.Parallel()

	env := setupReviewListTest(t)
	base := time.Now().UTC().Add(-2 * time.Hour)

	newer := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusPendingReview, base, 0, false)
	older := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusNeedsParentResponse, base.Add(-time.Minute), 0, false)
	env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusApproved, base.Add(-2*time.Minute), 0, true)

	page := env.fetch(t, "view=open")
	require.Equal(t, []string{idOf(newer), idOf(older)}, idsOf(page),
		"the open list carries exactly the undecided rows, newest first")
	assert.Equal(t, "enrollment", page.Data.Items[0].RequestType)
	assert.Equal(t, []string{"Quirina Zoffenbach", "Xander Zoffenbach"}, page.Data.Items[0].Data.ChildNames)
	require.Len(t, page.Data.Items[0].Data.Children, 2)
	assert.NotEmpty(t, page.Data.Items[0].Data.Children[0].CaseID)
	assert.Equal(t, "Quirina Zoffenbach", page.Data.Items[0].Data.Children[0].Name)
	assert.Equal(t, "Annegret Zoffenbach", page.Data.Items[0].Data.GuardianName)
	assert.Nil(t, page.Data.Items[0].Data.DecidedAt, "an open request has no decision")
	// The list carries what the detail view compares, so it can show the real
	// before → after instead of only naming the changed areas.
	assert.Equal(t, "Annegret", page.Data.Items[0].Data.BaseSnapshot["guardian_first_name"])
	assert.Equal(t, "Anne", page.Data.Items[0].Data.ProposedSnapshot["guardian_first_name"])
	assert.Equal(t, []any{"guardian_first_name"}, page.Data.Items[0].Data.Diff["changed"])
}

func TestChangeRequestReviewList_HistoryCarriesDecisionAndCursor(t *testing.T) {
	t.Parallel()

	env := setupReviewListTest(t)
	base := time.Now().UTC().Add(-2 * time.Hour)

	newest := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusApproved, base, 0, true)
	middle := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusRejected, base.Add(-time.Minute), 0, true)
	oldest := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusApproved, base.Add(-2*time.Minute), 0, true)
	env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusPendingReview, base.Add(time.Minute), 0, false)

	pageOne := env.fetch(t, "view=history&limit=2")
	require.Equal(t, []string{idOf(newest), idOf(middle)}, idsOf(pageOne),
		"decided requests are nachschlagbar, newest decision first")
	assert.Equal(t, "Rita Ruecksicht", pageOne.Data.Items[0].Data.DecidedByName)
	require.NotNil(t, pageOne.Data.Items[0].Data.DecidedAt)
	require.NotEmpty(t, pageOne.Data.NextCursor)

	pageTwo := env.fetch(t, "view=history&limit=2&cursor="+url.QueryEscape(pageOne.Data.NextCursor))
	assert.Equal(t, []string{idOf(oldest)}, idsOf(pageTwo))
	assert.Empty(t, pageTwo.Data.NextCursor, "the last page ends the keyset")
}

func TestChangeRequestReviewList_HistoryUsesDecisionInstantForOrderAndCursor(t *testing.T) {
	t.Parallel()

	env := setupReviewListTest(t)
	base := time.Now().UTC().Add(-2 * time.Hour)

	newerDecision := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusApproved, base, 0, true)
	olderDecision := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusRejected, base.Add(-time.Minute), 0, true)
	_, err := env.db.NewUpdate().
		Model((*enrollmentModels.ChangeRequest)(nil)).
		ModelTableExpr("enrollment.change_requests").
		Set("updated_at = ?", base.Add(time.Hour)).
		Where("id = ?", olderDecision.ID).
		Exec(testpkg.TenantContext(env.tenantID))
	require.NoError(t, err)

	pageOne := env.fetch(t, "view=history&limit=1")
	require.Equal(t, []string{idOf(newerDecision)}, idsOf(pageOne))
	require.Len(t, pageOne.Data.Items, 1)
	assert.Equal(t,
		newerDecision.DecisionInstant().UTC().Truncate(time.Microsecond),
		pageOne.Data.Items[0].OccurredAt.UTC(),
	)

	pageTwo := env.fetch(t, "view=history&limit=1&cursor="+url.QueryEscape(pageOne.Data.NextCursor))
	assert.Equal(t, []string{idOf(olderDecision)}, idsOf(pageTwo))
}

func TestChangeRequestReviewList_FiltersByNameStatusAndPeriod(t *testing.T) {
	t.Parallel()

	env := setupReviewListTest(t)
	base := timezone.NewDate(2026, 8, 19).BerlinMidnight().Add(12 * time.Hour)

	approved := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusApproved, base, env.childIDs[0], true)
	rejected := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusRejected, base.Add(-time.Minute), env.childIDs[1], true)

	// A request pinned to one child is found by that child's name only.
	byChild := env.fetch(t, "view=history&search=Quirina")
	assert.Equal(t, []string{idOf(approved)}, idsOf(byChild))
	assert.Equal(t, []string{"Quirina Zoffenbach"}, byChild.Data.Items[0].Data.ChildNames)

	byStatus := env.fetch(t, "view=history&status=rejected")
	assert.Equal(t, []string{idOf(rejected)}, idsOf(byStatus))

	today := timezone.DateFromTime(base).String()
	inPeriod := env.fetch(t, "view=history&from="+today+"&to="+today)
	assert.Equal(t, []string{idOf(approved), idOf(rejected)}, idsOf(inPeriod))

	tomorrow := timezone.DateFromTime(base).AddDays(1).String()
	outsidePeriod := env.fetch(t, "view=history&from="+tomorrow)
	assert.Empty(t, idsOf(outsidePeriod), "nothing was decided after the window")

	// Eine zurückgezogene Anfrage darf nicht aus jeder Liste fallen: sie steht
	// in der Historie und trägt in der gemeinsamen Liste den Status
	// „zurückgezogen".
	cancelled := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusCancelled, base.Add(-2*time.Minute), 0, true)
	assert.Equal(t, []string{idOf(cancelled)}, idsOf(env.fetch(t, "view=history&status=withdrawn")))
	assert.Contains(t, idsOf(env.fetch(t, "view=history")), idOf(cancelled))
}

func TestChangeRequestReviewList_EscapesNameSearchWildcards(t *testing.T) {
	t.Parallel()

	env := setupReviewListTest(t)
	base := time.Now().UTC().Add(-2 * time.Hour)

	percent := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusApproved, base, env.childIDs[0], true)
	underscore := env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusRejected, base.Add(-time.Minute), env.childIDs[1], true)
	_, err := env.db.NewUpdate().
		TableExpr("enrollment.request_children").
		Set("first_name = ?", `Qui%rina\`).
		Where("id = ?", env.childIDs[0]).
		Exec(testpkg.TenantContext(env.tenantID))
	require.NoError(t, err)
	_, err = env.db.NewUpdate().
		TableExpr("enrollment.request_children").
		Set("first_name = ?", "Xa_nder").
		Where("id = ?", env.childIDs[1]).
		Exec(testpkg.TenantContext(env.tenantID))
	require.NoError(t, err)

	assert.Equal(t, []string{idOf(percent)}, idsOf(env.fetch(t, "view=history&search="+url.QueryEscape("%"))))
	assert.Equal(t, []string{idOf(underscore)}, idsOf(env.fetch(t, "view=history&search="+url.QueryEscape("_"))))
	assert.Equal(t, []string{idOf(percent)}, idsOf(env.fetch(t, "view=history&search="+url.QueryEscape(`\`))))
}

// Der Zähler trägt das Badge am Anfragen-Eintrag; er zählt in der Datenbank,
// nicht die Länge einer Seite — sonst stünde ab der Seitengröße dauerhaft
// dieselbe Zahl dort.
func TestChangeRequestReviewCount_CountsOpenBeyondOnePage(t *testing.T) {
	t.Parallel()

	env := setupReviewListTest(t)
	base := time.Now().UTC().Add(-3 * time.Hour)

	for i := range 3 {
		env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusPendingReview, base.Add(time.Duration(i)*time.Minute), 0, false)
	}
	env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusNeedsParentResponse, base.Add(-time.Minute), 0, false)
	// Entschiedene zählen nicht mit: da wartet keine Arbeit mehr.
	env.insertChangeRequest(t, enrollmentModels.ChangeRequestStatusApproved, base.Add(-2*time.Minute), 0, true)

	rec := env.get(t, "", env.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	countRec := env.getPath(t, "/admin/change-requests/pending-count", env.token)
	require.Equal(t, http.StatusOK, countRec.Code, countRec.Body.String())
	var out struct {
		Data struct {
			PendingCount int `json:"pending_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(countRec.Body.Bytes(), &out), countRec.Body.String())
	assert.Equal(t, 3, out.Data.PendingCount)

	// Auch der Zähler hält seine Berechtigungsgrenze.
	withoutPermission := testutil.MintTestJWT(t, jwt.AppClaims{
		ID:          int(env.accountID),
		Sub:         "review-list@example.com",
		Username:    "reviewer",
		Roles:       []string{"user"},
		Permissions: []string{"users:read", "users:update"},
		TenantID:    env.tenantID,
	})
	denied := env.getPath(t, "/admin/change-requests/pending-count", withoutPermission)
	assert.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
}

func TestChangeRequestReviewList_RefusesBadQueryAndMissingPermission(t *testing.T) {
	t.Parallel()

	env := setupReviewListTest(t)

	for _, query := range []string{
		"view=nonsense",
		"view=open&status=approved",
		"view=open&from=2026-08-01",
		"view=history&status=erledigt",
		"view=history&from=2026-08-10&to=2026-08-01",
		"view=history&cursor=not-a-cursor",
		"view=history&limit=0",
	} {
		rec := env.get(t, query, env.token)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "query %q: %s", query, rec.Body.String())
	}

	// The queue keeps its own permission boundary: config:manage, nothing else.
	withoutPermission := testutil.MintTestJWT(t, jwt.AppClaims{
		ID:          int(env.accountID),
		Sub:         "review-list@example.com",
		Username:    "reviewer",
		Roles:       []string{"user"},
		Permissions: []string{"users:read", "users:update", "config:read"},
		TenantID:    env.tenantID,
	})
	rec := env.get(t, "view=history", withoutPermission)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}
