package test

import "github.com/uptrace/bun"

func ConfigRuntime(db *bun.DB) SettingsRuntimeAdapter { return SettingsRuntimeAdapter{db: db} }
