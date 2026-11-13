package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	enrollmenthttp "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func adminCaseService(env *requestTestEnv, requests compose.ChangeRequestRecords, children compose.IntakeChildren, catalog compose.IntakeCatalog) enrollmenthttp.ChangeRequestService {
	return newTestChangeRequestService(owners.EnrollmentChangeRequestSources{
		Bookings: requestTestBookingCommands(), Requests: requests, Children: children,
		Catalog: catalog, Settings: env.settings, ParentsURL: "http://parents.localhost:3000",
	})
}

func adminCaseRows(t *testing.T, env *requestTestEnv) []*enrollment.ChangeRequest {
	t.Helper()
	ctx := testpkg.Ctx(t)
	var rows []*enrollment.ChangeRequest
	for _, name := range []string{"First", "Second"} {
		phase := makeOwnerEligibilityPhase(name)
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertPhase, phase))
		req := makeOwnerRequest(phase.ID, uniqueToken("admin-cases"), name+"@example.test")
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequest, req))
		for range 2 {
			child := makeChild(req.ID, name, "Child")
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChild, child))
		}
		for _, status := range []string{enrollment.ChangeRequestStatusPendingReview, enrollment.ChangeRequestStatusRejected} {
			row := &enrollment.ChangeRequest{RequestID: req.ID, Status: status, Origin: enrollment.ChangeRequestOriginParent,
				BaseSnapshot: json.RawMessage(`{}`), ProposedSnapshot: json.RawMessage(`{}`), Diff: json.RawMessage(`{}`)}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequest, row))
			rows = append([]*enrollment.ChangeRequest{row}, rows...)
			for _, internal := range []bool{false, true} {
				message := &enrollment.ChangeRequestMessage{ChangeRequestID: row.ID, AuthorType: enrollment.ChangeRequestMessageAuthorStaff, Body: name, InternalOnly: internal}
				require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequestMessage, message))
			}
		}
	}
	return rows
}

func TestEnrollmentAdminChangeCasesMatchScalarDetails(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	rows := adminCaseRows(t, env)
	svc := adminCaseService(env, env.repos.Enrollment(), env.repos.Enrollment(), env.repos.Enrollment())
	ctx := testpkg.Ctx(t)
	listed, err := testpkg.TenantQuery1(t, ctx, env.db, svc.ListAdmin, enrollmenthttp.ChangeRequestFilters{})
	require.NoError(t, err)
	require.Len(t, listed, len(rows))
	for i, row := range listed {
		require.Equal(t, rows[i].ID, row.ChangeRequest.ID)
		detail, err := testpkg.TenantQuery1(t, ctx, env.db, svc.GetAdmin, row.ChangeRequest.ID)
		require.NoError(t, err)
		require.Equal(t, detail, row)
		require.Len(t, row.Children, 2)
		require.Len(t, row.Messages, 2)
		require.False(t, row.Messages[0].InternalOnly)
		require.True(t, row.Messages[1].InternalOnly)
	}
	filtered, err := testpkg.TenantQuery1(t, ctx, env.db, svc.ListAdmin, enrollmenthttp.ChangeRequestFilters{RequestID: rows[0].RequestID, Status: enrollment.ChangeRequestStatusRejected})
	require.NoError(t, err)
	require.Equal(t, listed[:1], filtered)
}

// A corrupt stored request or child fails the whole list. The owner's adapter
// decodes each batch as it reads it (ADR 0173), so the request batch, read
// before the children, reports its corrupt row first, naming row and column. Messages and phase
// reads come after both.
func TestEnrollmentAdminChangeCasesKeepDecodeErrorOrder(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	rows := adminCaseRows(t, env)
	ctx := testpkg.Ctx(t)
	_, err := env.db.NewRaw("UPDATE enrollment.requests SET custom_data = '[]'::jsonb WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), rows[2].RequestID).Exec(ctx)
	require.NoError(t, err)
	_, err = env.db.NewRaw("UPDATE enrollment.request_children SET custom_data = '[]'::jsonb WHERE tenant_id = ? AND request_id = ?", testpkg.Tenant(t), rows[0].RequestID).Exec(ctx)
	require.NoError(t, err)
	svc := adminCaseService(env, env.repos.Enrollment(), env.repos.Enrollment(), failingChangeRequestCatalog{IntakeCatalog: env.repos.Enrollment()})
	_, err = testpkg.TenantQuery1(t, ctx, env.db, svc.ListAdmin, enrollmenthttp.ChangeRequestFilters{})
	require.EqualError(t, err, fmt.Sprintf("decode enrollment request %d custom_data: json: cannot unmarshal array into Go value of type map[string]interface {}", rows[2].RequestID))
}

type omittedAdminCaseRequests struct {
	compose.ChangeRequestRecords
	requestID int64
}

func (r omittedAdminCaseRequests) RequestsByID(ctx context.Context, ids []int64) ([]*enrollment.Request, error) {
	rows, err := r.ChangeRequestRecords.RequestsByID(ctx, ids)
	var kept []*enrollment.Request
	for _, row := range rows {
		if row.ID != r.requestID {
			kept = append(kept, row)
		}
	}
	return kept, err
}

func TestEnrollmentAdminChangeCasesKeepScalarFallbacks(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	rows := adminCaseRows(t, env)
	ctx := testpkg.Ctx(t)
	requests := omittedAdminCaseRequests{ChangeRequestRecords: env.repos.Enrollment(), requestID: rows[0].RequestID}
	// This catalog exposes only the scalar phase port, like existing consumers.
	catalog := struct{ compose.IntakeCatalog }{env.repos.Enrollment()}
	svc := adminCaseService(env, requests, env.repos.Enrollment(), catalog)
	listed, err := testpkg.TenantQuery1(t, ctx, env.db, svc.ListAdmin, enrollmenthttp.ChangeRequestFilters{})
	require.NoError(t, err)
	require.Len(t, listed, len(rows))
	for _, row := range listed {
		detail, err := testpkg.TenantQuery1(t, ctx, env.db, svc.GetAdmin, row.ChangeRequest.ID)
		require.NoError(t, err)
		require.Equal(t, detail, row)
	}
	svc = adminCaseService(env, requests, env.repos.Enrollment(), omittedAdminCasePhases{IntakeCatalog: env.repos.Enrollment()})
	fallback, err := testpkg.TenantQuery1(t, ctx, env.db, svc.ListAdmin, enrollmenthttp.ChangeRequestFilters{})
	require.NoError(t, err)
	require.Equal(t, listed, fallback)
	// A case deleted after the initial list must still be reported missing.
	svc = adminCaseService(env, missingAdminCase{ChangeRequestRecords: requests, id: rows[0].ID}, env.repos.Enrollment(), catalog)
	_, err = testpkg.TenantQuery1(t, ctx, env.db, svc.ListAdmin, enrollmenthttp.ChangeRequestFilters{})
	require.ErrorIs(t, err, enrollment.ErrChangeRequestNotFound)
}

type omittedAdminCasePhases struct{ compose.IntakeCatalog }

func (omittedAdminCasePhases) PhasesByID(context.Context, []int64) ([]*enrollment.Phase, error) {
	return nil, nil
}

type missingAdminCase struct {
	compose.ChangeRequestRecords
	id int64
}

func (r missingAdminCase) ChangeRequestByID(ctx context.Context, id int64) (*enrollment.ChangeRequest, error) {
	if id == r.id {
		return nil, nil
	}
	return r.ChangeRequestRecords.ChangeRequestByID(ctx, id)
}

type adminCaseReadInterleave struct {
	compose.ChangeRequestRecords
	id     int64
	before func()
}

func (r adminCaseReadInterleave) ChangeRequestByID(ctx context.Context, id int64) (*enrollment.ChangeRequest, error) {
	if id == r.id {
		r.before()
	}
	return r.ChangeRequestRecords.ChangeRequestByID(ctx, id)
}

func TestEnrollmentAdminChangeCasesKeepInitiallyOpenSubjectsLive(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	rows := adminCaseRows(t, env)
	ctx := testpkg.Ctx(t)
	for _, status := range []string{enrollment.ChangeRequestStatusPendingReview, enrollment.ChangeRequestStatusRejected} {
		t.Run(status, func(t *testing.T) {
			require.NoError(t, testpkg.TenantCommand2(t, ctx, env.db, env.repos.Enrollment().SetChangeRequestStatus, rows[1].ID, enrollment.ChangeRequestStatusPendingReview))
			reader := adminCaseReadInterleave{ChangeRequestRecords: env.repos.Enrollment(), id: rows[1].ID, before: func() {
				message := &enrollment.ChangeRequestMessage{ChangeRequestID: rows[1].ID, AuthorType: enrollment.ChangeRequestMessageAuthorParent, Body: "Live reply " + status}
				require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequestMessage, message))
				require.NoError(t, testpkg.TenantCommand2(t, ctx, env.db, env.repos.Enrollment().SetChangeRequestStatus, rows[1].ID, status))
				_, err := env.db.NewRaw("UPDATE enrollment.request_children SET first_name = ? WHERE tenant_id = ? AND request_id = ?", status, testpkg.Tenant(t), rows[1].RequestID).Exec(ctx)
				require.NoError(t, err)
			}}
			svc := adminCaseService(env, reader, env.repos.Enrollment(), env.repos.Enrollment())
			listed, err := testpkg.TenantQuery1(t, ctx, env.db, svc.ListAdmin, enrollmenthttp.ChangeRequestFilters{})
			require.NoError(t, err)
			var found bool
			for _, row := range listed {
				if row.ChangeRequest.ID != rows[1].ID {
					continue
				}
				found = true
				require.Equal(t, status, row.ChangeRequest.Status)
				require.Equal(t, status, row.Children[0].FirstName)
				require.Equal(t, "Live reply "+status, row.Messages[len(row.Messages)-1].Body)
			}
			require.True(t, found)
		})
	}
}
