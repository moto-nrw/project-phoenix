package test

import (
	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/uptrace/bun"
)

func ConfigRuntime(db *bun.DB) repoBase.ConfigRuntime { return repoBase.NewConfigRuntime(db) }
