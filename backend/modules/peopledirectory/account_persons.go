package peopledirectory

// AccountSchool names one account at one school.
type AccountSchool struct {
	AccountID int64
	TenantID  int64
}

// LatestPersonsByAccountSchool is the person each account has at each school:
// the one updated last, the earlier one in the list on a tie. Persons
// without an account are left out.
func LatestPersonsByAccountSchool(persons []Person) map[AccountSchool]Person {
	result := make(map[AccountSchool]Person, len(persons))
	for _, person := range persons {
		if person.AccountID == nil {
			continue
		}
		key := AccountSchool{AccountID: *person.AccountID, TenantID: person.TenantID}
		current, found := result[key]
		if !found || person.UpdatedAt.After(current.UpdatedAt) {
			result[key] = person
		}
	}
	return result
}
