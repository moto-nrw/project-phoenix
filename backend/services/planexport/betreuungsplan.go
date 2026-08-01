package planexport

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// ExportBetreuungsplan renders the care week: which block runs when, in
// which room, with which staff, for how many children.
//
// Unlike the Dienstplan this cannot read StaffScheduleOverview: that
// projection exists to answer "is this staff member's task covered by a
// shift" and therefore drops cancelled blocks and blocks nobody is assigned
// to yet. A care plan has to print both.
func (s *service) ExportBetreuungsplan(ctx context.Context, params Params) (listexport.File, error) {
	if err := params.validate(TemplatesForBetreuungsplan); err != nil {
		return listexport.File{}, err
	}
	if s.deps.Instances == nil || s.deps.InstanceStaff == nil || s.deps.Renderer == nil {
		return listexport.File{}, errors.New("plan export service is not fully wired")
	}

	weeks, err := expandWeeks(params.From, params.To)
	if err != nil {
		return listexport.File{}, err
	}
	from, to := weeks[0].days[0], weeks[len(weeks)-1].last()

	data, err := s.loadBetreuungsplanData(ctx, from, to, params.Variant)
	if err != nil {
		return listexport.File{}, err
	}
	// The range is loaded Monday–Sunday; the sheet keeps the weekend only
	// where blocks actually run on it.
	weeks = narrowWeeks(weeks, data.plannedDays())

	doc := s.document(
		"Betreuungsplan", "Angebot",
		"Fett: Zeit und Raum · darunter das Personal",
		params, weeks, data.rows, data.emptyWeekRows,
	)

	s.getLogger().Info("betreuungsplan export rendered",
		"from", from.String(),
		"to", to.String(),
		"variant", string(params.Variant),
		"format", string(params.Format),
		"week_count", len(weeks),
	)
	return s.deps.Renderer.Render(doc, params.Format, filename("Betreuungsplan", weeks))
}

// betreuungsplanData is the loaded care week indexed by block title.
type betreuungsplanData struct {
	instances   []*scheduleModel.ActivityInstance
	staffByInst map[int64][]*scheduleModel.InstanceStaff
	staffNames  map[int64]string
	roomNames   map[int64]string
	childCounts map[int64]int
	// blockColors maps an instance id to its Kategorie colour, the same
	// colour the planner paints the block with on screen.
	blockColors map[int64]string
	closedDays  map[timezone.Date]string
	variant     Variant
}

func (s *service) loadBetreuungsplanData(
	ctx context.Context, from, to timezone.Date, variant Variant,
) (*betreuungsplanData, error) {
	instances, err := s.deps.Instances.FindByTenantAndDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load activity instances: %w", err)
	}

	instanceIDs := make([]int64, 0, len(instances))
	roomIDs := make([]int64, 0, len(instances))
	kept := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		kept = append(kept, instance)
		instanceIDs = append(instanceIDs, instance.ID)
		roomIDs = append(roomIDs, instance.RoomID)
	}

	staffRows, err := s.deps.InstanceStaff.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("load instance staff: %w", err)
	}
	staffByInst := make(map[int64][]*scheduleModel.InstanceStaff, len(kept))
	staffIDs := make([]int64, 0, len(staffRows))
	for _, row := range staffRows {
		if row == nil {
			continue
		}
		staffByInst[row.InstanceID] = append(staffByInst[row.InstanceID], row)
		staffIDs = append(staffIDs, row.StaffID)
		if row.RoomID != nil {
			roomIDs = append(roomIDs, *row.RoomID)
		}
	}

	data := &betreuungsplanData{
		instances:   kept,
		staffByInst: staffByInst,
		staffNames:  s.staffNames(ctx, staffIDs),
		roomNames:   s.roomNames(ctx, roomIDs),
		childCounts: s.childCounts(ctx, instanceIDs),
		blockColors: s.blockColors(ctx, kept),
		closedDays:  s.nonWorkingDays(ctx, from, to),
		variant:     variant,
	}
	return data, nil
}

// blockColors resolves each block's Kategorie colour via its template. Both
// readers are optional and every failure degrades to "no colour": the bar is
// recognition, never information the sheet depends on.
func (s *service) blockColors(ctx context.Context, instances []*scheduleModel.ActivityInstance) map[int64]string {
	colors := map[int64]string{}
	if s.deps.ActivityGroups == nil || s.deps.Categories == nil {
		return colors
	}

	groupIDs := make([]int64, 0, len(instances))
	for _, instance := range instances {
		if instance.ActivityGroupID != nil {
			groupIDs = append(groupIDs, *instance.ActivityGroupID)
		}
	}
	if len(groupIDs) == 0 {
		return colors
	}

	groups, err := s.deps.ActivityGroups.FindByIDs(ctx, groupIDs)
	if err != nil {
		s.getLogger().Warn("plan export: activity group lookup failed", "error", err.Error())
		return colors
	}
	categoryByGroup := make(map[int64]int64, len(groups))
	for _, group := range groups {
		if group != nil {
			categoryByGroup[group.ID] = group.CategoryID
		}
	}

	categories, err := s.deps.Categories.ListAll(ctx)
	if err != nil {
		s.getLogger().Warn("plan export: category lookup failed", "error", err.Error())
		return colors
	}
	colorByCategory := make(map[int64]string, len(categories))
	for _, category := range categories {
		if category != nil {
			colorByCategory[category.ID] = category.Color
		}
	}

	for _, instance := range instances {
		if instance.ActivityGroupID == nil {
			continue
		}
		if color := colorByCategory[categoryByGroup[*instance.ActivityGroupID]]; color != "" {
			colors[instance.ID] = color
		}
	}
	return colors
}

// staffNames resolves the names of the assigned staff. Unlike the instance
// list endpoint (which omits names to keep its payload small) a printed plan
// is useless without them.
func (s *service) staffNames(ctx context.Context, staffIDs []int64) map[int64]string {
	names := map[int64]string{}
	if s.deps.Staff == nil || len(staffIDs) == 0 {
		return names
	}
	members, err := s.deps.Staff.FindWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		s.getLogger().Warn("plan export: staff name lookup failed", "error", err.Error())
		return names
	}
	for id, member := range members {
		names[id] = staffShortName(member)
	}
	return names
}

func staffShortName(member *usersModel.Staff) string {
	if member == nil || member.Person == nil {
		return "Unbekannt"
	}
	return shortName(member.Person.FirstName, member.Person.LastName)
}

func (s *service) roomNames(ctx context.Context, roomIDs []int64) map[int64]string {
	names := map[int64]string{}
	if s.deps.Rooms == nil || len(roomIDs) == 0 {
		return names
	}
	rooms, err := s.deps.Rooms.FindByIDs(ctx, roomIDs)
	if err != nil {
		s.getLogger().Warn("plan export: room lookup failed", "error", err.Error())
		return names
	}
	for _, room := range rooms {
		if room != nil {
			names[room.ID] = room.Name
		}
	}
	return names
}

func (s *service) childCounts(ctx context.Context, instanceIDs []int64) map[int64]int {
	counts := map[int64]int{}
	if s.deps.Students == nil || len(instanceIDs) == 0 {
		return counts
	}
	loaded, err := s.deps.Students.CountNonAbsentByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		s.getLogger().Warn("plan export: child count lookup failed", "error", err.Error())
		return counts
	}
	return loaded
}

// offeringKey identifies the row a block belongs to. Two separately
// configured Angebote may carry the same name — group titles are not unique —
// and merging them would print one row that is neither of them. Template-
// backed blocks are therefore keyed by their Angebot, and only spontaneous
// blocks (which have no Angebot to be identified by) fall back to their title,
// so a spontaneous block repeated through the week still reads as one row.
type offeringKey struct {
	groupID int64
	title   string
}

func keyFor(instance *scheduleModel.ActivityInstance, title string) offeringKey {
	if instance.ActivityGroupID != nil {
		return offeringKey{groupID: *instance.ActivityGroupID}
	}
	return offeringKey{title: title}
}

// rows builds one row per Angebot for the given week.
func (d *betreuungsplanData) rows(w week) []listexport.Row {
	dayIndex := make(map[timezone.Date]int, len(w.days))
	for i, day := range w.days {
		dayIndex[day] = i
	}

	type offering struct {
		key      offeringKey
		title    string
		earliest string
		days     [fullWeekDays][]*scheduleModel.ActivityInstance
	}
	offerings := map[offeringKey]*offering{}

	for _, instance := range d.instances {
		index, ok := dayIndex[instance.Date]
		if !ok {
			continue
		}
		title := strings.TrimSpace(instance.Title)
		if title == "" {
			title = "Ohne Titel"
		}
		key := keyFor(instance, title)
		row, exists := offerings[key]
		if !exists {
			row = &offering{key: key, earliest: "99:99"}
			offerings[key] = row
		}
		// The row is labelled by its earliest occurrence, so a single
		// occurrence renamed on the day does not rename the whole row.
		if clock := instance.StartTime.Format(clockLayout); clock < row.earliest {
			row.earliest = clock
			row.title = title
		}
		row.days[index] = append(row.days[index], instance)
	}

	ordered := make([]*offering, 0, len(offerings))
	for _, row := range offerings {
		ordered = append(ordered, row)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].earliest != ordered[j].earliest {
			return ordered[i].earliest < ordered[j].earliest
		}
		if ordered[i].title != ordered[j].title {
			return ordered[i].title < ordered[j].title
		}
		// Two Angebote of the same name at the same time still need a fixed
		// order, or the sheet swaps their rows between two exports.
		if ordered[i].key.groupID != ordered[j].key.groupID {
			return ordered[i].key.groupID < ordered[j].key.groupID
		}
		return ordered[i].key.title < ordered[j].key.title
	})

	rows := make([]listexport.Row, 0, len(ordered))
	for _, row := range ordered {
		cells := newDayCells(row.title, w)
		for i, day := range w.days {
			lines := d.closedDayLines(day)
			instances := row.days[i]
			sort.SliceStable(instances, func(a, b int) bool {
				return instances[a].StartTime.Before(instances[b].StartTime)
			})
			for _, instance := range instances {
				lines = append(lines, d.instanceLines(instance)...)
			}
			cells.days[i] = lines
		}
		rows = append(rows, cells.toRow())
	}
	return rows
}

// plannedDays are the days blocks run on, so a Saturday Angebot gets its own
// column instead of vanishing from the printed plan.
func (d *betreuungsplanData) plannedDays() map[timezone.Date]bool {
	days := make(map[timezone.Date]bool, len(d.instances))
	for _, instance := range d.instances {
		days[instance.Date] = true
	}
	return days
}

func (d *betreuungsplanData) emptyWeekRows(w week) []listexport.Row {
	return emptyWeekRows(w, d.closedDays)
}

func (d *betreuungsplanData) closedDayLines(day timezone.Date) []listexport.Line {
	if label, ok := d.closedDays[day]; ok {
		return []listexport.Line{strong(label)}
	}
	return nil
}

// instanceLines renders one block: window and room in strong weight, who
// runs it in normal, the head count and any note muted. Three ranks, so a
// reader finds the time, the people, or the size of the group without
// reading the whole cell.
func (d *betreuungsplanData) instanceLines(instance *scheduleModel.ActivityInstance) []listexport.Line {
	head := timeRange(instance.StartTime, instance.EndTime)
	if room := distinctRoom(instance.Title, d.roomNames[instance.RoomID]); room != "" {
		head += " · " + room
	}

	cancelled := instance.Status == scheduleModel.InstanceStatusCancelled
	if cancelled {
		// A cancelled block stays on the sheet — "entfällt" is the
		// information the reader came for. The reason is internal.
		head += " · entfällt"
		lines := []listexport.Line{accented(head, d.blockColors[instance.ID])}
		lines = d.appendNote(lines, "Grund", instance.CancelReason)
		lines = d.appendNote(lines, "Notiz", instance.Notes)
		return lines
	}

	lines := []listexport.Line{accented(head, d.blockColors[instance.ID])}

	if names := d.staffLine(instance.ID, instance.RoomID); names != "" {
		lines = append(lines, normal(names))
	}
	if count := d.childCounts[instance.ID]; count > 0 {
		lines = append(lines, muted(fmt.Sprintf("%d Kinder", count)))
	}
	lines = d.appendNote(lines, "Hinweis", instance.UnderstaffedNote)
	lines = d.appendNote(lines, "Notiz", instance.Notes)
	return lines
}

// appendNote adds a muted "Label: text" line, but only on the internal sheet:
// what somebody wrote about a day — why it was cancelled, who is short, what
// happened — is written for the team, not for a corridor wall.
func (d *betreuungsplanData) appendNote(lines []listexport.Line, label string, note *string) []listexport.Line {
	if !d.variant.internal() || note == nil {
		return lines
	}
	text := strings.TrimSpace(*note)
	if text == "" {
		return lines
	}
	return append(lines, muted(label+": "+text))
}

// staffLine lists who runs the block. Absent staff are dropped from the wall
// sheet (they are not there) and marked on the internal one.
func (d *betreuungsplanData) staffLine(instanceID, primaryRoomID int64) string {
	rows := d.staffByInst[instanceID]
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.IsAbsent && !d.variant.internal() {
			continue
		}
		name := d.staffNames[row.StaffID]
		if name == "" {
			name = "Unbekannt"
		}
		if row.IsSubstitute {
			name += " (Vertretung)"
		}
		if row.IsAbsent {
			name += " (abwesend)"
		}
		if row.RoomID != nil && *row.RoomID != primaryRoomID {
			if room := d.roomNames[*row.RoomID]; room != "" {
				name += " (" + room + ")"
			}
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "Kein Personal eingeteilt"
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
