package enrollment

import (
	"encoding/json"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// mkRequest constructs an enrollment request with the supplied IDs.
func mkRequest(id int64, phaseID int64) *enrollmentModels.Request {
	return &enrollmentModels.Request{
		ID:      id,
		PhaseID: phaseID,
	}
}

// mkChild does the same for RequestChild.
func mkChild(id int64) *enrollmentModels.RequestChild {
	return &enrollmentModels.RequestChild{
		ID: id,
	}
}

// toAdminRequestSummary is the response-shaping helper between the
// decision service and the admin list/detail endpoints. It must
// stringify int64 IDs (per CLAUDE.md rule 4), preserve optional
// pointers, format DOB as YYYY-MM-DD, and tolerate a nil phase
// (the listing path leaves it nil to keep the payload light).

func TestToAdminRequestSummary_StringifiesIDsAndFormatsDOB(t *testing.T) {
	t.Parallel()

	phone := "+49 170 1234567"
	withdrawn := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	req := mkRequest(1234, 5678)
	req.GuardianFirstName = "Anna"
	req.GuardianLastName = "Beispiel"
	req.GuardianEmail = "anna@example.test"
	req.GuardianPhone = &phone
	req.SubmittedAt = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	req.WithdrawnAt = &withdrawn
	req.StatusToken = "token-abc"

	child := mkChild(999)
	child.FirstName = "Lara"
	child.LastName = "Beispiel"
	child.DateOfBirth = "2018-03-04"
	child.Status = "submitted"
	child.ActivationMode = "auto"

	in := &enrollmentService.RequestSummary{
		Request: req,
		Phase: &capability.Phase{
			Name:                      "Schuljahr 2026/27",
			CareOfferingSelectionMode: capability.PhaseCareOfferingSelectionOptional,
		},
		Children: []*enrollmentModels.RequestChild{child},
	}
	out := toAdminRequestSummary(in)
	assert.Equal(t, "1234", out.ID, "int64 ID stringified per CLAUDE rule 4")
	assert.Equal(t, "5678", out.PhaseID)
	assert.Equal(t, "Schuljahr 2026/27", out.PhaseName)
	assert.Equal(t, capability.PhaseCareOfferingSelectionOptional, out.CareOfferingSelectionMode)
	assert.Equal(t, "anna@example.test", out.GuardianEmail)
	require.NotNil(t, out.GuardianPhone)
	assert.Equal(t, phone, *out.GuardianPhone)
	require.NotNil(t, out.WithdrawnAt)
	assert.Equal(t, withdrawn, *out.WithdrawnAt)
	require.Len(t, out.Children, 1)
	assert.Equal(t, "999", out.Children[0].ID)
	assert.Equal(t, "2018-03-04", out.Children[0].DateOfBirth, "DOB rendered as YYYY-MM-DD")
	assert.Equal(t, "submitted", out.Children[0].Status)
	assert.Equal(t, "auto", out.Children[0].ActivationMode)
}

func TestToAdminRequestSummary_DoesNotExposeStatusToken(t *testing.T) {
	t.Parallel()

	req := mkRequest(1234, 5678)
	req.StatusToken = "token-abc"

	out := toAdminRequestSummary(&enrollmentService.RequestSummary{
		Request:  req,
		Children: []*enrollmentModels.RequestChild{},
	})
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	body := string(raw)
	assert.NotContains(t, body, "status_token")
	assert.NotContains(t, body, "token-abc")
}

func TestToAdminRequestSummary_NilPhaseLeavesPhaseNameEmpty(t *testing.T) {
	t.Parallel()

	// Listing endpoints intentionally skip Phase to keep the payload
	// light. The shaper must not panic on a nil Phase pointer.
	in := &enrollmentService.RequestSummary{
		Request:  mkRequest(1234, 5678),
		Phase:    nil,
		Children: []*enrollmentModels.RequestChild{},
	}
	out := toAdminRequestSummary(in)
	assert.Equal(t, "", out.PhaseName, "nil phase → empty PhaseName (no panic)")
	assert.Equal(t, "5678", out.PhaseID, "PhaseID still copied from Request, not Phase")
}

func TestToAdminRequestSummary_EmptyChildrenSliceNotNil(t *testing.T) {
	t.Parallel()

	// JSON consumers (frontend list page) iterate Children without a
	// null check; "" / nil distinction matters when marshalling.
	in := &enrollmentService.RequestSummary{
		Request:  mkRequest(1234, 5678),
		Children: nil,
	}
	out := toAdminRequestSummary(in)
	require.NotNil(t, out.Children, "Children must be []AdminRequestChild{}, not nil, for stable JSON")
	assert.Empty(t, out.Children)
}

func TestToAdminRequestSummary_PreservesNilOptionalPointers(t *testing.T) {
	t.Parallel()

	// GuardianPhone nil, WithdrawnAt nil, ReviewedAt/By nil per child —
	// they must round-trip as nil so omitempty drops them from JSON.
	req := mkRequest(1234, 5678)
	req.GuardianFirstName = "Anna"
	req.GuardianLastName = "Beispiel"

	child := mkChild(999)
	child.FirstName = "Lara"
	child.LastName = "Beispiel"
	child.DateOfBirth = "2018-03-04"
	child.Status = "submitted"

	in := &enrollmentService.RequestSummary{
		Request:  req,
		Children: []*enrollmentModels.RequestChild{child},
	}
	out := toAdminRequestSummary(in)
	assert.Nil(t, out.GuardianPhone)
	assert.Nil(t, out.WithdrawnAt)
	assert.Nil(t, out.Children[0].ReviewedAt)
	assert.Nil(t, out.Children[0].ReviewedBy)
	assert.Nil(t, out.Children[0].TargetGradeLevel)
	assert.Nil(t, out.Children[0].StatusReason)
}

func TestToAdminRequestSummary_TargetGradeLevelPassesThrough(t *testing.T) {
	t.Parallel()

	g := int16(2)
	reason := "Grade level above max"
	reviewed := time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC)
	reviewer := int64(42)
	child := mkChild(999)
	child.FirstName = "Lara"
	child.LastName = "Beispiel"
	child.DateOfBirth = "2018-03-04"
	child.TargetGradeLevel = &g
	child.Status = "pending_admin_review"
	child.StatusReason = &reason
	child.ReviewedAt = &reviewed
	child.ReviewedBy = &reviewer

	in := &enrollmentService.RequestSummary{
		Request:  mkRequest(1234, 5678),
		Children: []*enrollmentModels.RequestChild{child},
	}
	out := toAdminRequestSummary(in)
	require.NotNil(t, out.Children[0].TargetGradeLevel)
	assert.Equal(t, int16(2), *out.Children[0].TargetGradeLevel)
	require.NotNil(t, out.Children[0].StatusReason)
	assert.Equal(t, "Grade level above max", *out.Children[0].StatusReason)
	require.NotNil(t, out.Children[0].ReviewedBy)
	assert.Equal(t, int64(42), *out.Children[0].ReviewedBy)
}

func TestToAdminRequestSummary_PreservesCustomDataMap(t *testing.T) {
	t.Parallel()

	child := mkChild(999)
	child.FirstName = "Lara"
	child.LastName = "Beispiel"
	child.DateOfBirth = "2018-03-04"
	child.CustomData = map[string]any{"allergies": "Nüsse"}

	in := &enrollmentService.RequestSummary{
		Request:  mkRequest(1234, 5678),
		Children: []*enrollmentModels.RequestChild{child},
	}
	out := toAdminRequestSummary(in)
	require.Len(t, out.Children, 1)
	assert.Equal(t, "Nüsse", out.Children[0].CustomData["allergies"])
}
