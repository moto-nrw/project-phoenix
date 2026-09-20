package repositories

func CaregiverChainsForTests(membership staffLookup) caregiverChainQuery {
	return caregiverChainsFromMembership(membership)
}
