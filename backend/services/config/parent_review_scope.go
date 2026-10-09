package config

import "github.com/moto-nrw/project-phoenix/models/config"

// Project the compatibility values to the effective three-choice UI. The
// stored values and override provenance stay intact; merely viewing the schema
// never pins an inherited permission. Reset still removes the override.
//
// The request scope inherits the group-leader switch, and the absence scope
// inherits the effective request scope. The switch itself leaves the settings
// page: the request scope is its one visible successor.
func projectParentReviewScopes(settings, output map[string]*ResolvedSetting, snapshot *SettingsSnapshot) error {
	request := settings[config.KeyParentRequestReviewScope]
	absence := settings[config.KeyParentAbsenceReviewScope]
	if request == nil && absence == nil {
		return nil
	}
	groupLeaders, err := snapshot.Bool(config.KeyParentRequestGroupLeaderReviewEnabled)
	if err != nil {
		return err
	}
	inherited := config.ParentRequestReviewScopeAdmins
	if groupLeaders {
		inherited = config.ParentRequestReviewScopeGroupLeaders
	}
	requestScope := inherited
	if request != nil {
		// Read the stored choice before the projection replaces inherit.
		if stored, ok := request.Value.(string); ok && stored != config.ParentRequestReviewScopeInherit {
			requestScope = stored
		}
		projectInheritedReviewScope(request, inherited)
		delete(output, config.KeyParentRequestGroupLeaderReviewEnabled)
	}
	projectInheritedReviewScope(absence, requestScope)
	return nil
}

func projectInheritedReviewScope(setting *ResolvedSetting, inherited string) {
	if setting == nil {
		return
	}
	setting.Default = inherited
	if setting.Value == config.ParentRequestReviewScopeInherit {
		setting.Value = inherited
	}
	options := make([]config.SelectOption, 0, 3)
	for _, option := range setting.Options.Static {
		if option.Value != config.ParentRequestReviewScopeInherit {
			options = append(options, option)
		}
	}
	setting.Options = &config.SelectOptions{Static: options}
}
