package application

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/ports"
)

// TraegerStaff is the Personalstamm of ADR 0089 (#4705) in the Träger-Portal.
// It reads each school in that school's tenant transaction and writes the
// record in the Träger transaction; no transaction carries both (ADR 0105).
// Every read that shows people of a school writes an access entry in that
// school's tenant transaction before anything leaves (ADR 0084 section 7).
type TraegerStaff struct {
	store    ports.TraegerStaffStore
	ports    workforce.TraegerStaffPorts
	subjects ports.StaffSubjects
	audit    ports.StaffAdminAudit
	bank     workforce.BankDataCipher
}

// NewTraegerStaff composes the Personalstamm; every collaborator is required.
func NewTraegerStaff(store ports.TraegerStaffStore, collaborators workforce.TraegerStaffPorts, subjects ports.StaffSubjects, audit ports.StaffAdminAudit, bank workforce.BankDataCipher) *TraegerStaff {
	if store == nil || collaborators.Schools == nil || collaborators.Employments == nil || collaborators.Persons == nil || collaborators.Access == nil ||
		collaborators.Runtime == nil || subjects == nil || audit == nil || bank == nil {
		panic("workforce traeger staff: all dependencies are required")
	}
	return &TraegerStaff{store: store, ports: collaborators, subjects: subjects, audit: audit, bank: bank}
}

// changeTrailLimit bounds the trail a record detail shows.
const changeTrailLimit = 100

var _ workforce.TraegerStaffRecords = (*TraegerStaff)(nil)

// released runs fn in the Träger transaction of the actor's Träger after
// the release check.
func (t *TraegerStaff) released(ctx context.Context, actor workforce.TraegerStaffActor, fn func(context.Context) error) error {
	return t.ports.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
		release, err := t.store.TraegerStaffRelease(ctx, actor.OrganizationID)
		if err != nil {
			return err
		}
		if release == nil {
			return workforce.ErrTraegerStaffNotReleased
		}
		return fn(ctx)
	})
}

// schoolRead is what one school's tenant transaction returned.
type schoolRead struct {
	members   map[int64]workforce.TraegerSchoolStaffMember
	order     []int64
	financial map[int64]domain.StaffFinancialData
}

// readSchools reads the live employments of each school, all of them when
// wanted holds no entry for the school, and their bank rows with
// withFinancial. A school that shows people gets the access entry of view.
func (t *TraegerStaff) readSchools(ctx context.Context, actor workforce.TraegerStaffActor, view string, schools []int64, wanted map[int64][]int64, withFinancial bool) (map[int64]schoolRead, error) {
	reads := make(map[int64]schoolRead, len(schools))
	for _, schoolID := range schools {
		err := t.ports.Runtime.WithinSchool(ctx, schoolID, func(ctx context.Context) error {
			read, err := t.readSchool(ctx, wanted[schoolID], withFinancial)
			if err != nil || len(read.order) == 0 {
				reads[schoolID] = read
				return err
			}
			reads[schoolID] = read
			return t.ports.Access.RecordTraegerStaffAccess(ctx, workforce.TraegerStaffAccess{
				OrganizationID: actor.OrganizationID, AccountID: actor.AccountID, View: view, Rows: len(read.order),
			})
		})
		if err != nil {
			return nil, err
		}
	}
	return reads, nil
}

// schoolStaff joins the live employments of the school of the tenant
// transaction with their persons, in two reads.
func (t *TraegerStaff) schoolStaff(ctx context.Context, staffIDs []int64) ([]workforce.TraegerSchoolStaffMember, error) {
	if staffIDs != nil && len(staffIDs) == 0 {
		return nil, nil
	}
	employments, err := t.ports.Employments.SchoolEmployments(ctx, staffIDs)
	if err != nil || len(employments) == 0 {
		return nil, err
	}
	personIDs := make([]int64, 0, len(employments))
	for _, employment := range employments {
		personIDs = append(personIDs, employment.PersonID)
	}
	persons, err := t.ports.Persons.SchoolPersons(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]workforce.TraegerSchoolPerson, len(persons))
	for _, person := range persons {
		byID[person.PersonID] = person
	}
	members := make([]workforce.TraegerSchoolStaffMember, 0, len(employments))
	for _, employment := range employments {
		person, ok := byID[employment.PersonID]
		if !ok {
			continue
		}
		members = append(members, workforce.TraegerSchoolStaffMember{
			StaffID: employment.StaffID, PersonID: employment.PersonID, AccountID: person.AccountID,
			FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday, PersonnelNumber: employment.PersonnelNumber,
		})
	}
	return members, nil
}

func (t *TraegerStaff) readSchool(ctx context.Context, staffIDs []int64, withFinancial bool) (schoolRead, error) {
	members, err := t.schoolStaff(ctx, staffIDs)
	if err != nil {
		return schoolRead{}, err
	}
	read := schoolRead{members: make(map[int64]workforce.TraegerSchoolStaffMember, len(members)), financial: map[int64]domain.StaffFinancialData{}}
	for _, member := range members {
		read.members[member.StaffID] = member
		read.order = append(read.order, member.StaffID)
	}
	if !withFinancial || len(read.order) == 0 {
		return read, nil
	}
	rows, err := t.store.SchoolFinancialRows(ctx, read.order)
	if err != nil {
		return schoolRead{}, err
	}
	for _, row := range rows {
		read.financial[row.StaffID] = row
	}
	return read, nil
}

// Overview lists the release, the schools, the records and the proposals.
// Without the release it shows the schools only and reads no person.
func (t *TraegerStaff) Overview(ctx context.Context, actor workforce.TraegerStaffActor) (workforce.TraegerStaffOverview, error) {
	schools, err := t.ports.Schools.TraegerSchools(ctx, actor.OrganizationID)
	if err != nil {
		return workforce.TraegerStaffOverview{}, err
	}
	overview := workforce.TraegerStaffOverview{Schools: schools, Records: []workforce.TraegerStaffRecordSummary{}, Suggestions: []workforce.TraegerStaffSuggestion{}}
	var records []domain.TraegerStaffRecord
	err = t.ports.Runtime.WithinTraeger(ctx, actor.OrganizationID, func(ctx context.Context) error {
		release, err := t.store.TraegerStaffRelease(ctx, actor.OrganizationID)
		if err != nil || release == nil {
			return err
		}
		overview.Release = publicRelease(*release)
		records, err = t.store.ListStaffRecords(ctx, actor.OrganizationID)
		return err
	})
	if err != nil || overview.Release == nil {
		return overview, err
	}
	reads, err := t.readSchools(ctx, actor, workforce.TraegerStaffViewOverview, schoolIDs(schools), nil, false)
	if err != nil {
		return workforce.TraegerStaffOverview{}, err
	}
	names := schoolNames(schools)
	linked := make(map[[2]int64]bool)
	matches := make([]domain.StaffMatchRecord, 0, len(records))
	for _, record := range records {
		overview.Records = append(overview.Records, summarize(record, names, reads))
		matches = append(matches, matchRecord(record, reads))
		for _, link := range record.Links {
			linked[[2]int64{link.SchoolID, link.StaffID}] = true
		}
	}
	candidates := unlinkedCandidates(schools, reads, linked)
	for _, suggestion := range domain.SuggestStaffLinks(matches, candidates) {
		overview.Suggestions = append(overview.Suggestions, publicSuggestion(suggestion, names, reads))
	}
	return overview, nil
}

// Record shows one record with its masked bank values and its trail.
func (t *TraegerStaff) Record(ctx context.Context, actor workforce.TraegerStaffActor, recordID int64) (workforce.TraegerStaffRecordDetail, error) {
	schools, err := t.ports.Schools.TraegerSchools(ctx, actor.OrganizationID)
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, err
	}
	var record domain.TraegerStaffRecord
	var changes []domain.TraegerStaffChange
	err = t.released(ctx, actor, func(ctx context.Context) error {
		if record, err = t.store.FindStaffRecord(ctx, actor.OrganizationID, recordID, false); err != nil {
			return err
		}
		changes, err = t.store.ListStaffChanges(ctx, actor.OrganizationID, recordID, changeTrailLimit)
		return err
	})
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, publicTraegerStaffError(err)
	}
	reads, err := t.readSchools(ctx, actor, workforce.TraegerStaffViewRecord, linkSchools(record), linkWanted(record), false)
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, err
	}
	names := schoolNames(schools)
	detail := workforce.TraegerStaffRecordDetail{
		TraegerStaffRecordSummary: summarize(record, names, reads),
		Financial:                 maskedRecordFinancial(record),
		Changes:                   make([]workforce.TraegerStaffChange, 0, len(changes)),
	}
	for _, change := range changes {
		detail.Changes = append(detail.Changes, publicChange(change, names))
	}
	return detail, nil
}

// publicTraegerStaffError maps the stored refusals onto the contract.
func publicTraegerStaffError(err error) error {
	for _, pair := range []struct{ cause, kind error }{
		{domain.ErrTraegerStaffRecordMissing, workforce.ErrTraegerStaffRecordNotFound},
		{domain.ErrTraegerStaffEmploymentLinked, workforce.ErrTraegerStaffAlreadyLinked},
		{domain.ErrStaffPersonnelNumberTaken, workforce.ErrPersonnelNumberTaken},
		{domain.ErrPersonnelNumberInvalid, workforce.ErrPersonnelNumberInvalid},
		{domain.ErrStaffStammdatenInvalid, workforce.ErrStaffStammdatenInvalid},
		{domain.ErrTraegerStaffReleaseInUse, workforce.ErrTraegerStaffReleaseInUse},
		{domain.ErrTraegerStaffReleaseTaken, workforce.ErrTraegerStaffReleaseExists},
	} {
		if errors.Is(err, pair.cause) {
			return errors.Join(pair.kind, err)
		}
	}
	return err
}

func schoolIDs(schools []workforce.TraegerStaffSchool) []int64 {
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		ids = append(ids, school.ID)
	}
	return ids
}

func schoolNames(schools []workforce.TraegerStaffSchool) map[int64]string {
	names := make(map[int64]string, len(schools))
	for _, school := range schools {
		names[school.ID] = school.Name
	}
	return names
}

func linkSchools(record domain.TraegerStaffRecord) []int64 {
	schools := make([]int64, 0, len(record.Links))
	for _, link := range record.Links {
		schools = append(schools, link.SchoolID)
	}
	return schools
}

func linkWanted(record domain.TraegerStaffRecord) map[int64][]int64 {
	wanted := make(map[int64][]int64, len(record.Links))
	for _, link := range record.Links {
		wanted[link.SchoolID] = []int64{link.StaffID}
	}
	return wanted
}

// summarize names the record's Einsatzorte by school; an employment the
// school no longer holds as live stays listed as ended.
func summarize(record domain.TraegerStaffRecord, names map[int64]string, reads map[int64]schoolRead) workforce.TraegerStaffRecordSummary {
	summary := workforce.TraegerStaffRecordSummary{
		ID: record.ID, FirstName: record.FirstName, LastName: record.LastName, Birthday: record.Birthday,
		PersonnelNumber: record.PersonnelNumber, HasFinancial: record.HasFinancial(),
		Employments: make([]workforce.TraegerStaffEmployment, 0, len(record.Links)),
	}
	for _, link := range record.Links {
		_, active := reads[link.SchoolID].members[link.StaffID]
		summary.Employments = append(summary.Employments, workforce.TraegerStaffEmployment{
			SchoolID: link.SchoolID, SchoolName: names[link.SchoolID], StaffID: link.StaffID, Active: active,
		})
	}
	slices.SortStableFunc(summary.Employments, func(a, b workforce.TraegerStaffEmployment) int { return strings.Compare(a.SchoolName, b.SchoolName) })
	return summary
}

func matchRecord(record domain.TraegerStaffRecord, reads map[int64]schoolRead) domain.StaffMatchRecord {
	match := domain.StaffMatchRecord{
		ID: record.ID, FirstName: record.FirstName, LastName: record.LastName, Birthday: record.Birthday,
		PersonnelNumber: domain.Deref(record.PersonnelNumber),
	}
	for _, link := range record.Links {
		match.SchoolIDs = append(match.SchoolIDs, link.SchoolID)
		if member, ok := reads[link.SchoolID].members[link.StaffID]; ok && member.AccountID != nil {
			match.AccountIDs = append(match.AccountIDs, *member.AccountID)
		}
	}
	return match
}

// unlinkedCandidates lists the live employments no record holds, school by
// school in the order of the school list, people by name.
func unlinkedCandidates(schools []workforce.TraegerStaffSchool, reads map[int64]schoolRead, linked map[[2]int64]bool) []domain.StaffCandidate {
	var candidates []domain.StaffCandidate
	for _, school := range schools {
		read := reads[school.ID]
		for _, staffID := range read.order {
			if linked[[2]int64{school.ID, staffID}] {
				continue
			}
			member := read.members[staffID]
			candidates = append(candidates, domain.StaffCandidate{
				SchoolID: school.ID, StaffID: staffID, AccountID: member.AccountID, FirstName: member.FirstName,
				LastName: member.LastName, Birthday: member.Birthday, PersonnelNumber: domain.Deref(member.PersonnelNumber),
			})
		}
	}
	slices.SortStableFunc(candidates, func(a, b domain.StaffCandidate) int {
		if order := strings.Compare(a.LastName, b.LastName); order != 0 {
			return order
		}
		return strings.Compare(a.FirstName, b.FirstName)
	})
	return candidates
}

func publicSuggestion(suggestion domain.StaffSuggestion, names map[int64]string, reads map[int64]schoolRead) workforce.TraegerStaffSuggestion {
	public := workforce.TraegerStaffSuggestion{Reason: suggestion.Reason, Members: make([]workforce.TraegerStaffCandidate, 0, len(suggestion.Candidates))}
	if suggestion.RecordID > 0 {
		public.RecordID = new(suggestion.RecordID)
	}
	for _, candidate := range suggestion.Candidates {
		public.Members = append(public.Members, publicCandidate(reads[candidate.SchoolID].members[candidate.StaffID], candidate.SchoolID, names))
	}
	return public
}

func publicCandidate(member workforce.TraegerSchoolStaffMember, schoolID int64, names map[int64]string) workforce.TraegerStaffCandidate {
	return workforce.TraegerStaffCandidate{
		SchoolID: schoolID, SchoolName: names[schoolID], StaffID: member.StaffID, FirstName: member.FirstName,
		LastName: member.LastName, Birthday: member.Birthday, PersonnelNumber: member.PersonnelNumber,
		HasAccount: member.AccountID != nil,
	}
}

func publicRelease(release domain.TraegerStaffRelease) *workforce.TraegerStaffRelease {
	return &workforce.TraegerStaffRelease{
		Representation: release.Representation, ReleasedAt: release.ReleasedAt, ReleasedByOperatorID: release.ReleasedByOperatorID,
	}
}

func publicChange(change domain.TraegerStaffChange, names map[int64]string) workforce.TraegerStaffChange {
	public := workforce.TraegerStaffChange{Kind: change.Kind, Field: change.Field, SchoolID: change.SchoolID, OccurredAt: change.OccurredAt}
	switch {
	case change.ValueMasked != nil:
		public.Value = *change.ValueMasked
	case change.ValueText != nil:
		public.Value = *change.ValueText
	}
	if change.SchoolID != nil {
		public.SchoolName = names[*change.SchoolID]
	}
	return public
}

// maskedRecordFinancial shows only the stored mask and the presence of a
// sealed value; nothing is opened.
func maskedRecordFinancial(record domain.TraegerStaffRecord) workforce.TraegerStaffFinancial {
	masked := domain.MaskFinancial(record.ID, &domain.StaffFinancialData{
		IBANMasked: record.IBANMasked, IBANSealed: record.IBANSealed,
		TaxIDSealed: record.TaxIDSealed, SocialSecurityNumberSealed: record.SocialSecurityNumberSealed,
	})
	return workforce.TraegerStaffFinancial{IBAN: masked.IBANMasked, TaxID: masked.TaxIDMasked, SocialSecurityNumber: masked.SocialSecurityNumberMasked}
}
