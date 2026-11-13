package files_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestCommunication18AttachmentsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	routes, err := Communication18AttachmentRouterForTest(t, ctx, db)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/", routes)
	_, admin := testpkg.CreateTestStaffWithAccount(t, db, "Attachment", "Manager")
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	announcement := testpkg.CreateTestParentAnnouncement(t, db, admin.ID, "Attachment growth")
	_, err = db.NewRaw(`INSERT INTO users.parent_announcement_targets (tenant_id,announcement_id,target_type,target_ref_id) VALUES (?,?,'student',?)`, testpkg.Tenant(t), announcement.ID, chain.StudentID).Exec(ctx)
	require.NoError(t, err)

	staffToken := communication18AttachmentToken(t, admin.ID, admin.Email, testpkg.Tenant(t), "", []string{"user"}, []string{permissions.AdminWildcard})
	parentToken := communication18AttachmentToken(t, chain.AccountID, chain.Email, 0, "parent", []string{"parent"}, nil)
	var foreignFamily testpkg.ParentChain
	t.Run("foreign-family-fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		testpkg.SetupTestDB(t)
		foreignFamily = testpkg.CreateTestParentGuardianChain(t, db)
	})
	foreignTenant := foreignFamily.TenantID
	_, foreign := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", "Attachment")
	foreignStaff := communication18AttachmentToken(t, foreign.ID, foreign.Email, foreignTenant, "", []string{"user"}, []string{permissions.AdminWildcard})
	foreignParent := communication18AttachmentToken(t, foreignFamily.AccountID, foreignFamily.Email, 0, "parent", []string{"parent"}, nil)
	t.Cleanup(func() { require.NoError(t, Communication18AttachmentCleanupForTest(testpkg.Tenant(t))) })
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	request := func(path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil).WithContext(counter.Context(ctx))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return testutil.ExecuteRequestForTest(t, router, req)
	}
	type attachmentDTO struct {
		ID          string    `json:"id"`
		Filename    string    `json:"filename"`
		SizeBytes   int64     `json:"size_bytes"`
		ContentType string    `json:"content_type"`
		UploadedAt  time.Time `json:"uploaded_at"`
	}
	type listDTO struct {
		Attachments []attachmentDTO `json:"attachments"`
		MaxCount    int             `json:"max_count"`
		MaxBytes    int64           `json:"max_bytes"`
		Editable    bool            `json:"editable"`
	}
	attachments := []attachmentDTO{}
	pdf := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")
	for _, n := range []int{3, 5} {
		_, err = db.NewRaw(`UPDATE users.parent_announcements SET published_at = NULL WHERE id = ?`, announcement.ID).Exec(ctx)
		require.NoError(t, err)
		for len(attachments) < n {
			i := len(attachments)
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, e := writer.CreateFormFile("file", fmt.Sprintf("Attachment%02d.pdf", i))
			require.NoError(t, e)
			_, e = part.Write(pdf)
			require.NoError(t, e)
			require.NoError(t, writer.Close())
			req := httptest.NewRequest("POST", fmt.Sprintf("/api/announcement-attachments/%d", announcement.ID), &body)
			req.Header.Set("Authorization", "Bearer "+staffToken)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			rec := testutil.ExecuteRequestForTest(t, router, req)
			require.Equal(t, 201, rec.Code, rec.Body.String())
			var out struct{ Data attachmentDTO }
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
			require.Equal(t, fmt.Sprintf("Attachment%02d.pdf", i), out.Data.Filename)
			require.Equal(t, int64(len(pdf)), out.Data.SizeBytes)
			require.Equal(t, "application/pdf", out.Data.ContentType)
			require.NotEmpty(t, out.Data.ID)
			require.False(t, out.Data.UploadedAt.IsZero())
			attachments = append(attachments, out.Data)
		}
		for _, parent := range []bool{false, true} {
			prefix, token, other := "/api/announcement-attachments", staffToken, foreignStaff
			if parent {
				// This case needs an already-live publication, not the boundary
				// between the database clock and announcementIsLive's host clock.
				_, err = db.NewRaw(`UPDATE users.parent_announcements SET published_at = ? WHERE id = ? AND tenant_id = ?`, time.Now().Add(-time.Hour), announcement.ID, testpkg.Tenant(t)).Exec(ctx)
				require.NoError(t, err)
				prefix, token, other = "/parent-news-attachments", parentToken, foreignParent
			}
			path := fmt.Sprintf("%s/%d", prefix, announcement.ID)
			counter.Reset()
			counter.Start()
			rec := request(path, token)
			counter.Stop()
			require.Equal(t, 200, rec.Code, rec.Body.String())
			var out struct{ Data listDTO }
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
			wantAttachments := slices.Clone(attachments)
			slices.Reverse(wantAttachments)
			require.Equal(t, listDTO{Attachments: wantAttachments, MaxCount: 5, MaxBytes: 25 * 1024 * 1024, Editable: !parent}, out.Data)
			require.Zero(t, counter.WriteRows())
			if parent {
				if n == 3 {
					querybudgets.AssertQueryBudget(t, "communication18.attachments.parent.n3", counter.Measurement(counter.Queries()))
				} else {
					querybudgets.AssertQueryBudget(t, "communication18.attachments.parent.n5", counter.Measurement(counter.Queries()))
				}
			} else {
				if n == 3 {
					querybudgets.AssertQueryBudget(t, "communication18.attachments.staff.n3", counter.Measurement(counter.Queries()))
				} else {
					querybudgets.AssertQueryBudget(t, "communication18.attachments.staff.n5", counter.Measurement(counter.Queries()))
				}
			}
			denied := request(path, other)
			require.Equal(t, 404, denied.Code, denied.Body.String())
			unauth := request(path, "")
			require.Equal(t, http.StatusUnauthorized, unauth.Code, unauth.Body.String())
		}
	}
}

func communication18AttachmentToken(t *testing.T, id int64, subject string, tenantID int64, scope string, roles, perms []string) string {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.ID = int(id)
	claims.Sub = subject
	claims.TenantID = tenantID
	claims.Scope = scope
	claims.Roles = roles
	claims.Permissions = perms
	claims.IsAdmin = false
	return testutil.MintTestJWT(t, claims)
}
