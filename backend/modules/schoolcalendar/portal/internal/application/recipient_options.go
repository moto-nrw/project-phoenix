package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/collation"
)

func (s *service) RecipientOptions(ctx context.Context, query string, limit int) (*RecipientOptions, error) {
	if limit <= 0 || limit > 50 {
		limit = 25
	}
	search := recipientQueryFor(query)

	staffOptions, err := s.staffRecipientOptions(ctx, search.folded, limit)
	if err != nil {
		return nil, err
	}
	parentOptions, err := s.parentRecipientOptions(ctx, search, limit)
	if err != nil {
		return nil, err
	}
	groupOptions, err := s.groupRecipientOptions(ctx, search.folded, limit)
	if err != nil {
		return nil, err
	}
	classOptions, err := s.classRecipientOptions(ctx, search.folded, limit)
	if err != nil {
		return nil, err
	}
	studentOptions, err := s.studentRecipientOptions(ctx, search.folded, limit)
	if err != nil {
		return nil, err
	}

	return &RecipientOptions{
		Staff:    staffOptions,
		Parents:  parentOptions,
		Groups:   groupOptions,
		Classes:  classOptions,
		Students: studentOptions,
	}, nil
}

// recipientQuery is the search as typed, for the database search that folds
// on its own, and folded, for the in-memory lists (#5791). Folding before the
// database misses e-mail addresses with umlauts and letters Go folds but
// PostgreSQL keeps, such as й.
type recipientQuery struct {
	text   string
	folded string
}

func recipientQueryFor(query string) recipientQuery {
	return recipientQuery{text: strings.TrimSpace(query), folded: normalizeRecipientQuery(query)}
}

// normalizeRecipientQuery trims the search query and folds it for
// matchesRecipientQuery.
func normalizeRecipientQuery(query string) string {
	return collation.FoldForSearch(strings.TrimSpace(query))
}

// matchesRecipientQuery reports whether name matches the normalized search
// query, ignoring case and accents (#4949); an empty query matches everything.
func matchesRecipientQuery(name, query string) bool {
	return query == "" || strings.Contains(collation.FoldForSearch(name), query)
}

func (s *service) staffRecipientOptions(ctx context.Context, query string, limit int) ([]StaffOption, error) {
	staffRows, err := s.cfg.StaffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	// Only offer staff that target resolution would actually accept
	// (FindReachableCalendarStaffIDs), so the picker never presents a choice
	// that CreateStaffAppointment later rejects with "staff target is not
	// available".
	reachableStaff, err := s.cfg.StaffRepo.FindReachableCalendarStaffIDs(ctx, nil)
	if err != nil {
		return nil, err
	}
	staffOptions := make([]StaffOption, 0, min(limit, len(staffRows)))
	for _, row := range staffRows {
		if !reachableStaff[row.ID] {
			continue
		}
		name := staffName(row)
		if matchesRecipientQuery(name, query) {
			staffOptions = append(staffOptions, StaffOption{ID: formatID(row.ID), Name: name})
			if len(staffOptions) >= limit {
				break
			}
		}
	}
	return staffOptions, nil
}

func (s *service) parentRecipientOptions(ctx context.Context, query recipientQuery, limit int) ([]ParentOption, error) {
	// Over-fetch text matches before filtering: reachability (active portal
	// account + guardian role) and student visibility are applied below, so
	// capping SearchByText at `limit` would let unreachable matches consume the
	// budget and hide reachable parents ranked later. Fetch a larger candidate
	// pool, then cap the filtered result at `limit`.
	parents, err := s.cfg.GuardianProfileRepo.SearchByText(ctx, query.text, limit*parentSearchCandidateFactor)
	if err != nil {
		return nil, err
	}
	parentIDs := make([]int64, 0, len(parents))
	for _, parent := range parents {
		parentIDs = append(parentIDs, parent.ID)
	}
	activeParents, err := s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, parentIDs)
	if err != nil {
		return nil, err
	}
	visible, err := s.guardiansWithPortalVisibleStudent(ctx, parents, activeParents)
	if err != nil {
		return nil, err
	}
	parentOptions := make([]ParentOption, 0, min(limit, len(parents)))
	for _, parent := range parents {
		if len(parentOptions) >= limit {
			break
		}
		if _, ok := activeParents[parent.ID]; ok && visible[parent.ID] {
			parentOptions = append(parentOptions, ParentOption{ID: formatID(parent.ID), Name: guardianName(parent)})
		}
	}
	return parentOptions, nil
}

// guardiansWithPortalVisibleStudent names the active candidates holding
// parent_portal.access on at least one child relationship. One portal-contact
// read answers all of them (#5784, decision E2: an early shared read may
// replace the former per-parent READ COMMITTED reads; the picker takes no
// lock, and creating the appointment resolves its targets afresh).
func (s *service) guardiansWithPortalVisibleStudent(ctx context.Context, parents []*ports.GuardianProfile, active map[int64]*ports.GuardianProfile) (map[int64]bool, error) {
	ids := make([]int64, 0, len(active))
	for _, parent := range parents {
		if _, ok := active[parent.ID]; ok {
			ids = append(ids, parent.ID)
		}
	}
	visible := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return visible, nil
	}
	contacts, err := s.cfg.GuardianProfileRepo.ListPortalContacts(ctx, ids, nil)
	if err != nil {
		return nil, err
	}
	for _, contact := range contacts {
		if contact.Relationship != nil && contact.Relationship.PortalAccess {
			visible[contact.Profile.ID] = true
		}
	}
	return visible, nil
}

func (s *service) groupRecipientOptions(ctx context.Context, query string, limit int) ([]GroupOption, error) {
	groups, err := s.cfg.GroupRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	groupOptions := make([]GroupOption, 0, min(limit, len(groups)))
	for _, group := range groups {
		if matchesRecipientQuery(group.Name, query) {
			groupOptions = append(groupOptions, GroupOption{ID: formatID(group.ID), Name: group.Name})
			if len(groupOptions) >= limit {
				break
			}
		}
	}
	return groupOptions, nil
}

func (s *service) classRecipientOptions(ctx context.Context, query string, limit int) ([]string, error) {
	classes, err := s.cfg.StudentRepo.ListSchoolClasses(ctx)
	if err != nil {
		return nil, err
	}
	classOptions := make([]string, 0, min(limit, len(classes)))
	for _, className := range classes {
		if matchesRecipientQuery(className, query) {
			classOptions = append(classOptions, className)
			if len(classOptions) >= limit {
				break
			}
		}
	}
	return classOptions, nil
}

func (s *service) studentRecipientOptions(ctx context.Context, query string, limit int) ([]StudentOption, error) {
	students, err := s.cfg.StudentRepo.FindAllWithGroups(ctx)
	if err != nil {
		return nil, err
	}
	studentOptions := make([]StudentOption, 0, min(limit, len(students)))
	for _, row := range students {
		name := studentDisplayName(row)
		if !matchesRecipientQuery(name, query) {
			continue
		}
		var groupID *string
		if row.GroupID != nil {
			value := formatID(*row.GroupID)
			groupID = &value
		}
		studentOptions = append(studentOptions, StudentOption{
			ID:          formatID(row.ID),
			Name:        name,
			SchoolClass: row.SchoolClass,
			GroupID:     groupID,
		})
		if len(studentOptions) >= limit {
			break
		}
	}
	return studentOptions, nil
}

func staffName(staff *ports.Staff) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName)
}

func guardianName(parent *ports.GuardianProfile) string {
	if parent == nil {
		return ""
	}
	name := strings.TrimSpace(parent.FirstName + " " + parent.LastName)
	if name == "" && parent.Email != nil {
		return *parent.Email
	}
	return name
}

func studentDisplayName(student *ports.Student) string {
	if student == nil {
		return ""
	}
	if student.Person == nil {
		return fmt.Sprintf("Kind %d", student.ID)
	}
	return strings.TrimSpace(student.Person.FirstName + " " + student.Person.LastName)
}
