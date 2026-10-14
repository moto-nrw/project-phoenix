package api

import usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"

// The query shapes the retained School Structure substitution suites list
// with. The suites may import neither the retained models nor the shared
// fixtures' former People Directory vocabulary (#3966).
var (
	NewSchoolStructureSuiteQueryOptions = usersModels.NewQueryOptions
	NewSchoolStructureSuiteQueryFilter  = usersModels.NewQueryFilter
)
