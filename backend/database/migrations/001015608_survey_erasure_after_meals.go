package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	surveyErasureAfterMealsVersion     = "1.15.608"
	surveyErasureAfterMealsDescription = "Mandantenlöschung: Essensbestellungen nach den Umfragen erneut eintragen, für Datenbanken mit 1.15.605 vor 1.15.568 (#4781)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     surveyErasureAfterMealsVersion,
		Description: surveyErasureAfterMealsDescription,
		DependsOn: []string{
			mealCombinedRolePermissionsVersion, // the last migration before this one
			surveysVersion,                     // the reinstall that drops the meal tables when it runs after 1.15.605
			mealOrderErasureVersion,            // the erasure functions installed again here
		},
	})
	Migrations.MustRegister(surveyErasureAfterMealsUp, surveyErasureAfterMealsDown)
}

// A database that applied 1.15.605 before 1.15.568 existed (staging deploys
// every development push) runs 1.15.568 afterwards: bun applies every
// unapplied migration, older versions included. Its reinstall of
// development's erasure functions names the survey tables but drops the meal
// tables of 1.15.605, so the preflight rejects every school erasure. Up
// installs the 1.15.605 functions again, which name both; on a fresh
// database it repeats the same definitions.
func surveyErasureAfterMealsUp(ctx context.Context, db *bun.DB) error {
	return mealOrderErasureUp(ctx, db)
}

// Down keeps the installed functions: they are the ones 1.15.605 installs.
func surveyErasureAfterMealsDown(context.Context, *bun.DB) error {
	return nil
}
