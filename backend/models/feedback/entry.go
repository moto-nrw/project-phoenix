package feedback

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Feedback value constants for standardized feedback types
const (
	ValuePositive = "positive"
	ValueNeutral  = "neutral"
	ValueNegative = "negative"
)

// tableFeedbackEntries is the schema-qualified table name for feedback entries
const tableFeedbackEntries = "feedback.entries"

// Entry represents a feedback entry from a student
type Entry struct {
	base.Model      `bun:"schema:feedback,table:entries"`
	Value           string    `bun:"value,notnull" json:"value"`
	Day             time.Time `bun:"day,notnull" json:"day"`
	Time            time.Time `bun:"time,notnull" json:"time"`
	StudentID       int64     `bun:"student_id,notnull" json:"student_id"`
	IsMensaFeedback bool      `bun:"is_mensa_feedback,notnull,default:false" json:"is_mensa_feedback"`

	// Relations not stored in the database
	Student *users.Student `bun:"-" json:"student,omitempty"`
}

func (e *Entry) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableFeedbackEntries)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableFeedbackEntries)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableFeedbackEntries)
	}
	return nil
}

// TableName returns the database table name
func (e *Entry) TableName() string {
	return tableFeedbackEntries
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
	// Extract date components from the Day field
	year, month, day := e.Day.Date()

	// Extract time components from the Time field
	hour, min, sec := e.Time.Clock()

	// Combine into a single timestamp
	return time.Date(year, month, day, hour, min, sec, 0, time.UTC)
}

// GetFormattedDate returns the day in a formatted string
func (e *Entry) GetFormattedDate() string {
	return e.Day.Format("2006-01-02")
}

// GetFormattedTime returns the time in a formatted string
func (e *Entry) GetFormattedTime() string {
	return e.Time.Format("15:04:05")
}

// GetID returns the entity's ID
func (m *Entry) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Entry) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Entry) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
