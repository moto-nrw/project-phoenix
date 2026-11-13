//go:build testhooks

package staffmessaging

import "net/http"

// RenderStaffMessagingErrorForTest renders err the way the staff-messaging
// routes do, so the error-code tests pin the envelope without a route fixture.
func RenderStaffMessagingErrorForTest(w http.ResponseWriter, r *http.Request, err error) {
	renderStaffMessagingError(w, r, err)
}
