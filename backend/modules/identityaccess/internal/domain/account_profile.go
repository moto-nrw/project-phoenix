package domain

import "time"

type AccountMetadata struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Email         string
	Username      *string
	Avatar        string
	Active        bool
	IsPasswordOTP bool
	LastLogin     *time.Time
}

type AccountProfile struct {
	Bio      string
	Settings string
}
