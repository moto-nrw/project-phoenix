package users

import "time"

// ActiveCaregiver represents the canonical operational caregiver projection.
// A person is only included when the account has the canonical "user" role or
// the legacy "teacher" system role and resolves cleanly through
// person -> staff -> teacher.
type ActiveCaregiver struct {
	AccountID int64     `bun:"account_id" json:"account_id"`
	PersonID  int64     `bun:"person_id" json:"person_id"`
	StaffID   int64     `bun:"staff_id" json:"staff_id"`
	TeacherID int64     `bun:"teacher_id" json:"teacher_id"`
	FirstName string    `bun:"first_name" json:"first_name"`
	LastName  string    `bun:"last_name" json:"last_name"`
	Email     string    `bun:"email" json:"email"`
	CreatedAt time.Time `bun:"created_at" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at" json:"updated_at"`
}

// FullName returns the caregiver's display name.
func (c *ActiveCaregiver) FullName() string {
	return c.FirstName + " " + c.LastName
}
