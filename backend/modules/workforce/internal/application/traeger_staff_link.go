package application

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
)

// A link joins employments of the Träger's schools into one record (ADR
// 0089 section 5). The Personalverwaltung sees every differing value first
// and chooses each one; the values it does not take stay in the trail with
// their school. The bank and tax values move from the schools to the record
// (decision E1 of 13.11.2026): no two copies of one person are kept.

// linkSource is one employment of a link with its opened values.
type linkSource struct {
	ref       workforce.TraegerStaffMemberRef
	member    workforce.TraegerSchoolStaffMember
	financial map[string]string
}

// linkPlan is everything a link read before it writes.
type linkPlan struct {
	record  *domain.TraegerStaffRecord
	members []linkSource
	sources []domain.StaffSource
	names   map[int64]string
}

// PreviewLink shows the values the employments bring, field by field.
func (t *TraegerStaff) PreviewLink(ctx context.Context, actor workforce.TraegerStaffActor, request workforce.TraegerStaffLinkRequest) (workforce.TraegerStaffLinkPreview, error) {
	plan, err := t.planLink(ctx, actor, request)
	if err != nil {
		return workforce.TraegerStaffLinkPreview{}, err
	}
	preview := workforce.TraegerStaffLinkPreview{Members: make([]workforce.TraegerStaffCandidate, 0, len(plan.members))}
	if plan.record != nil {
		summary := summarize(*plan.record, plan.names, nil)
		preview.Record = &summary
	}
	for _, member := range plan.members {
		preview.Members = append(preview.Members, publicCandidate(member.member, member.ref.SchoolID, plan.names))
	}
	for _, field := range domain.PreviewStaffFields(plan.sources) {
		public := workforce.TraegerStaffFieldPreview{Field: field.Field, Conflict: field.Conflict(), Options: []workforce.TraegerStaffFieldOption{}}
		for _, option := range field.Options {
			for _, source := range option.Sources {
				public.Options = append(public.Options, workforce.TraegerStaffFieldOption{
					Source: source.Key, SchoolName: source.SchoolName, Value: source.Display[field.Field],
				})
			}
		}
		preview.Fields = append(preview.Fields, public)
	}
	return preview, nil
}

// Link creates the record or extends the existing one, then hands the
// schools' bank values over and gives every Einsatzort the record's number.
func (t *TraegerStaff) Link(ctx context.Context, actor workforce.TraegerStaffActor, request workforce.TraegerStaffLinkRequest) (workforce.TraegerStaffRecordDetail, error) {
	plan, err := t.planLink(ctx, actor, request)
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, err
	}
	resolved, err := domain.ResolveStaffFields(plan.sources, request.Choices)
	if choice, ok := errors.AsType[*domain.StaffChoiceError](err); ok {
		return workforce.TraegerStaffRecordDetail{}, &workforce.TraegerStaffLinkError{Field: "choices." + choice.Field, Reason: choice.Err.Error()}
	}
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, err
	}
	if number := resolved.Values[domain.StaffFieldPersonnelNumber]; number != "" {
		if err := t.numberFreeAtSchools(ctx, linkTargets(plan), number); err != nil {
			return workforce.TraegerStaffRecordDetail{}, err
		}
	}
	var saved domain.TraegerStaffRecord
	err = t.released(ctx, actor, func(ctx context.Context) error {
		saved, err = t.writeLink(ctx, actor, plan, resolved)
		return err
	})
	if err != nil {
		return workforce.TraegerStaffRecordDetail{}, publicTraegerStaffError(err)
	}
	if err := t.handOver(ctx, actor, saved); err != nil {
		return workforce.TraegerStaffRecordDetail{}, err
	}
	return t.Record(ctx, actor, saved.ID)
}

// planLink checks the request against the Träger's schools and records and
// reads the employments with their bank values. The read is logged in each
// school before any value leaves.
func (t *TraegerStaff) planLink(ctx context.Context, actor workforce.TraegerStaffActor, request workforce.TraegerStaffLinkRequest) (linkPlan, error) {
	schools, err := t.ports.Schools.TraegerSchools(ctx, actor.OrganizationID)
	if err != nil {
		return linkPlan{}, err
	}
	plan := linkPlan{names: schoolNames(schools)}
	if err := validateLinkMembers(request.Members, plan.names); err != nil {
		return linkPlan{}, err
	}
	err = t.released(ctx, actor, func(ctx context.Context) error {
		record, err := t.checkLinkTarget(ctx, actor, request)
		plan.record = record
		return err
	})
	if err != nil {
		return linkPlan{}, publicTraegerStaffError(err)
	}
	wanted := make(map[int64][]int64, len(request.Members))
	for _, ref := range request.Members {
		wanted[ref.SchoolID] = []int64{ref.StaffID}
	}
	reads, err := t.readSchools(ctx, actor, staffView{name: workforce.TraegerStaffViewLinkPreview}, slices.Sorted(maps.Keys(wanted)), wanted, true)
	if err != nil {
		return linkPlan{}, err
	}
	return t.collectSources(plan, request.Members, reads)
}

// checkLinkTarget refuses employments a record already holds and, for an
// existing record, schools that already are its Einsatzorte.
func (t *TraegerStaff) checkLinkTarget(ctx context.Context, actor workforce.TraegerStaffActor, request workforce.TraegerStaffLinkRequest) (*domain.TraegerStaffRecord, error) {
	schools, staff := make([]int64, 0, len(request.Members)), make([]int64, 0, len(request.Members))
	for _, ref := range request.Members {
		schools, staff = append(schools, ref.SchoolID), append(staff, ref.StaffID)
	}
	linked, err := t.store.LinkedEmployments(ctx, actor.OrganizationID, schools, staff)
	if err != nil {
		return nil, err
	}
	if len(linked) > 0 {
		return nil, domain.ErrTraegerStaffEmploymentLinked
	}
	if request.RecordID == nil {
		return nil, nil
	}
	record, err := t.store.FindStaffRecord(ctx, actor.OrganizationID, *request.RecordID, false)
	if err != nil {
		return nil, err
	}
	for _, link := range record.Links {
		if slices.Contains(schools, link.SchoolID) {
			return nil, domain.ErrTraegerStaffEmploymentLinked
		}
	}
	return &record, nil
}

// linkTargets lists every employment the record holds after the link.
func linkTargets(plan linkPlan) []domain.TraegerStaffLink {
	var targets []domain.TraegerStaffLink
	if plan.record != nil {
		targets = append(targets, plan.record.Links...)
	}
	for _, member := range plan.members {
		targets = append(targets, domain.TraegerStaffLink{SchoolID: member.ref.SchoolID, StaffID: member.ref.StaffID})
	}
	return targets
}

func validateLinkMembers(members []workforce.TraegerStaffMemberRef, schools map[int64]string) error {
	if len(members) == 0 || len(members) > domain.MaxLinkMembers {
		return &workforce.TraegerStaffLinkError{Field: "members", Reason: "between one and fifty employments are required"}
	}
	seen := make(map[int64]bool, len(members))
	for _, ref := range members {
		if _, ok := schools[ref.SchoolID]; !ok || ref.StaffID <= 0 {
			return &workforce.TraegerStaffLinkError{Field: "members", Reason: "an employment is not one of the Träger's schools"}
		}
		if seen[ref.SchoolID] {
			return &workforce.TraegerStaffLinkError{Field: "members", Reason: "a record holds one employment per school"}
		}
		seen[ref.SchoolID] = true
	}
	return nil
}

// collectSources opens the values of the record and of every employment.
func (t *TraegerStaff) collectSources(plan linkPlan, refs []workforce.TraegerStaffMemberRef, reads map[int64]schoolRead) (linkPlan, error) {
	if plan.record != nil {
		financial, err := t.openRecordFinancial(*plan.record)
		if err != nil {
			return linkPlan{}, err
		}
		plan.sources = append(plan.sources, staffSource(domain.StaffSourceRecord, 0, "", recordValues(*plan.record, financial)))
	}
	for _, ref := range refs {
		read := reads[ref.SchoolID]
		member, ok := read.members[ref.StaffID]
		if !ok {
			return linkPlan{}, &workforce.TraegerStaffLinkError{Field: "members", Reason: "an employment is no longer live at its school"}
		}
		source := linkSource{ref: ref, member: member, financial: map[string]string{}}
		if row, ok := read.financial[ref.StaffID]; ok {
			opened, err := t.openSchoolFinancial(row)
			if err != nil {
				return linkPlan{}, err
			}
			source.financial = opened
		}
		plan.members = append(plan.members, source)
		values := personValues(member.FirstName, member.LastName, member.Birthday, member.PersonnelNumber, source.financial)
		plan.sources = append(plan.sources, staffSource(domain.SchoolSourceKey(ref.SchoolID), ref.SchoolID, plan.names[ref.SchoolID], values))
	}
	return plan, nil
}

func staffSource(key string, schoolID int64, schoolName string, values map[string]string) domain.StaffSource {
	display := make(map[string]string, len(values))
	for field, value := range values {
		display[field] = value
		if domain.IsFinancialStaffField(field) {
			display[field] = domain.MaskStaffValue(field, value)
		}
	}
	return domain.StaffSource{Key: key, SchoolID: schoolID, SchoolName: schoolName, Values: values, Display: display}
}

// personValues are the fields of a record or an employment by field, the
// bank and tax values opened.
func personValues(firstName, lastName, birthday string, number *string, financial map[string]string) map[string]string {
	values := map[string]string{
		domain.StaffFieldFirstName: firstName, domain.StaffFieldLastName: lastName,
		domain.StaffFieldBirthday: birthday, domain.StaffFieldPersonnelNumber: domain.Deref(number),
	}
	for field, value := range financial {
		values[field] = value
	}
	return values
}

func recordValues(record domain.TraegerStaffRecord, financial map[string]string) map[string]string {
	return personValues(record.FirstName, record.LastName, record.Birthday, record.PersonnelNumber, financial)
}
