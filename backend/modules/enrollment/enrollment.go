// Package enrollment exposes the Enrollment query and command capability.
package enrollment

import (
	"context"
	"time"
)

// SubmissionRateLimitKeyType discriminates between IP-based and
// email-based rate limit rows in the same table.
const (
	SubmissionRateLimitKeyTypeIP    = "ip"
	SubmissionRateLimitKeyTypeEmail = "email"
)

// SubmissionRateLimitState is the post-increment view the service uses
// to decide whether to admit a submission.
type SubmissionRateLimitState struct {
	Attempts int
	RetryAt  time.Time
}

// SubmissionRateLimitRetention deletes the rate-limit buckets of the school
// in context after the fixed window of the retention list (#4046): a bucket
// keys an IP address or e-mail and goes one day after its window started,
// once the longest submission window is over.
type SubmissionRateLimitRetention interface {
	// DeleteExpiredSubmissionRateLimits reports the buckets it deleted and
	// the window in days.
	DeleteExpiredSubmissionRateLimits(ctx context.Context) (deleted, retentionDays int, err error)
}

// Owner composes Enrollment's query, command and retention capabilities.
// Consumers can depend on the smaller capability they need.
type Owner interface {
	SchemaQueries
	SchemaCommands
	PhaseQueries
	PhaseCommands
	PhaseSchemaLinks
	PhaseExpiryReports
	RequestQueries
	RequestCommands
	ChildQueries
	ChildCommands
	ClassSwitches
	ChangeRequestQueries
	ChangeRequestCommands
	LateInvitationRecords
	RequestGuardianRecords
	DeletionRecords
	OfferingFacts
	AnnouncementApplicants
	SubmissionAttempts
	RequestReads
	SubmissionRateLimitRetention
	SubmittedOfferingQueries
	SubmittedOfferingCommands
	RequestUploadRecords
}
