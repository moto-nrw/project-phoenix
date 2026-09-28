package config

import (
	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

// ReasonRequiredFor reports whether the given side of a parent request must
// state a reason under the school's policy. staff=true asks for the deciding
// staff member, staff=false for the submitting family.
//
// An unknown policy value is treated as "both": the strictest reading is the
// safe one, because asking for a reason that was not required is a nuisance
// while skipping one that was required loses information nobody can recover
// later.
//
// Rejections are NOT covered here — a refused request always carries a reason,
// whatever the policy says.
func ReasonRequiredFor(policy string, staff bool) bool {
	switch policy {
	case configModels.ReasonPolicyNobody:
		return false
	case configModels.ReasonPolicyGuardians:
		return !staff
	case configModels.ReasonPolicyStaff:
		return staff
	default:
		return true
	}
}
