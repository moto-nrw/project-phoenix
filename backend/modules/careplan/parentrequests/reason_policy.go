package parentrequests

// Values of operations.parent_request_reason_policy, the school's answer to
// who must give a reason on a parent request (#2267, story 28). They equal the
// Settings registry's options.
const (
	ReasonPolicyNobody    = "nobody"
	ReasonPolicyGuardians = "guardians"
	ReasonPolicyStaff     = "staff"
	ReasonPolicyBoth      = "both"
)

// ReasonSide is the side of a parent request the reason policy is asked for.
type ReasonSide int

const (
	// ReasonSideGuardian is the family submitting the request.
	ReasonSideGuardian ReasonSide = iota
	// ReasonSideStaff is the staff member approving it.
	ReasonSideStaff
)

// ReasonRequired reports whether the given side must state a reason under the
// school's policy. The parents portal and the staff routes both ask here, so
// the two sides cannot drift apart.
//
// An unknown policy value is treated as "both": asking for a reason that was
// not required is a nuisance, while skipping one that was required loses
// information nobody can recover later.
//
// Rejections are NOT covered here: a refused request always carries a reason,
// whatever the policy says.
func ReasonRequired(policy string, side ReasonSide) bool {
	switch policy {
	case ReasonPolicyNobody:
		return false
	case ReasonPolicyGuardians:
		return side == ReasonSideGuardian
	case ReasonPolicyStaff:
		return side == ReasonSideStaff
	default:
		return true
	}
}
