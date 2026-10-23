package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	auditRecords "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	mergeCompose "github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge/compose"
	mergeHTTP "github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
