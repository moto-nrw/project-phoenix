package enrollment

import "context"

// GuardianAutofill is the profile a guardian account holds in one school,
// with the children linked to it, as the enrollment form prefills it.
// People Directory owns the rows; the composition root binds
// GuardianAutofillReader to them (#2734).
type GuardianAutofill struct {
	FirstName, LastName string
	Email               *string
	PrimaryPhone        string
	Children            []GuardianAutofillChild
}

// GuardianAutofillChild is one child linked to the guardian. EnrollmentSubmit
// reports whether this guardian's relationship grants the enrollment submit
// permission for this child; Status is the child's lifecycle status.
type GuardianAutofillChild struct {
	StudentID                        int64
	FirstName, LastName, SchoolClass string
	EnrollmentSubmit                 bool
	Status                           string
}

// GuardianAutofillReader loads the profile an account holds in one school.
// An account without a profile there is (nil, nil).
type GuardianAutofillReader interface {
	LoadForTenant(ctx context.Context, accountID, tenantID int64) (*GuardianAutofill, error)
}
