package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The everyday life of the marketing school at the reference clock (#3762):
// rooms in use, the present children inside running supervisions, unread
// messages for the school admin, a notice for the team, and news plus a
// staff reply for the families. The shot list (#3764) adds an open pickup
// change and an excused absence from parents, and the week's meal plan.
// Everything goes through the same HTTP endpoints the portals use.

type marketingRoom struct {
	key      string
	name     string
	category string
	color    string
	capacity int
}

// marketingSession is a planned supervision that runs at the reference clock.
// Its caregiver leads the group the children come from.
type marketingSession struct {
	title    string
	category string // one of the default activity categories
	room     string
	group    string
	leader   string
}

// marketingRooms take colors from the demo room palette: the API rejects the
// colors reserved for presence states.
func marketingRooms() []marketingRoom {
	return []marketingRoom{
		{key: "bauraum", name: "Bauraum", category: "Spielen", color: "#E65100", capacity: 12},
		{key: "turnhalle", name: "Turnhalle", category: "Bewegen", color: "#283593", capacity: 25},
		{key: "kreativraum", name: "Kreativraum", category: "Kreativ", color: "#2E7D32", capacity: 14},
		{key: "leseecke", name: "Leseecke", category: "Ruhe", color: "#006064", capacity: 8},
	}
}

func marketingSessions() []marketingSession {
	return []marketingSession{
		{title: "Bauecke", category: "Spiele", room: "bauraum", group: "fuchsbau", leader: "Miriam Sommer"},
		{title: "Fußball", category: "Sport", room: "turnhalle", group: "eulennest", leader: "Jonas Albrecht"},
	}
}

const (
	marketingTeamMessage    = "Kannst du um 14 Uhr die Hausaufgabenbetreuung im Fuchsbau übernehmen? Ich bin beim Elterngespräch."
	marketingParentQuestion = "Lina wird heute um 15 Uhr von ihrer Oma abgeholt. Ist das in Ordnung?"
	marketingParentMessage  = "Emir hat heute seine Turnschuhe vergessen. Kann er trotzdem beim Fußball mitmachen?"
	marketingStaffReply     = "Kein Problem, wir haben Ersatzschuhe in der Turnhalle. Viele Grüße, Jonas Albrecht"
	marketingNoticeTitle    = "Ausflug zur Stadtbücherei"
	marketingNewsTitle      = "Herbstfest am Freitag"
)

// marketingMeals is the week's lunch, Monday to Friday.
var marketingMeals = [5][]map[string]any{
	{{"dish": "Gemüselasagne"}, {"dish": "Apfelkompott"}},
	{{"dish": "Hähnchen mit Reis und Erbsen", "note": "Vegetarisch: Gemüsebratling"}},
	{{"dish": "Kartoffelsuppe mit Brötchen"}, {"dish": "Joghurt mit Beeren"}},
	{{"dish": "Spaghetti Bolognese", "note": "Vegetarisch: Linsen-Bolognese"}},
	{{"dish": "Fischstäbchen mit Kartoffelpüree", "note": "Vegetarisch: Gemüsestäbchen"}, {"dish": "Obst"}},
}

func seedMarketingDailyLife(ctx context.Context, rt *Runtime, data manualProfileData, admin AccountCredentials, staff []AccountCredentials, parents []ParentCredentials) error {
	rt.Client.BindAuth(rt.TenantAuth)
	defer rt.Client.BindAuth(rt.TenantAuth)
	rooms := make(map[string]int64, len(marketingRooms()))
	for _, room := range marketingRooms() {
		raw, err := rt.Client.Post("/api/rooms", map[string]any{
			"name": room.name, "category": room.category, "color": room.color, "capacity": room.capacity,
		})
		if err != nil {
			return fmt.Errorf("create marketing room %s: %w", room.name, err)
		}
		id, err := decodeSeedEntityID(raw)
		if err != nil {
			return fmt.Errorf("decode marketing room %s: %w", room.name, err)
		}
		rooms[room.key] = id
	}
	if err := seedMarketingSessions(rt, data, staff, rooms); err != nil {
		return err
	}
	if err := seedMarketingNotice(rt); err != nil {
		return err
	}
	if err := seedMarketingNews(rt); err != nil {
		return err
	}
	if err := seedMarketingTeamMessage(ctx, rt, admin, staff); err != nil {
		return err
	}
	if err := seedMarketingParentMessages(ctx, rt, data, parents); err != nil {
		return err
	}
	if err := seedMarketingParentRequests(ctx, rt, data, parents); err != nil {
		return err
	}
	if err := seedMarketingAppointments(rt); err != nil {
		return err
	}
	return seedMarketingMealPlan(rt)
}

// marketingNewsFriday is the Friday the news item announces: the first
// Friday after today.
func marketingNewsFriday(today seedDate) seedDate {
	day := today.AddDays(1)
	for day.Weekday() != time.Friday {
		day = day.AddDays(1)
	}
	return day
}

// seedMarketingAppointments puts a team meeting into today's calendar week
// and the autumn festival of the news item into the families' calendar,
// where they can accept it.
func seedMarketingAppointments(rt *Runtime) error {
	today := todaySeedDate().String()
	friday := marketingNewsFriday(todaySeedDate()).String()
	appointments := []map[string]any{
		{
			"title": "Teambesprechung", "location": "Kreativraum",
			"description": "Wochenrückblick und Planung für das Herbstfest.",
			"start_date":  today, "end_date": today, "start_time": "13:30", "end_time": "14:15", "all_day": false,
			"delivery_mode": "informational", "targets": []map[string]any{{"type": "all_staff"}}, "send_email": false,
		},
		{
			"title": marketingNewsTitle, "location": "Schulhof",
			"description": "Die Kinder haben Lieder und Spiele vorbereitet. Wir freuen uns auf Sie!",
			"start_date":  friday, "end_date": friday, "start_time": "15:00", "end_time": "18:00", "all_day": false,
			"delivery_mode": "rsvp_required", "overview_visibility": "all",
			"targets":    []map[string]any{{"type": "all_school_parents"}, {"type": "all_staff"}},
			"send_email": false,
		},
	}
	for _, appointment := range appointments {
		if _, err := rt.Client.Post("/api/calendar/appointments", appointment); err != nil {
			return fmt.Errorf("create marketing appointment %s: %w", appointment["title"], err)
		}
	}
	return nil
}

// marketingNextWeekday is the first Monday-to-Friday day after today.
func marketingNextWeekday(today seedDate) seedDate {
	day := today.AddDays(1)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDays(1)
	}
	return day
}

// seedMarketingParentRequests files two requests for the next weekday from
// the parents portal. The pickup change stays open, so the request inbox has
// work and the parents-portal account sees its pending request; the excused
// absence fills the absence list without changing today's presence.
func seedMarketingParentRequests(ctx context.Context, rt *Runtime, data manualProfileData, parents []ParentCredentials) error {
	byKey := make(map[string]ParentCredentials, len(parents))
	for _, parent := range parents {
		byKey[parent.Key] = parent
	}
	day := marketingNextWeekday(todaySeedDate()).String()
	requests := []struct {
		parent, child, kind string
		body                map[string]any
	}{
		{parent: "sarah-yilmaz", child: "elif-yilmaz", kind: "care-exception", body: map[string]any{
			"date": day, "pickup_time": "14:30", "reason": "Elif hat einen Termin beim Kinderarzt.",
		}},
		{parent: "julia-wagner", child: "mia-wagner", kind: "sick-note", body: map[string]any{
			"dates": []string{day}, "status": "excused", "reason": "Mia fährt mit zum Familienfest nach Hamburg.",
		}},
	}
	for _, request := range requests {
		parent, ok := byKey[request.parent]
		if !ok {
			return fmt.Errorf("marketing parent %s missing", request.parent)
		}
		auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
		if err != nil {
			return fmt.Errorf("marketing parent request login %s: %w", parent.Key, err)
		}
		path := fmt.Sprintf("/parent/me/children/%d/%s", data.students[request.child].ID, request.kind)
		if _, err := rt.Client.PostWithAuth(auth, path, request.body); err != nil {
			return fmt.Errorf("file marketing %s for %s: %w", request.kind, request.child, err)
		}
	}
	return nil
}

// seedMarketingMealPlan publishes lunch for every weekday of the current
// week, the week the tenant meal plan and the parents portal open on.
func seedMarketingMealPlan(rt *Runtime) error {
	today := todaySeedDate()
	monday := today.AddDays(-(int(today.Weekday()) + 6) % 7)
	for offset, dishes := range marketingMeals {
		day := monday.AddDays(offset).String()
		if _, err := rt.Client.Put("/api/meal-plan/"+day, map[string]any{"dishes": dishes}); err != nil {
			return fmt.Errorf("seed marketing meal plan %s: %w", day, err)
		}
	}
	return nil
}

// seedMarketingSessions plans each supervision around the reference clock
// with the present children of its group, starts it, and checks the
// children in, so the rooms are occupied and the day plan shows "Läuft".
func seedMarketingSessions(rt *Runtime, data manualProfileData, staff []AccountCredentials, rooms map[string]int64) error {
	// Planned blocks need the school's calendar periods; the bootstrap
	// creates the current school year the way the planning page does.
	if _, err := rt.Client.Post("/api/timetable/periods/bootstrap", nil); err != nil {
		return fmt.Errorf("bootstrap marketing calendar periods: %w", err)
	}
	categories, err := marketingActivityCategories(rt)
	if err != nil {
		return err
	}
	staffByName := make(map[string]AccountCredentials, len(staff))
	for _, member := range staff {
		staffByName[member.Name] = member
	}
	for _, session := range marketingSessions() {
		leader, ok := staffByName[session.leader]
		if !ok {
			return fmt.Errorf("marketing session %s: caregiver %s missing", session.title, session.leader)
		}
		// Each block belongs to its own activity; without one, every block
		// lands in the shared "Spontane Aktivität" and the running groups
		// share one name.
		activityRaw, err := rt.Client.Post("/api/activities", map[string]any{
			"name": session.title, "max_participants": 20, "is_open": true,
			"category_id": categories[session.category], "planned_room_id": rooms[session.room],
		})
		if err != nil {
			return fmt.Errorf("create marketing activity %s: %w", session.title, err)
		}
		activityID, err := decodeSeedEntityID(activityRaw)
		if err != nil {
			return fmt.Errorf("decode marketing activity %s: %w", session.title, err)
		}
		children := marketingSessionChildren(data, session.group)
		raw, err := rt.Client.Post("/api/timetable/instances", map[string]any{
			"date": todaySeedDate().String(), "start_time": marketingClock(-45), "end_time": marketingClock(75),
			"title": session.title, "room_id": rooms[session.room], "activity_group_id": activityID,
			"staff_ids": []int64{leader.StaffID}, "student_ids": children,
		})
		if err != nil {
			return fmt.Errorf("plan marketing session %s: %w", session.title, err)
		}
		instanceID, err := parseEnvelopeStringID(raw)
		if err != nil {
			return fmt.Errorf("decode marketing session %s: %w", session.title, err)
		}
		if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/start", instanceID), nil); err != nil {
			return fmt.Errorf("start marketing session %s: %w", session.title, err)
		}
		for _, studentID := range children {
			path := fmt.Sprintf("/api/timetable/operations/instances/%d/students/%d/check-in", instanceID, studentID)
			if _, err := rt.Client.Post(path, map[string]any{}); err != nil {
				return fmt.Errorf("check student %d into marketing session %s: %w", studentID, session.title, err)
			}
		}
	}
	return nil
}

// marketingActivityCategories maps the school's activity category names to
// their ids.
func marketingActivityCategories(rt *Runtime) (map[string]int64, error) {
	raw, err := rt.Client.Get("/api/activities/categories")
	if err != nil {
		return nil, fmt.Errorf("read marketing activity categories: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode marketing activity categories: %w", err)
	}
	categories := make(map[string]int64, len(envelope.Data))
	for _, category := range envelope.Data {
		categories[category.Name] = category.ID
	}
	for _, session := range marketingSessions() {
		if categories[session.category] == 0 {
			return nil, fmt.Errorf("marketing activity category %s missing", session.category)
		}
	}
	return categories, nil
}

// marketingSessionChildren lists the present children of a group.
func marketingSessionChildren(data manualProfileData, group string) []int64 {
	ids := []int64{}
	for _, family := range marketingFamilies() {
		for _, source := range family.children {
			if source.group == group && source.presence == marketingPresent {
				ids = append(ids, data.students[semanticKey(source.firstName+" "+source.lastName)].ID)
			}
		}
	}
	return ids
}

func seedMarketingNotice(rt *Runtime) error {
	_, err := rt.Client.Post("/api/staff-notices/", map[string]any{
		"title":    marketingNoticeTitle,
		"body":     "Die Gruppe Eulennest geht um 14:30 Uhr los. Bitte die Warnwesten mitnehmen.",
		"priority": "info", "audience": "all", "valid_from": todaySeedDate().String(),
		"weekdays": []int16{}, "week_pattern": 0, "requires_acknowledgement": false, "active": true,
	})
	if err != nil {
		return fmt.Errorf("create marketing staff notice: %w", err)
	}
	return nil
}

// seedMarketingNews publishes one announcement to all families. Nobody
// reads it, so the parents portal shows it as new.
func seedMarketingNews(rt *Runtime) error {
	raw, err := rt.Client.Post("/api/parent-announcements/", map[string]any{
		"title":          marketingNewsTitle,
		"body":           "Liebe Eltern, am Freitag feiern wir ab 15 Uhr unser Herbstfest im Schulhof. Die Kinder haben Lieder und Spiele vorbereitet. Wir freuen uns auf Sie!",
		"priority":       "info",
		"email_audience": "portal_only",
		"targets":        []map[string]any{{"target_type": "school_all"}},
	})
	if err != nil {
		return fmt.Errorf("create marketing news: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse marketing news: %w", err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/parent-announcements/%d/publish", id), nil); err != nil {
		return fmt.Errorf("publish marketing news: %w", err)
	}
	return nil
}

// seedMarketingTeamMessage lets the first caregiver write to the school
// admin. The admin never opens it, so the home page counts it as unread.
func seedMarketingTeamMessage(ctx context.Context, rt *Runtime, admin AccountCredentials, staff []AccountCredentials) error {
	if len(staff) == 0 {
		return fmt.Errorf("marketing team message needs a caregiver")
	}
	sender := staff[0]
	auth, err := rt.Adapter.LoginTenant(ctx, sender.Email, sender.Password, rt.Bootstrap.TenantSlug)
	if err != nil {
		return fmt.Errorf("marketing team message login %s: %w", sender.Key, err)
	}
	raw, err := rt.Client.PostWithAuth(auth, "/api/staff-messages/threads/open", map[string]any{
		"account_id": strconv.FormatInt(admin.AccountID, 10),
	})
	if err != nil {
		return fmt.Errorf("open marketing team conversation: %w", err)
	}
	threadID, err := threadIDFromResponse(raw)
	if err != nil {
		return fmt.Errorf("decode marketing team conversation: %w", err)
	}
	if _, err := rt.Client.PostWithAuth(auth, "/api/staff-messages/threads/"+threadID, map[string]any{"body": marketingTeamMessage}); err != nil {
		return fmt.Errorf("send marketing team message: %w", err)
	}
	return nil
}

// seedMarketingParentMessages writes two parent messages. The team answers
// the first, so that family sees a reply; the second stays open for the
// school, so the home page shows an unread parent message.
func seedMarketingParentMessages(ctx context.Context, rt *Runtime, data manualProfileData, parents []ParentCredentials) error {
	byKey := make(map[string]ParentCredentials, len(parents))
	for _, parent := range parents {
		byKey[parent.Key] = parent
	}
	messages := []struct {
		parent, child, body string
		answer              bool
	}{
		{parent: "sarah-yilmaz", child: "emir-yilmaz", body: marketingParentMessage, answer: true},
		{parent: "thomas-becker", child: "lina-becker", body: marketingParentQuestion},
	}
	for _, message := range messages {
		parent, ok := byKey[message.parent]
		if !ok {
			return fmt.Errorf("marketing parent %s missing", message.parent)
		}
		auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
		if err != nil {
			return fmt.Errorf("marketing parent message login %s: %w", parent.Key, err)
		}
		student := data.students[message.child]
		raw, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/messages/children/%d", student.ID), map[string]any{"body": message.body})
		if err != nil {
			return fmt.Errorf("send marketing parent message %s: %w", parent.Key, err)
		}
		if !message.answer {
			continue
		}
		threadID, messageID, err := parseThreadMessageIDs(raw)
		if err != nil {
			return fmt.Errorf("parse marketing parent conversation %s: %w", parent.Key, err)
		}
		if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/messages/threads/%d", threadID), map[string]any{
			"body": marketingStaffReply, "handled_up_to_message_id": strconv.FormatInt(messageID, 10),
		}); err != nil {
			return fmt.Errorf("reply to marketing parent %s: %w", parent.Key, err)
		}
	}
	return nil
}

// verifyMarketingDailyLife reads back what the reference clock should show:
// every present child sits in an occupied room, and both supervisions run
// with children on the day plan.
func verifyMarketingDailyLife(rt *Runtime) error {
	raw, err := rt.Client.Get("/api/students?page=1&page_size=100")
	if err != nil {
		return fmt.Errorf("read marketing children: %w", err)
	}
	var students struct {
		Data []struct {
			ID       int64  `json:"id"`
			Location string `json:"current_location"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &students); err != nil {
		return fmt.Errorf("decode marketing children: %w", err)
	}
	inRoom := 0
	for _, student := range students.Data {
		if strings.HasPrefix(student.Location, "Anwesend - ") {
			inRoom++
		}
	}
	if want := marketingProfileDefinition().Expected.PresentStudents; inRoom != want {
		return fmt.Errorf("marketing school: expected %d children in rooms, got %d", want, inRoom)
	}
	raw, err = rt.Client.Get("/api/timetable/operations/planned-now?scope=day")
	if err != nil {
		return fmt.Errorf("read marketing day plan: %w", err)
	}
	var plan struct {
		Data struct {
			Instances []struct {
				Title   string `json:"title"`
				Status  string `json:"status"`
				Present int    `json:"present_students_count"`
			} `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		return fmt.Errorf("decode marketing day plan: %w", err)
	}
	present := map[string]int{}
	for _, instance := range plan.Data.Instances {
		if instance.Status == "active" {
			present[instance.Title] = instance.Present
		}
	}
	for _, session := range marketingSessions() {
		if present[session.title] == 0 {
			return fmt.Errorf("marketing session %s is not running with children: %v", session.title, present)
		}
	}
	return nil
}
