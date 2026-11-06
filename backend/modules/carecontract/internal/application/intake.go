package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// maxCreateBatch bounds a creation of several drafts in one request.
const maxCreateBatch = 200

// The views of the access log of a school.
const (
	accessViewCandidates = "care_contract_candidates"
	accessViewCreate     = "care_contract_create"
)

// Intake serves carecontract.Intake. It opens each unit itself: first the
// tenant transaction of every school it reads, then the Träger transaction
// it writes in (ADR 0153 point 3); no transaction holds both.
type Intake struct {
	store        Store
	schools      carecontract.TraegerSchools
	applications carecontract.SchoolApplications
	bookings     carecontract.SchoolBookings
	access       carecontract.SchoolAccessLog
	runtime      carecontract.Runtime
	today        func() calendar.Date
}

// IntakePorts are the root-bound ports of the intake.
type IntakePorts struct {
	Schools      carecontract.TraegerSchools
	Applications carecontract.SchoolApplications
	Bookings     carecontract.SchoolBookings
	Access       carecontract.SchoolAccessLog
	Runtime      carecontract.Runtime
}

var _ carecontract.Intake = (*Intake)(nil)

// NewIntake builds the intake over the root-bound ports.
func NewIntake(store Store, ports IntakePorts, today func() calendar.Date) *Intake {
	if store == nil || ports.Schools == nil || ports.Applications == nil || ports.Bookings == nil || ports.Access == nil ||
		ports.Runtime == nil || today == nil {
		panic("care contract: the store, the schools, the applications, the bookings, the access log, the runtime and the clock are required")
	}
	return &Intake{store: store, schools: ports.Schools, applications: ports.Applications, bookings: ports.Bookings,
		access: ports.Access, runtime: ports.Runtime, today: today}
}

// schoolRead is what the intake read in one school's tenant transaction.
type schoolRead struct {
	school       carecontract.School
	applications []carecontract.Application
	offerings    map[int64]carecontract.Offering
}

// Candidates lists the accepted application children with a booked offering
// that have no contract for it yet.
func (in *Intake) Candidates(ctx context.Context, reviewer carecontract.Reviewer, schoolID int64) ([]carecontract.Candidate, error) {
	if err := checkReviewer(reviewer); err != nil {
		return nil, err
	}
	schools, err := in.schoolsOf(ctx, reviewer.OrganizationID, schoolID)
	if err != nil {
		return nil, err
	}
	reads, err := in.readSchools(ctx, reviewer, schools, nil, accessViewCandidates)
	if err != nil {
		return nil, err
	}
	var existing map[ContractKey]bool
	var templated map[[2]int64]bool
	err = in.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		if existing, err = in.store.ContractKeys(txCtx, reviewer.OrganizationID, schoolIDs(schools)); err != nil {
			return err
		}
		templated, err = in.templatedOfferings(txCtx, reviewer.OrganizationID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("care contract: read contracts: %w", err)
	}
	today := in.today()
	candidates := []carecontract.Candidate{}
	for _, read := range reads {
		for _, application := range read.applications {
			for _, booking := range currentBookings(application.Bookings, today) {
				key := ContractKey{SchoolID: read.school.ID, RequestChildID: application.RequestChildID, CareOfferingID: booking.CareOfferingID}
				offering, ok := read.offerings[booking.CareOfferingID]
				if existing[key] || !ok {
					continue
				}
				candidates = append(candidates, candidate(read.school, application, booking, offering,
					templated[[2]int64{read.school.ID, offering.ID}], today))
			}
		}
	}
	slices.SortStableFunc(candidates, func(a, b carecontract.Candidate) int {
		return firstNonZero(strings.Compare(a.SchoolName, b.SchoolName), strings.Compare(a.ChildLastName, b.ChildLastName),
			strings.Compare(a.ChildFirstName, b.ChildFirstName), compareIDs(a.CareOfferingID, b.CareOfferingID))
	})
	return candidates, nil
}

func candidate(school carecontract.School, application carecontract.Application, booking carecontract.Booking,
	offering carecontract.Offering, hasTemplate bool, today calendar.Date) carecontract.Candidate {
	return carecontract.Candidate{
		CandidateKey:   carecontract.CandidateKey{SchoolID: school.ID, RequestChildID: application.RequestChildID, CareOfferingID: offering.ID},
		SchoolName:     school.Name,
		ChildFirstName: application.ChildFirstName, ChildLastName: application.ChildLastName,
		GuardianName:  fullName(application.GuardianFirstName, application.GuardianLastName),
		GuardianEmail: application.GuardianEmail,
		OfferingName:  offering.Name, PhaseName: offering.PhaseName,
		BookedDays:    orderedDays(booking.Days),
		EffectiveFrom: effectiveFrom(application, booking, today),
		HasTemplate:   hasTemplate,
	}
}

// CreateContracts creates a draft for every key with a template and no
// contract yet. Every key must name an accepted application child that books
// the offering at a school of the Träger.
func (in *Intake) CreateContracts(ctx context.Context, reviewer carecontract.Reviewer, keys []carecontract.CandidateKey) (carecontract.CreateResult, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.CreateResult{}, err
	}
	keys = uniqueKeys(keys)
	if len(keys) == 0 || len(keys) > maxCreateBatch {
		return carecontract.CreateResult{}, &carecontract.ValidationError{Fields: map[string]string{"contracts": "count"}}
	}
	all, err := in.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return carecontract.CreateResult{}, fmt.Errorf("care contract: schools: %w", err)
	}
	children := map[int64][]int64{}
	for _, key := range keys {
		if !slices.ContainsFunc(all, func(s carecontract.School) bool { return s.ID == key.SchoolID }) {
			return carecontract.CreateResult{}, carecontract.ErrSchoolOutsideTraeger
		}
		children[key.SchoolID] = append(children[key.SchoolID], key.RequestChildID)
	}
	schools := slices.DeleteFunc(slices.Clone(all), func(s carecontract.School) bool { return children[s.ID] == nil })
	reads, err := in.readSchools(ctx, reviewer, schools, children, accessViewCreate)
	if err != nil {
		return carecontract.CreateResult{}, err
	}
	result := carecontract.CreateResult{Created: []carecontract.ContractSummary{}, Skipped: []carecontract.CreateSkip{}}
	err = in.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		result, err = in.create(txCtx, reviewer, keys, reads)
		return err
	})
	if err != nil {
		return carecontract.CreateResult{}, err
	}
	return result, nil
}

func (in *Intake) create(ctx context.Context, reviewer carecontract.Reviewer, keys []carecontract.CandidateKey, reads []schoolRead) (carecontract.CreateResult, error) {
	result := carecontract.CreateResult{Created: []carecontract.ContractSummary{}, Skipped: []carecontract.CreateSkip{}}
	rows, err := in.store.Templates(ctx, reviewer.OrganizationID)
	if err != nil {
		return result, err
	}
	templates := map[[2]int64]carecontract.Template{}
	for _, row := range rows {
		templates[[2]int64{row.SchoolID, row.OfferingID}] = row.Template
	}
	existing, err := in.store.ContractKeys(ctx, reviewer.OrganizationID, readSchoolIDs(reads))
	if err != nil {
		return result, err
	}
	today := in.today()
	for _, key := range keys {
		draft, reason := in.draft(key, reads, templates, existing, today)
		if reason != "" {
			result.Skipped = append(result.Skipped, carecontract.CreateSkip{CandidateKey: key, Reason: reason})
			continue
		}
		id, err := in.store.InsertContract(ctx, reviewer.OrganizationID, reviewer.AccountID, draft)
		if errors.Is(err, carecontract.ErrContractExists) {
			result.Skipped = append(result.Skipped, carecontract.CreateSkip{CandidateKey: key, Reason: carecontract.SkipContractExists})
			continue
		}
		if err != nil {
			return result, err
		}
		schoolID := key.SchoolID
		if err := in.store.InsertEvents(ctx, reviewer.OrganizationID, []Event{{
			ContractID: &id, SchoolID: &schoolID, AccountID: reviewer.AccountID, Action: ActionCreate,
			Details: map[string]any{"request_child_id": key.RequestChildID, "care_offering_id": key.CareOfferingID},
		}}); err != nil {
			return result, err
		}
		record, err := in.store.Contract(ctx, reviewer.OrganizationID, id, false)
		if err != nil {
			return result, err
		}
		result.Created = append(result.Created, record.Summary)
	}
	return result, nil
}

// draft builds the draft of one key, or names why there is none.
func (in *Intake) draft(key carecontract.CandidateKey, reads []schoolRead, templates map[[2]int64]carecontract.Template,
	existing map[ContractKey]bool, today calendar.Date) (NewContract, string) {
	contractKey := ContractKey{SchoolID: key.SchoolID, RequestChildID: key.RequestChildID, CareOfferingID: key.CareOfferingID}
	if existing[contractKey] {
		return NewContract{}, carecontract.SkipContractExists
	}
	read, ok := findRead(reads, key.SchoolID)
	if !ok {
		return NewContract{}, carecontract.SkipNotFound
	}
	index := slices.IndexFunc(read.applications, func(a carecontract.Application) bool { return a.RequestChildID == key.RequestChildID })
	offering, offered := read.offerings[key.CareOfferingID]
	if index < 0 || !offered {
		return NewContract{}, carecontract.SkipNotFound
	}
	application := read.applications[index]
	bookings := currentBookings(application.Bookings, today)
	at := slices.IndexFunc(bookings, func(b carecontract.Booking) bool { return b.CareOfferingID == key.CareOfferingID })
	if at < 0 {
		return NewContract{}, carecontract.SkipNotFound
	}
	template, ok := templates[[2]int64{key.SchoolID, key.CareOfferingID}]
	if !ok {
		return NewContract{}, carecontract.SkipTemplateMissing
	}
	booking := bookings[at]
	from := effectiveFrom(application, booking, today)
	version := carecontract.Version{
		Number: 1, Status: carecontract.VersionRequested, EffectiveFrom: from, EndsOn: endsOn(template.TemplateInput, from, booking),
		ChildFirstName: application.ChildFirstName, ChildLastName: application.ChildLastName, ChildBirthDate: application.ChildBirthDate,
		SchoolName: read.school.Name, OfferingName: offering.Name, PhaseName: firstText(offering.PhaseName, application.PhaseName),
		BookedDays: orderedDays(booking.Days), Terms: template.TemplateInput,
	}
	if offering.PriceCents != nil {
		price := int64(*offering.PriceCents)
		version.PriceCents = &price
	}
	return NewContract{
		Key: contractKey, StudentID: application.StudentID, Version: version,
		Party: NewParty{
			FirstName: application.GuardianFirstName, LastName: application.GuardianLastName,
			Email: strings.TrimSpace(application.GuardianEmail), AccountID: application.GuardianAccountID,
		},
	}, ""
}

// Templates lists every offering of the Träger's schools with its template.
func (in *Intake) Templates(ctx context.Context, reviewer carecontract.Reviewer) ([]carecontract.OfferingTemplate, error) {
	if err := checkReviewer(reviewer); err != nil {
		return nil, err
	}
	schools, err := in.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("care contract: schools: %w", err)
	}
	offerings := map[int64][]carecontract.Offering{}
	for _, school := range schools {
		err := in.runtime.WithinSchool(ctx, school.ID, func(txCtx context.Context) error {
			found, err := in.offerings(txCtx, nil)
			offerings[school.ID] = found
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("care contract: read offerings: %w", err)
		}
	}
	var rows []TemplateRow
	err = in.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		rows, err = in.store.Templates(txCtx, reviewer.OrganizationID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("care contract: read templates: %w", err)
	}
	byOffering := map[[2]int64]carecontract.Template{}
	for _, row := range rows {
		byOffering[[2]int64{row.SchoolID, row.OfferingID}] = row.Template
	}
	result := []carecontract.OfferingTemplate{}
	for _, school := range schools {
		for _, offering := range offerings[school.ID] {
			item := offeringTemplate(school, offering, nil)
			if template, ok := byOffering[[2]int64{school.ID, offering.ID}]; ok {
				item.Template = &template
			}
			result = append(result, item)
		}
	}
	slices.SortStableFunc(result, func(a, b carecontract.OfferingTemplate) int {
		if a.Active != b.Active {
			if a.Active {
				return -1
			}
			return 1
		}
		return firstNonZero(strings.Compare(a.SchoolName, b.SchoolName), strings.Compare(b.PhaseName, a.PhaseName),
			strings.Compare(a.OfferingName, b.OfferingName))
	})
	return result, nil
}

// SaveTemplate creates or replaces the template of one offering of a school
// of the Träger.
func (in *Intake) SaveTemplate(ctx context.Context, reviewer carecontract.Reviewer, schoolID, offeringID int64, input carecontract.TemplateInput) (carecontract.OfferingTemplate, error) {
	if err := checkReviewer(reviewer); err != nil {
		return carecontract.OfferingTemplate{}, err
	}
	input = normalizeTemplate(input)
	if fields := validateTemplate(input); len(fields) > 0 {
		return carecontract.OfferingTemplate{}, &carecontract.ValidationError{Fields: fields}
	}
	schools, err := in.schoolsOf(ctx, reviewer.OrganizationID, schoolID)
	if err != nil {
		return carecontract.OfferingTemplate{}, err
	}
	var offering carecontract.Offering
	err = in.runtime.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		found, err := in.offerings(txCtx, []int64{offeringID})
		if err != nil {
			return err
		}
		index := slices.IndexFunc(found, func(o carecontract.Offering) bool { return o.ID == offeringID })
		if index < 0 {
			return carecontract.ErrOfferingNotFound
		}
		offering = found[index]
		return nil
	})
	if err != nil {
		return carecontract.OfferingTemplate{}, err
	}
	var saved carecontract.Template
	err = in.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		if saved, err = in.store.UpsertTemplate(txCtx, reviewer.OrganizationID, reviewer.AccountID, schoolID, offeringID, input); err != nil {
			return err
		}
		return in.store.InsertEvents(txCtx, reviewer.OrganizationID, []Event{{
			SchoolID: &schoolID, AccountID: reviewer.AccountID, Action: ActionTemplate,
			Details: map[string]any{"care_offering_id": offeringID, "template_id": saved.ID},
		}})
	})
	if err != nil {
		return carecontract.OfferingTemplate{}, err
	}
	return offeringTemplate(schools[0], offering, &saved), nil
}

// schoolsOf is every school of the Träger, or only schoolID, which must be
// one of them.
func (in *Intake) schoolsOf(ctx context.Context, organizationID, schoolID int64) ([]carecontract.School, error) {
	schools, err := in.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("care contract: schools: %w", err)
	}
	if schoolID <= 0 {
		return schools, nil
	}
	index := slices.IndexFunc(schools, func(s carecontract.School) bool { return s.ID == schoolID })
	if index < 0 {
		return nil, carecontract.ErrSchoolOutsideTraeger
	}
	return []carecontract.School{schools[index]}, nil
}

// readSchools reads the accepted applications and their offerings of every
// school in its own tenant transaction and records the read in that
// school's access log.
func (in *Intake) readSchools(ctx context.Context, reviewer carecontract.Reviewer, schools []carecontract.School,
	children map[int64][]int64, view string) ([]schoolRead, error) {
	reads := make([]schoolRead, 0, len(schools))
	for _, school := range schools {
		read := schoolRead{school: school, offerings: map[int64]carecontract.Offering{}}
		err := in.runtime.WithinSchool(ctx, school.ID, func(txCtx context.Context) error {
			applications, err := in.acceptedApplications(txCtx, children[school.ID])
			if err != nil {
				return err
			}
			read.applications = applications
			if len(applications) == 0 {
				return nil
			}
			offerings, err := in.offerings(txCtx, bookedOfferingIDs(applications))
			if err != nil {
				return err
			}
			for _, offering := range offerings {
				read.offerings[offering.ID] = offering
			}
			return in.access.RecordAccess(txCtx, carecontract.SchoolAccess{
				OrganizationID: reviewer.OrganizationID, AccountID: reviewer.AccountID, View: view, Rows: len(applications),
			})
		})
		if err != nil {
			return nil, fmt.Errorf("care contract: read school %d: %w", school.ID, err)
		}
		reads = append(reads, read)
	}
	return reads, nil
}

// acceptedApplications reads the accepted application children with their
// bookings in the caller's tenant transaction.
func (in *Intake) acceptedApplications(ctx context.Context, requestChildIDs []int64) ([]carecontract.Application, error) {
	applications, err := in.applications.AcceptedApplications(ctx, requestChildIDs)
	if err != nil || len(applications) == 0 {
		return applications, err
	}
	ids := make([]int64, len(applications))
	for i, application := range applications {
		ids[i] = application.RequestChildID
	}
	bookings, err := in.bookings.Bookings(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range applications {
		applications[i].Bookings = bookings[applications[i].RequestChildID]
	}
	return applications, nil
}

// offerings reads the offerings with the names of their phases in the
// caller's tenant transaction.
func (in *Intake) offerings(ctx context.Context, ids []int64) ([]carecontract.Offering, error) {
	offerings, err := in.bookings.Offerings(ctx, ids)
	if err != nil || len(offerings) == 0 {
		return offerings, err
	}
	return in.applications.NamePhases(ctx, offerings)
}

func (in *Intake) templatedOfferings(ctx context.Context, organizationID int64) (map[[2]int64]bool, error) {
	rows, err := in.store.Templates(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	result := make(map[[2]int64]bool, len(rows))
	for _, row := range rows {
		result[[2]int64{row.SchoolID, row.OfferingID}] = true
	}
	return result, nil
}

func offeringTemplate(school carecontract.School, offering carecontract.Offering, template *carecontract.Template) carecontract.OfferingTemplate {
	return carecontract.OfferingTemplate{
		SchoolID: school.ID, SchoolName: school.Name, OfferingID: offering.ID, OfferingName: offering.Name,
		PhaseName: offering.PhaseName, PriceCents: offering.PriceCents, Active: offering.Active, Template: template,
	}
}

// currentBookings keeps per offering the booking that holds today or starts
// later, the earliest of them; ended bookings drop out.
func currentBookings(bookings []carecontract.Booking, today calendar.Date) []carecontract.Booking {
	chosen := map[int64]carecontract.Booking{}
	order := []int64{}
	for _, booking := range bookings {
		if booking.ValidUntil != nil && !booking.ValidUntil.After(today) {
			continue
		}
		previous, seen := chosen[booking.CareOfferingID]
		if !seen {
			order = append(order, booking.CareOfferingID)
		}
		if !seen || startsBefore(booking.ValidFrom, previous.ValidFrom) {
			chosen[booking.CareOfferingID] = booking
		}
	}
	result := make([]carecontract.Booking, 0, len(order))
	for _, id := range order {
		result = append(result, chosen[id])
	}
	return result
}

func startsBefore(a, b *calendar.Date) bool {
	switch {
	case a == nil:
		return b != nil
	case b == nil:
		return false
	}
	return a.Before(*b)
}

// effectiveFrom is the date the contract takes effect: the start of the
// booking, else the activation of the child, else the start of the service
// of the phase, else today.
func effectiveFrom(application carecontract.Application, booking carecontract.Booking, today calendar.Date) calendar.Date {
	for _, date := range []*calendar.Date{booking.ValidFrom, application.ActivateOn, application.ServiceStartDate} {
		if date != nil && !date.IsZero() {
			return *date
		}
	}
	return today
}

// endsOn is the last day of a contract with a fixed term: the day before
// the term's months have passed, or the booking's end when it comes first.
// An open or school-year term ends with the booking, if at all.
func endsOn(terms carecontract.TemplateInput, from calendar.Date, booking carecontract.Booking) *calendar.Date {
	var end *calendar.Date
	if booking.ValidUntil != nil {
		last := booking.ValidUntil.AddDays(-1)
		end = &last
	}
	if terms.TermKind == carecontract.TermFixedMonths && terms.TermMonths != nil {
		last := addMonths(from, *terms.TermMonths).AddDays(-1)
		if end == nil || last.Before(*end) {
			end = &last
		}
	}
	if end != nil && end.Before(from) {
		return nil
	}
	return end
}

func addMonths(date calendar.Date, months int) calendar.Date {
	instant := date.BerlinMidnight()
	return calendar.DateFromTime(instant.AddDate(0, months, 0))
}

func orderedDays(days []string) []string {
	result := []string{}
	for _, day := range weekdayOrder {
		if slices.Contains(days, day) {
			result = append(result, day)
		}
	}
	return result
}

func bookedOfferingIDs(applications []carecontract.Application) []int64 {
	ids := []int64{}
	for _, application := range applications {
		for _, booking := range application.Bookings {
			if !slices.Contains(ids, booking.CareOfferingID) {
				ids = append(ids, booking.CareOfferingID)
			}
		}
	}
	return ids
}

func schoolIDs(schools []carecontract.School) []int64 {
	ids := make([]int64, len(schools))
	for i, school := range schools {
		ids[i] = school.ID
	}
	return ids
}

func readSchoolIDs(reads []schoolRead) []int64 {
	ids := make([]int64, len(reads))
	for i, read := range reads {
		ids[i] = read.school.ID
	}
	return ids
}

func findRead(reads []schoolRead, schoolID int64) (schoolRead, bool) {
	for _, read := range reads {
		if read.school.ID == schoolID {
			return read, true
		}
	}
	return schoolRead{}, false
}

func uniqueKeys(keys []carecontract.CandidateKey) []carecontract.CandidateKey {
	result := make([]carecontract.CandidateKey, 0, len(keys))
	for _, key := range keys {
		if key.SchoolID > 0 && key.RequestChildID > 0 && key.CareOfferingID > 0 && !slices.Contains(result, key) {
			result = append(result, key)
		}
	}
	return result
}

func firstNonZero(orders ...int) int {
	for _, order := range orders {
		if order != 0 {
			return order
		}
	}
	return 0
}

func compareIDs(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
