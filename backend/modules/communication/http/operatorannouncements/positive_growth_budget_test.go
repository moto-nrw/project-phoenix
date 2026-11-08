package operatorannouncements_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	compose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	operatorHTTP "github.com/moto-nrw/project-phoenix/modules/communication/http/operatorannouncements"
	platformHTTP "github.com/moto-nrw/project-phoenix/modules/communication/http/platformannouncements"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

type budgetViewerNames struct{ query peopledirectory.Query }

func (v budgetViewerNames) NamesByAccount(ctx context.Context, ids []int64) (map[int64]string, compose.OperationStats, error) {
	rows, err := v.query.ListPersonsByAccount(ctx, ids)
	names := make(map[int64]string, len(rows))
	for _, row := range rows {
		if row.AccountID != nil {
			names[*row.AccountID] = row.FullName()
		}
	}
	return names, compose.OperationStats{Queries: 1, Rows: int64(len(rows))}, err
}
func TestMain(m *testing.M) { testpkg.PerTestTenants(); testpkg.Run(m) }

func TestPlatformAnnouncementPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	people := owners.MustNewPeopleDirectory(database)
	module, err := api.NewCommunicationAnnouncements(compose.Dependencies{DB: db, Organizations: organizations, Viewers: budgetViewerNames{people}, Audit: api.NewCommunicationAnnouncementAudit(owners.NewOperatorAuditLog(database).Create), Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	var ids []int64
	// Global rows are removed through the real command before operator cleanup.
	t.Cleanup(func() {
		for _, id := range ids {
			require.NoError(t, module.DeleteAnnouncement(ctx, id, operator.ID, nil))
		}
	})
	_, viewer := testpkg.CreateTestPersonWithAccount(t, db, "Budget", "Viewer")
	testpkg.EnsureAccountTenant(t, db, viewer.ID, testpkg.Tenant(t))
	var organizationID int64
	require.NoError(t, db.NewRaw("SELECT organization_id FROM platform.schools WHERE id = ?", testpkg.Tenant(t)).Scan(ctx, &organizationID))
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, Roles: []string{"user"}, TenantID: testpkg.Tenant(t), OrgID: organizationID})
	runtime := platformHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router)) {
			router.Group(func(r chi.Router) { api.TenantSessionChain(r); register(r) })
		},
		Viewer: func(r *http.Request) platformHTTP.Viewer {
			claims := sessiontoken.ClaimsFromCtx(r.Context())
			return platformHTTP.Viewer{AccountID: int64(claims.ID), Roles: claims.Roles, TenantID: claims.TenantID, OrgID: claims.OrgID}
		},
		IDParam: common.ParseInt64IDWithError, Success: common.Respond, Failure: func(w http.ResponseWriter, r *http.Request, f platformHTTP.Failure) {
			common.RenderError(w, r, common.ErrorInternalServerWrap(f.Message, f.Err))
		},
	}
	platformRouter := platformHTTP.NewResource(platformHTTP.ResourceConfig{AnnouncementsService: module, Runtime: runtime}).Router()
	operatorResource := operatorHTTP.NewAnnouncementsResource(module)
	operatorRouter := chi.NewRouter()
	operatorRouter.Get("/", operatorResource.ListAnnouncements)
	operatorRouter.Get("/{id}/views", operatorResource.GetViewDetails)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	createdAt := time.Now()
	created := 0
	var viewedID int64
	var expectedTitles []string
	var expectedReaderIDs []int64
	for _, n := range []int{3, 8} {
		for created < n {
			announcement := &communication.Announcement{Title: fmt.Sprintf("Budget-%02d", created), CreatedAt: createdAt.Add(time.Duration(created*2) * time.Second), Content: "Positive growth", Type: communication.TypeAnnouncement, Severity: communication.SeverityInfo, Active: true, TargetTenantIDs: []int64{testpkg.Tenant(t)}}
			require.NoError(t, module.CreateAnnouncement(ctx, announcement, operator.ID, nil))
			ids = append(ids, announcement.ID)
			require.NoError(t, module.PublishAnnouncement(ctx, announcement.ID, operator.ID, nil))
			expectedTitles = append([]string{announcement.Title}, expectedTitles...)
			if created == 0 {
				viewedID = announcement.ID
			}
			_, reader := testpkg.CreateTestPersonWithAccount(t, db, "Reader", fmt.Sprintf("%02d", created))
			testpkg.EnsureAccountTenant(t, db, reader.ID, testpkg.Tenant(t))
			require.NoError(t, module.MarkSeen(ctx, reader.ID, viewedID))
			expectedReaderIDs = append(expectedReaderIDs, reader.ID)
			// Targeted foreign-school announcements must not leak into unread DTOs.
			foreign := &communication.Announcement{Title: fmt.Sprintf("Foreign-%02d", created), CreatedAt: createdAt.Add(time.Duration(created*2+1) * time.Second), Content: "Excluded", Type: communication.TypeAnnouncement, Severity: communication.SeverityInfo, Active: true, TargetTenantIDs: []int64{foreignTenant}}
			require.NoError(t, module.CreateAnnouncement(ctx, foreign, operator.ID, nil))
			ids = append(ids, foreign.ID)
			require.NoError(t, module.PublishAnnouncement(ctx, foreign.ID, operator.ID, nil))
			created++
		}
		for _, entry := range []struct {
			name, path string
			router     chi.Router
		}{{"unread", "/announcements/unread", testpkg.MountedQueryBudgetRouter("/api/platform", platformRouter)}, {"list", "/", testpkg.MountedQueryBudgetRouter("/operator/announcements", operatorRouter)}, {"views", fmt.Sprintf("/%d/views", viewedID), testpkg.MountedQueryBudgetRouter("/operator/announcements", operatorRouter)}} {
			req := httptest.NewRequest("GET", entry.path, nil).WithContext(counter.Context(ctx))
			req.Header.Set("Authorization", "Bearer "+token)
			counter.Reset()
			counter.Start()
			rec := testutil.ExecuteRequestForTest(t, entry.router, req)
			counter.Stop()
			require.Equal(t, 200, rec.Code, rec.Body.String())
			t.Logf("MEASURE platform_announcements %s N%d=%d", entry.name, n, counter.Total())
			switch entry.name {
			case "unread":
				var out struct {
					Data []platformHTTP.AnnouncementResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data, n)
				for i, row := range out.Data {
					require.Equal(t, expectedTitles[i], row.Title)
					require.Positive(t, row.ID)
					require.Equal(t, "Positive growth", row.Content)
				}
			case "list":
				var out struct {
					Data []operatorHTTP.AnnouncementResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data, n*2)
				for i, row := range out.Data {
					require.Equal(t, ids[len(ids)-1-i], row.ID)
					require.Contains(t, ids, row.ID)
					require.Equal(t, "published", row.Status)
				}
			case "views":
				var out struct {
					Data []operatorHTTP.AnnouncementViewDetailResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data, n)
				for _, row := range out.Data {
					require.Contains(t, expectedReaderIDs, row.UserID)
					require.Contains(t, row.UserName, "Reader")
					require.NotEmpty(t, row.SeenAt)
					require.False(t, row.Dismissed)
				}
			}
			switch entry.name {
			case "unread":
				querybudgets.AssertQueryBudget(t, "communication.platform_announcements.unread.positive_growth", counter.Measurement(counter.Queries()))
			case "list":
				querybudgets.AssertQueryBudget(t, "communication.platform_announcements.list.positive_growth", counter.Measurement(counter.Queries()))
			case "views":
				querybudgets.AssertQueryBudget(t, "communication.platform_announcements.views.positive_growth", counter.Measurement(counter.Queries()))
			}
		}
	}
}
