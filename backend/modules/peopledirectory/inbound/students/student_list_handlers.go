package students

import (
	context "context"
	http "net/http"
	sort "sort"
	strings "strings"

	common "github.com/moto-nrw/project-phoenix/api/common"
	collation "github.com/moto-nrw/project-phoenix/sharedkernel/collation"
)

func (rs *Resource) listSchoolClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := rs.PeopleDirectory.ListSchoolClasses(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	classes, err = rs.appendClassListEntryClasses(r.Context(), classes)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, classes, "School classes retrieved successfully")
}

// appendClassListEntryClasses unions the classes that exist only through
// class-list-only entries (#2382) into the dropdown list: a class whose
// children are all list entries must still be selectable for class lists.
// Dedupe uses the LOWER(TRIM(...)) identity every class comparison uses.
func (rs *Resource) appendClassListEntryClasses(ctx context.Context, classes []string) ([]string, error) {
	if rs.ClassListEntries == nil {
		return classes, nil
	}
	entries, err := rs.ClassListEntries.ListClassListEntriesInDisplayOrder(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(classes))
	for _, class := range classes {
		seen[strings.ToLower(strings.TrimSpace(class))] = true
	}
	added := false
	for _, entry := range entries {
		key := strings.ToLower(strings.TrimSpace(entry.SchoolClass))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		classes = append(classes, strings.TrimSpace(entry.SchoolClass))
		added = true
	}
	if added {
		sort.SliceStable(classes, func(i, j int) bool {
			return collation.CompareSchoolClasses(classes[i], classes[j]) < 0
		})
	}
	return classes, nil
}
