package files_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The parent attachment contract of ADR 0161 step 3 (#5894), observed over
// HTTP on a real database. Behaviour assertions (status, body, refusals, the
// statement after a failure) hold on the code before the bundle as well; the
// frame-shape test names the shared school unit explicitly.

type parentAttachmentRig struct {
	db             *bun.DB
	router         chi.Router
	announcementID int64
	attachmentID   string
	parentToken    string
	foreignToken   string
	strangerToken  string
	tenantID       int64
}

func newParentAttachmentRig(t *testing.T) parentAttachmentRig {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	routes, err := Communication18AttachmentRouterForTest(t, ctx, db)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/", routes)
	_, admin := testpkg.CreateTestStaffWithAccount(t, db, "Attachment", "Frames")
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	stranger := testpkg.CreateTestParentGuardianChain(t, db)
	announcement := testpkg.CreateTestParentAnnouncement(t, db, admin.ID, "Attachment frames")
	_, err = db.NewRaw(`INSERT INTO users.parent_announcement_targets (tenant_id,announcement_id,target_type,target_ref_id) VALUES (?,?,'student',?)`, testpkg.Tenant(t), announcement.ID, chain.StudentID).Exec(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, Communication18AttachmentCleanupForTest(testpkg.Tenant(t))) })

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "Elternbrief.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/announcement-attachments/%d", announcement.ID), &body)
	req.Header.Set("Authorization", "Bearer "+communication18AttachmentToken(t, admin.ID, admin.Email, testpkg.Tenant(t), "", []string{"user"}, []string{permissions.AdminWildcard}))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := testutil.ExecuteRequestForTest(t, router, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct{ Data struct{ ID string } }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	_, err = db.NewRaw(`UPDATE users.parent_announcements SET published_at = ? WHERE id = ?`, time.Now().Add(-time.Hour), announcement.ID).Exec(ctx)
	require.NoError(t, err)

	var foreign testpkg.ParentChain
	t.Run("school B", func(t *testing.T) {
		testpkg.EnsureTestTenant(t, db, testpkg.OwnTenant(t))
		foreign = testpkg.CreateTestParentGuardianChain(t, db)
	})
	return parentAttachmentRig{
		db: db, router: router, announcementID: announcement.ID, attachmentID: created.Data.ID, tenantID: testpkg.Tenant(t),
		parentToken:   communication18AttachmentToken(t, chain.AccountID, chain.Email, 0, "parent", []string{"parent"}, nil),
		foreignToken:  communication18AttachmentToken(t, foreign.AccountID, foreign.Email, 0, "parent", []string{"parent"}, nil),
		strangerToken: communication18AttachmentToken(t, stranger.AccountID, stranger.Email, 0, "parent", []string{"parent"}, nil),
	}
}

func (r parentAttachmentRig) listPath() string {
	return fmt.Sprintf("/parent-news-attachments/%d", r.announcementID)
}

func (r parentAttachmentRig) downloadPath() string {
	return fmt.Sprintf("/parent-news-attachments/%d/%s/download", r.announcementID, r.attachmentID)
}

func (r parentAttachmentRig) get(t *testing.T, path, token string) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	counter := testpkg.CaptureQueries(t, r.db)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := testutil.ExecuteRequestForTest(t, r.router, req)
	queries := counter.Queries()
	counter.Stop()
	return rec, queries
}

func (r parentAttachmentRig) newsEnabled(t *testing.T, on bool) {
	t.Helper()
	_, err := r.db.NewRaw(`UPDATE config.setting_values SET value = ?::jsonb WHERE tenant_id = ? AND setting_key = 'operations.parent_news_enabled'`, fmt.Sprint(on), r.tenantID).Exec(context.Background())
	require.NoError(t, err)
}

func isSwitchRead(q string) bool { return strings.Contains(q, `"setting_value"`) }

func isAttachmentRowRead(q string) bool { return strings.Contains(q, `"document"."id"`) }

func isAudienceRead(q string) bool { return strings.Contains(q, `"parent_announcement"."id"`) }

func isAttachmentTenantSwitch(q string) bool {
	return strings.Contains(q, "set_config('app.current_tenant_id'")
}

// attachmentUnit is one transaction of a request: its scope and statements.
type attachmentUnit struct {
	admin      bool
	statements []string
	end        string
}

// attachmentUnits splits a request's statements into its transactions.
func attachmentUnits(queries []string) []attachmentUnit {
	var units []attachmentUnit
	var current *attachmentUnit
	for _, q := range queries {
		switch {
		case q == "BEGIN":
			units = append(units, attachmentUnit{})
			current = &units[len(units)-1]
		case current == nil:
		case q == "COMMIT" || q == "ROLLBACK":
			current.end = q
			current = nil
		case q == "SET LOCAL ROLE phoenix_admin":
			current.admin = true
		default:
			current.statements = append(current.statements, q)
		}
	}
	return units
}

func unitShape(units []attachmentUnit) string {
	parts := make([]string, 0, len(units))
	for _, unit := range units {
		scope := "tenant"
		if unit.admin {
			scope = "admin"
		}
		parts = append(parts, scope)
	}
	return strings.Join(parts, ",")
}

func attachmentIndex(queries []string, match func(string) bool) int {
	for i, q := range queries {
		if match(q) {
			return i
		}
	}
	return -1
}

func attachmentRefusal(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	require.Equal(t, "general.input", body["code"])
	delete(body, "instance")
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return fmt.Sprintf("%d %s %s", rec.Code, rec.Header().Get("Content-Type"), raw)
}

// TestParentAttachmentSwitchAndRowsShareOneSchoolUnit pins step 3: the
// first pass is the audience (admin) and one school unit that reads the
// switch before the attachment row; the check after the read stays one
// administrative unit, the download's byte read its own school unit. These
// are the frames of ADR 0161 minus the unit between switch and row.
func TestParentAttachmentSwitchAndRowsShareOneSchoolUnit(t *testing.T) {
	t.Parallel()
	rig := newParentAttachmentRig(t)

	for _, tc := range []struct {
		name, path, shape string
		statements        int
	}{
		{"list", rig.listPath(), "admin,tenant,admin", 18},
		{"download", rig.downloadPath(), "admin,tenant,admin,tenant", 22},
	} {
		rec, queries := rig.get(t, tc.path, rig.parentToken)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", tc.name, rec.Body.String())
		units := attachmentUnits(queries)
		require.Equal(t, tc.shape, unitShape(units), "%s: %q", tc.name, queries)
		require.Len(t, queries, tc.statements, "%s: %q", tc.name, queries)
		first := units[1].statements
		require.True(t, isAttachmentTenantSwitch(first[0]), "%s: %q", tc.name, first)
		switchRead, rowRead := attachmentIndex(first, isSwitchRead), attachmentIndex(first, isAttachmentRowRead)
		require.True(t, switchRead > 0 && switchRead < rowRead, "%s: switch before row in one unit: %q", tc.name, first)
		require.Equal(t, "COMMIT", units[1].end, tc.name)
		// The check after the read asks the audience again; the switch comes
		// from the request's settings cache.
		require.True(t, units[2].admin, tc.name)
		require.Equal(t, -1, attachmentIndex(units[2].statements, isSwitchRead), "%s: %q", tc.name, units[2].statements)
		require.GreaterOrEqual(t, attachmentIndex(units[2].statements, isAudienceRead), 0, tc.name)
	}

	// A download of attachment id 0 is refused after the switch and before
	// the row, and the shared unit ends with ROLLBACK.
	_, queries := rig.get(t, fmt.Sprintf("/parent-news-attachments/%d/0/download", rig.announcementID), rig.parentToken)
	units := attachmentUnits(queries)
	require.Equal(t, "admin,tenant", unitShape(units), "%q", queries)
	require.GreaterOrEqual(t, attachmentIndex(units[1].statements, isSwitchRead), 0, "%q", queries)
	require.Equal(t, -1, attachmentIndex(units[1].statements, isAttachmentRowRead), "%q", queries)
	require.Equal(t, "ROLLBACK", units[1].end, "%q", queries)

	// The pooled connection keeps neither role nor school, after a commit as
	// after a rollback.
	rig.db.SetMaxIdleConns(1)
	rig.db.SetMaxOpenConns(1)
	var loginRole string
	require.NoError(t, rig.db.NewRaw("SELECT current_user").Scan(context.Background(), &loginRole))
	for _, path := range []string{rig.listPath(), fmt.Sprintf("/parent-news-attachments/%d/0/download", rig.announcementID)} {
		rig.get(t, path, rig.parentToken)
		var role, school string
		require.NoError(t, rig.db.NewRaw("SELECT current_user, coalesce(current_setting('app.current_tenant_id', true), '')").Scan(context.Background(), &role, &school))
		require.Equal(t, loginRole, role)
		require.Empty(t, school)
	}
}

// TestParentAttachmentRefusalsKeepTheirAnswer: outside the audience, another
// school's parent, the switch off, a draft and a missing announcement all get
// the same 404, and none of them reads an attachment row.
func TestParentAttachmentRefusalsKeepTheirAnswer(t *testing.T) {
	t.Parallel()
	rig := newParentAttachmentRig(t)
	ctx := context.Background()
	var missing int64
	require.NoError(t, rig.db.NewRaw("SELECT nextval(pg_get_serial_sequence('users.parent_announcements', 'id'))").Scan(ctx, &missing))

	for _, route := range []struct {
		name    string
		path    func(announcementID int64) string
		allowed int
	}{
		{"list", func(id int64) string { return fmt.Sprintf("/parent-news-attachments/%d", id) }, http.StatusOK},
		{"download", func(id int64) string {
			return fmt.Sprintf("/parent-news-attachments/%d/%s/download", id, rig.attachmentID)
		}, http.StatusOK},
	} {
		unknownRec, _ := rig.get(t, route.path(missing), rig.parentToken)
		unknown := attachmentRefusal(t, unknownRec)
		check := func(who string, rec *httptest.ResponseRecorder, queries []string) {
			t.Helper()
			require.Equal(t, unknown, attachmentRefusal(t, rec), "%s / %s", route.name, who)
			require.Equal(t, -1, attachmentIndex(queries, isAttachmentRowRead), "%s / %s reads no row: %q", route.name, who, queries)
		}
		rec, queries := rig.get(t, route.path(rig.announcementID), rig.strangerToken)
		check("outside the audience", rec, queries)
		require.Equal(t, -1, attachmentIndex(queries, isAttachmentTenantSwitch), "%q", queries)
		rec, queries = rig.get(t, route.path(rig.announcementID), rig.foreignToken)
		check("parent of another school", rec, queries)
		require.Equal(t, -1, attachmentIndex(queries, isAttachmentTenantSwitch), "%q", queries)

		rig.newsEnabled(t, false)
		rec, queries = rig.get(t, route.path(rig.announcementID), rig.parentToken)
		rig.newsEnabled(t, true)
		check("switch off", rec, queries)
		require.GreaterOrEqual(t, attachmentIndex(queries, isSwitchRead), 0, "%q", queries)

		_, err := rig.db.NewRaw(`UPDATE users.parent_announcements SET published_at = NULL WHERE id = ?`, rig.announcementID).Exec(ctx)
		require.NoError(t, err)
		rec, queries = rig.get(t, route.path(rig.announcementID), rig.parentToken)
		_, err = rig.db.NewRaw(`UPDATE users.parent_announcements SET published_at = ? WHERE id = ?`, time.Now().Add(-time.Hour), rig.announcementID).Exec(ctx)
		require.NoError(t, err)
		check("draft", rec, queries)

		ok, _ := rig.get(t, route.path(rig.announcementID), rig.parentToken)
		require.Equal(t, route.allowed, ok.Code, "%s: %s", route.name, ok.Body.String())
	}
}

// TestParentAttachmentDatabaseFaults injects a real database failure into
// the audience, the switch and the row read. The audience and the row answer
// 500, the unreadable switch hides the attachment like before, and no
// statement but ROLLBACK follows the failing one.
func TestParentAttachmentDatabaseFaults(t *testing.T) {
	t.Parallel()
	rig := newParentAttachmentRig(t)

	for _, fault := range []struct {
		name, table, role string
		failing           func(string) bool
		status            int
		code              string
	}{
		{"audience", "users.parent_announcements", "phoenix_admin", isAudienceRead, http.StatusInternalServerError, "general.server"},
		{"switch", "config.setting_values", "phoenix_tenant", isSwitchRead, http.StatusNotFound, "general.input"},
		{"attachment row", "documents.announcement_attachments", "phoenix_tenant", isAttachmentRowRead, http.StatusInternalServerError, "general.server"},
	} {
		for _, path := range []string{rig.listPath(), rig.downloadPath()} {
			t.Run(fault.name+" "+path, func(t *testing.T) {
				ctx := context.Background()
				_, err := rig.db.NewRaw("REVOKE SELECT ON " + fault.table + " FROM " + fault.role).Exec(ctx)
				require.NoError(t, err)
				rec, queries := rig.get(t, path, rig.parentToken)
				_, err = rig.db.NewRaw("GRANT SELECT ON " + fault.table + " TO " + fault.role).Exec(ctx)
				require.NoError(t, err)

				require.Equal(t, fault.status, rec.Code, rec.Body.String())
				var body map[string]any
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.Equal(t, fault.code, body["code"])
				failed := attachmentIndex(queries, fault.failing)
				require.GreaterOrEqual(t, failed, 0, "%q", queries)
				require.Equal(t, []string{"ROLLBACK"}, queries[failed+1:], "%q", queries)

				ok, _ := rig.get(t, path, rig.parentToken)
				require.Equal(t, http.StatusOK, ok.Code, "grant restored")
			})
		}
	}
}
