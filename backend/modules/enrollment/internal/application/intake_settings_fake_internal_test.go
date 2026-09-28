package application

import "context"

// intakeSettingsFake answers the typed intake settings port with fixed
// values. errFor fails a read by the setting's name (settingCollectGradeLevel,
// settingCollectSchoolClass, settingCareOfferingsEnabled); legalErr fails the
// legal settings read.
type intakeSettingsFake struct {
	enrollmentEnabled    bool
	allowSubmissionEdit  bool
	collectGradeLevel    bool
	collectSchoolClass   bool
	careOfferingsEnabled bool
	gradeLevelMax        int
	duplicateHandling    string
	legal                LegalSettings
	legalErr             error
	errFor               map[string]error
}

var _ IntakeSettings = intakeSettingsFake{}

func (f intakeSettingsFake) EnrollmentEnabled(context.Context) bool   { return f.enrollmentEnabled }
func (f intakeSettingsFake) AllowSubmissionEdit(context.Context) bool { return f.allowSubmissionEdit }
func (f intakeSettingsFake) StatusTokenTTLDays(context.Context) int   { return 365 }
func (f intakeSettingsFake) AdminNotificationEmails(context.Context) string {
	return ""
}

func (f intakeSettingsFake) DuplicateHandling(context.Context) (string, error) {
	return f.duplicateHandling, nil
}

func (f intakeSettingsFake) CollectGradeLevel(context.Context) (bool, error) {
	return f.collectGradeLevel, f.errFor[settingCollectGradeLevel]
}

func (f intakeSettingsFake) CollectSchoolClass(context.Context) (bool, error) {
	return f.collectSchoolClass, f.errFor[settingCollectSchoolClass]
}

func (f intakeSettingsFake) CareOfferingsEnabled(context.Context) (bool, error) {
	return f.careOfferingsEnabled, f.errFor[settingCareOfferingsEnabled]
}

func (f intakeSettingsFake) GradeLevelMax(context.Context) (int, error) {
	return f.gradeLevelMax, f.errFor[settingGradeLevelMax]
}

func (f intakeSettingsFake) LegalSettings(context.Context) (LegalSettings, error) {
	return f.legal, f.legalErr
}

func (f intakeSettingsFake) ChangeRequestMailsEnabled(context.Context) bool { return false }

// schoolClassSettings is the stand-in for the old concrete-class settings
// stub: grade collection and care offerings are on, concrete-class
// collection follows collect.
func schoolClassSettings(collect bool) intakeSettingsFake {
	return intakeSettingsFake{collectGradeLevel: true, careOfferingsEnabled: true, collectSchoolClass: collect}
}
