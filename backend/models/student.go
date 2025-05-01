// Package models contains application specific entities.
package models

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"
)

// Student represents a student in the system
type Student struct {
	ID           int64       `json:"id" bun:"id,pk,autoincrement"`
	SchoolClass  string      `json:"school_class" bun:"school_class,notnull"`
	Bus          bool        `json:"bus" bun:"bus,notnull,default:false"`
	NameLG       string      `json:"name_lg" bun:"name_lg,notnull"`       // Legal Guardian name
	ContactLG    string      `json:"contact_lg" bun:"contact_lg,notnull"` // Legal Guardian contact
	InHouse      bool        `json:"in_house" bun:"in_house,notnull,default:false"`
	WC           bool        `json:"wc" bun:"wc,notnull,default:false"`
	SchoolYard   bool        `json:"school_yard" bun:"school_yard,notnull,default:false"`
	CustomUserID int64       `json:"custom_users_id" bun:"custom_users_id,notnull"`
	CustomUser   *CustomUser `json:"custom_user,omitempty" bun:"rel:belongs-to,join:custom_users_id=id"`
	GroupID      int64       `json:"group_id" bun:"group_id,notnull"`
	Group        *Group      `json:"group,omitempty" bun:"rel:belongs-to,join:group_id=id"`
	CreatedAt    time.Time   `json:"created_at" bun:"created_at,notnull"`
	ModifiedAt   time.Time   `json:"updated_at" bun:"modified_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (s *Student) BeforeInsert(db *bun.DB) error {
	now := time.Now()
	s.CreatedAt = now
	s.ModifiedAt = now
	return s.Validate()
}

// BeforeUpdate hook executed before database update operation.
func (s *Student) BeforeUpdate(db *bun.DB) error {
	s.ModifiedAt = time.Now()
	return s.Validate()
}

// Validate validates Student struct and returns validation errors.
func (s *Student) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.SchoolClass, validation.Required),
		validation.Field(&s.NameLG, validation.Required),
		validation.Field(&s.ContactLG, validation.Required),
	)
}

// StudentList represents a simplified view of a student for list displays
type StudentList struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	SchoolClass string `json:"school_class"`
	GroupName   string `json:"group_name"`
	InHouse     bool   `json:"in_house"`
}

// Visit represents a record of a student's visit to a room
type Visit struct {
	ID              int64          `json:"id" bun:"id,pk,autoincrement"`
	Day             time.Time      `json:"day" bun:"day,notnull"`
	StudentID       int64          `json:"student_id" bun:"student_id,notnull"`
	Student         *Student       `json:"student,omitempty" bun:"rel:belongs-to,join:student_id=id"`
	RoomID          int64          `json:"room_id" bun:"room_id,notnull"`
	Room            *Room          `json:"room,omitempty" bun:"rel:belongs-to,join:room_id=id"`
	CombinedGroupID *int64         `json:"combined_group_id,omitempty" bun:"combined_group_id"`
	CombinedGroup   *CombinedGroup `json:"combined_group,omitempty" bun:"rel:belongs-to,join:combined_group_id=id"`
	TimespanID      int64          `json:"timespan_id" bun:"timespan_id,notnull"`
	Timespan        *Timespan      `json:"timespan,omitempty" bun:"rel:belongs-to,join:timespan_id=id"`
	CreatedAt       time.Time      `json:"created_at" bun:"created_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (v *Visit) BeforeInsert(db *bun.DB) error {
	v.CreatedAt = time.Now()
	return nil
}

// Feedback represents feedback given by a student
type Feedback struct {
	ID            int64     `json:"id" bun:"id,pk,autoincrement"`
	FeedbackValue string    `json:"feedback_value" bun:"feedback_value,notnull"`
	Day           time.Time `json:"day" bun:"day,notnull"`
	Time          time.Time `json:"time" bun:"time,notnull"`
	StudentID     int64     `json:"student_id" bun:"student_id,notnull"`
	Student       *Student  `json:"student,omitempty" bun:"rel:belongs-to,join:student_id=id"`
	MensaFeedback bool      `json:"mensa_feedback" bun:"mensa_feedback,notnull,default:false"`
	CreatedAt     time.Time `json:"created_at" bun:"created_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (f *Feedback) BeforeInsert(db *bun.DB) error {
	f.CreatedAt = time.Now()
	return nil
}
