// Router-level tests for /api/files (#2596): folder visibility is resolved
// per viewer (all staff / admins / selected roles and persons), folder
// management is gated on files:manage, uploads by non-managers hang on the
// files.staff_upload_enabled setting, and files are served as attachments.
package filestore_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	filestoreAPI "github.com/moto-nrw/project-phoenix/api/filestore"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var fakePDF = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")

// fakeXLSX carries the two OOXML parts the upload validator keys off.
func fakeXLSX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"[Content_Types].xml": "<Types/>",
		"xl/workbook.xml":     "<workbook/>",
	} {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

type apiContext struct {
	t      *testing.T
	db     *bun.DB
	svc    *services.Factory
	router chi.Router
	// admin holds admin:*; member holds only a plain permission.
	admin  int64
	member int64
}

func setupAPI(t *testing.T) *apiContext {
	t.Helper()
	db, svc := testutil.SetupAPITest(t)
	resource := filestoreAPI.NewResource(svc.FileStore, db, slog.Default())
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/files", resource.Router())

	suffix := time.Now().UnixNano()
	_, adminAccount := testpkg.CreateTestPersonWithAccount(t, db, "Leitung", fmt.Sprintf("Admin-%d", suffix))
	_, memberAccount := testpkg.CreateTestPersonWithAccount(t, db, "Team", fmt.Sprintf("Mitglied-%d", suffix))
	t.Cleanup(func() {
		if pubDir, err := common.ResolvePublicDir(); err == nil {
			_ = os.RemoveAll(filepath.Join(pubDir, "uploads", "files", fmt.Sprint(testpkg.Tenant(t))))
		}
	})
	return &apiContext{t: t, db: db, svc: svc, router: router, admin: adminAccount.ID, member: memberAccount.ID}
}

func (c *apiContext) token(accountID int64, perms ...string) string {
	claims := testutil.DefaultTestClaims()
	claims.ID = int(accountID)
	claims.Permissions = perms
	claims.Roles = []string{"user"}
	claims.IsAdmin = false
	return testutil.MintTestJWT(c.t, claims)
}

func (c *apiContext) do(method, path string, body io.Reader, contentType string, accountID int64, perms ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+c.token(accountID, perms...))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	return rec
}

func (c *apiContext) json(method, path string, payload any, accountID int64, perms ...string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	if payload != nil {
		require.NoError(c.t, json.NewEncoder(&body).Encode(payload))
	}
	return c.do(method, path, &body, "application/json", accountID, perms...)
}

func (c *apiContext) upload(folderID int64, filename string, content []byte, accountID int64, perms ...string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(c.t, err)
	_, err = part.Write(content)
	require.NoError(c.t, err)
	require.NoError(c.t, writer.Close())
	return c.do(http.MethodPost, fmt.Sprintf("/files/folders/%d/files", folderID), &buf, writer.FormDataContentType(), accountID, perms...)
}

// folderPayload mirrors what the frontend sends: ids as decimal strings.
type folderPayload struct {
	Name       string   `json:"name"`
	Visibility string   `json:"visibility"`
	RoleIDs    []string `json:"role_ids,omitempty"`
	AccountIDs []string `json:"account_ids,omitempty"`
}

// idStr renders an id the way the client carries it.
func idStr(id int64) string { return strconv.FormatInt(id, 10) }

func (c *apiContext) createFolder(payload folderPayload) int64 {
	rec := c.json(http.MethodPost, "/files/folders", payload, c.admin, permissions.AdminWildcard)
	require.Equal(c.t, http.StatusCreated, rec.Code, rec.Body.String())
	// Ids leave this API quoted: a bigint id does not survive JSON.parse as a
	// number. Pinned on the raw body, because the decode below accepts both.
	assert.Contains(c.t, rec.Body.String(), `"id":"`, "folder id must be a string")
	var resp struct {
		Data struct {
			ID common.JSONID `json:"id"`
		} `json:"data"`
	}
	require.NoError(c.t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data.ID.Int64()
}

type folderListData struct {
	Folders []struct {
		ID         common.JSONID `json:"id"`
		Name       string        `json:"name"`
		Visibility string        `json:"visibility"`
		FileCount  int64         `json:"file_count"`
		RoleIDs    []string      `json:"role_ids"`
		AccountIDs []string      `json:"account_ids"`
	} `json:"folders"`
	CanManage          bool `json:"can_manage"`
	CanUpload          bool `json:"can_upload"`
	StaffUploadEnabled bool `json:"staff_upload_enabled"`
}

func (c *apiContext) listFolders(accountID int64, perms ...string) folderListData {
	rec := c.do(http.MethodGet, "/files/folders", nil, "", accountID, perms...)
	require.Equal(c.t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Data folderListData `json:"data"`
	}
	require.NoError(c.t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

func folderIDs(list folderListData) []int64 {
	ids := make([]int64, 0, len(list.Folders))
	for _, folder := range list.Folders {
		ids = append(ids, folder.ID.Int64())
	}
	return ids
}

func (c *apiContext) enableStaffUpload() {
	err := c.svc.Settings.SetValue(testpkg.Ctx(c.t), configModel.KeyFilesStaffUploadEnabled, true, nil, nil)
	require.NoError(c.t, err)
}

func (c *apiContext) assignRole(accountID, roleID int64) {
	_, err := c.db.NewInsert().
		TableExpr("auth.account_roles").
		Model(&map[string]any{
			"tenant_id":  testpkg.Tenant(c.t),
			"account_id": accountID,
			"role_id":    roleID,
		}).
		Exec(testpkg.Ctx(c.t))
	require.NoError(c.t, err)
}

func TestFolderVisibilityPerViewer(t *testing.T) {
	t.Parallel()
	c := setupAPI(t)

	role := testpkg.CreateTestRoleForTenant(t, c.db, "hausaufgaben", testpkg.Tenant(t))
	_, viaRole := testpkg.CreateTestPersonWithAccount(t, c.db, "Rolle", "Mitglied")
	c.assignRole(viaRole.ID, role.ID)

	all := c.createFolder(folderPayload{Name: "Konzeption", Visibility: "all_staff"})
	admins := c.createFolder(folderPayload{Name: "Leitung intern", Visibility: "admins"})
	selectedRole := c.createFolder(folderPayload{Name: "Hausaufgaben", Visibility: "selected", RoleIDs: []string{idStr(role.ID)}})
	selectedPerson := c.createFolder(folderPayload{Name: "Nur Team-Mitglied", Visibility: "selected", AccountIDs: []string{idStr(c.member)}})

	adminList := c.listFolders(c.admin, permissions.AdminWildcard)
	assert.True(t, adminList.CanManage)
	assert.True(t, adminList.CanUpload)
	assert.ElementsMatch(t, []int64{all, admins, selectedRole, selectedPerson}, folderIDs(adminList))

	memberList := c.listFolders(c.member, permissions.UsersRead)
	assert.False(t, memberList.CanManage)
	assert.False(t, memberList.CanUpload, "staff upload is off by default")
	assert.ElementsMatch(t, []int64{all, selectedPerson}, folderIDs(memberList))
	for _, folder := range memberList.Folders {
		assert.Empty(t, folder.RoleIDs, "share lists are for managers only")
		assert.Empty(t, folder.AccountIDs, "share lists are for managers only")
	}

	roleList := c.listFolders(viaRole.ID, permissions.UsersRead)
	assert.ElementsMatch(t, []int64{all, selectedRole}, folderIDs(roleList))

	// A folder the viewer cannot see is not probeable by ID.
	rec := c.do(http.MethodGet, fmt.Sprintf("/files/folders/%d/files", admins), nil, "", c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	// files:manage without the admin wildcard is enough to see everything.
	managerList := c.listFolders(c.member, permissions.FilesManage)
	assert.True(t, managerList.CanManage)
	assert.ElementsMatch(t, []int64{all, admins, selectedRole, selectedPerson}, folderIDs(managerList))

	// A token can outlive a membership revocation. It must not keep the
	// account able to browse or probe the school file storage.
	_, err := c.db.NewUpdate().
		TableExpr("auth.account_tenants").
		Set("status = ?", "inactive").
		Where("account_id = ?", c.member).
		Where("tenant_id = ?", testpkg.Tenant(c.t)).
		Exec(testpkg.Ctx(c.t))
	require.NoError(t, err)
	assert.Empty(t, folderIDs(c.listFolders(c.member, permissions.UsersRead)))
	rec = c.do(http.MethodGet, fmt.Sprintf("/files/folders/%d/files", all), nil, "", c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestFolderManagementRequiresPermission(t *testing.T) {
	t.Parallel()
	c := setupAPI(t)

	rec := c.json(http.MethodPost, "/files/folders", folderPayload{Name: "Verboten", Visibility: "all_staff"}, c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = c.do(http.MethodGet, "/files/audience", nil, "", c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	folder := c.createFolder(folderPayload{Name: "Formulare", Visibility: "all_staff"})

	rec = c.json(http.MethodPut, fmt.Sprintf("/files/folders/%d", folder), folderPayload{Name: "Umbenannt", Visibility: "admins"}, c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = c.json(http.MethodPost, "/files/folders", folderPayload{Name: "Formulare", Visibility: "all_staff"}, c.admin, permissions.AdminWildcard)
	assert.Equal(t, http.StatusConflict, rec.Code, "duplicate folder name")

	rec = c.json(http.MethodPost, "/files/folders", folderPayload{Name: "Leer", Visibility: "selected"}, c.admin, permissions.AdminWildcard)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "selected folder needs an audience")

	// A role of another school: RLS on auth.roles keeps it out of the audience
	// list, so sharing with it is refused. Derived from a fixture rather than
	// written down as a literal id.
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, c.db, foreignTenant)
	foreignRole := testpkg.CreateTestRoleForTenant(t, c.db, "fremde-schule", foreignTenant)

	rec = c.json(http.MethodPost, "/files/folders", folderPayload{Name: "Fremd", Visibility: "selected", RoleIDs: []string{idStr(foreignRole.ID)}}, c.admin, permissions.AdminWildcard)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a role of another school is refused")

	rec = c.json(http.MethodPut, fmt.Sprintf("/files/folders/%d", folder), folderPayload{Name: "Umbenannt", Visibility: "admins"}, c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, folderIDs(c.listFolders(c.member, permissions.UsersRead)), "renamed to admins-only")

	rec = c.do(http.MethodGet, "/files/audience", nil, "", c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var audience struct {
		Data struct {
			Roles []struct {
				ID common.JSONID `json:"id"`
			} `json:"roles"`
			Accounts []struct {
				AccountID common.JSONID `json:"account_id"`
			} `json:"accounts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &audience))
	accountIDs := make([]int64, 0, len(audience.Data.Accounts))
	for _, account := range audience.Data.Accounts {
		accountIDs = append(accountIDs, account.AccountID.Int64())
	}
	assert.Contains(t, accountIDs, c.member)
	assert.Contains(t, accountIDs, c.admin)
	assert.NotEmpty(t, audience.Data.Roles)
}

func TestUploadDownloadDeleteLifecycle(t *testing.T) {
	t.Parallel()
	c := setupAPI(t)
	folder := c.createFolder(folderPayload{Name: "Vorlagen", Visibility: "all_staff"})

	// Members cannot upload while the setting is off.
	rec := c.upload(folder, "Elternbrief.pdf", fakePDF, c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = c.upload(folder, "Elternbrief.pdf", fakePDF, c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"id":"`, "file id must be a string")
	var uploaded struct {
		Data struct {
			ID        common.JSONID `json:"id"`
			Filename  string        `json:"filename"`
			SizeBytes int64         `json:"size_bytes"`
			CanDelete bool          `json:"can_delete"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &uploaded))
	assert.Equal(t, "Elternbrief.pdf", uploaded.Data.Filename)
	assert.Equal(t, int64(len(fakePDF)), uploaded.Data.SizeBytes)

	rec = c.upload(folder, "script.html", []byte("<html><script>alert(1)</script></html>"), c.admin, permissions.AdminWildcard)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "HTML is never accepted")

	// Listing for a member: visible, but not deletable.
	rec = c.do(http.MethodGet, fmt.Sprintf("/files/folders/%d/files", folder), nil, "", c.member, permissions.UsersRead)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var listed struct {
		Data struct {
			Files []struct {
				ID        common.JSONID `json:"id"`
				CanDelete bool          `json:"can_delete"`
			} `json:"files"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Len(t, listed.Data.Files, 1)
	assert.False(t, listed.Data.Files[0].CanDelete)

	// Download serves the original name as an attachment.
	rec = c.do(http.MethodGet, fmt.Sprintf("/files/folders/%d/files/%d/download", folder, uploaded.Data.ID.Int64()), nil, "", c.member, permissions.UsersRead)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "filename=Elternbrief.pdf")
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Equal(t, fakePDF, rec.Body.Bytes())

	// ?inline=1 shows a PDF in the browser, sandboxed; an office file stays a
	// download regardless of the flag.
	rec = c.do(http.MethodGet, fmt.Sprintf("/files/folders/%d/files/%d/download?inline=1", folder, uploaded.Data.ID.Int64()), nil, "", c.member, permissions.UsersRead)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "inline; filename=Elternbrief.pdf", rec.Header().Get("Content-Disposition"))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "sandbox")
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))

	rec = c.upload(folder, "Plan.xlsx", fakeXLSX(t), c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var sheet struct {
		Data struct {
			ID common.JSONID `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sheet))
	rec = c.do(http.MethodGet, fmt.Sprintf("/files/folders/%d/files/%d/download?inline=1", folder, sheet.Data.ID.Int64()), nil, "", c.member, permissions.UsersRead)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment;")
	assert.Empty(t, rec.Header().Get("Content-Security-Policy"))

	// Members cannot delete somebody else's file.
	rec = c.do(http.MethodDelete, fmt.Sprintf("/files/folders/%d/files/%d", folder, uploaded.Data.ID.Int64()), nil, "", c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = c.do(http.MethodDelete, fmt.Sprintf("/files/folders/%d/files/%d", folder, uploaded.Data.ID.Int64()), nil, "", c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = c.do(http.MethodGet, fmt.Sprintf("/files/folders/%d/files/%d/download", folder, uploaded.Data.ID.Int64()), nil, "", c.admin, permissions.AdminWildcard)
	assert.Equal(t, http.StatusNotFound, rec.Code, "deleted file is gone")

	var events int
	require.NoError(t, c.db.NewSelect().
		TableExpr("audit.file_events").
		ColumnExpr("COUNT(*)").
		Where("folder_id = ?", folder).
		Scan(testpkg.Ctx(t), &events))
	assert.Equal(t, 4, events, "folder_created + 2x file_uploaded + file_deleted")
}

func TestStaffUploadSetting(t *testing.T) {
	t.Parallel()
	c := setupAPI(t)
	folder := c.createFolder(folderPayload{Name: "Team-Ablage", Visibility: "all_staff"})
	hidden := c.createFolder(folderPayload{Name: "Leitung", Visibility: "admins"})
	c.enableStaffUpload()

	list := c.listFolders(c.member, permissions.UsersRead)
	assert.True(t, list.CanUpload)
	assert.True(t, list.StaffUploadEnabled)

	rec := c.upload(folder, "Ausflug.pdf", fakePDF, c.member, permissions.UsersRead)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var own struct {
		Data struct {
			ID common.JSONID `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &own))

	// The setting opens visible folders only; an invisible one stays closed.
	rec = c.upload(hidden, "Ausflug.pdf", fakePDF, c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	rec = c.upload(folder, "Plan.pdf", fakePDF, c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var foreign struct {
		Data struct {
			ID common.JSONID `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &foreign))

	// Own upload: deletable. Somebody else's: not.
	rec = c.do(http.MethodDelete, fmt.Sprintf("/files/folders/%d/files/%d", folder, foreign.Data.ID.Int64()), nil, "", c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = c.do(http.MethodDelete, fmt.Sprintf("/files/folders/%d/files/%d", folder, own.Data.ID.Int64()), nil, "", c.member, permissions.UsersRead)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestDeleteFolderQueuesCleanupForItsFiles(t *testing.T) {
	t.Parallel()
	c := setupAPI(t)
	folder := c.createFolder(folderPayload{Name: "Temporär", Visibility: "all_staff"})

	rec := c.upload(folder, "Alt.pdf", fakePDF, c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = c.do(http.MethodDelete, fmt.Sprintf("/files/folders/%d", folder), nil, "", c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Empty(t, folderIDs(c.listFolders(c.admin, permissions.AdminWildcard)))

	ctx := testpkg.Ctx(t)
	var rows int
	require.NoError(t, c.db.NewSelect().TableExpr("documents.files").ColumnExpr("COUNT(*)").Where("folder_id = ?", folder).Scan(ctx, &rows))
	assert.Equal(t, 0, rows, "file rows cascade with the folder")

	var pending int
	require.NoError(t, c.db.NewSelect().TableExpr("documents.file_cleanup").ColumnExpr("COUNT(*)").
		Where("owner_id = ? AND cleaned_at IS NULL AND retry_after <= ?", folder, time.Now()).Scan(ctx, &pending))
	assert.Equal(t, 1, pending, "the stored object has an immediately eligible cleanup intent")

	// The scheduler pass reclaims the object and settles the intent.
	resource := filestoreAPI.NewResource(c.svc.FileStore, c.db, slog.Default())
	removed, err := resource.CleanupOrphanedFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
}
