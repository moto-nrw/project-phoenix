package application

import (
	"slices"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

// The booked care of a row of an import of existing contracts (#4723): the
// modules the row names at its school, on the weekdays it names, with the
// conditions the Träger keeps for them. An imported contract books modules
// like every contract and ends with their phase.

// importBooking is what a row books: the services per weekday and module,
// the conditions, the price and the last day of the contract, nil for an
// open one.
type importBooking struct {
	services  []carecontract.Service
	terms     carecontract.TemplateInput
	phaseName string
	price     *int64
	endsOn    *calendar.Date
}

// defaultImportWeekdays are the days of a module without fixed days when the
// file names none.
var defaultImportWeekdays = []string{"mon", "tue", "wed", "thu", "fri"}

// booking reads the modules, the weekdays and the end of the row.
func (r rowReader) booking(school importSchool, from calendar.Date, fileEnd *calendar.Date) importBooking {
	modules := r.modules(school, from)
	days, daysOK := r.weekdays()
	booking := importBooking{services: []carecontract.Service{}}
	if len(modules) == 0 || !daysOK {
		booking.endsOn = fileEnd
		return booking
	}
	ids := make([]int64, len(modules))
	for i, module := range modules {
		ids[i] = module.ID
		r.row.ModuleNames = append(r.row.ModuleNames, module.Name)
		booking.phaseName = firstText(booking.phaseName, module.PhaseName)
		booking.services = append(booking.services, r.moduleServices(module, days)...)
	}
	sortServices(booking.services)
	r.row.Weekdays = serviceWeekdays(booking.services)
	if len(modules) == 1 && modules[0].PriceCents != nil {
		price := int64(*modules[0].PriceCents)
		booking.price = &price
	}
	terms, issue := r.check.templates.terms(school.school.ID, ids)
	switch issue {
	case carecontract.SkipTemplateMissing:
		r.problem(carecontract.ContractImportFieldModule, carecontract.ContractImportProblemConditionsMissing)
	case carecontract.SkipTemplatesDiffer:
		r.problem(carecontract.ContractImportFieldModule, carecontract.ContractImportProblemConditionsDiffer)
	}
	booking.terms = terms
	booking.endsOn = r.endsOn(modules, terms, from, fileEnd)
	return booking
}

// modules finds the modules the row names at its school: the chosen module,
// else the cell as one name, else its parts between "+", ",", "/" and "|".
func (r rowReader) modules(school importSchool, from calendar.Date) []carecontract.Offering {
	c := r.check
	value := c.index.moduleName
	if c.index.has(carecontract.ContractImportFieldModule) {
		value = c.index.cell(r.cells, carecontract.ContractImportFieldModule)
	}
	if strings.TrimSpace(value) == "" {
		r.problem(carecontract.ContractImportFieldModule, carecontract.ContractImportProblemMissing)
		return nil
	}
	names := []string{value}
	if !namesOffering(school.offerings, value) {
		names = strings.FieldsFunc(value, func(r rune) bool { return r == '+' || r == ',' || r == '/' || r == '|' })
	}
	reference := from
	if c.today.After(reference) {
		reference = c.today
	}
	var modules []carecontract.Offering
	for _, name := range names {
		module, code := pickImportModule(school.offerings, name, reference)
		if code == "" && module.ServiceStart != nil && reference.Before(*module.ServiceStart) {
			// An older contract books the module of the running phase; a
			// module whose phase has not begun by the reference day is none
			// the child can be in yet.
			code = carecontract.ContractImportProblemBeforePhase
		}
		if code != "" {
			r.problem(carecontract.ContractImportFieldModule, code)
			return nil
		}
		if !slices.ContainsFunc(modules, func(o carecontract.Offering) bool { return o.ID == module.ID }) {
			modules = append(modules, module)
		}
	}
	return modules
}

func namesOffering(offerings []carecontract.Offering, name string) bool {
	key := textnorm.HeaderKey(name)
	return key != "" && slices.ContainsFunc(offerings, func(o carecontract.Offering) bool { return textnorm.HeaderKey(o.Name) == key })
}

// pickImportModule finds the module of a name on the reference day, the
// first day of the contract or today when that has passed: the offering
// whose phase has not ended by then. Of several, the one whose phase runs on
// that day goes first, then the active one; a choice that stays open is
// ambiguous.
func pickImportModule(offerings []carecontract.Offering, name string, reference calendar.Date) (carecontract.Offering, string) {
	key := textnorm.HeaderKey(name)
	var named, open []carecontract.Offering
	for _, offering := range offerings {
		if key == "" || textnorm.HeaderKey(offering.Name) != key {
			continue
		}
		named = append(named, offering)
		if offering.ServiceEnd == nil || !offering.ServiceEnd.Before(reference) {
			open = append(open, offering)
		}
	}
	switch {
	case len(named) == 0:
		return carecontract.Offering{}, carecontract.ContractImportProblemUnknown
	case len(open) == 0:
		return carecontract.Offering{}, carecontract.ContractImportProblemOutsidePhase
	}
	for _, narrow := range []func(carecontract.Offering) bool{
		func(o carecontract.Offering) bool { return o.ServiceStart == nil || !o.ServiceStart.After(reference) },
		func(o carecontract.Offering) bool { return o.Active },
	} {
		if len(open) == 1 {
			break
		}
		if kept := filterOfferings(open, narrow); len(kept) > 0 {
			open = kept
		}
	}
	if len(open) > 1 {
		return carecontract.Offering{}, carecontract.ContractImportProblemAmbiguous
	}
	return open[0], ""
}

func filterOfferings(offerings []carecontract.Offering, keep func(carecontract.Offering) bool) []carecontract.Offering {
	var kept []carecontract.Offering
	for _, offering := range offerings {
		if keep(offering) {
			kept = append(kept, offering)
		}
	}
	return kept
}

// weekdays reads the weekdays the row names; nil when the file names none
// and each module books its own days. ok is false for a cell that names no
// weekdays.
func (r rowReader) weekdays() ([]string, bool) {
	value := r.check.index.cell(r.cells, carecontract.ContractImportFieldWeekdays)
	if value == "" {
		return nil, true
	}
	days, ok := parseImportWeekdays(value)
	if !ok {
		r.problem(carecontract.ContractImportFieldWeekdays, carecontract.ContractImportProblemInvalid)
		return nil, false
	}
	return days, true
}

// moduleServices are the services of one module on the days of the row, or
// on its own days when the row names none. A day the module does not run on
// is a problem of the row.
func (r rowReader) moduleServices(module carecontract.Offering, days []string) []carecontract.Service {
	offered := module.AvailableDays
	if days == nil {
		days = offered
		if len(days) == 0 {
			days = defaultImportWeekdays
		}
	}
	services := make([]carecontract.Service, 0, len(days))
	for _, day := range days {
		if len(offered) > 0 && !slices.Contains(offered, day) {
			r.problem(carecontract.ContractImportFieldWeekdays, carecontract.ContractImportProblemNotOffered)
			return nil
		}
		services = append(services, carecontract.Service{Weekday: day, CareOfferingID: module.ID, ModuleName: module.Name,
			EndsAt: module.EndTimes[day], LastDay: module.ServiceEnd})
	}
	return services
}

func serviceWeekdays(services []carecontract.Service) []string {
	days := []string{}
	for _, day := range weekdayOrder {
		if slices.ContainsFunc(services, func(s carecontract.Service) bool { return s.Weekday == day }) {
			days = append(days, day)
		}
	}
	return days
}

// endsOn is the last day of the contract: the end the file names, the end
// of the phase of its modules or the end of a fixed term, whichever comes
// first; nil for an open contract. An end before the first day or before
// today is a problem: the import takes over running contracts only.
func (r rowReader) endsOn(modules []carecontract.Offering, terms carecontract.TemplateInput, from calendar.Date, fileEnd *calendar.Date) *calendar.Date {
	if fileEnd != nil && fileEnd.Before(from) {
		r.problem(carecontract.ContractImportFieldEndsOn, carecontract.ContractImportProblemBeforeStart)
		return fileEnd
	}
	// The modules end with their phase; the contract holds as long as the
	// longest of them, and not at all when one of them has no end.
	var phaseEnd *calendar.Date
	for i, module := range modules {
		if module.ServiceEnd == nil {
			phaseEnd = nil
			break
		}
		if i == 0 || module.ServiceEnd.After(*phaseEnd) {
			phaseEnd = module.ServiceEnd
		}
	}
	var until *calendar.Date
	for _, end := range []*calendar.Date{fileEnd, phaseEnd} {
		if end != nil && (until == nil || end.AddDays(1).Before(*until)) {
			exclusive := end.AddDays(1)
			until = &exclusive
		}
	}
	ends := endsOn(terms, from, until)
	last := ends
	if last == nil && until != nil {
		// The phase ended before the first day: endsOn names no end then.
		day := until.AddDays(-1)
		last = &day
	}
	switch {
	case last == nil:
	case last.Before(from):
		r.problem(carecontract.ContractImportFieldModule, carecontract.ContractImportProblemOutsidePhase)
	case last.Before(r.check.today):
		r.problem(carecontract.ContractImportFieldEndsOn, carecontract.ContractImportProblemEnded)
	}
	return last
}
