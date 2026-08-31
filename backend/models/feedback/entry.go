package feedback

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Feedback value constants for standardized feedback types
const (
	ValuePositive = "positive"
	ValueNeutral  = "neutral"
	ValueNegative = "negative"
)

// Entry represents a feedback entry from a student
type Entry struct {
	base.Model `bun:"schema:feedback,table:entries"`
	base.TenantModel
	Value           string        `bun:"value,notnull" json:"value"`
	Day             timezone.Date `bun:"day,notnull,type:date" json:"day"`
	Time            time.Time     `bun:"time,notnull" json:"time"`
	StudentID       int64         `bun:"student_id,notnull" json:"student_id"`
	IsMensaFeedback bool          `bun:"is_mensa_feedback,notnull,default:false" json:"is_mensa_feedback"`

	// Relations not stored in the database
	Student *users.Student `bun:"-" json:"student,omitempty"`
}

// Validate ensures feedback entry data is valid
func (e *Entry) Validate() error {
	if e.Value == "" {
		return errors.New("feedback value is required")
	}

	// Trim spaces from feedback value
	e.Value = strings.TrimSpace(e.Value)

	// Validate feedback value is one of the allowed values
	if e.Value != ValuePositive && e.Value != ValueNeutral && e.Value != ValueNegative {
		return errors.New("value must be 'positive', 'neutral', or 'negative'")
	}

	if e.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	// Ensure day is not zero
	if e.Day.IsZero() {
		return errors.New("day is required")
	}

	// Ensure time is not zero
	if e.Time.IsZero() {
		return errors.New("time is required")
	}

	return nil
}

// SetStudent links this feedback entry to a student
func (e *Entry) SetStudent(student *users.Student) {
	e.Student = student
	if student != nil {
		e.StudentID = student.ID
	}
}

// IsForMensa returns whether this feedback is related to the cafeteria
func (e *Entry) IsForMensa() bool {
	return e.IsMensaFeedback
}

// SetMensaFeedback sets whether this feedback is related to the cafeteria
func (e *Entry) SetMensaFeedback(isMensa bool) {
	e.IsMensaFeedback = isMensa
}

// GetTimestamp returns a full timestamp combining the day and time fields
func (e *Entry) GetTimestamp() time.Time {
	// Extract time components from the Time field
	hour, min, sec := e.Time.Clock()

	// Combine into a single timestamp
	return time.Date(e.Day.Year(), e.Day.Month(), e.Day.Day(), hour, min, sec, 0, time.UTC)
}

// GetFormattedDate returns the day in a formatted string
func (e *Entry) GetFormattedDate() string {
	return e.Day.String()
}

// GetFormattedTime returns the time in a formatted string
func (e *Entry) GetFormattedTime() string {
	return e.Time.Format("15:04:05")
}
