package domain

import (
	"errors"
	"time"
)

var (
	ErrSchoolNotFound       = errors.New("school not found")
	ErrSchoolSlugConflict   = errors.New("school slug already exists in organization")
	ErrSchoolDomainConflict = errors.New("school subdomain already exists")
	ErrSchoolAlreadyDeleted = errors.New("school is already deleted")
	ErrSchoolNotDeleted     = errors.New("school is not deleted")
	ErrOrganizationDeleted  = errors.New("school organization is deleted")
)

type School struct {
	ID             int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Active         bool
	Hidden         bool
	DeletedAt      *time.Time
	Settings       string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
	DevicePinHash  string
	Organization   *Organization
}

func (s School) IsDeleted() bool { return s.DeletedAt != nil }

type CreateSchool struct {
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Active         bool
	Hidden         bool
	Settings       string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
	DevicePinHash  string
}

type UpdateSchool struct {
	ID             int64
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Active         bool
	Hidden         bool
	Settings       string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
	DevicePinHash  string
}
