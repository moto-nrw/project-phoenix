package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/uptrace/bun"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func growthGET(t *testing.T, router http.Handler, counter *testpkg.QueryCounter, token, path string) (json.RawMessage, []string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	counter.Reset()
	router.ServeHTTP(rec, req)
	queries := counter.Queries()
	require.Equal(t, http.StatusOK, rec.Code, "%s: %s", path, rec.Body.String())
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data)
	return body.Data, queries
}

func TestEnrollmentPhaseLegalRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	router := f.router(t)
	counter := testpkg.CaptureQueries(t, env.db)
	// A matching phase in another school must not enter either list.
	foreignTenant, _ := testpkg.CreateTestTenant(t, env.db)
	foreignPhase := *env.phase
	foreignPhase.ID = 0
	foreignPhase.TenantID = foreignTenant
	foreignPhase.FormSchemaID = nil
	foreignPhase.Name = "Foreign phase"
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), env.db, foreignTenant, func(txCtx context.Context, _ bun.Tx) error {
		return env.repos.Enrollment().InsertPhase(txCtx, &foreignPhase)
	}))
	ids := []string{strconv.FormatInt(env.phaseID, 10)}
	add := func(n int) {
		for range n {
			p := *env.phase
			p.ID = 0
			p.Name = fmt.Sprintf("Growth phase %02d", len(ids))
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertPhase, &p))
			ids = append(ids, strconv.FormatInt(p.ID, 10))
		}
	}
	add(2)
	runPhases := func(n int) [][]string {
		result := make([][]string, 2)
		for i, path := range []string{"/phases/", "/phases/public/" + f.slug} {
			raw, q := growthGET(t, router, counter, f.token, path)
			result[i] = q
			var rows []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			require.NoError(t, json.Unmarshal(raw, &rows))
			require.Len(t, rows, n)
			for j, row := range rows {
				index := j
				if i == 0 {
					index = n - 1 - j
				}
				require.Equal(t, ids[index], row.ID)
				require.NotEmpty(t, row.Name)
			}
		}
		return result
	}
	small := runPhases(3)
	add(5)
	large := runPhases(8)
	for i := range small {
		require.Equal(t, len(small[i]), len(large[i]))
		t.Logf("phase route %d N3=%d N8=%d", i, len(small[i]), len(large[i]))
	}
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.phases", large[0])
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.public_phases", large[1])

	for _, key := range []string{settingKeyLegalTermsEnabled, settingKeyLegalDSGVOEnabled, settingKeyLegalEmailContactEnabled, settingKeyLegalPhotoEnabled} {
		require.NoError(t, testpkg.TenantCommand4(t, ctx, env.db, f.settings.Settings.SetValue, key, any(true), &env.creatorID, []string{"config:update", "config:manage"}))
	}
	for _, key := range []string{settingKeyLegalAGBText, settingKeyLegalDSGVOText, settingKeyLegalEmailContactText, settingKeyLegalPhotoText} {
		require.NoError(t, testpkg.TenantCommand4(t, ctx, env.db, f.settings.Settings.SetValue, key, any("Tenant legal notice"), &env.creatorID, []string{"config:update", "config:manage"}))
	}
	runLegal := func(n int) [][]string {
		blocks := make([]enrollment.FormLegalBlock, n)
		for i := range blocks {
			blocks[i] = enrollment.FormLegalBlock{Key: fmt.Sprintf("notice%d", i), Kind: "notice", Title: fmt.Sprintf("Notice %d", i), Text: "Contract notice", Enabled: true, SortOrder: i}
		}
		encoded, err := json.Marshal(blocks)
		require.NoError(t, err)
		_, err = env.db.NewUpdate().TableExpr("enrollment.form_schemas").Set("legal_blocks = ?::jsonb", string(encoded)).Where("id = ?", env.schemaID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(ctx)
		require.NoError(t, err)
		raw, q := growthGET(t, router, counter, "", fmt.Sprintf("/legal/%s/%d", f.slug, env.phaseID))
		var legal struct {
			Blocks []enrollment.LegalBlock `json:"blocks"`
		}
		require.NoError(t, json.Unmarshal(raw, &legal))
		require.Len(t, legal.Blocks, n)
		for i, block := range legal.Blocks {
			require.Equal(t, blocks[i].Key, block.Key)
			require.Equal(t, "Contract notice", block.Text)
		}
		tenantRaw, tenantQ := growthGET(t, router, counter, "", "/legal/"+f.slug)
		var tenantLegal struct {
			Blocks []enrollment.LegalBlock `json:"blocks"`
		}
		require.NoError(t, json.Unmarshal(tenantRaw, &tenantLegal))
		require.Len(t, tenantLegal.Blocks, 4, "tenant contract is the fixed four standard blocks, independent of template cardinality")
		for _, block := range tenantLegal.Blocks {
			require.Equal(t, "Tenant legal notice", block.Text)
		}
		return [][]string{q, tenantQ}
	}
	legalSmall, legalLarge := runLegal(3), runLegal(8)
	for i := range legalSmall {
		require.Equal(t, len(legalSmall[i]), len(legalLarge[i]))
		t.Logf("legal route %d N3=%d N8=%d", i, len(legalSmall[i]), len(legalLarge[i]))
	}
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.phase_legal", legalLarge[0])
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.tenant_legal", legalLarge[1])
}
