package application

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type classRosterApprovedEnrollment struct {
	request   *enrollmentModels.Request
	child     *reportChild
	links     []*enrollment.RequestChildOfferingRecord
	guardians []*enrollment.RequestGuardian
}

func classRosterStudentIDs(students []*RosterStudent) []int64 {
	ids := make([]int64, 0, len(students))
	seen := map[int64]bool{}
	for _, student := range students {
		if student == nil || student.ID <= 0 || seen[student.ID] {
			continue
		}
		seen[student.ID] = true
		ids = append(ids, student.ID)
	}
	return ids
}

// classRosterAllStudentIDs lists the id of every loaded student.
func classRosterAllStudentIDs(students []*RosterStudent) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student != nil {
			ids = append(ids, student.ID)
		}
	}
	return ids
}

func classRosterStudentsByID(students []*RosterStudent) map[int64]*RosterStudent {
	byID := make(map[int64]*RosterStudent, len(students))
	for _, student := range students {
		if student != nil {
			byID[student.ID] = student
		}
	}
	return byID
}

func classRosterPersonIDs(students []*RosterStudent) []int64 {
	ids := make([]int64, 0, len(students))
	seen := map[int64]bool{}
	for _, student := range students {
		if student == nil || student.PersonID <= 0 || seen[student.PersonID] {
			continue
		}
		seen[student.PersonID] = true
		ids = append(ids, student.PersonID)
	}
	return ids
}

func classRosterGroupIDs(students []*RosterStudent) []int64 {
	ids := make([]int64, 0, len(students))
	seen := map[int64]bool{}
	for _, student := range students {
		if student == nil || student.GroupID == nil || *student.GroupID <= 0 || seen[*student.GroupID] {
			continue
		}
		seen[*student.GroupID] = true
		ids = append(ids, *student.GroupID)
	}
	return ids
}

func classRosterGroupName(student *RosterStudent, groups map[int64]string) string {
	if student == nil || student.GroupID == nil || groups == nil {
		return ""
	}
	return groups[*student.GroupID]
}

func classRosterRequestIDs(requests []*enrollmentModels.Request) []int64 {
	ids := make([]int64, 0, len(requests))
	seen := map[int64]bool{}
	for _, req := range requests {
		if req == nil || req.ID <= 0 || seen[req.ID] {
			continue
		}
		seen[req.ID] = true
		ids = append(ids, req.ID)
	}
	return ids
}

func classRosterRequestsByID(requests []*enrollmentModels.Request) map[int64]*enrollmentModels.Request {
	out := make(map[int64]*enrollmentModels.Request, len(requests))
	for _, req := range requests {
		if req != nil {
			out[req.ID] = req
		}
	}
	return out
}

func requestGuardiansByRequestID(guardians []*enrollment.RequestGuardian) map[int64][]*enrollment.RequestGuardian {
	out := make(map[int64][]*enrollment.RequestGuardian)
	for _, guardian := range guardians {
		if guardian == nil || guardian.RequestID <= 0 {
			continue
		}
		out[guardian.RequestID] = append(out[guardian.RequestID], guardian)
	}
	return out
}

func classRosterChildrenForStudents(children []*reportChild, studentByID map[int64]*RosterStudent) []*reportChild {
	out := make([]*reportChild, 0, len(children))
	for _, child := range children {
		if child == nil || child.CreatedStudentID == nil {
			continue
		}
		if studentByID[*child.CreatedStudentID] != nil {
			out = append(out, child)
		}
	}
	return out
}

func classRosterApprovedEnrollments(
	children []*reportChild,
	requestByID map[int64]*enrollmentModels.Request,
	studentByID map[int64]*RosterStudent,
) (map[int64]*classRosterApprovedEnrollment, []int64) {
	out := make(map[int64]*classRosterApprovedEnrollment)
	for _, child := range children {
		if child == nil || child.Status != enrollment.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
			continue
		}
		studentID := *child.CreatedStudentID
		req := requestByID[child.RequestID]
		if studentByID[studentID] == nil || req == nil {
			continue
		}
		current := out[studentID]
		if current == nil {
			out[studentID] = &classRosterApprovedEnrollment{request: req, child: child}
		} else if classRosterChildIsNewer(req, child, current.request, current.child) {
			current.request = req
			current.child = child
		}
	}
	childIDs := make([]int64, 0, len(out))
	for _, approved := range out {
		if approved != nil && approved.child != nil && approved.child.ID > 0 {
			childIDs = append(childIDs, approved.child.ID)
		}
	}
	sort.Slice(childIDs, func(i, j int) bool { return childIDs[i] < childIDs[j] })
	return out, childIDs
}

func classRosterChildIsNewer(candidateReq *enrollmentModels.Request, candidateChild *reportChild, currentReq *enrollmentModels.Request, currentChild *reportChild) bool {
	if currentReq == nil {
		return true
	}
	if candidateReq == nil {
		return false
	}
	if !candidateReq.SubmittedAt.Equal(currentReq.SubmittedAt) {
		return candidateReq.SubmittedAt.After(currentReq.SubmittedAt)
	}
	if currentChild == nil {
		return true
	}
	if candidateChild == nil {
		return false
	}
	return candidateChild.ID > currentChild.ID
}

func classRosterAttachOfferingLinks(enrollments map[int64]*classRosterApprovedEnrollment, links []*enrollment.RequestChildOfferingRecord) {
	studentIDByChildID := make(map[int64]int64)
	for studentID, approved := range enrollments {
		if approved == nil || approved.child == nil || approved.child.ID <= 0 {
			continue
		}
		studentIDByChildID[approved.child.ID] = studentID
	}
	for _, link := range links {
		if link == nil {
			continue
		}
		studentID, ok := studentIDByChildID[link.RequestChildID]
		if !ok {
			continue
		}
		enrollments[studentID].links = append(enrollments[studentID].links, link)
	}
}

func classRosterAttachRequestGuardians(enrollments map[int64]*classRosterApprovedEnrollment, guardiansByRequestID map[int64][]*enrollment.RequestGuardian) {
	for _, approved := range enrollments {
		if approved == nil || approved.request == nil {
			continue
		}
		approved.guardians = guardiansByRequestID[approved.request.ID]
	}
}

// row builds the roster row of one student: the live student's name, group,
// contacts and departure plan, overlaid with the approved enrollment of the
// phase when there is one.
func (in *classRosterInputs) row(student *RosterStudent) (enrollment.ClassRosterRow, error) {
	studentContactGuardians := normalizeClassRosterGuardians(in.studentGuardians[student.ID])
	companions := in.companions[student.ID]
	row := enrollment.ClassRosterRow{
		StudentID:           student.ID,
		SchoolClass:         student.SchoolClass,
		GroupName:           classRosterGroupName(student, in.groups),
		EnrollmentSummary:   "Keine Anmeldung",
		CareDays:            []string{},
		OfferingsByDay:      map[string][]string{},
		ArrivalByDay:        map[string]string{},
		PickupByDay:         map[string]string{},
		SchedulePickupByDay: map[string]string{},
		Guardians:           studentContactGuardians,
	}
	if person := in.persons[student.PersonID]; person != nil {
		row.FirstName = person.FirstName
		row.LastName = person.LastName
	}
	row.DepartureByDay = classRosterDepartureByDayFromStudent(student, companions)
	approved := in.enrollments[student.ID]
	if approved != nil && approved.request != nil && approved.child != nil {
		if err := in.applyEnrollment(&row, student, approved); err != nil {
			return row, err
		}
		if len(row.Guardians) == 0 {
			row.Guardians = studentContactGuardians
		}
	}
	if byDay := in.schedulePickup[student.ID]; byDay != nil {
		row.SchedulePickupByDay = byDay
	}
	return row, nil
}

// applyEnrollment fills the enrollment columns from the approved enrollment's
// answers and offering selections.
func (in *classRosterInputs) applyEnrollment(row *enrollment.ClassRosterRow, student *RosterStudent, approved *classRosterApprovedEnrollment) error {
	pickupByDay, err := careUsagePickupByDay(approved.request, approved.child, in.schemas)
	if err != nil {
		return fmt.Errorf("class roster report: child %d pickup schedule: %w", approved.child.ID, err)
	}
	arrivalByDay, err := careUsageScheduleByTarget(approved.request, approved.child, in.schemas, enrollment.TargetScheduleArrival)
	if err != nil {
		return fmt.Errorf("class roster report: child %d arrival schedule: %w", approved.child.ID, err)
	}
	departureByDay, err := classRosterDepartureByDay(approved.request, approved.child, in.schemas, student, in.companions[student.ID])
	if err != nil {
		return fmt.Errorf("class roster report: child %d departure: %w", approved.child.ID, err)
	}
	mergedLinks := classRosterMergeOfferingLinks(approved.links)
	careRow := careUsageRow(approved.request, approved.child, mergedLinks, in.offeringByID, classRosterIncludedOfferingIDs(mergedLinks), pickupByDay)
	row.Registered = true
	row.EnrollmentSummary = classRosterEnrollmentSummary(careRow.Offerings)
	row.Offerings = careRow.Offerings
	row.OfferingsByDay = classRosterOfferingsByDay(careRow.Offerings)
	row.CareDays = classRosterCareDays(careRow.EffectiveDays, in.offeringByID, in.careOfferingsActive)
	row.PickupByDay = pickupByDay
	row.ArrivalByDay = arrivalByDay
	row.DepartureByDay = departureByDay
	row.Guardians = classRosterEnrollmentGuardians(approved.request, approved.guardians)
	return nil
}

// classRosterCareDays is the roster-side mirror of the form's relevant care
// days: booked offering days when the phase has an active catalog (or
// leftover selections), every weekday when it does not. The form only loads
// offerings when enrollment.care_offerings_enabled is on, and only active
// rows; leftover active catalog entries after the setting is turned off must
// not hide pickup times the form treated as unrestricted. EffectiveDays is
// derived only from booked offerings and stays empty in the unconstrained
// case.
func classRosterCareDays(effectiveDays []string, offeringByID map[int64]*enrollmentModels.CareOffering, careOfferingsEnabled bool) []string {
	if (careOfferingsEnabled && classRosterHasActiveOfferings(offeringByID)) || len(effectiveDays) > 0 {
		return effectiveDays
	}
	return sortedDayCodes([]string{"mon", "tue", "wed", "thu", "fri"})
}

func classRosterHasActiveOfferings(offeringByID map[int64]*enrollmentModels.CareOffering) bool {
	for _, offering := range offeringByID {
		if offering != nil && offering.IsActive {
			return true
		}
	}
	return false
}

type classRosterContactAccumulator struct {
	contacts map[string]enrollment.ClassRosterGuardian
	order    []string
}

func classRosterStudentGuardianContactsFromRows(rows []GuardianContactRow) map[int64][]enrollment.ClassRosterGuardian {
	byStudent := map[int64]*classRosterContactAccumulator{}
	for _, row := range rows {
		if row.StudentID <= 0 {
			continue
		}
		name := strings.TrimSpace(row.FirstName + " " + row.LastName)
		email := strings.TrimSpace(row.Email)
		phone := strings.TrimSpace(row.PhoneNumber)
		if name == "" && email == "" && phone == "" {
			continue
		}
		key := classRosterStudentGuardianContactKey(row)
		acc := byStudent[row.StudentID]
		if acc == nil {
			acc = &classRosterContactAccumulator{contacts: map[string]enrollment.ClassRosterGuardian{}}
			byStudent[row.StudentID] = acc
		}
		contact, exists := acc.contacts[key]
		if !exists {
			acc.order = append(acc.order, key)
		}
		if contact.Name == "" {
			contact.Name = name
		}
		if contact.Email == "" {
			contact.Email = email
		}
		contact.Phone = joinUnique(contact.Phone, phone)
		acc.contacts[key] = contact
	}
	out := make(map[int64][]enrollment.ClassRosterGuardian, len(byStudent))
	for studentID, acc := range byStudent {
		contacts := make([]enrollment.ClassRosterGuardian, 0, len(acc.order))
		for _, key := range acc.order {
			contacts = append(contacts, acc.contacts[key])
		}
		out[studentID] = normalizeClassRosterGuardians(contacts)
	}
	return out
}

func classRosterStudentGuardianContactKey(row GuardianContactRow) string {
	if row.GuardianProfileID > 0 {
		return strconv.FormatInt(row.GuardianProfileID, 10)
	}
	return strings.ToLower(strings.TrimSpace(row.FirstName + " " + row.LastName + "|" + row.Email))
}

// joinUnique splits every value on ";", keeps each trimmed, non-empty part
// once (case-insensitive, first spelling wins) and joins them with "; ".
func joinUnique(values ...string) string {
	seen := make(map[string]bool, len(values))
	parts := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ";") {
			trimmed := strings.TrimSpace(part)
			key := strings.ToLower(trimmed)
			if trimmed == "" || seen[key] {
				continue
			}
			seen[key] = true
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "; ")
}

func classRosterEnrollmentGuardians(req *enrollmentModels.Request, additional []*enrollment.RequestGuardian) []enrollment.ClassRosterGuardian {
	contacts := []enrollment.ClassRosterGuardian{}
	if req != nil {
		contacts = append(contacts, enrollment.ClassRosterGuardian{
			Name:  strings.TrimSpace(req.GuardianFirstName + " " + req.GuardianLastName),
			Email: strings.TrimSpace(req.GuardianEmail),
			Phone: stringPtrValue(req.GuardianPhone),
		})
	}
	for _, guardian := range additional {
		if guardian == nil {
			continue
		}
		contacts = append(contacts, enrollment.ClassRosterGuardian{
			Name:  strings.TrimSpace(guardian.FirstName + " " + guardian.LastName),
			Email: stringPtrValue(guardian.Email),
			Phone: stringPtrValue(guardian.Phone),
		})
	}
	return normalizeClassRosterGuardians(contacts)
}

func normalizeClassRosterGuardians(contacts []enrollment.ClassRosterGuardian) []enrollment.ClassRosterGuardian {
	if len(contacts) == 0 {
		return []enrollment.ClassRosterGuardian{}
	}
	out := make([]enrollment.ClassRosterGuardian, 0, len(contacts))
	seen := map[string]bool{}
	for _, contact := range contacts {
		contact.Name = strings.TrimSpace(contact.Name)
		contact.Email = strings.TrimSpace(contact.Email)
		contact.Phone = strings.TrimSpace(contact.Phone)
		if contact.Name == "" && contact.Email == "" && contact.Phone == "" {
			continue
		}
		key := strings.ToLower(contact.Name + "|" + contact.Email + "|" + contact.Phone)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, contact)
	}
	return out
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func classRosterOfferingsByDay(offerings []enrollment.CareUsageRowOffering) map[string][]string {
	out := map[string][]string{}
	seenByDay := map[string]map[string]bool{}
	for _, offering := range offerings {
		name := strings.TrimSpace(offering.Name)
		if name == "" {
			continue
		}
		for _, day := range offering.Days {
			day = strings.ToLower(strings.TrimSpace(day))
			if day == "" {
				continue
			}
			if seenByDay[day] == nil {
				seenByDay[day] = map[string]bool{}
			}
			if seenByDay[day][name] {
				continue
			}
			seenByDay[day][name] = true
			out[day] = append(out[day], name)
		}
	}
	for day := range out {
		sort.Strings(out[day])
	}
	return out
}

func classRosterMergeOfferingLinks(links []*enrollment.RequestChildOfferingRecord) []*enrollment.RequestChildOfferingRecord {
	if len(links) == 0 {
		return nil
	}
	byID := make(map[int64]*enrollment.RequestChildOfferingRecord, len(links))
	for _, link := range links {
		if link == nil || link.CareOfferingID <= 0 {
			continue
		}
		merged := byID[link.CareOfferingID]
		if merged == nil {
			copied := *link
			copied.SelectedDays = sortedDayCodes(copied.SelectedDays)
			copied.ManualSelectedDays = sortedDayCodes(copied.ManualSelectedDays)
			copied.AutomaticSelectedDays = sortedDayCodes(copied.AutomaticSelectedDays)
			byID[link.CareOfferingID] = &copied
			continue
		}
		merged.SelectedDays = mergeDayCodes(merged.SelectedDays, link.SelectedDays)
		merged.ManualSelectedDays = mergeDayCodes(merged.ManualSelectedDays, link.ManualSelectedDays)
		merged.AutomaticSelectedDays = mergeDayCodes(merged.AutomaticSelectedDays, link.AutomaticSelectedDays)
	}
	out := make([]*enrollment.RequestChildOfferingRecord, 0, len(byID))
	for _, link := range byID {
		out = append(out, link)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CareOfferingID < out[j].CareOfferingID })
	return out
}

func mergeDayCodes(a, b []string) []string {
	combined := make([]string, 0, len(a)+len(b))
	combined = append(combined, a...)
	combined = append(combined, b...)
	return sortedDayCodes(combined)
}

func classRosterIncludedOfferingIDs(links []*enrollment.RequestChildOfferingRecord) map[int64]bool {
	out := make(map[int64]bool, len(links))
	for _, link := range links {
		if link != nil && link.CareOfferingID > 0 {
			out[link.CareOfferingID] = true
		}
	}
	return out
}

func classRosterEnrollmentSummary(offerings []enrollment.CareUsageRowOffering) string {
	names := make([]string, 0, len(offerings))
	seen := map[string]bool{}
	for _, offering := range offerings {
		name := strings.TrimSpace(offering.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "Angemeldet"
	}
	return "Angemeldet: " + strings.Join(names, ", ")
}
