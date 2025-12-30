package feedback

import (
	"errors"
	"net/http"
)

// IoTFeedbackRequest represents a feedback submission from an IoT device
type IoTFeedbackRequest struct {
	StudentID int64  `json:"student_id"`
	Value     string `json:"value"` // "positive", "neutral", or "negative"
}

// Bind validates the feedback request
func (req *IoTFeedbackRequest) Bind(_ *http.Request) error {
	if req.StudentID <= 0 {
		return errors.New("student_id is required and must be positive")
	}
	if req.Value == "" {
		return errors.New("value is required")
	}
	// Value validation is handled by the model's Validate() method
	return nil
}
