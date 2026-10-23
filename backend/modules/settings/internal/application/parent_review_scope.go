package config

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Project the compatibility values to the effective three-choice UI. The
// stored values and override provenance stay intact; merely viewing the schema
// never pins an inherited permission. Reset still removes the override.
//
// The request scope inherits the group-leader switch, and the absence scope
// inherits the effective request scope. The switch itself leaves the settings
// page: the request scope is its one visible successor.
//
// A school without fixed groups has no group leaders to choose (#1480). The
// option leaves both selects unless it is the effective value, so the page
// still shows what applies and stored choices stay untouched. Inherit ignores
// the old switch there and falls to admins, as the review policy enforces.
func projectParentReviewScopes(settings, output map[string]*config.ResolvedSetting, snapshot *config.SettingsSnapshot) error {
	request := settings[config.KeyParentRequestReviewScope]
	absence := settings[config.KeyParentAbsenceReviewScope]
	if request == nil && absence == nil {
		return nil
	}
	groupMode, err := snapshot.String(config.KeyGroupMode)
	if err != nil {
		return err
	}
	offerGroupLeaders := config.GroupModeOf(groupMode) == config.GroupModeFixedGroups
	inherited := config.ParentRequestReviewScopeAdmins
	if offerGroupLeaders {
		groupLeaders, err := snapshot.Bool(config.KeyParentRequestGroupLeaderReviewEnabled)
		if err != nil {
			return err
		}
		if groupLeaders {
			inherited = config.ParentRequestReviewScopeGroupLeaders
		}
	}
	requestScope := inherited
	if request != nil {
		// Read the stored choice before the projection replaces inherit.
		if stored, ok := request.Value.(string); ok && stored != config.ParentRequestReviewScopeInherit {
			requestScope = stored
		}
		projectInheritedReviewScope(request, inherited, offerGroupLeaders)
		delete(output, config.KeyParentRequestGroupLeaderReviewEnabled)
	}
	projectInheritedReviewScope(absence, requestScope, offerGroupLeaders)
	return nil
}

func projectInheritedReviewScope(setting *config.ResolvedSetting, inherited string, offerGroupLeaders bool) {
	if setting == nil {
		return
	}
	setting.Default = inherited
	if setting.Value == config.ParentRequestReviewScopeInherit {
		setting.Value = inherited
	}
	options := make([]config.SelectOption, 0, 3)
	for _, option := range setting.Options.Static {
		if option.Value == config.ParentRequestReviewScopeInherit {
			continue
		}
		if option.Value == config.ParentRequestReviewScopeGroupLeaders && !offerGroupLeaders && setting.Value != option.Value {
			continue
		}
		options = append(options, option)
	}
	setting.Options = &config.SelectOptions{Static: options}
}
