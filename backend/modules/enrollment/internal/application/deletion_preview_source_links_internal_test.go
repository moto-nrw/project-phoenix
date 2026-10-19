package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// sourceLinkDeletionQueries answers the owner reads of the deletion preview
// for a request with two children and nothing else attached.
type sourceLinkDeletionQueries struct{}

func (sourceLinkDeletionQueries) ChildrenForRequest(context.Context, int64, bool) ([]*enrollment.RequestChild, error) {
	return []*enrollment.RequestChild{{ID: 11}, {ID: 12}}, nil
}

func (sourceLinkDeletionQueries) DeletionRequestCounts(context.Context, int64) (*enrollment.DeletionRequestCounts, error) {
	return &enrollment.DeletionRequestCounts{Requests: 1, RequestChildren: 2}, nil
}

func (sourceLinkDeletionQueries) DeletionChildTarget(context.Context, int64, int64) (*enrollment.DeletionChildTarget, error) {
	return &enrollment.DeletionChildTarget{TargetChildren: 1, AllChildren: 2}, nil
}

func (sourceLinkDeletionQueries) DeletionChildCounts(context.Context, int64, int64) (*enrollment.DeletionChildCounts, error) {
	return &enrollment.DeletionChildCounts{}, nil
}

func (sourceLinkDeletionQueries) DeletionGuardianProfileIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}

func (sourceLinkDeletionQueries) DeletionBlockingStudentIDs(context.Context, int64, *int64) ([]int64, error) {
	return nil, nil
}

type emptyGuardianDirectory struct{}

func (emptyGuardianDirectory) ListGuardiansByAccount(context.Context, []int64) ([]DirectoryGuardian, error) {
	return nil, nil
}

func (emptyGuardianDirectory) ListGuardiansByID(context.Context, []int64) ([]DirectoryGuardian, error) {
	return nil, nil
}

func (emptyGuardianDirectory) CountGuardianLinks(context.Context, []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}

type sourceEnrollmentCall struct {
	requestID int64
	childID   *int64
}

func sourceLinkPreview(counts func(context.Context, int64, *int64) (int, error)) *DeletionPreview {
	return NewDeletionPreview(DeletionPreviewDependencies{
		Enrollment:             sourceLinkDeletionQueries{},
		Guardians:              emptyGuardianDirectory{},
		CountAuditAdjustments:  func(context.Context, int64, *int64) (int, error) { return 0, nil },
		CountBookings:          func(context.Context, []int64) (int, error) { return 0, nil },
		CountSourceEnrollments: counts,
		Runtime:                Runtime{Transactions: sourceLinkTransactions{}, TenantID: func(context.Context) int64 { return 7 }},
	})
}

// TestDeletionPreviewCountsTimetableSourceLinks pins that the preview reports
// the timetable enrollments whose source link the deletion clears, read
// through the consumer-owned port for the request and for one child.
func TestDeletionPreviewCountsTimetableSourceLinks(t *testing.T) {
	t.Parallel()

	var calls []sourceEnrollmentCall
	preview := sourceLinkPreview(func(_ context.Context, requestID int64, childID *int64) (int, error) {
		calls = append(calls, sourceEnrollmentCall{requestID: requestID, childID: childID})
		if childID == nil {
			return 3, nil
		}
		return 2, nil
	})

	request, err := preview.PreviewRequest(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, 3, request.Counts.StudentSourceLinksCleared)

	child, err := preview.PreviewChild(context.Background(), 5, 12)
	require.NoError(t, err)
	assert.Equal(t, 2, child.Counts.StudentSourceLinksCleared)

	require.Len(t, calls, 2)
	assert.Equal(t, int64(5), calls[0].requestID)
	assert.Nil(t, calls[0].childID)
	assert.Equal(t, int64(5), calls[1].requestID)
	require.NotNil(t, calls[1].childID)
	assert.Equal(t, int64(12), *calls[1].childID)
}

func TestDeletionPreviewRequiresTheSourceEnrollmentCount(t *testing.T) {
	t.Parallel()

	preview := sourceLinkPreview(nil)

	_, err := preview.PreviewRequest(context.Background(), 5)
	require.ErrorContains(t, err, "source enrollment count capability is required")
	_, err = preview.PreviewChild(context.Background(), 5, 12)
	require.ErrorContains(t, err, "source enrollment count capability is required")
}

type sourceLinkTransactions struct{}

func (sourceLinkTransactions) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
