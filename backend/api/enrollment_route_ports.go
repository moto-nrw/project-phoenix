package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// parentEnrollmentForms binds the parents portal's form flow to the
// enrollment routes (#2734); without the intake the port stays unbound and
// the portal answers its enrollment routes as not configured.
func parentEnrollmentForms(requests enrollmentAPI.RequestService, uploads enrollmentAPI.UploadService, files enrollmentAPI.UploadFileStorage) parentAPI.EnrollmentForms {
	if requests == nil || uploads == nil || files == nil {
		return nil
	}
	return enrollmentAPI.ParentForms{Requests: requests, Uploads: uploads, UploadFiles: files}
}

// parentGuardianProfiles binds the parents portal's enrollment autofill read
// to the guardian profile loader People Directory composes (#3755). It only
// translates the loaded rows into the HTTP values the response renders, so
// the portal names no People Directory model (#3421).
type parentGuardianProfiles struct {
	loader *peopleCompose.GuardianProfileLoader
}

// newParentGuardianProfiles binds the loader; without one the port stays
// unbound and the profile route answers as not wired.
func newParentGuardianProfiles(loader *peopleCompose.GuardianProfileLoader) parentAPI.GuardianProfileLoader {
	if loader == nil {
		return nil
	}
	return parentGuardianProfiles{loader: loader}
}

// LoadForTenant loads the profile the account holds in the school.
func (p parentGuardianProfiles) LoadForTenant(ctx context.Context, accountID, tenantID int64) (*common.GuardianProfileData, error) {
	loaded, err := p.loader.LoadForTenant(ctx, accountID, tenantID)
	if err != nil || loaded == nil || loaded.Profile == nil {
		return nil, err
	}
	data := &common.GuardianProfileData{
		Present: true, FirstName: loaded.Profile.FirstName, LastName: loaded.Profile.LastName,
		Email: loaded.Profile.Email, PrimaryPhone: loaded.PrimaryPhone,
		Children: make([]common.GuardianProfileChildData, 0, len(loaded.Children)),
	}
	for _, child := range loaded.Children {
		data.Children = append(data.Children, common.GuardianProfileChildData{
			StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, EnrollmentSubmit: child.EnrollmentSubmit, Status: child.Status,
		})
	}
	return data, nil
}
