package domain

import "time"

// RetentionWindow is the fixed retention of the delivery data (#3955): a
// mail or push intent and the delivery log of a mail are deleted 30 days
// after the intent was sent, finally failed or was cancelled. Intents still
// being tried stay.
const RetentionWindow = 30 * 24 * time.Hour

// KeyOnlyTemplates are the mail templates whose idempotency key must outlive
// the window: the guardian welcome goes out once per school however often a
// guardian is linked later (#3534). Retention clears their related entity,
// recipient, content and provider answer and keeps only the key.
var KeyOnlyTemplates = []string{"guardian_welcome"}

// RetentionResult counts the rows one school's retention removed or cleared.
type RetentionResult struct {
	EmailIntents  int
	ClearedEmails int
	DeliveryLogs  int
	PushIntents   int
}

// Total is the number of rows the retention deleted or cleared.
func (r RetentionResult) Total() int {
	return r.EmailIntents + r.ClearedEmails + r.DeliveryLogs + r.PushIntents
}
