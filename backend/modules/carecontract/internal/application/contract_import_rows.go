package application

import (
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

// Reading the rows of an import of existing contracts (#4723): every row
// becomes a candidate with what the import does with it. Nothing here reads
// or writes; the service gives the schools, the conditions and the running
// contracts it read.

const (
	maxImportNameLength  = 100
	maxImportEmailLength = 320
)

// earliestImportDay bounds the dates a file may name: a year before it is a
// typing mistake.
var earliestImportDay = calendar.NewDate(1990, 1, 1)

// importSchool is a school of the Träger with every offering it has and the
// modules a file can still name: those whose phase has not ended.
type importSchool struct {
	school    carecontract.School
	offerings []carecontract.Offering
	modules   []carecontract.Offering
}

// importCandidate is a row with what the import does with it and, for a
// ready row, the contract it stores.
type importCandidate struct {
	row      carecontract.ContractImportRow
	contract ports.ImportedContract
	// childKey names the child at its school for the comparison with the
	// running contracts and the other rows; empty while the row names none.
	childKey importChildKey
	until    calendar.Date
}

// importChildKey is a child at a school as a file names it: the names
// without case and the date of birth, empty without one.
type importChildKey struct {
	schoolID int64
	name     string
	birth    calendar.Date
}

// same tells whether two keys name one child: the same school and names,
// and no two different dates of birth. A missing date of birth cannot tell
// two children apart, so it never makes a second contract.
func (k importChildKey) same(other importChildKey) bool {
	return k.schoolID == other.schoolID && k.name == other.name && (k.birth == other.birth || k.birth.IsZero() || other.birth.IsZero())
}

func childKeyOf(schoolID int64, firstName, lastName string, birthDate *calendar.Date) importChildKey {
	key := importChildKey{schoolID: schoolID, name: foldImportName(firstName) + "\x1f" + foldImportName(lastName)}
	if birthDate != nil {
		key.birth = *birthDate
	}
	return key
}

// foldImportName compares names without case and without repeated blanks.
func foldImportName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// contractImportCheck reads rows against what the Träger has.
type contractImportCheck struct {
	index     contractImportIndex
	schools   []importSchool
	templates templateIndex
	running   []importChildKey
	contracts []int64
	// origins are the contracts earlier imports stored, by school and
	// origin, whatever became of them.
	origins map[importOrigin]int64
	today   calendar.Date
	seen    []importChildKey
	lines   []int
}

// importOrigin names a contract an import stored at a school.
type importOrigin struct {
	schoolID int64
	origin   string
}

// newContractImportCheck prepares the check over the children of the
// contracts of the Träger: those that still hold today count as running.
func newContractImportCheck(index contractImportIndex, schools []importSchool, templates templateIndex,
	running []ports.ContractChild, today calendar.Date) *contractImportCheck {
	check := &contractImportCheck{index: index, schools: schools, templates: templates, today: today, origins: map[importOrigin]int64{}}
	for _, child := range running {
		if child.ImportOrigin != "" {
			check.origins[importOrigin{child.SchoolID, child.ImportOrigin}] = child.ContractID
		}
		if child.Status == carecontract.StatusWithdrawn || child.Status == carecontract.StatusEnded ||
			(child.EndsOn != nil && child.EndsOn.Before(today)) {
			continue
		}
		check.running = append(check.running, childKeyOf(child.SchoolID, child.FirstName, child.LastName, child.BirthDate))
		check.contracts = append(check.contracts, child.ContractID)
	}
	return check
}

// rowReader collects the problems of one row.
type rowReader struct {
	check *contractImportCheck
	cells []string
	row   *carecontract.ContractImportRow
}

func (r rowReader) problem(field, code string) {
	r.row.Problems = append(r.row.Problems, carecontract.ContractImportProblem{Field: field, Code: code})
}

// text reads a name of at most maxImportNameLength characters.
func (r rowReader) text(field string, required bool) string {
	value := strings.Join(strings.Fields(r.check.index.cell(r.cells, field)), " ")
	switch {
	case value == "" && required:
		r.problem(field, carecontract.ContractImportProblemMissing)
	case utf8.RuneCountInString(value) > maxImportNameLength:
		r.problem(field, carecontract.ContractImportProblemTooLong)
		return ""
	}
	return value
}

// date reads a day; nil for an empty cell or a value that is none.
func (r rowReader) date(field string, required bool) *calendar.Date {
	value := r.check.index.cell(r.cells, field)
	if value == "" {
		if required {
			r.problem(field, carecontract.ContractImportProblemMissing)
		}
		return nil
	}
	date, ok := calendar.ParseFileDate(value)
	if !ok || date.Before(earliestImportDay) {
		r.problem(field, carecontract.ContractImportProblemInvalid)
		return nil
	}
	return &date
}

// firstDay is the first day of the contract: the day the row names, or the
// day the office chose for a file without that column.
func (r rowReader) firstDay() *calendar.Date {
	if chosen := r.check.index.effectiveFrom; chosen != nil && !r.check.index.has(carecontract.ContractImportFieldEffectiveFrom) {
		day := *chosen
		return &day
	}
	return r.date(carecontract.ContractImportFieldEffectiveFrom, true)
}

// candidate reads one row.
func (c *contractImportCheck) candidate(cells []string, line int) importCandidate {
	candidate := importCandidate{row: carecontract.ContractImportRow{Line: line, Problems: []carecontract.ContractImportProblem{},
		Hints: []string{}, ModuleNames: []string{}, Weekdays: []string{}}}
	row := &candidate.row
	reader := rowReader{check: c, cells: cells, row: row}

	row.ChildFirstName = reader.text(carecontract.ContractImportFieldChildFirstName, true)
	row.ChildLastName = reader.text(carecontract.ContractImportFieldChildLastName, true)
	row.ChildBirthDate = reader.date(carecontract.ContractImportFieldChildBirthDate, false)
	if row.ChildBirthDate != nil && row.ChildBirthDate.After(c.today) {
		reader.problem(carecontract.ContractImportFieldChildBirthDate, carecontract.ContractImportProblemFuture)
		row.ChildBirthDate = nil
	}
	party := reader.party()
	row.EffectiveFrom = reader.firstDay()
	fileEnd := reader.date(carecontract.ContractImportFieldEndsOn, false)
	concludedOn := reader.date(carecontract.ContractImportFieldConcludedOn, false)
	if concludedOn != nil && concludedOn.After(c.today) {
		reader.problem(carecontract.ContractImportFieldConcludedOn, carecontract.ContractImportProblemFuture)
	}
	row.DebtorNumber = strings.TrimSpace(c.index.cell(cells, carecontract.ContractImportFieldDebtorNumber))

	school, hasSchool := reader.school()
	var booking importBooking
	fileStart := row.EffectiveFrom
	if hasSchool && row.EffectiveFrom != nil {
		booking = reader.booking(school, *row.EffectiveFrom, fileEnd)
		row.EndsOn = booking.endsOn
		if booking.startsOn.After(*row.EffectiveFrom) {
			start := booking.startsOn
			row.EffectiveFrom, row.Hints = &start, append(row.Hints, carecontract.ContractImportHintStartMoved)
		}
	} else if fileEnd != nil {
		row.EndsOn = fileEnd
	}
	if hasSchool && row.ChildFirstName != "" && row.ChildLastName != "" {
		candidate.childKey = childKeyOf(school.school.ID, row.ChildFirstName, row.ChildLastName, row.ChildBirthDate)
	}
	c.decide(&candidate)
	if row.Status == carecontract.ContractImportReady {
		candidate.contract = importedContract(line, school, *row, booking, party, firstDate(concludedOn, fileStart), c.today)
		if booking.endsOn != nil {
			candidate.until = *booking.endsOn
		}
	}
	return candidate
}

// party reads the contract party: a row that names a parent needs the last
// name, and the mail address must be one when the file gives it. A row that
// names no parent at all, as for a child whose family pays without a
// mandate in the master data of OGS Connect, stores a contract without a
// party and says so.
func (r rowReader) party() ports.NewParty {
	party := ports.NewParty{
		FirstName: r.text(carecontract.ContractImportFieldPartyFirstName, false),
		LastName:  r.text(carecontract.ContractImportFieldPartyLastName, false),
	}
	email := strings.TrimSpace(r.check.index.cell(r.cells, carecontract.ContractImportFieldPartyEmail))
	switch {
	case party.LastName != "":
	case party.FirstName == "" && email == "":
		r.row.Hints = append(r.row.Hints, carecontract.ContractImportHintNoParty)
	default:
		r.problem(carecontract.ContractImportFieldPartyLastName, carecontract.ContractImportProblemMissing)
	}
	if email != "" {
		parsed, err := mail.ParseAddress(email)
		if err != nil || parsed.Address != email || utf8.RuneCountInString(email) > maxImportEmailLength {
			r.problem(carecontract.ContractImportFieldPartyEmail, carecontract.ContractImportProblemInvalid)
			email = ""
		}
	}
	party.Email = email
	r.row.PartyName, r.row.PartyEmail = strings.TrimSpace(party.FirstName+" "+party.LastName), email
	return party
}

// school finds the school of the row: the chosen one, else the one the cell
// names without case and punctuation.
func (r rowReader) school() (importSchool, bool) {
	c := r.check
	if !c.index.has(carecontract.ContractImportFieldSchool) {
		for _, school := range c.schools {
			if school.school.ID == c.index.schoolID {
				r.row.SchoolID, r.row.SchoolName = school.school.ID, school.school.Name
				return school, true
			}
		}
		return importSchool{}, false
	}
	value := c.index.cell(r.cells, carecontract.ContractImportFieldSchool)
	r.row.SchoolName = value
	if value == "" {
		r.problem(carecontract.ContractImportFieldSchool, carecontract.ContractImportProblemMissing)
		return importSchool{}, false
	}
	var found []importSchool
	for _, school := range c.schools {
		if textnorm.HeaderKey(school.school.Name) == textnorm.HeaderKey(value) {
			found = append(found, school)
		}
	}
	switch len(found) {
	case 0:
		r.problem(carecontract.ContractImportFieldSchool, carecontract.ContractImportProblemUnknown)
		return importSchool{}, false
	case 1:
		r.row.SchoolID, r.row.SchoolName = found[0].school.ID, found[0].school.Name
		return found[0], true
	}
	r.problem(carecontract.ContractImportFieldSchool, carecontract.ContractImportProblemAmbiguous)
	return importSchool{}, false
}

// decide gives the row its status: a problem makes it invalid, a child
// another row names a conflict, a child with a running contract a
// duplicate; else it is ready.
func (c *contractImportCheck) decide(candidate *importCandidate) {
	row := &candidate.row
	if len(row.Problems) > 0 {
		row.Status = carecontract.ContractImportInvalid
		return
	}
	for i, key := range c.seen {
		if key.same(candidate.childKey) {
			row.Problems = append(row.Problems, carecontract.ContractImportProblem{
				Field: carecontract.ContractImportFieldChildLastName, Code: carecontract.ContractImportProblemRepeated})
			row.Status, row.RepeatsLine = carecontract.ContractImportConflict, c.lines[i]
			return
		}
	}
	c.seen, c.lines = append(c.seen, candidate.childKey), append(c.lines, row.Line)
	for i, key := range c.running {
		if key.same(candidate.childKey) {
			row.Status, row.ContractID = carecontract.ContractImportDuplicate, c.contracts[i]
			return
		}
	}
	if id, stored := c.origins[importOrigin{candidate.childKey.schoolID, importOriginOf(*row)}]; stored {
		// An earlier import stored this contract; it ended or was withdrawn
		// since. It is not stored a second time.
		row.Status, row.ContractID = carecontract.ContractImportDuplicate, id
		row.Hints = append(row.Hints, carecontract.ContractImportHintImportedBefore)
		return
	}
	row.Status = carecontract.ContractImportReady
}

// importOriginOf is the origin of the contract a row without problems
// names: the child without case, its date of birth and the first day.
func importOriginOf(row carecontract.ContractImportRow) string {
	birth := ""
	if row.ChildBirthDate != nil {
		birth = row.ChildBirthDate.String()
	}
	return strings.Join([]string{foldImportName(row.ChildFirstName), foldImportName(row.ChildLastName), birth, row.EffectiveFrom.String()}, "\x1f")
}

func firstDate(dates ...*calendar.Date) *calendar.Date {
	for _, date := range dates {
		if date != nil {
			return date
		}
	}
	return nil
}

// importedContract is the contract a ready row stores: concluded on paper
// on the day the file names (its conclusion, else its first day there, never
// after today), its first version in force from the first day in moto.
func importedContract(line int, school importSchool, row carecontract.ContractImportRow, booking importBooking,
	party ports.NewParty, concludedOn *calendar.Date, today calendar.Date) ports.ImportedContract {
	from := *row.EffectiveFrom
	concluded := from
	if concludedOn != nil {
		concluded = *concludedOn
	}
	if concluded.After(today) {
		concluded = today
	}
	return ports.ImportedContract{
		Line: line, SchoolID: school.school.ID, ConcludedAt: concluded.BerlinMidnight(), Party: party, ImportKey: importOriginOf(row),
		Version: carecontract.Version{
			Number: 1, Status: carecontract.VersionEffective, Source: carecontract.SourceImport, EffectiveFrom: from,
			EndsOn: booking.endsOn, ChildFirstName: row.ChildFirstName, ChildLastName: row.ChildLastName,
			ChildBirthDate: row.ChildBirthDate, SchoolName: school.school.Name, PhaseName: booking.phaseName,
			Services: boundServices(booking.services, booking.endsOn), PriceCents: booking.price, Terms: booking.terms,
		},
	}
}
