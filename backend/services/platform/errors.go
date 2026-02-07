package platform

import "fmt"

// OperatorNotFoundError is returned when an operator is not found
type OperatorNotFoundError struct {
	OperatorID int64
	Email      string
}

func (e *OperatorNotFoundError) Error() string {
	if e.Email != "" {
		return fmt.Sprintf("operator with email '%s' not found", e.Email)
	}
	return fmt.Sprintf("operator with ID %d not found", e.OperatorID)
}

// InvalidCredentialsError is returned when credentials are invalid
type InvalidCredentialsError struct{}

func (e *InvalidCredentialsError) Error() string {
	return "invalid credentials"
}

// OperatorInactiveError is returned when an operator account is inactive
type OperatorInactiveError struct {
	OperatorID int64
}

func (e *OperatorInactiveError) Error() string {
	return fmt.Sprintf("operator account %d is inactive", e.OperatorID)
}

// AnnouncementNotFoundError is returned when an announcement is not found
type AnnouncementNotFoundError struct {
	AnnouncementID int64
}

func (e *AnnouncementNotFoundError) Error() string {
	return fmt.Sprintf("announcement with ID %d not found", e.AnnouncementID)
}

// InvalidDataError is returned when data validation fails
type InvalidDataError struct {
	Err error
}

func (e *InvalidDataError) Error() string {
	return fmt.Sprintf("invalid data: %v", e.Err)
}

// PostNotFoundError is returned when a suggestion post is not found
type PostNotFoundError struct {
	PostID int64
}

func (e *PostNotFoundError) Error() string {
	return fmt.Sprintf("suggestion post with ID %d not found", e.PostID)
}

// CommentNotFoundError is returned when an operator comment is not found
type CommentNotFoundError struct {
	CommentID int64
}

func (e *CommentNotFoundError) Error() string {
	return fmt.Sprintf("operator comment with ID %d not found", e.CommentID)
}

// PasswordMismatchError is returned when the current password does not match
type PasswordMismatchError struct{}

func (e *PasswordMismatchError) Error() string {
	return "current password is incorrect"
}
