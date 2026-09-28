package compose

import (
	"context"
	"sort"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// dayPlanMetadata is what the Tagesplan cards (#2383) show beyond the base
// payload, loaded with one read per kind instead of one per template or
// track: missing ids are simply absent and read as "no metadata".
type dayPlanMetadata struct {
	staff           map[int64]*usersModels.Staff
	templates       map[int64]*activitiesModels.Group
	tracks          map[int64]timetable.PlanningTrack
	educationGroups map[int64]string
}

// enrichDayPlan decorates whole-day-scope blocks (#2383) with the staff
// display names, the planning-track colour of the template and the
// template's education group ("Zielgruppe"). candidates and out are
// parallel slices.
func (s *operations) enrichDayPlan(ctx context.Context, candidates []plannedNowCandidate, out []timetable.OperationPlannedInstance) error {
	meta, err := s.loadDayPlanMetadata(ctx, candidates)
	if err != nil {
		return err
	}
	for i, candidate := range candidates {
		out[i].StaffNames = dayPlanStaffNames(candidate, meta.staff)
		if groupID := candidate.instance.ActivityGroupID; groupID != nil {
			meta.decorate(&out[i], meta.templates[*groupID])
		}
	}
	return nil
}

// decorate sets the template's education group and planning track on a
// card; a missing template or reference leaves the card as it is.
func (meta dayPlanMetadata) decorate(card *timetable.OperationPlannedInstance, group *activitiesModels.Group) {
	if group == nil {
		return
	}
	if group.EducationGroupID != nil {
		if name, ok := meta.educationGroups[*group.EducationGroupID]; ok {
			card.GroupName = &name
		}
	}
	if group.PlanningTrackID != nil {
		if track, ok := meta.tracks[*group.PlanningTrackID]; ok {
			trackName, trackColor := track.Name, track.Color
			card.PlanningTrackName = &trackName
			card.PlanningTrackColor = &trackColor
		}
	}
}

func (s *operations) loadDayPlanMetadata(ctx context.Context, candidates []plannedNowCandidate) (dayPlanMetadata, error) {
	meta := dayPlanMetadata{staff: map[int64]*usersModels.Staff{}, tracks: map[int64]timetable.PlanningTrack{}, educationGroups: map[int64]string{}}
	if staffIDs := dayPlanStaffIDs(candidates); len(staffIDs) > 0 {
		staff, err := s.deps.People.GetStaffWithPersonByIDs(ctx, staffIDs)
		if err != nil {
			return dayPlanMetadata{}, err
		}
		meta.staff = staff
	}
	templates, educationGroupIDs, trackIDs, err := s.loadDayPlanTemplates(ctx, candidates)
	if err != nil {
		return dayPlanMetadata{}, err
	}
	meta.templates = templates
	if len(trackIDs) > 0 && s.deps.PlanningTracks != nil {
		tracks, err := s.deps.PlanningTracks.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{IDs: trackIDs})
		if err != nil {
			return dayPlanMetadata{}, err
		}
		for _, track := range tracks {
			meta.tracks[track.ID] = track
		}
	}
	if len(educationGroupIDs) > 0 {
		names, err := s.deps.EducationGroups.EducationGroupNames(ctx, educationGroupIDs)
		if err != nil {
			return dayPlanMetadata{}, err
		}
		meta.educationGroups = names
	}
	return meta, nil
}

// loadDayPlanTemplates reads the blocks' templates in one read and collects
// the distinct education groups and planning tracks they reference.
func (s *operations) loadDayPlanTemplates(ctx context.Context, candidates []plannedNowCandidate) (map[int64]*activitiesModels.Group, []int64, []int64, error) {
	templates := map[int64]*activitiesModels.Group{}
	groupIDs := make([]int64, 0)
	seen := map[int64]bool{}
	for _, candidate := range candidates {
		groupID := candidate.instance.ActivityGroupID
		if groupID == nil || *groupID <= 0 || seen[*groupID] {
			continue
		}
		seen[*groupID] = true
		groupIDs = append(groupIDs, *groupID)
	}
	educationGroupIDs, trackIDs := make([]int64, 0), make([]int64, 0)
	if len(groupIDs) == 0 {
		return templates, educationGroupIDs, trackIDs, nil
	}
	groups, err := s.deps.Templates.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	educationSeen, trackSeen := map[int64]bool{}, map[int64]bool{}
	for _, group := range groups {
		templates[group.ID] = group
		if group.EducationGroupID != nil && !educationSeen[*group.EducationGroupID] {
			educationSeen[*group.EducationGroupID] = true
			educationGroupIDs = append(educationGroupIDs, *group.EducationGroupID)
		}
		if group.PlanningTrackID != nil && !trackSeen[*group.PlanningTrackID] {
			trackSeen[*group.PlanningTrackID] = true
			trackIDs = append(trackIDs, *group.PlanningTrackID)
		}
	}
	return templates, educationGroupIDs, trackIDs, nil
}

func dayPlanStaffIDs(candidates []plannedNowCandidate) []int64 {
	seen := map[int64]bool{}
	ids := make([]int64, 0)
	for _, candidate := range candidates {
		for _, row := range candidate.staffRows {
			if !row.IsAbsent && !seen[row.StaffID] {
				seen[row.StaffID] = true
				ids = append(ids, row.StaffID)
			}
		}
	}
	return ids
}

// dayPlanStaffNames names the block's present staff, sorted by name.
func dayPlanStaffNames(candidate plannedNowCandidate, staffByID map[int64]*usersModels.Staff) []timetable.OperationStaffName {
	names := make([]timetable.OperationStaffName, 0, len(candidate.staffRows))
	for _, row := range candidate.staffRows {
		if row.IsAbsent {
			continue
		}
		staff := staffByID[row.StaffID]
		if staff == nil || staff.Person == nil {
			continue
		}
		names = append(names, timetable.OperationStaffName{
			StaffID:      row.StaffID,
			DisplayName:  staff.Person.GetFullName(),
			IsSubstitute: row.IsSubstitute,
		})
	}
	sort.Slice(names, func(a, b int) bool { return names[a].DisplayName < names[b].DisplayName })
	return names
}
