package fileformat

import (
	"regexp"
	"slices"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// IServ user export (#1446). The IServ user list exports the columns it
// shows, so the file is recognized by its "Benutzertyp" column, not by a
// fixed layout. Its "Klasse/Information" and "Geburtsdatum" columns reach the
// student fields through columnAliases; the other IServ columns (Account,
// E-Mail-Adresse, Gruppen, …) have no student field and are ignored.
const (
	iservUserTypeColumn = "benutzertyp"
	iservStatusColumn   = "status"
	iservDeactivated    = "deaktiviert"
)

// iservPupilType matches the user type names IServ suggests for pupils
// ("Schüler(innen)", "Schüler", "Schueler", "Schülerinnen und Schüler").
// It only preselects; the person confirms or changes the choice in the
// preview.
var iservPupilType = regexp.MustCompile(`(?i)sch(ü|ue)ler|student`)

// iservFilter keeps the rows of the chosen user types and counts what the
// file contains for the preview.
type iservFilter struct {
	chosen      map[string]bool // nil: preselect pupil types
	counts      map[string]int
	deactivated int
}

// newIServFilter returns nil when the header is no IServ user export.
func newIServFilter(mapping map[string]int, opts importModels.StudentFileOptions) *iservFilter {
	if _, ok := mapping[iservUserTypeColumn]; !ok {
		return nil
	}
	filter := &iservFilter{counts: make(map[string]int)}
	if opts.IServUserTypes != nil {
		filter.chosen = make(map[string]bool, len(opts.IServUserTypes))
		for _, value := range opts.IServUserTypes {
			filter.chosen[strings.TrimSpace(value)] = true
		}
	}
	return filter
}

// keep counts the row and reports whether it is imported. Deactivated
// accounts are left out: IServ deactivates the accounts of people who left.
func (f *iservFilter) keep(mapper *ColumnMapper) bool {
	if f == nil {
		return true
	}
	if strings.EqualFold(mapper.GetRawCol(iservStatusColumn), iservDeactivated) {
		f.deactivated++
		return false
	}
	userType := mapper.GetRawCol(iservUserTypeColumn)
	f.counts[userType]++
	return f.selected(userType)
}

func (f *iservFilter) selected(userType string) bool {
	if f.chosen == nil {
		return iservPupilType.MatchString(userType)
	}
	return f.chosen[userType]
}

// summary lists the user types, most frequent first.
func (f *iservFilter) summary() *importModels.IServSummary {
	if f == nil {
		return nil
	}
	types := make([]importModels.IServUserType, 0, len(f.counts))
	for value, count := range f.counts {
		types = append(types, importModels.IServUserType{Value: value, Count: count, Selected: f.selected(value)})
	}
	slices.SortFunc(types, func(a, b importModels.IServUserType) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Value, b.Value)
	})
	return &importModels.IServSummary{UserTypes: types, DeactivatedCount: f.deactivated}
}
