package fixed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// Room data structure for seeding
type roomData struct {
	Name          string
	RoomNumber    string
	Building      string
	Floor         int
	Capacity      int
	RoomType      string
	IsAccessible  bool
	HasProjector  bool
	HasSmartboard bool
	Description   string
}

// seedRooms creates all facility rooms
func (s *Seeder) seedRooms(ctx context.Context) error {
	rooms := []roomData{
		// Main building - Classrooms (Ground Floor)
		{Name: "Klassenzimmer 1A", RoomNumber: "101", Building: "Hauptgebäude", Floor: 0, Capacity: 30, RoomType: "classroom", IsAccessible: true, HasProjector: true, HasSmartboard: true, Description: "Klassenzimmer für die 1. Klasse A"},
		{Name: "Klassenzimmer 1B", RoomNumber: "102", Building: "Hauptgebäude", Floor: 0, Capacity: 30, RoomType: "classroom", IsAccessible: true, HasProjector: true, HasSmartboard: true, Description: "Klassenzimmer für die 1. Klasse B"},
		{Name: "Klassenzimmer 2A", RoomNumber: "103", Building: "Hauptgebäude", Floor: 0, Capacity: 30, RoomType: "classroom", IsAccessible: true, HasProjector: true, HasSmartboard: false, Description: "Klassenzimmer für die 2. Klasse A"},

		// Main building - Classrooms (First Floor)
		{Name: "Klassenzimmer 2B", RoomNumber: "201", Building: "Hauptgebäude", Floor: 1, Capacity: 30, RoomType: "classroom", IsAccessible: false, HasProjector: true, HasSmartboard: false, Description: "Klassenzimmer für die 2. Klasse B"},
		{Name: "Klassenzimmer 3A", RoomNumber: "202", Building: "Hauptgebäude", Floor: 1, Capacity: 30, RoomType: "classroom", IsAccessible: false, HasProjector: true, HasSmartboard: true, Description: "Klassenzimmer für die 3. Klasse A"},
		{Name: "Klassenzimmer 3B", RoomNumber: "203", Building: "Hauptgebäude", Floor: 1, Capacity: 30, RoomType: "classroom", IsAccessible: false, HasProjector: true, HasSmartboard: true, Description: "Klassenzimmer für die 3. Klasse B"},

		// Main building - Science rooms
		{Name: "Naturwissenschaftsraum", RoomNumber: "204", Building: "Hauptgebäude", Floor: 1, Capacity: 25, RoomType: "laboratory", IsAccessible: false, HasProjector: true, HasSmartboard: true, Description: "Raum für naturwissenschaftlichen Unterricht"},
		{Name: "Forscherraum", RoomNumber: "205", Building: "Hauptgebäude", Floor: 1, Capacity: 20, RoomType: "laboratory", IsAccessible: false, HasProjector: true, HasSmartboard: false, Description: "Experimentierraum für junge Forscher"},

		// Main building - Special rooms
		{Name: "Computerraum", RoomNumber: "110", Building: "Hauptgebäude", Floor: 0, Capacity: 25, RoomType: "computer_lab", IsAccessible: true, HasProjector: true, HasSmartboard: true, Description: "Computerraum mit 25 Arbeitsplätzen"},
		{Name: "Bibliothek", RoomNumber: "111", Building: "Hauptgebäude", Floor: 0, Capacity: 40, RoomType: "library", IsAccessible: true, HasProjector: false, HasSmartboard: false, Description: "Schulbibliothek mit Leseecken"},
		{Name: "Musikraum", RoomNumber: "112", Building: "Hauptgebäude", Floor: 0, Capacity: 30, RoomType: "music_room", IsAccessible: true, HasProjector: true, HasSmartboard: false, Description: "Musikraum mit Instrumenten"},
		{Name: "Kunstraum", RoomNumber: "113", Building: "Hauptgebäude", Floor: 0, Capacity: 25, RoomType: "art_room", IsAccessible: true, HasProjector: true, HasSmartboard: false, Description: "Kunstraum mit Werkbänken und Waschbecken"},

		// Nebengebäude - Classrooms
		{Name: "Klassenzimmer 4A", RoomNumber: "301", Building: "Nebengebäude", Floor: 0, Capacity: 30, RoomType: "classroom", IsAccessible: true, HasProjector: true, HasSmartboard: true, Description: "Klassenzimmer für die 4. Klasse A"},
		{Name: "Klassenzimmer 4B", RoomNumber: "302", Building: "Nebengebäude", Floor: 0, Capacity: 30, RoomType: "classroom", IsAccessible: true, HasProjector: true, HasSmartboard: true, Description: "Klassenzimmer für die 4. Klasse B"},
		{Name: "Klassenzimmer 5A", RoomNumber: "303", Building: "Nebengebäude", Floor: 0, Capacity: 30, RoomType: "classroom", IsAccessible: true, HasProjector: true, HasSmartboard: false, Description: "Klassenzimmer für die 5. Klasse A"},
		{Name: "Klassenzimmer 5B", RoomNumber: "304", Building: "Nebengebäude", Floor: 0, Capacity: 30, RoomType: "classroom", IsAccessible: true, HasProjector: true, HasSmartboard: false, Description: "Klassenzimmer für die 5. Klasse B"},

		// Nebengebäude - Special rooms
		{Name: "Werkraum", RoomNumber: "305", Building: "Nebengebäude", Floor: 0, Capacity: 20, RoomType: "workshop", IsAccessible: true, HasProjector: false, HasSmartboard: false, Description: "Werkraum für handwerkliche Arbeiten"},
		{Name: "Töpferraum", RoomNumber: "306", Building: "Nebengebäude", Floor: 0, Capacity: 15, RoomType: "art_room", IsAccessible: true, HasProjector: false, HasSmartboard: false, Description: "Töpferraum mit Brennofen"},

		// Sports facilities
		{Name: "Sporthalle", RoomNumber: "GYM1", Building: "Sporthalle", Floor: 0, Capacity: 60, RoomType: "gymnasium", IsAccessible: true, HasProjector: false, HasSmartboard: false, Description: "Große Sporthalle für Sport und Bewegung"},
		{Name: "Kleine Sporthalle", RoomNumber: "GYM2", Building: "Sporthalle", Floor: 0, Capacity: 30, RoomType: "gymnasium", IsAccessible: true, HasProjector: false, HasSmartboard: false, Description: "Kleine Halle für Gymnastik und Tanz"},

		// OGS Building
		{Name: "OGS-Raum 1", RoomNumber: "OGS1", Building: "OGS-Gebäude", Floor: 0, Capacity: 35, RoomType: "activity_room", IsAccessible: true, HasProjector: true, HasSmartboard: false, Description: "Hauptraum für Nachmittagsbetreuung"},
		{Name: "OGS-Raum 2", RoomNumber: "OGS2", Building: "OGS-Gebäude", Floor: 0, Capacity: 25, RoomType: "activity_room", IsAccessible: true, HasProjector: false, HasSmartboard: false, Description: "Ruheraum mit Sofaecke"},
		{Name: "Mensa", RoomNumber: "MENSA", Building: "OGS-Gebäude", Floor: 0, Capacity: 120, RoomType: "cafeteria", IsAccessible: true, HasProjector: true, HasSmartboard: false, Description: "Mensa für Mittagessen"},

		// Administrative
		{Name: "Lehrerzimmer", RoomNumber: "STAFF", Building: "Hauptgebäude", Floor: 0, Capacity: 40, RoomType: "staff_room", IsAccessible: true, HasProjector: true, HasSmartboard: true, Description: "Lehrerzimmer mit Arbeitsplätzen"},

		// Outdoor areas
		{Name: "Schulhof", RoomNumber: "OUTDOOR", Building: "Außenbereich", Floor: 0, Capacity: 100, RoomType: "schulhof", IsAccessible: true, HasProjector: false, HasSmartboard: false, Description: "Schulhof für Freispiel und Pausen"},
	}

	for _, data := range rooms {
		// Map RoomType to Category
		category := mapRoomTypeToCategory(data.RoomType)

		room := &facilities.Room{
			Name:     data.Name,
			Building: data.Building,
			Floor:    data.Floor,
			Capacity: data.Capacity,
			Category: category,
			Color:    getRoomColor(data.RoomType),
		}
		room.CreatedAt = time.Now()
		room.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(room).ModelTableExpr("facilities.rooms").Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create room %s: %w", data.Name, err)
		}

		s.result.Rooms = append(s.result.Rooms, room)
		s.result.RoomByID[room.ID] = room
	}

	if s.verbose {
		log.Printf("Created %d rooms", len(s.result.Rooms))
	}

	return nil
}

// mapRoomTypeToCategory maps the old room type to the new category field
func mapRoomTypeToCategory(roomType string) string {
	switch roomType {
	case "classroom":
		return "Classroom"
	case "laboratory":
		return "Laboratory"
	case "computer_lab":
		return "Computer Lab"
	case "library":
		return "Library"
	case "music_room":
		return "Music Room"
	case "art_room":
		return "Art Room"
	case "workshop":
		return "Workshop"
	case "gymnasium":
		return "Sports"
	case "activity_room":
		return "Activity Room"
	case "cafeteria":
		return "Cafeteria"
	case "staff_room":
		return "Staff Room"
	case "schulhof":
		return "Schulhof"
	default:
		return "Other"
	}
}

// getRoomColor returns a color based on room type
func getRoomColor(roomType string) string {
	switch roomType {
	case "classroom":
		return "#4A90E2" // Blue
	case "laboratory":
		return "#7ED321" // Green
	case "computer_lab":
		return "#9013FE" // Purple
	case "library":
		return "#F5A623" // Orange
	case "music_room":
		return "#BD10E0" // Magenta
	case "art_room":
		return "#50E3C2" // Turquoise
	case "workshop":
		return "#B8E986" // Light Green
	case "gymnasium":
		return "#F8E71C" // Yellow
	case "activity_room":
		return "#417505" // Dark Green
	case "cafeteria":
		return "#D0021B" // Red
	case "staff_room":
		return "#9B9B9B" // Gray
	case "schulhof":
		return "#7ED321" // Green (outdoor/nature)
	default:
		return "#FFFFFF" // White
	}
}
