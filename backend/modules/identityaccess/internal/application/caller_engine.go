package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// callerEngine serves the public caller-context interfaces from the
// application, translating its values and errors.
type CallerEngine struct {
	App *CallerContext
}

func (e CallerEngine) Account(ctx context.Context) (identityaccess.AccountMetadata, error) {
	account, err := e.App.Account(ctx)
	return identityaccess.AccountMetadata(account), mapCallerError(err)
}

func (e CallerEngine) Person(ctx context.Context) (identityaccess.CallerPerson, error) {
	person, err := e.App.Person(ctx)
	return identityaccess.CallerPerson(person), mapCallerError(err)
}

func (e CallerEngine) StaffID(ctx context.Context) (int64, error) {
	id, err := e.App.StaffID(ctx)
	return id, mapCallerError(err)
}

func (e CallerEngine) TeacherID(ctx context.Context) (int64, error) {
	id, err := e.App.TeacherID(ctx)
	return id, mapCallerError(err)
}

func (e CallerEngine) HasCurrentStaff(ctx context.Context) (bool, error) {
	found, err := e.App.HasCurrentStaff(ctx)
	return found, mapCallerError(err)
}

func (e CallerEngine) StudentAccess(ctx context.Context) identityaccess.StudentAccess {
	return identityaccess.StudentAccess(e.App.StudentAccess(ctx))
}

func (e CallerEngine) MyGroupIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.App.MyGroupIDs(ctx)
	return ids, mapCallerError(err)
}

func (e CallerEngine) MyTeacherGroupIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.App.MyTeacherGroupIDs(ctx)
	return ids, mapCallerError(err)
}

func (e CallerEngine) SubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error) {
	ids, err := e.App.SubstitutedGroupIDs(ctx)
	return ids, mapCallerError(err)
}

func (e CallerEngine) MySchoolClasses(ctx context.Context) ([]string, error) {
	classes, err := e.App.MySchoolClasses(ctx)
	return classes, mapCallerError(err)
}

func (e CallerEngine) MyActivityGroupIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.App.MyActivityGroupIDs(ctx)
	return ids, mapCallerError(err)
}

func (e CallerEngine) MyActiveSessionIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.App.MyActiveSessionIDs(ctx)
	return ids, mapCallerError(err)
}

func (e CallerEngine) MySupervisedSessionIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.App.MySupervisedSessionIDs(ctx)
	return ids, mapCallerError(err)
}

func (e CallerEngine) GroupStudentIDs(ctx context.Context, sessionID int64) ([]int64, error) {
	ids, err := e.App.GroupStudentIDs(ctx, sessionID)
	return ids, mapCallerError(err)
}

func (e CallerEngine) GroupVisits(ctx context.Context, sessionID int64) ([]identityaccess.CallerVisit, error) {
	visits, err := e.App.GroupVisits(ctx, sessionID)
	if err != nil {
		return nil, mapCallerError(err)
	}
	var result []identityaccess.CallerVisit
	for _, visit := range visits {
		result = append(result, identityaccess.CallerVisit(visit))
	}
	return result, nil
}

func (e CallerEngine) Navigation(ctx context.Context) (identityaccess.CallerNavigation, error) {
	navigation, err := e.App.Navigation(ctx)
	if err != nil {
		return identityaccess.CallerNavigation{}, mapCallerError(err)
	}
	groups := make([]identityaccess.CallerGroup, 0, len(navigation.Groups))
	for _, group := range navigation.Groups {
		groups = append(groups, identityaccess.CallerGroup(group))
	}
	return identityaccess.CallerNavigation{
		Groups:               groups,
		SupervisedSessionIDs: navigation.SupervisedSessionIDs,
		StaffID:              navigation.StaffID,
		Incomplete:           navigation.Incomplete,
		UnavailableSections:  navigation.UnavailableSections,
	}, nil
}

func (e CallerEngine) SSESubscription(ctx context.Context) (identityaccess.SSESubscription, error) {
	subscription, err := e.App.SSESubscription(ctx)
	return identityaccess.SSESubscription(subscription), mapCallerError(err)
}

func (e CallerEngine) Profile(ctx context.Context) (identityaccess.CallerProfile, error) {
	profile, err := e.App.Profile(ctx)
	return publicProfile(profile), mapCallerError(err)
}

func (e CallerEngine) UpdateProfile(ctx context.Context, update identityaccess.CallerProfileUpdate) (identityaccess.CallerProfile, error) {
	profile, err := e.App.UpdateProfile(ctx, domain.CallerProfileUpdate(update))
	return publicProfile(profile), mapCallerError(err)
}

func (e CallerEngine) UpdateAvatar(ctx context.Context, avatarURL string) (identityaccess.CallerProfile, error) {
	profile, err := e.App.UpdateAvatar(ctx, avatarURL)
	return publicProfile(profile), mapCallerError(err)
}

func publicProfile(profile domain.CallerProfile) identityaccess.CallerProfile {
	result := identityaccess.CallerProfile{
		Account:  identityaccess.AccountMetadata(profile.Account),
		Bio:      profile.Bio,
		Settings: profile.Settings,
	}
	if profile.Person != nil {
		person := identityaccess.CallerPerson(*profile.Person)
		result.Person = &person
	}
	return result
}
