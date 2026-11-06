package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

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
	modes        carecontract.ConclusionModes
	today        func() calendar.Date
}

// IntakePorts are the root-bound ports of the intake.
type IntakePorts struct {
	Schools      carecontract.TraegerSchools
	Applications carecontract.SchoolApplications
	Bookings     carecontract.SchoolBookings
	Access       carecontract.SchoolAccessLog
	Runtime      carecontract.Runtime
	Modes        carecontract.ConclusionModes
}

var _ carecontract.Intake = (*Intake)(nil)

// NewIntake builds the intake over the root-bound ports.
func NewIntake(store Store, ports IntakePorts, today func() calendar.Date) *Intake {
	if store == nil || ports.Schools == nil || ports.Applications == nil || ports.Bookings == nil || ports.Access == nil ||
		ports.Runtime == nil || ports.Modes == nil || today == nil {
		panic("care contract: the store, the schools, the applications, the bookings, the access log, the runtime, the conclusion modes and the clock are required")
	}
	return &Intake{store: store, schools: ports.Schools, applications: ports.Applications, bookings: ports.Bookings,
		access: ports.Access, runtime: ports.Runtime, modes: ports.Modes, today: today}
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
	var templates templateIndex
	err = in.runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(txCtx context.Context) error {
		var err error
		if existing, err = in.store.ContractKeys(txCtx, reviewer.OrganizationID, schoolIDs(schools)); err != nil {
			return err
		}
		templates, err = in.templateIndex(txCtx, reviewer.OrganizationID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("care contract: read contracts: %w", err)
	}
	candidates := collectCandidates(reads, existing, templates, in.today())
	sortCandidates(candidates)
	return candidates, nil
}

// collectCandidates keeps every accepted application child with booked care
// and no contract.
func collectCandidates(reads []schoolRead, existing map[ContractKey]bool, templates templateIndex, today calendar.Date) []carecontract.Candidate {
	candidates := []carecontract.Candidate{}
	for _, read := range reads {
		for _, application := range read.applications {
			key := ContractKey{SchoolID: read.school.ID, RequestChildID: application.RequestChildID}
			booked := bookedCare(read, application, today)
			if existing[key] || len(booked.services) == 0 {
				continue
			}
			_, issue := templates.terms(read.school.ID, booked.offerings)
			candidates = append(candidates, carecontract.Candidate{
				CandidateKey:   carecontract.CandidateKey{SchoolID: read.school.ID, RequestChildID: application.RequestChildID},
				SchoolName:     read.school.Name,
				ChildFirstName: application.ChildFirstName, ChildLastName: application.ChildLastName,
				GuardianName:  fullName(application.GuardianFirstName, application.GuardianLastName),
				GuardianEmail: application.GuardianEmail,
				PhaseName:     firstText(booked.phaseName, application.PhaseName),
				Services:      boundServices(booked.services, lastDay(booked.until)),
				EffectiveFrom: booked.effectiveFrom(application, today),
				TemplateIssue: issue,
			})
		}
	}
	return candidates
}

func sortCandidates(candidates []carecontract.Candidate) {
	slices.SortStableFunc(candidates, func(a, b carecontract.Candidate) int {
		return firstNonZero(strings.Compare(a.SchoolName, b.SchoolName), strings.Compare(a.ChildLastName, b.ChildLastName),
			strings.Compare(a.ChildFirstName, b.ChildFirstName), compareIDs(a.RequestChildID, b.RequestChildID))
	})
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
	templates, err := in.templateIndex(ctx, reviewer.OrganizationID)
	if err != nil {
		return result, err
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
			Details: map[string]any{"request_child_id": key.RequestChildID, "care_offering_ids": draftOfferings(draft)},
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

// draft builds the draft of one key over every module the child books, or
// names why there is none.
func (in *Intake) draft(key carecontract.CandidateKey, reads []schoolRead, templates templateIndex,
	existing map[ContractKey]bool, today calendar.Date) (NewContract, string) {
	contractKey := ContractKey{SchoolID: key.SchoolID, RequestChildID: key.RequestChildID}
	if existing[contractKey] {
		return NewContract{}, carecontract.SkipContractExists
	}
	read, ok := findRead(reads, key.SchoolID)
	if !ok {
		return NewContract{}, carecontract.SkipNotFound
	}
	index := slices.IndexFunc(read.applications, func(a carecontract.Application) bool { return a.RequestChildID == key.RequestChildID })
	if index < 0 {
		return NewContract{}, carecontract.SkipNotFound
	}
	application := read.applications[index]
	booked := bookedCare(read, application, today)
	if len(booked.services) == 0 {
		return NewContract{}, carecontract.SkipNotFound
	}
	terms, issue := templates.terms(key.SchoolID, booked.offerings)
	if issue != "" {
		return NewContract{}, issue
	}
	from := booked.effectiveFrom(application, today)
	ends := endsOn(terms, from, booked.until)
	version := carecontract.Version{
		Number: 1, Status: carecontract.VersionRequested, EffectiveFrom: from, EndsOn: ends,
		ChildFirstName: application.ChildFirstName, ChildLastName: application.ChildLastName, ChildBirthDate: application.ChildBirthDate,
		SchoolName: read.school.Name, PhaseName: firstText(booked.phaseName, application.PhaseName),
		Services: boundServices(booked.services, ends), PriceCents: booked.price, Terms: terms,
	}
	return NewContract{
		Key: contractKey, StudentID: application.StudentID, Version: version,
		Party: NewParty{
			FirstName: application.GuardianFirstName, LastName: application.GuardianLastName,
			Email: strings.TrimSpace(application.GuardianEmail), AccountID: application.GuardianAccountID,
		},
	}, ""
}

func draftOfferings(draft NewContract) []int64 {
	ids := []int64{}
	for _, service := range draft.Version.Services {
		if !slices.Contains(ids, service.CareOfferingID) {
			ids = append(ids, service.CareOfferingID)
		}
	}
	return ids
}

// ConclusionModes tells for every school of the Träger how its contracts
// come about (#4680): its own value, else the Träger's, else Weg a. It reads
// a setting, no personal data, in each school's tenant transaction.
func (in *Intake) ConclusionModes(ctx context.Context, reviewer carecontract.Reviewer) ([]carecontract.SchoolConclusion, error) {
	if err := checkReviewer(reviewer); err != nil {
		return nil, err
	}
	schools, err := in.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("care contract: schools: %w", err)
	}
	result := make([]carecontract.SchoolConclusion, 0, len(schools))
	for _, school := range schools {
		var mode string
		err := in.runtime.WithinSchool(ctx, school.ID, func(txCtx context.Context) error {
			var err error
			mode, err = in.modes.ConclusionMode(txCtx, school.ID)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("care contract: conclusion mode of school %d: %w", school.ID, err)
		}
		result = append(result, carecontract.SchoolConclusion{SchoolID: school.ID, SchoolName: school.Name, Mode: mode})
	}
	return result, nil
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
	offerings, err := in.schoolOfferings(ctx, schools)
	if err != nil {
		return nil, err
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
	sortTemplates(result)
	return result, nil
}

// schoolOfferings reads the offerings of every school in its own tenant
// transaction.
func (in *Intake) schoolOfferings(ctx context.Context, schools []carecontract.School) (map[int64][]carecontract.Offering, error) {
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
	return offerings, nil
}

// sortTemplates puts the active offerings first, then by school, the newest
// phase and the name.
func sortTemplates(templates []carecontract.OfferingTemplate) {
	slices.SortStableFunc(templates, func(a, b carecontract.OfferingTemplate) int {
		if a.Active != b.Active {
			if a.Active {
				return -1
			}
			return 1
		}
		return firstNonZero(strings.Compare(a.SchoolName, b.SchoolName), strings.Compare(b.PhaseName, a.PhaseName),
			strings.Compare(a.OfferingName, b.OfferingName))
	})
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
			if err := in.readSchool(txCtx, &read, children[school.ID]); err != nil || len(read.applications) == 0 {
				return err
			}
			return in.access.RecordAccess(txCtx, carecontract.SchoolAccess{
				OrganizationID: reviewer.OrganizationID, AccountID: reviewer.AccountID,
				Data: carecontract.SchoolAccessApplications, View: view, Rows: len(read.applications),
			})
		})
		if err != nil {
			return nil, fmt.Errorf("care contract: read school %d: %w", school.ID, err)
		}
		reads = append(reads, read)
	}
	return reads, nil
}

// readSchool reads, in the school's tenant transaction the caller holds, its
// accepted application children (all, or only requestChildIDs) with their
// bookings and the offerings they book.
func (in *Intake) readSchool(ctx context.Context, read *schoolRead, requestChildIDs []int64) error {
	applications, err := in.acceptedApplications(ctx, requestChildIDs)
	if err != nil {
		return err
	}
	read.applications = applications
	if len(applications) == 0 {
		return nil
	}
	offerings, err := in.offerings(ctx, bookedOfferingIDs(applications))
	if err != nil {
		return err
	}
	for _, offering := range offerings {
		read.offerings[offering.ID] = offering
	}
	return nil
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

func (in *Intake) templateIndex(ctx context.Context, organizationID int64) (templateIndex, error) {
	rows, err := in.store.Templates(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	index := make(templateIndex, len(rows))
	for _, row := range rows {
		index[[2]int64{row.SchoolID, row.OfferingID}] = row.Template.TemplateInput
	}
	return index, nil
}

func offeringTemplate(school carecontract.School, offering carecontract.Offering, template *carecontract.Template) carecontract.OfferingTemplate {
	return carecontract.OfferingTemplate{
		SchoolID: school.ID, SchoolName: school.Name, OfferingID: offering.ID, OfferingName: offering.Name,
		PhaseName: offering.PhaseName, PriceCents: offering.PriceCents, Active: offering.Active, Template: template,
	}
}

// fixedTermEnd is the day before the matching day in the target month.
// When that day does not exist, the term includes the target month's last day.
func fixedTermEnd(date calendar.Date, months int) calendar.Date {
	first := calendar.NewDate(date.Year(), date.Month()+time.Month(months), 1)
	last := calendar.NewDate(first.Year(), first.Month()+1, 1).AddDays(-1)
	if date.Day() > last.Day() {
		return last
	}
	return calendar.NewDate(first.Year(), first.Month(), date.Day()).AddDays(-1)
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
		if key.SchoolID > 0 && key.RequestChildID > 0 && !slices.Contains(result, key) {
			result = append(result, key)
		}
	}
	// A fixed order: two overlapping batches insert the same keys one after
	// the other instead of waiting on each other's uncommitted rows.
	slices.SortFunc(result, func(a, b carecontract.CandidateKey) int {
		return firstNonZero(compareIDs(a.SchoolID, b.SchoolID), compareIDs(a.RequestChildID, b.RequestChildID))
	})
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
