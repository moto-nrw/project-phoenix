package users

import (
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// Constants for response messages (S1192 - avoid duplicate string literals)
const (
	msgPersonRetrieved = "Person retrieved successfully"
)

// PersonResponse represents a simplified person response
type PersonResponse struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email,omitempty"`
	TagID     string    `json:"tag_id,omitempty"`
	AccountID int64     `json:"account_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PersonProfileResponse represents a full person profile response
type PersonProfileResponse struct {
	PersonResponse
	RFIDCard *RFIDCardResponse `json:"rfid_card,omitempty"`
	Account  *AccountResponse  `json:"account,omitempty"`
}

// RFIDCardResponse represents an RFID card response
type RFIDCardResponse struct {
	TagID     string    `json:"tag_id"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AccountResponse represents a simplified account response
type AccountResponse struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	IsActive bool   `json:"is_active"`
}

// PersonRequest represents a person creation/update request
type PersonRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	TagID     string `json:"tag_id,omitempty"`
	AccountID int64  `json:"account_id,omitempty"`
}

// RFIDLinkRequest represents an RFID card link request
type RFIDLinkRequest struct {
	TagID string `json:"tag_id"`
}

// AccountLinkRequest represents an account link request
type AccountLinkRequest struct {
	AccountID int64 `json:"account_id"`
}

// Bind validates the person request
func (req *PersonRequest) Bind(_ *http.Request) error {
	// Basic validation
	if req.FirstName == "" {
		return errors.New("first name is required")
	}
	if req.LastName == "" {
		return errors.New("last name is required")
	}
	// Note: TagID and AccountID are optional - they can be linked later
	// This aligns with service layer validation (see person_service.go:188-189)
	return nil
}

// Bind validates the RFID link request
func (req *RFIDLinkRequest) Bind(_ *http.Request) error {
	if req.TagID == "" {
		return errors.New("tag ID is required")
	}
	return nil
}

// Bind validates the account link request
func (req *AccountLinkRequest) Bind(_ *http.Request) error {
	if req.AccountID == 0 {
		return errors.New("account ID is required")
	}
	return nil
}

// newPersonResponse creates a person response from a person model
func newPersonResponse(person *users.Person) PersonResponse {
	response := PersonResponse{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		CreatedAt: person.CreatedAt,
		UpdatedAt: person.UpdatedAt,
	}

	if person.TagID != nil {
		response.TagID = *person.TagID
	}

	if person.AccountID != nil {
		response.AccountID = *person.AccountID
	}

	// If account information is available
	if person.Account != nil {
		response.Email = person.Account.Email
	}

	return response
}

// newPersonProfileResponse creates a full person profile response
func newPersonProfileResponse(person *users.Person) PersonProfileResponse {
	response := PersonProfileResponse{
		PersonResponse: newPersonResponse(person),
	}

	// Add RFID card if available
	if person.RFIDCard != nil {
		response.RFIDCard = &RFIDCardResponse{
			TagID:     person.RFIDCard.ID,
			IsActive:  person.RFIDCard.Active,
			CreatedAt: person.RFIDCard.CreatedAt,
			UpdatedAt: person.RFIDCard.UpdatedAt,
		}
	}

	// Add account if available
	if person.Account != nil {
		response.Account = &AccountResponse{
			ID:       person.Account.ID,
			Email:    person.Account.Email,
			IsActive: person.Account.Active,
		}
	}

	return response
}
