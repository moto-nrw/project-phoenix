package architecture

import "slices"

// studentProjectionReplacementGrants is the fixed ADR 0025 / #3432 cutover,
// not a general epoch permission. The immutable base must still carry the
// compatibility-view grant, so a completed replacement cannot be replayed.
func studentProjectionReplacementGrants(base, candidate *Policy) map[string]struct{} {
	grants := make(map[string]struct{})
	if base.ModulePath != "github.com/moto-nrw/project-phoenix" ||
		candidate.ModulePath != base.ModulePath || candidate.PolicyEpoch <= base.PolicyEpoch {
		return grants
	}
	targets := []DataObject{
		{Name: "users.student_profiles", WriteOwner: "people-directory"},
		{Name: "users.student_school_memberships", WriteOwner: "school-membership"},
		{Name: "users.student_care_profiles", WriteOwner: "care-plan"},
	}
	baseObjects, candidateObjects := dataObjectsByName(base), dataObjectsByName(candidate)
	for _, target := range targets {
		if baseObjects[target.Name] != target || candidateObjects[target.Name] != target {
			return grants
		}
	}
	for _, tuple := range []struct{ id, path string }{
		{"parent-message-inbox", "modules/communication/internal/adapters/parentinbox"},
		{"parent-announcement-audience", "modules/communication/internal/adapters/parentaudience"},
		{"care-exit-view", "modules/careplan/legacy/careexitview"},
	} {
		if !unchangedStudentProjection(base, candidate, tuple.id, tuple.path) {
			continue
		}
		before := studentProjectionAt(base, tuple.id, tuple.path)
		after := studentProjectionAt(candidate, tuple.id, tuple.path)
		if !exactStudentProjectionReplacement(before, after, targets) {
			continue
		}
		for _, target := range targets {
			grants[candidate.absolutePackage(tuple.path)+"|"+target.Name] = struct{}{}
		}
	}
	return grants
}

func unchangedStudentProjection(base, candidate *Policy, id, path string) bool {
	baseOwner, baseOwned := ownersByID(base)[id]
	candidateOwner, candidateOwned := ownersByID(candidate)[id]
	before, baseClassified := base.packageMap()[base.absolutePackage(path)]
	after, candidateClassified := candidate.packageMap()[candidate.absolutePackage(path)]
	return baseOwned && candidateOwned && baseOwner.Kind == "projection" && baseOwner == candidateOwner &&
		baseClassified && candidateClassified && before == after && before.Owner == id && before.Role == "postgres"
}

func studentProjectionAt(policy *Policy, id, path string) ReadProjection {
	for _, projection := range policy.ReadProjections {
		if projection.ID == id && projection.Package == path {
			return projection
		}
	}
	return ReadProjection{}
}

func exactStudentProjectionReplacement(before, after ReadProjection, targets []DataObject) bool {
	if !before.TenantSafe || !after.TenantSafe || !slices.Contains(before.DataObjects, "users.students") {
		return false
	}
	expected := make([]string, 0, len(before.DataObjects)+len(targets)-1)
	for _, object := range before.DataObjects {
		if object != "users.students" {
			expected = append(expected, object)
		}
	}
	for _, target := range targets {
		if !slices.Contains(expected, target.Name) {
			expected = append(expected, target.Name)
		}
	}
	actual := slices.Clone(after.DataObjects)
	slices.Sort(expected)
	slices.Sort(actual)
	return slices.Equal(expected, actual)
}
