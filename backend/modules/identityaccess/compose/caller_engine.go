package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// callerEngine serves the public caller-context interfaces from the
// application, translating its values and errors.
type callerEngine struct {
	app *application.CallerContext
}

func (e callerEngine) Account(ctx context.Context) (identityaccess.AccountMetadata, error) {
	account, err := e.app.Account(ctx)
	return identityaccess.AccountMetadata(account), mapCallerError(err)
}

func (e callerEngine) Person(ctx context.Context) (identityaccess.CallerPerson, error) {
	person, err := e.app.Person(ctx)
	return identityaccess.CallerPerson(person), mapCallerError(err)
}

func (e callerEngine) StaffID(ctx context.Context) (int64, error) {
	id, err := e.app.StaffID(ctx)
	return id, mapCallerError(err)
}

func (e callerEngine) TeacherID(ctx context.Context) (int64, error) {
	id, err := e.app.TeacherID(ctx)
	return id, mapCallerError(err)
}

func (e callerEngine) HasCurrentStaff(ctx context.Context) (bool, error) {
	found, err := e.app.HasCurrentStaff(ctx)
	return found, mapCallerError(err)
}

func (e callerEngine) StudentAccess(ctx context.Context) identityaccess.StudentAccess {
	return identityaccess.StudentAccess(e.app.StudentAccess(ctx))
}

func (e callerEngine) MyGroupIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.app.MyGroupIDs(ctx)
	return ids, mapCallerError(err)
}

func (e callerEngine) SubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error) {
	ids, err := e.app.SubstitutedGroupIDs(ctx)
	return ids, mapCallerError(err)
}

func (e callerEngine) MySchoolClasses(ctx context.Context) ([]string, error) {
	classes, err := e.app.MySchoolClasses(ctx)
	return classes, mapCallerError(err)
}

func (e callerEngine) MyActivityGroupIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.app.MyActivityGroupIDs(ctx)
	return ids, mapCallerError(err)
}

func (e callerEngine) MyActiveSessionIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.app.MyActiveSessionIDs(ctx)
	return ids, mapCallerError(err)
}

func (e callerEngine) MySupervisedSessionIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.app.MySupervisedSessionIDs(ctx)
	return ids, mapCallerError(err)
}

func (e callerEngine) GroupStudentIDs(ctx context.Context, sessionID int64) ([]int64, error) {
	ids, err := e.app.GroupStudentIDs(ctx, sessionID)
	return ids, mapCallerError(err)
}

func (e callerEngine) GroupVisits(ctx context.Context, sessionID int64) ([]identityaccess.CallerVisit, error) {
	visits, err := e.app.GroupVisits(ctx, sessionID)
	if err != nil {
		return nil, mapCallerError(err)
	}
	var result []identityaccess.CallerVisit
	for _, visit := range visits {
		result = append(result, identityaccess.CallerVisit(visit))
	}
	return result, nil
}

func (e callerEngine) Navigation(ctx context.Context) (identityaccess.CallerNavigation, error) {
	navigation, err := e.app.Navigation(ctx)
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

func (e callerEngine) SSESubscription(ctx context.Context) (identityaccess.SSESubscription, error) {
	subscription, err := e.app.SSESubscription(ctx)
	return identityaccess.SSESubscription(subscription), mapCallerError(err)
}

func (e callerEngine) Profile(ctx context.Context) (identityaccess.CallerProfile, error) {
	profile, err := e.app.Profile(ctx)
	return publicProfile(profile), mapCallerError(err)
}

func (e callerEngine) UpdateProfile(ctx context.Context, update identityaccess.CallerProfileUpdate) (identityaccess.CallerProfile, error) {
	profile, err := e.app.UpdateProfile(ctx, domain.CallerProfileUpdate(update))
	return publicProfile(profile), mapCallerError(err)
}

func (e callerEngine) UpdateAvatar(ctx context.Context, avatarURL string) (identityaccess.CallerProfile, error) {
	profile, err := e.app.UpdateAvatar(ctx, avatarURL)
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
