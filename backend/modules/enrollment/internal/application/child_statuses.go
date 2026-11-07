package application

import enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

// allChildStatuses is every status the child model knows: the rollover
// preview counts them and the care usage report accepts them as a filter.
var allChildStatuses = []string{
	enrollmentNative.ChildStatusSubmitted,
	enrollmentNative.ChildStatusUnderReview,
	enrollmentNative.ChildStatusApproved,
	enrollmentNative.ChildStatusWaitlisted,
	enrollmentNative.ChildStatusRejected,
	enrollmentNative.ChildStatusWithdrawn,
	enrollmentNative.ChildStatusPendingRenewal,
	enrollmentNative.ChildStatusAutoRenewed,
	enrollmentNative.ChildStatusPendingAdminReview,
}
