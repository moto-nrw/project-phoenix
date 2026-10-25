package api

import (
	"context"
	"net/http"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/analytics"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/settings"
)

func analyticsCollectionPolicy(demo bool, reader settings.TenantReader, parents identityaccess.GuardianPortalQuery, schools organizationtenancy.SchoolQuery) analytics.CollectionPolicy {
	return analytics.CollectionPolicy{
		Demo: demo,
		SchoolEnabled: func(ctx context.Context, id int64) (bool, error) {
			return reader.ResolveBoolForTenant(ctx, id, settings.KeyAnalyticsEnabled)
		},
		ParentSchools: func(ctx context.Context, id int64) (map[int64][]int64, error) {
			return parents.FindActiveGuardianMemberships(ctx, []int64{id})
		},
		PublicSchool: func(ctx context.Context, slug string) (int64, error) {
			school, err := schools.FindSchoolBySubdomain(ctx, slug)
			return school.ID, err
		},
	}
}

// analyticsConsent binds the account's consent (#4901) to the core-action
// analytics. An undecided account and a failed lookup count as no consent.
func analyticsConsent(consent identityaccess.AccountAnalyticsConsent) func(context.Context, int64) bool {
	return func(ctx context.Context, accountID int64) bool {
		answer, err := consent.FindAnalyticsConsent(ctx, accountID)
		return err == nil && answer.Granted != nil && *answer.Granted
	}
}

func analyticsRequestActor(r *http.Request) (analytics.Actor, bool) {
	raw, ok := analytics.VerifiedSessionClaims(r)
	if !ok {
		return analytics.Actor{}, false
	}
	var claims sessiontoken.AppClaims
	if claims.ParseClaims(raw) != nil {
		return analytics.Actor{}, false
	}
	return analyticsActor(claims)
}

// workerAnalytics binds both capture paths after the worker has constructed
// its settings, without giving the owner-composition root an HTTP dependency.
func workerAnalytics(tracker owners.ProductEventTracker, reader settings.TenantReader, demo bool) owners.ProductEventTracker {
	gated := analytics.GateSchoolTracker(tracker, demo, analyticsCollectionPolicy(false, reader, nil, nil).SchoolEnabled)
	return gated
}

func buildServeAnalyticsSettings(db transactionruntime.Database, unit tenant.UnitOfWork, env owners.RootEnvironment, organizations organizationtenancy.Capability, hub *realtime.Hub, bookings *enrollmentOwner.BookingProjection, tracker analytics.Tracker) (owners.SettingsOwner, analytics.Tracker, error) {
	values, err := owners.BuildSettingsOwner(db, unit, env, organizations, hub, bookings)
	if err != nil {
		return owners.SettingsOwner{}, nil, err
	}
	return values, analytics.GateSchoolTracker(tracker, IsDemoEnvironment(env.Config.AppEnv), analyticsCollectionPolicy(false, values.Service, nil, nil).SchoolEnabled), nil
}
