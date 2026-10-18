//go:build testhooks

package schedule

// IsTerminal reports whether the row is in a final state. Only pending rows
// accept a staff decision or a guardian withdrawal.
func (c *CareScheduleChangeRequest) IsTerminal() bool {
	switch c.Status {
	case CareRequestStatusApproved, CareRequestStatusRejected, CareRequestStatusWithdrawn,
		CareRequestStatusCareEnded, CareRequestStatusDone:
		return true
	default:
		return false
	}
}
