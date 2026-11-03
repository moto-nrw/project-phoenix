package application

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The Träger-Büro changes the details a family entered (#5629, ADR 0114
// point 6): parents, children, contacts and the answers without health
// information. The edit runs through change, which locks the period and the
// registration, so two edits of one registration run one after the other and
// each reads what the other wrote. The checks are those of the public form;
// the bookings, the benefits, the consents and the health information stay
// as they are, so the contribution does not change.

var errHolidayHealthAnswerNotEditable = errors.New("an answer with health information cannot be changed here")

// EditHolidayRegistration changes the details of a registration that is not
// cancelled. The history names the changed fields without their values; an
// edit that changes nothing writes nothing.
func (h *HolidayRegistrationReview) EditHolidayRegistration(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, edit enrollment.HolidayRegistrationEdit, schools []enrollment.HolidaySchool) (enrollment.HolidayRegistrationDetail, error) {
	return h.change(ctx, reviewer, periodID, registrationID, func(period enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		fields, err := h.edit(ctx, reviewer.OrganizationID, period, current, edit, schools)
		if err != nil || len(fields) == 0 {
			return "", nil, err
		}
		return enrollment.HolidayAuditEdit, enrollment.HolidayRegistrationEventDetails{Fields: fields}, nil
	})
}

// holidayEditScope names the sections an edit replaces; children holds one
// flag per child of the registration, in its order. familyAnswers and
// childAnswers are the answers as submitted, before they are sanitized.
type holidayEditScope struct {
	guardians, contacts, family bool
	children                    []bool
	familyAnswers               enrollment.HolidayAnswers
	childAnswers                []enrollment.HolidayAnswers
}

// holidayStoredDetails are the details of a registration as stored: the
// input without health information, the IDs of its children in their order
// and the form version it was submitted with.
type holidayStoredDetails struct {
	input    enrollment.HolidayRegistrationInput
	childIDs []int64
	form     enrollment.HolidayPeriodForm
}

func (h *HolidayRegistrationReview) edit(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, current holidayRegistrationState, edit enrollment.HolidayRegistrationEdit, schools []enrollment.HolidaySchool) ([]string, error) {
	if current.Status == enrollment.HolidayRegistrationCancelled {
		return nil, enrollment.ErrHolidayRegistrationStateConflict
	}
	stored, err := h.storedDetails(ctx, organizationID, period.ID, current.ID)
	if err != nil {
		return nil, err
	}
	next, scope, err := applyHolidayEdit(stored, edit)
	if err != nil {
		return nil, err
	}
	next, err = h.checkHolidayEdit(next, scope, stored.form, schools)
	if err != nil {
		return nil, err
	}
	fields := holidayEditFields(stored.input, next)
	if len(fields) == 0 {
		return nil, nil
	}
	write, childIDs, schoolChanged := holidayChangedChildren(stored, next)
	if err := h.store.WriteHolidayRegistrationDetails(ctx, organizationID, current.ID, write, childIDs, schoolChanged); err != nil {
		return nil, fmt.Errorf("enrollment: write holiday registration details: %w", err)
	}
	return fields, nil
}

// holidayChangedChildren keeps only the children the edit changed, with
// their IDs and whether their school changed: an unchanged child is not
// written again.
func holidayChangedChildren(stored holidayStoredDetails, next enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationInput, []int64, []bool) {
	write := next
	write.Children = nil
	childIDs, schoolChanged := []int64{}, []bool{}
	for i, before := range stored.input.Children {
		after := next.Children[i]
		if len(holidayChildFields("", before, after)) == 0 {
			continue
		}
		write.Children = append(write.Children, after)
		childIDs = append(childIDs, stored.childIDs[i])
		schoolChanged = append(schoolChanged, before.HomeSchoolID != after.HomeSchoolID)
	}
	return write, childIDs, schoolChanged
}

// storedDetails reads what the edit may change, without health information.
func (h *HolidayRegistrationReview) storedDetails(ctx context.Context, organizationID, periodID, registrationID int64) (holidayStoredDetails, error) {
	read, _, err := h.store.ReadHolidayReview(ctx, organizationID, periodID, registrationID)
	if err != nil {
		return holidayStoredDetails{}, fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	if len(read) == 0 {
		return holidayStoredDetails{}, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	detail := read[0]
	if detail.Contacts, err = h.store.ReadHolidayContacts(ctx, organizationID, registrationID); err != nil {
		return holidayStoredDetails{}, fmt.Errorf("enrollment: read holiday registration contacts: %w", err)
	}
	if err := h.detailAnswers(ctx, organizationID, periodID, &detail); err != nil {
		return holidayStoredDetails{}, err
	}
	return holidayStoredFromDetail(detail), nil
}

func holidayStoredFromDetail(detail enrollment.HolidayRegistrationDetail) holidayStoredDetails {
	childAnswers := map[int64]enrollment.HolidayAnswers{}
	for _, child := range detail.ChildCustomData {
		childAnswers[child.ChildID] = child.CustomData
	}
	stored := holidayStoredDetails{form: *detail.Form, input: enrollment.HolidayRegistrationInput{
		Guardian: enrollment.HolidayGuardianInput{
			FirstName: detail.GuardianFirstName, LastName: detail.GuardianLastName,
			Email: detail.GuardianEmail, Phone: detail.GuardianPhone,
		},
		GuardianAddress: detail.GuardianAddress, SecondGuardian: detail.SecondGuardian, CustomData: detail.CustomData,
		EmergencyContacts: []enrollment.HolidayContactInput{}, PickupPersons: []enrollment.HolidayContactInput{},
	}}
	for _, child := range detail.Children {
		var homeSchoolID int64
		if child.HomeSchoolID != nil {
			homeSchoolID = *child.HomeSchoolID
		}
		stored.childIDs = append(stored.childIDs, child.ID)
		stored.input.Children = append(stored.input.Children, enrollment.HolidayChildInput{
			FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			HomeSchoolID: homeSchoolID, GuestSchoolName: child.GuestSchoolName, CustomData: childAnswers[child.ID],
		})
	}
	for _, contact := range detail.Contacts {
		entry := enrollment.HolidayContactInput{Name: contact.Name, Phone: contact.Phone, Relationship: contact.Relationship}
		if contact.Kind == enrollment.HolidayContactPickup {
			stored.input.PickupPersons = append(stored.input.PickupPersons, entry)
		} else {
			stored.input.EmergencyContacts = append(stored.input.EmergencyContacts, entry)
		}
	}
	return stored
}

// applyHolidayEdit lays the sections of the edit over the stored details. A
// child the registration does not have is not found; a child named twice is
// a field error.
func applyHolidayEdit(stored holidayStoredDetails, edit enrollment.HolidayRegistrationEdit) (enrollment.HolidayRegistrationInput, holidayEditScope, error) {
	next := stored.input
	next.Children = slices.Clone(stored.input.Children)
	scope := holidayEditScope{children: make([]bool, len(next.Children)), childAnswers: make([]enrollment.HolidayAnswers, len(next.Children))}
	if edit.Guardians != nil {
		scope.guardians = true
		next.Guardian, next.SecondGuardian = edit.Guardians.Guardian, edit.Guardians.SecondGuardian
		next.GuardianAddress = edit.Guardians.GuardianAddress
	}
	if edit.Contacts != nil {
		scope.contacts = true
		next.EmergencyContacts, next.PickupPersons = edit.Contacts.EmergencyContacts, edit.Contacts.PickupPersons
	}
	if edit.CustomData != nil {
		scope.family, scope.familyAnswers = true, *edit.CustomData
		next.CustomData = *edit.CustomData
	}
	for _, child := range edit.Children {
		i := slices.Index(stored.childIDs, child.ChildID)
		if i < 0 {
			return next, scope, enrollment.ErrHolidayRegistrationEntryNotFound
		}
		if scope.children[i] {
			return next, scope, validation.Errors{"children": errors.New("a child is named twice")}
		}
		scope.children[i], scope.childAnswers[i] = true, child.CustomData
		next.Children[i] = enrollment.HolidayChildInput{
			FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			HomeSchoolID: child.HomeSchoolID, GuestSchoolName: child.GuestSchoolName, CustomData: child.CustomData,
		}
	}
	return normalizeHolidayRegistration(next), scope, nil
}

// checkHolidayEdit checks the edited sections with the checks of the public
// form and returns the details with the checked answers. The answers are
// checked against the form version of the registration without its health
// questions: their answers stay sealed and unchanged.
func (h *HolidayRegistrationReview) checkHolidayEdit(next enrollment.HolidayRegistrationInput, scope holidayEditScope, form enrollment.HolidayPeriodForm, schools []enrollment.HolidaySchool) (enrollment.HolidayRegistrationInput, error) {
	errs := validation.Errors{}
	if scope.guardians {
		addHolidayGuardianErrors(errs, next, form.AddressRequirement)
	}
	childErrs := validation.Errors{}
	if slices.Contains(scope.children, true) {
		all := validateHolidayChildren(next.Children, schools, h.today())
		for i, edited := range scope.children {
			if err, ok := all[fmt.Sprint(i)]; ok && edited {
				childErrs[fmt.Sprint(i)] = err
			}
		}
	}
	answers := holidayEditAnswers(next, scope, form)
	addHolidayEditAnswerErrors(errs, childErrs, scope, form, answers)
	if len(childErrs) > 0 {
		errs["children"] = childErrs
	}
	if scope.contacts {
		addHolidayContactErrors(errs, next)
	}
	if len(errs) > 0 {
		return next, errs
	}
	return withHolidayAnswers(next, answers)
}

// addHolidayGuardianErrors checks the parent, the postal address against
// the address block of the registration's form version (#5625) and the
// second guardian.
func addHolidayGuardianErrors(errs validation.Errors, next enrollment.HolidayRegistrationInput, address enrollment.HolidayAddressRequirement) {
	if guardianErrs := validateHolidayGuardian(next.Guardian); len(guardianErrs) > 0 {
		errs["guardian"] = guardianErrs
	}
	if addressErrs := validateHolidayAddress(address, next.GuardianAddress); len(addressErrs) > 0 {
		errs["guardian_address"] = addressErrs
	}
	if next.SecondGuardian != nil {
		if guardianErrs := validateHolidaySecondGuardian(*next.SecondGuardian); len(guardianErrs) > 0 {
			errs["second_guardian"] = guardianErrs
		}
	}
}

func addHolidayContactErrors(errs validation.Errors, next enrollment.HolidayRegistrationInput) {
	if len(next.EmergencyContacts) == 0 {
		errs["emergency_contacts"] = errors.New("at least one emergency contact is required")
	} else if contactErrs := validateHolidayContacts(next.EmergencyContacts, enrollment.HolidayRegistrationMaxEmergencyContacts, true); contactErrs != nil {
		errs["emergency_contacts"] = contactErrs
	}
	if contactErrs := validateHolidayContacts(next.PickupPersons, enrollment.HolidayRegistrationMaxPickupPersons, false); contactErrs != nil {
		errs["pickup_persons"] = contactErrs
	}
}

// holidayEditForm is the form version the edit checks answers against. Its
// health questions are left out: their answers stay sealed, and the edit
// neither reads nor changes them. A question whose condition depends on a
// health question, directly or through another such question, cannot be
// evaluated without the sealed answer; it counts as visible and optional, so
// the edit keeps its answer instead of deleting it (#5629).
func holidayEditForm(form enrollment.HolidayPeriodForm) enrollment.HolidayPeriodForm {
	sealed := map[string]bool{}
	for _, field := range form.Fields {
		if field.Health {
			sealed[field.Key] = true
		}
	}
	unknown := maps.Clone(sealed)
	for changed := true; changed; {
		changed = false
		for _, field := range form.Fields {
			if condition := field.VisibleWhen; condition != nil && condition.Source == enrollment.ConditionSourceField &&
				unknown[condition.Field] && !unknown[field.Key] {
				unknown[field.Key], changed = true, true
			}
		}
	}
	fields := make([]enrollment.HolidayFormField, 0, len(form.Fields))
	for _, field := range form.Fields {
		if sealed[field.Key] {
			continue
		}
		if unknown[field.Key] {
			field.VisibleWhen, field.Required = nil, false
		}
		fields = append(fields, field)
	}
	form.Fields = fields
	return form
}

// holidayEditChildRefreshed reports whether the edit checks the answers of
// child i again: when the edit names the child, and when it changes the
// family's answers, which child questions may depend on.
func holidayEditChildRefreshed(scope holidayEditScope, i int) bool {
	return scope.family || scope.children[i]
}

// holidayEditAnswers decodes the answers and sanitizes the edited scopes as
// the public form does, every child's answers too when the family's answers
// change: an answer whose question is now hidden goes. The stored answers of
// the other scopes stay as they are.
func holidayEditAnswers(next enrollment.HolidayRegistrationInput, scope holidayEditScope, form enrollment.HolidayPeriodForm) holidayAnswerSet {
	set := decodeHolidayAnswers(next)
	sanitized := sanitizeHolidayAnswers(holidayEditForm(form), set)
	if scope.family {
		set.family = sanitized.family
	}
	for i := range scope.children {
		if holidayEditChildRefreshed(scope, i) {
			set.children[i] = sanitized.children[i]
		}
	}
	return set
}

// addHolidayEditAnswerErrors checks the answers of the edited scopes and
// refuses every answer to a question with health information. A child the
// edit does not name is checked after a change of the family's answers, but
// its required questions are not: in the edit of the family's answers no
// child question can be answered, and the next edit of the child asks for
// them.
func addHolidayEditAnswerErrors(errs, childErrs validation.Errors, scope holidayEditScope, form enrollment.HolidayPeriodForm, answers holidayAnswerSet) {
	checked := holidayEditForm(form)
	byKey := buildFieldsByKey(holidayFormSchema(checked))
	if scope.family {
		answerErrs := validateHolidayAnswers(checked, false, answers.family,
			fieldVisibilityContext{guardianAnswers: answers.family, fieldsByKey: byKey})
		refuseHolidayHealthAnswers(answerErrs, form, scope.familyAnswers)
		if len(answerErrs) > 0 {
			errs["custom_data"] = answerErrs
		}
	}
	optional := holidayFormWithoutRequired(checked)
	for i := range scope.children {
		if !holidayEditChildRefreshed(scope, i) {
			continue
		}
		childForm := checked
		if !scope.children[i] {
			childForm = optional
		}
		answerErrs := validateHolidayAnswers(childForm, true, answers.children[i], fieldVisibilityContext{
			guardianAnswers: answers.family, childAnswers: answers.children[i], fieldsByKey: byKey,
		})
		refuseHolidayHealthAnswers(answerErrs, form, scope.childAnswers[i])
		if len(answerErrs) > 0 {
			nestHolidayChildError(childErrs, i, "custom_data", answerErrs)
		}
	}
}

// holidayFormWithoutRequired is the form version with every question
// optional.
func holidayFormWithoutRequired(form enrollment.HolidayPeriodForm) enrollment.HolidayPeriodForm {
	fields := slices.Clone(form.Fields)
	for i := range fields {
		fields[i].Required = false
	}
	form.Fields = fields
	return form
}

func refuseHolidayHealthAnswers(answerErrs validation.Errors, form enrollment.HolidayPeriodForm, submitted enrollment.HolidayAnswers) {
	for _, field := range form.Fields {
		if _, ok := submitted[field.Key]; ok && field.Health {
			answerErrs[field.Key] = errHolidayHealthAnswerNotEditable
		}
	}
}

// holidayEditFields names the changed fields as field paths of the request,
// in the order of the form: parents, children, contacts, answers.
func holidayEditFields(before, after enrollment.HolidayRegistrationInput) []string {
	fields := holidayGuardianFields("guardian", before.Guardian, after.Guardian)
	if !reflect.DeepEqual(before.GuardianAddress, after.GuardianAddress) {
		fields = append(fields, "guardian_address")
	}
	switch {
	case (before.SecondGuardian == nil) != (after.SecondGuardian == nil):
		fields = append(fields, "second_guardian")
	case before.SecondGuardian != nil:
		fields = append(fields, holidayGuardianFields("second_guardian", *before.SecondGuardian, *after.SecondGuardian)...)
	}
	for i := range before.Children {
		fields = append(fields, holidayChildFields(fmt.Sprintf("children.%d", i), before.Children[i], after.Children[i])...)
	}
	fields = append(fields, holidayContactFields("emergency_contacts", before.EmergencyContacts, after.EmergencyContacts)...)
	fields = append(fields, holidayContactFields("pickup_persons", before.PickupPersons, after.PickupPersons)...)
	return append(fields, holidayAnswerFields("custom_data", before.CustomData, after.CustomData)...)
}

func holidayGuardianFields(prefix string, before, after enrollment.HolidayGuardianInput) []string {
	return changedHolidayFields(prefix, []string{"first_name", "last_name", "email", "phone"},
		[]string{before.FirstName, before.LastName, before.Email, before.Phone},
		[]string{after.FirstName, after.LastName, after.Email, after.Phone})
}

func holidayChildFields(prefix string, before, after enrollment.HolidayChildInput) []string {
	fields := changedHolidayFields(prefix, []string{"first_name", "last_name", "birth_date", "home_school_id", "guest_school_name"},
		[]string{before.FirstName, before.LastName, before.BirthDate.String(), fmt.Sprint(before.HomeSchoolID), before.GuestSchoolName},
		[]string{after.FirstName, after.LastName, after.BirthDate.String(), fmt.Sprint(after.HomeSchoolID), after.GuestSchoolName})
	return append(fields, holidayAnswerFields(prefix+".custom_data", before.CustomData, after.CustomData)...)
}

// holidayContactFields names an added or removed contact by its position and
// a changed one by its changed fields.
func holidayContactFields(prefix string, before, after []enrollment.HolidayContactInput) []string {
	fields := []string{}
	for i := range max(len(before), len(after)) {
		path := fmt.Sprintf("%s.%d", prefix, i)
		if i >= len(before) || i >= len(after) {
			fields = append(fields, path)
			continue
		}
		fields = append(fields, changedHolidayFields(path, []string{"name", "phone", "relationship"},
			[]string{before[i].Name, before[i].Phone, before[i].Relationship},
			[]string{after[i].Name, after[i].Phone, after[i].Relationship})...)
	}
	return fields
}

func changedHolidayFields(prefix string, names, before, after []string) []string {
	fields := []string{}
	for i, name := range names {
		if before[i] != after[i] {
			fields = append(fields, prefix+"."+name)
		}
	}
	return fields
}

// holidayAnswerFields names every question whose answer was added, changed
// or removed, sorted by key.
func holidayAnswerFields(prefix string, before, after enrollment.HolidayAnswers) []string {
	old, current := decodeAnswers(before), decodeAnswers(after)
	keys := []string{}
	for key, value := range old {
		if other, ok := current[key]; !ok || !reflect.DeepEqual(value, other) {
			keys = append(keys, key)
		}
	}
	for key := range current {
		if _, ok := old[key]; !ok {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	fields := make([]string, len(keys))
	for i, key := range keys {
		fields[i] = prefix + "." + key
	}
	return fields
}
