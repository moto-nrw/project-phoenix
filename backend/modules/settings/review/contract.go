// Package review names the tenant setting consumed by request-review policy.
package review

// GroupLeaderEnabled is resolved through the tenant settings registry. The
// contract carries no fallback value; resolution errors remain errors.
const GroupLeaderEnabled = "operations.parent_request_group_leader_review_enabled"

const BookingsAuthoritative = "enrollment.bookings_authoritative"
