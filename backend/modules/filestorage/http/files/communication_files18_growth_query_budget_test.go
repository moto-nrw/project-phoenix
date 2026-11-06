package files_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"log/slog"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	filesHTTP "github.com/moto-nrw/project-phoenix/modules/filestorage/http/files"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func growthData[T any](t *testing.T, body []byte) T {
	t.Helper()
	var envelope struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Data
}

// These are the literal production resource paths. The context counter spans
// the complete protected-router transaction and all real owner capabilities.
func TestCommunicationFiles18PositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	c := setupFileStoreRoute(t)
	c.router.Mount("/api/files", filesHTTP.NewResource(c.files, slog.Default()).Router(rootAPI.ProtectedTenantRoutes))
	counter := testpkg.CaptureQueriesForContext(t, c.db)
	counter.Stop()
	foreignTenant, _ := testpkg.CreateTestTenant(t, c.db)
	foreignRole := testpkg.CreateTestRoleForTenant(t, c.db, "foreign-growth", foreignTenant)
	_, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, c.db, foreignTenant, "Foreign", "Growth")
	var foreignFolder int64
	require.NoError(t, c.db.NewRaw(`INSERT INTO documents.folders (tenant_id,name,visibility,created_by) VALUES (?,'Foreign folder','all_staff',?) RETURNING id`, foreignTenant, foreignAccount.ID).Scan(testpkg.Ctx(t), &foreignFolder))
	_, err := c.db.NewRaw(`INSERT INTO documents.files (tenant_id,category,filename_display,filename_stored,size_bytes,content_type,uploaded_by,folder_id) VALUES (?,'file','Foreign.pdf','foreign-fixture.pdf',123,'application/pdf',?,?)`, foreignTenant, foreignAccount.ID, foreignFolder).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	foreignToken := communication18AttachmentToken(t, foreignAccount.ID, foreignAccount.Email, foreignTenant, "", []string{"user"}, []string{permissions.AdminWildcard})
	var folders []filesHTTP.FolderResponse
	var files []filesHTTP.FileResponse
	var roles []filesHTTP.AudienceRoleResponse
	// The route harness creates these two real person/account memberships.
	var people []filesHTTP.AudienceAccountResponse
	for _, id := range []int64{c.admin, c.member} {
		var row struct {
			First string `bun:"first_name"`
			Last  string `bun:"last_name"`
		}
		require.NoError(t, c.db.NewRaw(`SELECT first_name,last_name FROM users.persons WHERE account_id = ?`, id).Scan(testpkg.Ctx(t), &row))
		people = append(people, filesHTTP.AudienceAccountResponse{AccountID: idStr(id), FirstName: row.First, LastName: row.Last})
	}
	// Shared system roles are part of every tenant audience. Resolve their fixture
	// IDs by their exact seeded names, not by hardcoded identifiers.
	for _, name := range []string{"admin", "ag-leitung", "guest", "lehrkraft", "schulverwaltung", "user"} {
		var id int64
		require.NoError(t, c.db.NewRaw(`SELECT id FROM auth.roles WHERE name = ? AND is_system = true`, name).Scan(testpkg.Ctx(t), &id))
		roles = append(roles, filesHTTP.AudienceRoleResponse{ID: idStr(id), Name: name})
	}
	for _, n := range []int{3, 8} {
		for len(folders) < n {
			i := len(folders)
			role := testpkg.CreateTestRoleForTenant(t, c.db, fmt.Sprintf("Growth%02d", i), testpkg.Tenant(t))
			roles = append(roles, filesHTTP.AudienceRoleResponse{ID: idStr(role.ID), Name: role.Name})
			if len(people) < n {
				person, account := testpkg.CreateTestPersonWithAccount(t, c.db, fmt.Sprintf("Person%02d", i), "Growth")
				people = append(people, filesHTTP.AudienceAccountResponse{AccountID: idStr(account.ID), FirstName: person.FirstName, LastName: person.LastName})
			}
			rec := c.json("POST", "/api/files/folders", folderPayload{Name: fmt.Sprintf("Folder%02d", i), Visibility: "selected", RoleIDs: []string{idStr(role.ID)}, AccountIDs: []string{idStr(c.member)}}, c.admin, permissions.AdminWildcard)
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			folder := growthData[filesHTTP.FolderResponse](t, rec.Body.Bytes())
			require.Equal(t, fmt.Sprintf("Folder%02d", i), folder.Name)
			require.Equal(t, "selected", folder.Visibility)
			require.Equal(t, []string{idStr(role.ID)}, folder.RoleIDs)
			require.Equal(t, []string{idStr(c.member)}, folder.AccountIDs)
			require.False(t, folder.CreatedAt.IsZero())
			folders = append(folders, folder)
			rec = c.upload(mustGrowthID(t, folders[0].ID), fmt.Sprintf("File%02d.pdf", i), fakePDF, c.admin, permissions.AdminWildcard)
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			file := growthData[filesHTTP.FileResponse](t, rec.Body.Bytes())
			require.Equal(t, fmt.Sprintf("File%02d.pdf", i), file.Filename)
			require.Equal(t, folders[0].ID, file.FolderID)
			require.Equal(t, int64(len(fakePDF)), file.SizeBytes)
			require.Equal(t, "application/pdf", file.ContentType)
			require.Equal(t, idStr(c.admin), file.UploadedBy)
			require.True(t, file.CanDelete)
			require.False(t, file.UploadedAt.IsZero())
			files = append([]filesHTTP.FileResponse{file}, files...)
		}
		folders[0].FileCount = int64(n)
		slices.SortFunc(roles, func(a, b filesHTTP.AudienceRoleResponse) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
		slices.SortFunc(people, func(a, b filesHTTP.AudienceAccountResponse) int {
			if x := strings.Compare(strings.ToLower(a.LastName), strings.ToLower(b.LastName)); x != 0 {
				return x
			}
			return strings.Compare(strings.ToLower(a.FirstName), strings.ToLower(b.FirstName))
		})
		paths := []string{"/api/files/audience", "/api/files/folders", fmt.Sprintf("/api/files/folders/%s/files", folders[0].ID)}
		for _, path := range paths {
			counter.Reset()
			counter.Start()
			rec := c.request(counter.Context(testpkg.Ctx(t)), "GET", path, nil, "", c.admin, permissions.AdminWildcard)
			counter.Stop()
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			switch path {
			case paths[0]:
				got := growthData[filesHTTP.AudienceResponse](t, rec.Body.Bytes())
				require.Equal(t, roles, got.Roles)
				require.Equal(t, people, got.Accounts)
				for _, role := range got.Roles {
					require.NotEqual(t, idStr(foreignRole.ID), role.ID)
				}
				for _, p := range got.Accounts {
					require.NotEqual(t, idStr(foreignAccount.ID), p.AccountID)
				}
				if n == 3 {
					querybudgets.AssertQueryBudget(t, "api.files.audience.communication18.n3", counter.Queries())
				} else {
					querybudgets.AssertQueryBudget(t, "api.files.audience.communication18.n8", counter.Queries())
				}
			case paths[1]:
				got := growthData[filesHTTP.FolderListResponse](t, rec.Body.Bytes())
				require.Equal(t, folders, got.Folders)
				require.True(t, got.CanManage)
				require.True(t, got.CanUpload)
				require.False(t, got.StaffUploadEnabled)
				require.Equal(t, int64(n*len(fakePDF)), got.UsedBytes)
				require.Equal(t, int64(1<<30), got.MaxBytes)
				if n == 3 {
					querybudgets.AssertQueryBudget(t, "api.files.folders.communication18.n3", counter.Queries())
				} else {
					querybudgets.AssertQueryBudget(t, "api.files.folders.communication18.n8", counter.Queries())
				}
			default:
				got := growthData[filesHTTP.FileListResponse](t, rec.Body.Bytes())
				want := folders[0]
				want.RoleIDs = []string{}
				want.AccountIDs = []string{}
				require.Equal(t, want, got.Folder)
				require.Equal(t, files, got.Files)
				if n == 3 {
					querybudgets.AssertQueryBudget(t, "api.files.files.communication18.n3", counter.Queries())
				} else {
					querybudgets.AssertQueryBudget(t, "api.files.files.communication18.n8", counter.Queries())
				}
			}
			require.Zero(t, counter.WriteRows())
			foreignReq := httptest.NewRequest("GET", path, nil)
			foreignReq.Header.Set("Authorization", "Bearer "+foreignToken)
			foreignRec := testutil.ExecuteRequestForTest(t, c.router, foreignReq)
			if path == paths[2] {
				require.Equal(t, 404, foreignRec.Code, foreignRec.Body.String())
			} else {
				require.Equal(t, 200, foreignRec.Code, foreignRec.Body.String())
				require.NotContains(t, foreignRec.Body.String(), "Folder00")
				require.NotContains(t, foreignRec.Body.String(), "Growth00")
			}
			foreignProbe := c.do("GET", fmt.Sprintf("/api/files/folders/%d/files", foreignFolder), nil, "", c.admin, permissions.AdminWildcard)
			require.Equal(t, 404, foreignProbe.Code, foreignProbe.Body.String())

			unauth := httptest.NewRecorder()
			c.router.ServeHTTP(unauth, httptest.NewRequest("GET", path, nil))
			require.Equal(t, http.StatusUnauthorized, unauth.Code, unauth.Body.String())
		}
	}
}

func mustGrowthID(t *testing.T, s string) int64 {
	t.Helper()
	var id int64
	_, err := fmt.Sscan(s, &id)
	require.NoError(t, err)
	return id
}
