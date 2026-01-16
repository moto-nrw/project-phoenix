package feedback

import "net/http"

// SubmitFeedbackHandler returns the deviceSubmitFeedback handler.
func (rs *Resource) SubmitFeedbackHandler() http.HandlerFunc { return rs.deviceSubmitFeedback }
