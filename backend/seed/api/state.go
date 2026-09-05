package api

import demoprofile "github.com/moto-nrw/project-phoenix/demoprofile"

const DefaultSeedStatePath = demoprofile.DefaultSeedStatePath
const CurrentSeedStateVersion = demoprofile.CurrentSeedStateVersion
const DefaultProfileKey = demoprofile.DefaultProfileKey
const ManualProfileKey = demoprofile.ManualProfileKey

const (
	SettingManagedByOperator = demoprofile.SettingManagedByOperator
	SettingManagedByTenant   = demoprofile.SettingManagedByTenant
)

type SeedState = demoprofile.SeedState
type SeedOrganization = demoprofile.SeedOrganization
type SeedProfile = demoprofile.SeedProfile
type SeedOrganizationRef = demoprofile.SeedOrganizationRef
type SeedSchoolRef = demoprofile.SeedSchoolRef
type SeedSetting = demoprofile.SeedSetting
type SeedProfileEntities = demoprofile.SeedProfileEntities
type SeedEntityRef = demoprofile.SeedEntityRef
type SeedExpectedState = demoprofile.SeedExpectedState
type SeedScheduledActivityState = demoprofile.SeedScheduledActivityState
type SeedStateCredentials = demoprofile.SeedStateCredentials
type SeedOperatorCredentials = demoprofile.SeedOperatorCredentials
type SeedStateTopology = demoprofile.SeedStateTopology
type SeedStateEntities = demoprofile.SeedStateEntities
type SeedStateLookups = demoprofile.SeedStateLookups
type SeedStateScenarios = demoprofile.SeedStateScenarios
type SeedStateBootstrap = demoprofile.SeedStateBootstrap
type SeedStateAccounts = demoprofile.SeedStateAccounts
type ParentCredentials = demoprofile.ParentCredentials
type BootstrapAdminCredentials = demoprofile.BootstrapAdminCredentials
type AccountCredentials = demoprofile.AccountCredentials
type SeedDevice = demoprofile.SeedDevice
type SeedStudent = demoprofile.SeedStudent
type SeedEnrollmentState = demoprofile.SeedEnrollmentState
type SeedEnrollmentRequest = demoprofile.SeedEnrollmentRequest
type SeedParentPortalAction = demoprofile.SeedParentPortalAction

func WriteSeedState(state *SeedState, path string) error {
	return demoprofile.WriteSeedState(state, path)
}

func cloneEnrollmentState(state SeedEnrollmentState) SeedEnrollmentState {
	return demoprofile.CloneEnrollmentState(state)
}

func semanticKey(value string) string {
	return demoprofile.SemanticKey(value)
}
