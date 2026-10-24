package contracttest_test

import "context"

// The tenant setting keys the suites resolve and expect in broadcasts, as
// config.setting_values stores them. The contract suite may not import the
// Settings Platform, so it spells them.
const (
	settingStudentChangeLogRetentionDays = "gdpr.student_change_log_retention_days"
	settingStudentPhotosEnabled          = "operations.student_photos_enabled"
)

// settingsDouble is a func-field double for the settings ports this
// package's code under test declares. A nil field answers the zero value.
type settingsDouble struct {
	ResolveIntFn func(ctx context.Context, key string) (int, error)
}

func (s *settingsDouble) ResolveInt(ctx context.Context, key string) (int, error) {
	if s.ResolveIntFn != nil {
		return s.ResolveIntFn(ctx, key)
	}
	return 0, nil
}
