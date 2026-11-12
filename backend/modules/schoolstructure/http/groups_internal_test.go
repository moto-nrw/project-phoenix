// Package schoolstructurehttp internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package schoolstructurehttp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
)

// =============================================================================
// Request Type Tests
// =============================================================================

func TestGroupRequest_Fields(t *testing.T) {
	t.Parallel()

	roomID := int64(5)
	req := GroupRequest{
		Name:       "Test Group",
		RoomID:     &roomID,
		TeacherIDs: []int64{10, 20},
	}

	assert.Equal(t, "Test Group", req.Name)
	assert.Equal(t, int64(5), *req.RoomID)
	assert.Len(t, req.TeacherIDs, 2)
}

func TestGroupRequest_Bind_Valid(t *testing.T) {
	t.Parallel()

	req := GroupRequest{Name: "Valid Group"}
	err := req.Bind(nil)
	assert.NoError(t, err)
}

func TestGroupRequest_Bind_EmptyName(t *testing.T) {
	t.Parallel()

	req := GroupRequest{Name: ""}
	err := req.Bind(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

// =============================================================================
// Response Type Tests
// =============================================================================

func TestGroupResponse_Fields(t *testing.T) {
	t.Parallel()

	roomID := int64(100)
	repID := int64(200)
	now := time.Now()

	resp := GroupResponse{
		ID:               1,
		Name:             "Response Group",
		RoomID:           &roomID,
		RepresentativeID: &repID,
		StudentCount:     25,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, "Response Group", resp.Name)
	assert.Equal(t, int64(100), *resp.RoomID)
	assert.Equal(t, int64(200), *resp.RepresentativeID)
	assert.Equal(t, 25, resp.StudentCount)
}

func TestTeacherResponse_Fields(t *testing.T) {
	t.Parallel()

	resp := TeacherResponse{
		ID:             1,
		StaffID:        2,
		FirstName:      "John",
		LastName:       "Doe",
		Specialization: "Math",
		Role:           "lead",
		FullName:       "John Doe",
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, int64(2), resp.StaffID)
	assert.Equal(t, "John", resp.FirstName)
	assert.Equal(t, "Doe", resp.LastName)
	assert.Equal(t, "Math", resp.Specialization)
	assert.Equal(t, "lead", resp.Role)
}

// NOTE: NewResource requires non-nil services, tested via integration tests in groups_test.go

type careDayRule struct {
	holds bool
	calls int
}

func (c *careDayRule) InClassStillHolds(context.Context) (bool, error) {
	c.calls++
	return c.holds, nil
}

// After the care day a child still "Im Unterricht" reads "Abwesend", like in
// the staff views (#5710). Before it the label stays.
func TestEndInClassAfterCareDay(t *testing.T) {
	t.Parallel()
	list := func() []GroupStudentResponse {
		return []GroupStudentResponse{
			{ID: 1, Location: studentpresence.InClassLocationLabel},
			{ID: 2, Location: "Anwesend - Raum 1"},
			{ID: 3, Location: studentpresence.InClassLocationLabel},
		}
	}

	over := &careDayRule{holds: false}
	afterEnd := list()
	require.NoError(t, (&Resource{InClass: over}).endInClassAfterCareDay(t.Context(), afterEnd))
	assert.Equal(t, studentpresence.AbsentLocationLabel, afterEnd[0].Location)
	assert.Equal(t, "Anwesend - Raum 1", afterEnd[1].Location)
	assert.Equal(t, studentpresence.AbsentLocationLabel, afterEnd[2].Location)
	assert.Equal(t, 1, over.calls, "one settings read per list")

	running := &careDayRule{holds: true}
	beforeEnd := list()
	require.NoError(t, (&Resource{InClass: running}).endInClassAfterCareDay(t.Context(), beforeEnd))
	assert.Equal(t, studentpresence.InClassLocationLabel, beforeEnd[0].Location)

	idle := &careDayRule{}
	require.NoError(t, (&Resource{InClass: idle}).endInClassAfterCareDay(t.Context(), []GroupStudentResponse{{ID: 4, Location: "Abwesend"}}))
	assert.Zero(t, idle.calls, "no child in class, no settings read")
}
