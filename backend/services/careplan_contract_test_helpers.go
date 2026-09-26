package services

import (
	"github.com/uptrace/bun"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
)

// Test bindings for Care Plan's contract suite (#3565), which drives the
// enrollment intake and decision flow without naming the Settings Platform
// keys or the Identity & Access composition itself.

// CarePlanContractSettingKeys are the tenant setting keys and values the
// suite's settings double answers.
type CarePlanContractSettingKeys struct {
	Enabled, AllowSubmissionEdit, CollectGradeLevel, CareOfferingsEnabled string
	WaitlistEnabled, DuplicateHandling, DuplicateHandlingWarn             string
	GradeLevelMax, StatusTokenTTLDays                                     string
	LegalTermsEnabled, LegalDSGVOEnabled                                  string
	LegalEmailContactEnabled, LegalPhotoEnabled                           string
	LegalAGBText, LegalDSGVOText, LegalEmailContactText, LegalPhotoText   string
	DefaultActivationMode, NotifyPerDecision, NotifyPerDecisionImmediate  string
	BookingsAuthoritative, OfferingChangesEnabled                         string
	OfferingChangesLeadDays, ParentCourseRequestsEnabled                  string
}

// CarePlanContractKeys names the keys of the Settings Platform registry.
var CarePlanContractKeys = CarePlanContractSettingKeys{
	Enabled:                     configModels.KeyEnrollmentEnabled,
	AllowSubmissionEdit:         configModels.KeyEnrollmentAllowSubmissionEdit,
	CollectGradeLevel:           configModels.KeyEnrollmentCollectGradeLevel,
	CareOfferingsEnabled:        configModels.KeyEnrollmentCareOfferingsEnabled,
	WaitlistEnabled:             configModels.KeyEnrollmentWaitlistEnabled,
	DuplicateHandling:           configModels.KeyEnrollmentDuplicateHandling,
	DuplicateHandlingWarn:       configModels.EnrollmentDuplicateHandlingWarn,
	GradeLevelMax:               configModels.KeyEnrollmentGradeLevelMax,
	StatusTokenTTLDays:          configModels.KeyEnrollmentStatusTokenTTLDays,
	LegalTermsEnabled:           configModels.KeyEnrollmentLegalTermsEnabled,
	LegalDSGVOEnabled:           configModels.KeyEnrollmentLegalDSGVOEnabled,
	LegalEmailContactEnabled:    configModels.KeyEnrollmentLegalEmailContactEnabled,
	LegalPhotoEnabled:           configModels.KeyEnrollmentLegalPhotoEnabled,
	LegalAGBText:                configModels.KeyEnrollmentLegalAGBText,
	LegalDSGVOText:              configModels.KeyEnrollmentLegalDSGVOText,
	LegalEmailContactText:       configModels.KeyEnrollmentLegalEmailContactText,
	LegalPhotoText:              configModels.KeyEnrollmentLegalPhotoText,
	DefaultActivationMode:       configModels.KeyEnrollmentDefaultActivationMode,
	NotifyPerDecision:           configModels.KeyEnrollmentNotifyPerDecision,
	NotifyPerDecisionImmediate:  configModels.EnrollmentNotifyPerDecisionImmediate,
	BookingsAuthoritative:       configModels.KeyEnrollmentBookingsAuthoritative,
	OfferingChangesEnabled:      configModels.KeyEnrollmentOfferingChangesEnabled,
	OfferingChangesLeadDays:     configModels.KeyEnrollmentOfferingChangesLeadDays,
	ParentCourseRequestsEnabled: configModels.KeyEnrollmentParentCourseRequestsEnabled,
}

// NewCarePlanContractGuardianAccess composes the Identity & Access
// capability an approval recognises a parent's portal account through.
func NewCarePlanContractGuardianAccess(db *bun.DB) (enrollmentCompose.DecisionGuardianAccess, error) {
	return identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
}
