package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	auditRecords "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	mergeCompose "github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge/compose"
	mergeHTTP "github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffPersonMergePreviewPersistsRequestIP(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	h := newStaffMergeHarness(t)
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFFTrustedProxies(1))
	router.Mount("/", h.router)
	for _, tc := range []struct{ name, forwardedFor, want string }{
		{"direct IPv4", "", "198.51.100.42"},
		{"forwarded IPv6", "203.0.113.99, 2001:db8::42", "2001:db8::42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/%d", h.schoolID), nil,
				testutil.WithClaims(t, sessiontoken.AppClaims{ID: int(h.operatorID), Scope: "platform"}))
			req.RemoteAddr = "198.51.100.42:12345"
			if tc.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tc.forwardedFor)
			}
			response := testutil.ExecuteRequest(router, req)
			require.Equal(t, http.StatusOK, response.Code)
			var storedIP string
			require.NoError(t, db.NewRaw(`SELECT COALESCE(host(request_ip), '') FROM platform.operator_audit_log
    WHERE operator_id = ? AND action = 'view' AND changes->>'view' = 'staff_person_merge_candidates'
    ORDER BY id DESC LIMIT 1`, h.operatorID).Scan(testpkg.Ctx(t), &storedIP))
			assert.Equal(t, tc.want, storedIP)
		})
	}
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
  WHERE operator_id = ? AND action = 'view' AND changes->>'view' = 'staff_person_merge_candidates'`,
		h.operatorID).Scan(testpkg.Ctx(t), &count))
	assert.Equal(t, 2, count, "one audit entry per preview request")
}

func TestStaffPersonMergePreviewRejectsAuditFailure(t *testing.T) {
	t.Parallel()
	h := newStaffMergeHarness(t)
	calls := 0
	workflow, err := mergeCompose.New(peopleCompose.NewStaffPersonMerge(), membershipCompose.NewStaffPersonMerge(), staffPersonMergeAudit{create: func(_ context.Context, entry *auditRecords.OperatorAuditEntry) error {
		calls++
		assert.Equal(t, "view", entry.Action)
		assert.Equal(t, "person", entry.ResourceType)
		assert.Nil(t, entry.ResourceID)
		assert.JSONEq(t, fmt.Sprintf(`{"school_id":%d,"view":"staff_person_merge_candidates"}`, h.schoolID), string(entry.Changes))
		return errors.New("private diagnostic")
	}}, operatorIDFromSession)
	require.NoError(t, err)
	h.router = chi.NewRouter()
	h.router.Mount("/", mergeHTTP.Router(workflow, TenantScopeMiddleware))
	response := h.request(t, http.MethodGet, h.schoolID, nil)
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Equal(t, 1, calls)
	assert.Contains(t, response.Body.String(), `"code":"operator.audit_unavailable"`)
	assert.NotContains(t, response.Body.String(), "private diagnostic")
}
