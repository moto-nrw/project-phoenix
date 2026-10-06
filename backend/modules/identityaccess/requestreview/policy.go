// Package requestreview is the Identity & Access review-scope capability.
// It evaluates request identity facts, never JWTs or models. The review scope
// itself is the caller context's ParentRequestReviews (#3804).
package requestreview

import "errors"

var ErrAbsenceReadRequired = errors.New("the users:read permission is required alongside users:absence")

// Principal contains effective permission facts resolved at the inbound
// identity boundary, including wildcard permissions.
type Principal struct {
	Admin        bool
	UsersUpdate  bool
	UsersRead    bool
	UsersAbsence bool
}
