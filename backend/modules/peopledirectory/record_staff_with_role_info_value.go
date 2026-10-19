package peopledirectory

import (
	"time"
)

type StaffWithRoleInfo struct {
	StaffID   int64     `json:"staff_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	PersonID  int64     `json:"person_id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	AccountID int64     `json:"account_id"`
	Email     string    `json:"email"`
}
