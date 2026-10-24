package api

import (
	"context"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

// EnrollmentGuardianAutofill binds the autofill profile the enrollment form
// prefills (#2734) to the retained guardian profile loader, which still reads
// the People Directory rows. It only translates them into Enrollment's
// public values.
type EnrollmentGuardianAutofill struct {
	Loader *peopleCompose.GuardianProfileLoader
}

// NewEnrollmentGuardianAutofill binds the retained loader; without one the
// reader stays unbound and the route answers as not wired.
func NewEnrollmentGuardianAutofill(loader *peopleCompose.GuardianProfileLoader) enrollmentOwner.GuardianAutofillReader {
	if loader == nil {
		return nil
	}
	return EnrollmentGuardianAutofill{Loader: loader}
}

// LoadForTenant loads the profile the account holds in the school.
func (a EnrollmentGuardianAutofill) LoadForTenant(ctx context.Context, accountID, tenantID int64) (*enrollmentOwner.GuardianAutofill, error) {
	loaded, err := a.Loader.LoadForTenant(ctx, accountID, tenantID)
	if err != nil || loaded == nil || loaded.Profile == nil {
		return nil, err
	}
	profile := &enrollmentOwner.GuardianAutofill{
		FirstName: loaded.Profile.FirstName, LastName: loaded.Profile.LastName,
		Email: loaded.Profile.Email, PrimaryPhone: loaded.PrimaryPhone,
		Children: make([]enrollmentOwner.GuardianAutofillChild, 0, len(loaded.Children)),
	}
	for _, child := range loaded.Children {
		profile.Children = append(profile.Children, enrollmentOwner.GuardianAutofillChild{
			StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, EnrollmentSubmit: child.EnrollmentSubmit, Status: child.Status,
		})
	}
	return profile, nil
}
