package config

import "context"

// OnValueSet installs the hook that runs after a value is set; tests drive
// it directly.
func (rs *SettingsResource) OnValueSet(hook func(context.Context, int64, string, any) (func(), error)) {
	if rs.operations != nil {
		rs.operations.SetValueSetHook(hook)
	}
}
