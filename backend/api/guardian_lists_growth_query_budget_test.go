package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// Drive the registered guardian resource, including JWT, permissions and the
// tenant transaction. Fixture writes and token creation are outside capture.
func TestGuardianListsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	c := setupGuardiansCompositionRoute(t)
	router := chi.NewRouter()
	router.Mount("/api", c.router)
	db := testpkg.SetupTestDB(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	token := testutil.MintTestJWT(t, withPerms(testutil.DefaultTestClaims(), "admin:*", "guardians:financial"))
	var guardianIDs, studentIDs, linkIDs, phoneIDs []int64
	var emails []string
	var numbers []string
	for _, n := range []int{3, 8} {
		for len(guardianIDs) < n {
			i := len(guardianIDs)
			gid, email := c.createNamedGuardian("BudgetGuardian", fmt.Sprintf("Family%02d", i), fmt.Sprintf("budget-%02d", i))
			sid, _ := c.createStudent(fmt.Sprintf("Child%02d", i), "BudgetFamily", "2b")
			guardianIDs = append(guardianIDs, gid)
			emails = append(emails, email)
			studentIDs = append(studentIDs, sid)
			linkIDs = append(linkIDs, c.link(sid, guardianIDs[0], "primary_guardian"))
			if gid != guardianIDs[0] {
				c.link(sid, gid, "primary_guardian")
			}
			number := fmt.Sprintf("+492111234%03d", i)
			rr := c.do(t, testutil.DefaultTestClaims(), http.MethodPost, fmt.Sprintf("/%d/phone-numbers", guardianIDs[0]), map[string]any{"phone_number": number, "phone_type": "mobile"})
			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			var phone struct {
				Data struct {
					ID int64 `json:"id"`
				}
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &phone))
			require.Positive(t, phone.Data.ID)
			phoneIDs = append(phoneIDs, phone.Data.ID)
			numbers = append(numbers, number)
			rr = c.do(t, withPerms(testutil.DefaultTestClaims(), "admin:*", "guardians:financial"), http.MethodPut, fmt.Sprintf("/%d/payment", gid), map[string]any{"iban": testIBAN, "account_holder": fmt.Sprintf("Holder%02d", i)})
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			c.requirePayerSet(t, sid, &gid)
		}
		for _, name := range []string{"search", "without-account", "invitable", "detail", "delete-preview", "phones", "students", "payment_overview", "payment_export", "bulk_invite"} {
			path := map[string]string{"search": "/search?q=BudgetGuardian", "without-account": "/without-account", "invitable": "/invitable", "detail": fmt.Sprintf("/%d", guardianIDs[0]), "delete-preview": fmt.Sprintf("/%d/delete-preview", guardianIDs[0]), "phones": fmt.Sprintf("/%d/phone-numbers/", guardianIDs[0]), "students": fmt.Sprintf("/%d/students", guardianIDs[0])}[name]
			method := http.MethodGet
			var body any
			switch name {
			case "payment_overview":
				path = "/payment-overview"
			case "payment_export":
				method = http.MethodPost
				path = "/payment-overview/export"
				body = map[string]any{"format": "xlsx"}
			case "bulk_invite":
				method = http.MethodPost
				path = "/bulk-invite"
				body = map[string]any{"student_ids": studentIDs, "dry_run": true}
			}
			req := testutil.NewAuthenticatedRequest(t, method, "/api/guardians"+path, body, testutil.WithJWTBearer(token))
			req = req.WithContext(counter.Context(testpkg.Ctx(t)))
			counter.Reset()
			counter.Start()
			rr := testutil.ExecuteRequest(router, req)
			counter.Stop()
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			var envelope struct{ Data json.RawMessage }
			if name != "payment_export" {
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
			}
			switch name {
			case "payment_overview":
				var rows []struct {
					StudentID    string `json:"student_id"`
					StudentName  string `json:"student_name"`
					GuardianID   string `json:"guardian_id"`
					GuardianName string `json:"guardian_name"`
					Holder       string `json:"account_holder"`
					Masked       string `json:"iban_masked"`
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				seen := map[string]bool{}
				for _, row := range rows {
					require.False(t, seen[row.StudentID])
					seen[row.StudentID] = true
					idx := -1
					for i, id := range studentIDs {
						if fmt.Sprint(id) == row.StudentID {
							idx = i
						}
					}
					require.GreaterOrEqual(t, idx, 0)
					require.Equal(t, fmt.Sprint(guardianIDs[idx]), row.GuardianID)
					require.Equal(t, fmt.Sprintf("Child%02d BudgetFamily", idx), row.StudentName)
					require.Equal(t, fmt.Sprintf("BudgetGuardian Family%02d", idx), row.GuardianName)
					require.Equal(t, fmt.Sprintf("Holder%02d", idx), row.Holder)
					require.NotEmpty(t, row.Masked)
					require.NotEqual(t, testIBAN, row.Masked)
				}
			case "payment_export":
				require.Contains(t, rr.Header().Get("Content-Disposition"), "bankverbindungen.xlsx")
				text := xlsxSharedStrings(t, rr.Body.Bytes())
				require.Contains(t, text, testIBAN)
				for i := range studentIDs {
					require.Contains(t, text, fmt.Sprintf("Child%02d BudgetFamily", i))
					require.Contains(t, text, fmt.Sprintf("Holder%02d", i))
				}
				archive, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
				require.NoError(t, err)
				found := false
				for _, file := range archive.File {
					if strings.HasPrefix(file.Name, "xl/worksheets/") && strings.HasSuffix(file.Name, ".xml") {
						reader, err := file.Open()
						require.NoError(t, err)
						data, err := io.ReadAll(reader)
						require.NoError(t, err)
						require.NoError(t, reader.Close())
						var sheet struct {
							Rows []struct {
								Index int `xml:"r,attr"`
							} `xml:"sheetData>row"`
						}
						require.NoError(t, xml.Unmarshal(data, &sheet))
						bodyRows := 0
						for _, row := range sheet.Rows {
							if row.Index > 6 {
								bodyRows++
							}
						}
						require.Equal(t, n, bodyRows)
						found = true
					}
				}
				require.True(t, found)
			case "bulk_invite":
				var row struct {
					Dry      bool `json:"dry_run"`
					Invited  int
					Problems []any
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &row))
				require.True(t, row.Dry)
				require.Equal(t, n, row.Invited)
				require.Empty(t, row.Problems)
			case "search", "without-account", "invitable":
				var rows []struct {
					ID                  int64
					FirstName           string `json:"first_name"`
					LastName            string `json:"last_name"`
					Email               string
					LinkedChildrenCount int `json:"linked_children_count"`
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				seen := map[int64]bool{}
				for _, row := range rows {
					require.False(t, seen[row.ID])
					seen[row.ID] = true
					idx := -1
					for i, id := range guardianIDs {
						if id == row.ID {
							idx = i
						}
					}
					require.GreaterOrEqual(t, idx, 0)
					require.Equal(t, "BudgetGuardian", row.FirstName)
					require.Equal(t, fmt.Sprintf("Family%02d", idx), row.LastName)
					require.Equal(t, emails[idx], row.Email)
					if name == "search" {
						want := 1
						if idx == 0 {
							want = n
						}
						require.Equal(t, want, row.LinkedChildrenCount)
					}
				}
			case "detail", "phones":
				type phone struct {
					ID     int64
					Number string `json:"phone_number"`
					Type   string `json:"phone_type"`
				}
				var rows []phone
				if name == "detail" {
					var row struct {
						ID           int64
						FirstName    string  `json:"first_name"`
						PhoneNumbers []phone `json:"phone_numbers"`
					}
					require.NoError(t, json.Unmarshal(envelope.Data, &row))
					require.Equal(t, guardianIDs[0], row.ID)
					require.Equal(t, "BudgetGuardian", row.FirstName)
					rows = row.PhoneNumbers
				} else {
					require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				}
				require.Len(t, rows, n)
				got := map[int64]string{}
				for _, row := range rows {
					require.Equal(t, "mobile", row.Type)
					got[row.ID] = row.Number
				}
				require.Len(t, got, n)
				for i, id := range phoneIDs {
					require.Equal(t, numbers[i], got[id])
				}
			case "delete-preview":
				var row struct {
					Count   int      `json:"linked_count"`
					Names   []string `json:"affected_names"`
					Links   []string `json:"affected_link_ids"`
					Warning string
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &row))
				require.Equal(t, n, row.Count)
				require.Len(t, row.Names, n)
				require.Len(t, row.Links, n)
				require.NotEmpty(t, row.Warning)
				for i, id := range linkIDs {
					require.Contains(t, row.Links, fmt.Sprint(id))
					require.Contains(t, row.Names, fmt.Sprintf("Child%02d BudgetFamily", i))
				}
			case "students":
				var rows []struct {
					ID        int64  `json:"student_id"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
					Class     string `json:"school_class"`
					Link      int64  `json:"relationship_id"`
					Role      string `json:"guardian_role"`
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, n)
				seen := map[int64]bool{}
				for _, row := range rows {
					require.False(t, seen[row.ID])
					seen[row.ID] = true
					idx := -1
					for i, id := range studentIDs {
						if row.ID == id {
							idx = i
						}
					}
					require.GreaterOrEqual(t, idx, 0)
					require.Equal(t, fmt.Sprintf("Child%02d", idx), row.FirstName)
					require.Equal(t, "BudgetFamily", row.LastName)
					require.Equal(t, "2b", row.Class)
					require.Equal(t, linkIDs[idx], row.Link)
					require.Equal(t, "primary_guardian", row.Role)
				}
			}
			t.Logf("MEASURE guardian.%s N%d=%d", name, n, counter.Total())
			switch name {
			case "search":
				querybudgets.AssertQueryBudget(t, "api.guardians.search.http_positive_growth", counter.Queries())
			case "without-account":
				querybudgets.AssertQueryBudget(t, "api.guardians.without_account.http_positive_growth", counter.Queries())
			case "invitable":
				querybudgets.AssertQueryBudget(t, "api.guardians.invitable.http_positive_growth", counter.Queries())
			case "detail":
				querybudgets.AssertQueryBudget(t, "api.guardians.detail.http_positive_growth", counter.Queries())
			case "delete-preview":
				querybudgets.AssertQueryBudget(t, "api.guardians.delete_preview.http_positive_growth", counter.Queries())
			case "phones":
				querybudgets.AssertQueryBudget(t, "api.guardians.phones.http_positive_growth", counter.Queries())
			case "students":
				querybudgets.AssertQueryBudget(t, "api.guardians.students.http_positive_growth", counter.Queries())
			case "payment_overview":
				querybudgets.AssertQueryBudget(t, "api.guardians.payment_overview.http_positive_growth", counter.Queries())
			case "payment_export":
				querybudgets.AssertQueryBudget(t, "api.guardians.payment_export.http_positive_growth", counter.Queries())
			case "bulk_invite":
				querybudgets.AssertQueryBudget(t, "api.guardians.bulk_invite.http_positive_growth", counter.Queries())
			}
		}
	}
}
