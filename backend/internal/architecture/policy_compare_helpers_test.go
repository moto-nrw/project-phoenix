package architecture

// comparePolicyStrictness is comparePolicyStrictnessWithRenames for a
// candidate without renamed tables, the shape most rule tests need.
func comparePolicyStrictness(base, candidate *Policy, createdDataObjects, createdPackages, candidateOnlyExternal, deletedLegacySymbols map[string]struct{}, verifiedRelocations map[string]string) error {
	return comparePolicyStrictnessWithRenames(base, candidate, createdDataObjects, nil, createdPackages, candidateOnlyExternal, deletedLegacySymbols, verifiedRelocations)
}
