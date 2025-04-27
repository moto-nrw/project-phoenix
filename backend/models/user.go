// Package models contains application specific entities.
package models

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/uptrace/bun"
)

// CustomUser represents a user in the system with RFID capabilities
type CustomUser struct {
	ID         int64     `json:"id" bun:"id,pk,autoincrement"`
	FirstName  string    `json:"first_name" bun:"first_name,notnull"`
	SecondName string    `json:"second_name" bun:"second_name,notnull"`
	TagID      *string   `json:"tag_id,omitempty" bun:"tag_id,unique"`
	AccountID  *int64    `json:"account_id,omitempty" bun:"account_id,unique"`
	CreatedAt  time.Time `json:"created_at" bun:"created_at,notnull"`
	ModifiedAt time.Time `json:"updated_at" bun:"modified_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (u *CustomUser) BeforeInsert(db *bun.DB) error {
	now := time.Now()
	u.CreatedAt = now
	u.ModifiedAt = now
	return u.Validate()
}

// BeforeUpdate hook executed before database update operation.
func (u *CustomUser) BeforeUpdate(db *bun.DB) error {
	u.ModifiedAt = time.Now()
	return u.Validate()
}

// Validate validates CustomUser struct and returns validation errors.
func (u *CustomUser) Validate() error {
	return validation.ValidateStruct(u,
		validation.Field(&u.FirstName, validation.Required),
		validation.Field(&u.SecondName, validation.Required),
		validation.Field(&u.TagID, is.Alphanumeric),
	)
}

// PedagogicalSpecialist represents a staff member with special permissions
type PedagogicalSpecialist struct {
	ID            int64       `json:"id" bun:"id,pk,autoincrement"`
	Role          string      `json:"role" bun:"role,notnull"`
	CustomUserID  int64       `json:"custom_user_id" bun:"custom_user_id,notnull"`
	CustomUser    *CustomUser `json:"custom_user,omitempty" bun:"rel:belongs-to,join:custom_user_id=id"`
	UserID        int64       `json:"user_id" bun:"user_id,notnull,unique"`
	IsPasswordOTP bool        `json:"is_password_otp" bun:"is_password_otp,notnull,default:true"`
	CreatedAt     time.Time   `json:"created_at" bun:"created_at,notnull"`
	ModifiedAt    time.Time   `json:"updated_at" bun:"modified_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (p *PedagogicalSpecialist) BeforeInsert(db *bun.DB) error {
	p.CreatedAt = time.Now()
	return p.Validate()
}

// Validate validates PedagogicalSpecialist struct and returns validation errors.
func (p *PedagogicalSpecialist) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Role, validation.Required),
		validation.Field(&p.CustomUserID, validation.Required),
		validation.Field(&p.UserID, validation.Required),
	)
}

// Device represents a device associated with a user
type Device struct {
	ID        int64     `json:"id" bun:"id,pk,autoincrement"`
	UserID    int64     `json:"user_id" bun:"user_id,notnull"`
	DeviceID  string    `json:"device_id" bun:"device_id,notnull,unique"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (d *Device) BeforeInsert(db *bun.DB) error {
	d.CreatedAt = time.Now()
	return d.Validate()
}

// Validate validates Device struct and returns validation errors.
func (d *Device) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.UserID, validation.Required),
		validation.Field(&d.DeviceID, validation.Required),
	)
}
