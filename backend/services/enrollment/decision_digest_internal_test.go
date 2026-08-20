package enrollment

import (
	"context"
	"errors"
	"testing"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type digestSettingsStub struct {
	mode string
	err  error
}

func (s digestSettingsStub) ResolveString(context.Context, string) (string, error) {
	return s.mode, s.err
}

func TestResolveDecisionNotificationMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings decisionNotificationSettings
		want     string
		wantErr  bool
	}{
		{name: "nil uses registry digest default", want: configModel.EnrollmentNotifyPerDecisionDigest},
		{name: "empty uses registry digest default", settings: digestSettingsStub{}, want: configModel.EnrollmentNotifyPerDecisionDigest},
		{name: "immediate override", settings: digestSettingsStub{mode: configModel.EnrollmentNotifyPerDecisionImmediate}, want: configModel.EnrollmentNotifyPerDecisionImmediate},
		{name: "resolution error", settings: digestSettingsStub{err: errors.New("unavailable")}, wantErr: true},
		{name: "unknown mode", settings: digestSettingsStub{mode: "later"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDecisionNotificationMode(context.Background(), tt.settings)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecisionDigestIdempotencyKeyTracksCanonicalStatusVector(t *testing.T) {
	t.Parallel()

	firstReview := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	secondReview := firstReview.Add(time.Minute)
	children := []*enrollmentModels.RequestChild{
		{RequestID: 9, Status: enrollmentModels.ChildStatusRejected, ReviewedAt: &firstReview},
		{RequestID: 9, Status: enrollmentModels.ChildStatusWaitlisted, ReviewedAt: &firstReview},
	}
	children[0].ID = 12
	children[1].ID = 11

	first := decisionDigestIdempotencyKey(9, children)
	reordered := decisionDigestIdempotencyKey(9, []*enrollmentModels.RequestChild{children[1], children[0]})
	assert.Equal(t, first, reordered, "slice order must not change the material state key")

	children[1].Status = enrollmentModels.ChildStatusApproved
	promoted := decisionDigestIdempotencyKey(9, children)
	assert.NotEqual(t, first, promoted, "a later status transition must create a new key")
	assert.Equal(t, promoted, decisionDigestIdempotencyKey(9, children), "same-state retries must remain stable")

	children[1].Status = enrollmentModels.ChildStatusWaitlisted
	children[1].ReviewedAt = &secondReview
	repeatedStatus := decisionDigestIdempotencyKey(9, children)
	assert.NotEqual(t, first, repeatedStatus, "reopening and deciding the same final status is a new generation")
	previousGeneration := *children[1]
	previousGeneration.ReviewedAt = &firstReview
	assert.NotEqual(t, decisionEmailIdempotencyKey(9, &previousGeneration), decisionEmailIdempotencyKey(9, children[1]))
}
