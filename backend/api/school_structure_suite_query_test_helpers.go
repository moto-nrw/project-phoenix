//go:build testhooks

package api

import "github.com/moto-nrw/project-phoenix/modules/workforce"

// SchoolStructureSuiteSubstitutionPage is the first page of size group
// substitutions. The retained School Structure substitution suites may not
// import Workforce (#3966).
func SchoolStructureSuiteSubstitutionPage(size int) workforce.GroupSubstitutionFilter {
	return workforce.GroupSubstitutionFilter{Limit: size}
}
