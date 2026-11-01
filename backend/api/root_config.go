package api

import (
	"context"
	"errors"
	"os"

	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	securitycontract "github.com/moto-nrw/project-phoenix/modules/securityruntime/ports"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/spf13/viper"
)

type SubstitutionActorResolver struct {
	Identity identityaccess.CallerIdentities
}

func (r SubstitutionActorResolver) ResolveActor(ctx context.Context) (*education.Actor, error) {
	staffID, err := r.Identity.StaffID(ctx)
	if err != nil {
		if expectedMissingSubstitutionIdentity(err) {
			return nil, nil
		}
		return nil, err
	}
	teacherID, err := r.Identity.TeacherID(ctx)
	if err != nil {
		if expectedMissingSubstitutionIdentity(err) {
			return nil, nil
		}
		return nil, err
	}
	return &education.Actor{StaffID: staffID, TeacherID: teacherID}, nil
}

func expectedMissingSubstitutionIdentity(err error) bool {
	return errors.Is(err, identityaccess.ErrCallerNotLinkedToPerson) ||
		errors.Is(err, identityaccess.ErrCallerNotLinkedToStaff) ||
		errors.Is(err, identityaccess.ErrCallerNotLinkedToTeacher)
}

type MealPlanSettingsBinder func(
	func(context.Context) (bool, error),
	func(context.Context) (bool, error),
	func(context.Context) (string, error),
)

type FeedbackSettingsBinder func(
	func(context.Context) (bool, error),
	func(context.Context) (int, error),
)

func CurrentServiceConfig() owners.ServiceConfig {
	return owners.ServiceConfig{
		EmailSMTPHost:              viper.GetString("email_smtp_host"),
		EmailSMTPPort:              viper.GetInt("email_smtp_port"),
		EmailSMTPUser:              viper.GetString("email_smtp_user"),
		EmailSMTPPassword:          viper.GetString("email_smtp_password"),
		EmailFromName:              viper.GetString("email_from_name"),
		EmailFromAddress:           viper.GetString("email_from_address"),
		FrontendURL:                viper.GetString("frontend_url"),
		PublicAPIURL:               viper.GetString("next_public_api_url"),
		ParentsURL:                 viper.GetString("parents_url"),
		SchoolURL:                  viper.GetString("school_url"),
		CatererURL:                 viper.GetString("caterer_url"),
		TraegerURL:                 viper.GetString("traeger_url"),
		AppEnv:                     viper.GetString("app_env"),
		InvitationTokenExpiryHours: viper.GetInt("invitation_token_expiry_hours"),
		PasswordResetExpiryMinutes: viper.GetInt("password_reset_token_expiry_minutes"),
		PostHogAPIKey:              viper.GetString("posthog_api_key"),
		PostHogHost:                viper.GetString("posthog_host"),
		RateLimitEnabled:           viper.GetBool("rate_limit_enabled"),
		JWTSecret:                  viper.GetString("auth_jwt_secret"),
		JWTExpiry:                  viper.GetDuration("auth_jwt_expiry"),
		JWTRefreshExpiry:           viper.GetDuration("auth_jwt_refresh_expiry"),
		TenantDomain:               viper.GetString("tenant_domain"),
		OperatorHostname:           viper.GetString("next_public_operator_hostname"),
		VAPIDPublicKey:             viper.GetString("vapid_public_key"),
		VAPIDPrivateKey:            viper.GetString("vapid_private_key"),
		VAPIDSubscriber:            viper.GetString("vapid_subscriber"),
		StudentDailyCheckoutTime:   os.Getenv("STUDENT_DAILY_CHECKOUT_TIME"),
		HealthDataKeys:             viper.GetString("health_data_keys"),
	}
}

// GuardianInvitationReads serves the People Directory guardian list and the
// parents portal over the guardian invitations.
type GuardianInvitationReads interface {
	peoplePorts.GuardianInvitations
	ListByProfile(ctx context.Context, guardianProfileID int64) ([]parentportal.GuardianInvitationRecord, error)
}

// CallerRowReads is the caller context with the retained rows its consumers
// in this graph read.
type CallerRowReads interface {
	Caller() identityaccess.CallerContext
	owners.CareRequestStaff
	ClassDaySlotListStaff
	securitycontract.StudentAccessUserContext
	classdayCompose.CallerReader
	HasCurrentStaff(context.Context) (bool, error)
}
