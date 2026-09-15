package config

import "github.com/moto-nrw/project-phoenix/models/config"

// Project the compatibility value to the effective three-choice UI. The
// stored value and override provenance stay intact; merely viewing the schema
// never pins an inherited permission. Reset still removes the override.
func projectParentAbsenceReviewScope(settings map[string]*ResolvedSetting, snapshot *SettingsSnapshot) error {
	setting := settings[config.KeyParentAbsenceReviewScope]
	if setting == nil {
		return nil
	}
	groupLeaders, err := snapshot.Bool(config.KeyParentRequestGroupLeaderReviewEnabled)
	if err != nil {
		return err
	}
	inherited := config.ParentAbsenceReviewScopeAdmins
	if groupLeaders {
		inherited = config.ParentAbsenceReviewScopeGroupLeaders
	}
	setting.Default = inherited
	if setting.Value == config.ParentAbsenceReviewScopeInherit {
		setting.Value = inherited
	}
	options := make([]config.SelectOption, 0, 3)
	for _, option := range setting.Options.Static {
		if option.Value != config.ParentAbsenceReviewScopeInherit {
			options = append(options, option)
		}
	}
	setting.Options = &config.SelectOptions{Static: options}
	return nil
}
