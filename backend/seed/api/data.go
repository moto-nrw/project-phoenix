package api

// DemoRoom represents a room to be created via API
type DemoRoom struct {
	Name     string
	Type     string
	Capacity int
}

// DemoStaffMember represents a staff member to be created via API
type DemoStaffMember struct {
	FirstName string
	LastName  string
	Role      string // "Lead" or "Supervisor"
}

// DemoStudent represents a student to be created via API
type DemoStudent struct {
	FirstName string
	LastName  string
	Class     string // "1a", "2b", "3c"
}

// DemoActivity represents an activity to be created via API
type DemoActivity struct {
	Name         string
	DefaultRoom  string // Name of room to assign
	DurationMins int    // Default session duration in minutes
}

// DemoDevice represents an IoT device to be created via API
type DemoDevice struct {
	DeviceID string
	Name     string
}

// RuntimeConfig configures the runtime snapshot creation
type RuntimeConfig struct {
	ActiveSessions    int // Number of activity sessions to start (3-4)
	CheckedInStudents int // Number of students to check in (~30)
	StudentsUnterwegs int // Students "on the way" (1-2)
}

// DemoRooms defines the 6 rooms for the demo environment
var DemoRooms = []DemoRoom{
	{Name: "OGS-Raum 1", Type: "activity_room", Capacity: 25},
	{Name: "OGS-Raum 2", Type: "activity_room", Capacity: 20},
	{Name: "Mensa", Type: "cafeteria", Capacity: 80},
	{Name: "Sporthalle", Type: "sports", Capacity: 40},
	{Name: "Kreativraum", Type: "activity_room", Capacity: 20},
	{Name: "Schulhof", Type: "outdoor", Capacity: 100},
}

// DemoStaff defines the 7 staff members for the demo environment
var DemoStaff = []DemoStaffMember{
	{FirstName: "Anna", LastName: "Müller", Role: "Lead"},
	{FirstName: "Thomas", LastName: "Weber", Role: "Supervisor"},
	{FirstName: "Sarah", LastName: "Schmidt", Role: "Supervisor"},
	{FirstName: "Michael", LastName: "Hoffmann", Role: "Supervisor"},
	{FirstName: "Lisa", LastName: "Wagner", Role: "Supervisor"},
	{FirstName: "Jan", LastName: "Becker", Role: "Supervisor"},
	{FirstName: "Maria", LastName: "Fischer", Role: "Supervisor"},
}

// DemoStudents defines the 45 students across 3 classes (15 each)
var DemoStudents = []DemoStudent{
	// Klasse 1a (15 students)
	{FirstName: "Felix", LastName: "Schneider", Class: "1a"},
	{FirstName: "Emma", LastName: "Meyer", Class: "1a"},
	{FirstName: "Leon", LastName: "Koch", Class: "1a"},
	{FirstName: "Mia", LastName: "Bauer", Class: "1a"},
	{FirstName: "Noah", LastName: "Richter", Class: "1a"},
	{FirstName: "Hannah", LastName: "Klein", Class: "1a"},
	{FirstName: "Paul", LastName: "Wolf", Class: "1a"},
	{FirstName: "Lina", LastName: "Schröder", Class: "1a"},
	{FirstName: "Lukas", LastName: "Neumann", Class: "1a"},
	{FirstName: "Sophie", LastName: "Schwarz", Class: "1a"},
	{FirstName: "Jonas", LastName: "Zimmermann", Class: "1a"},
	{FirstName: "Emilia", LastName: "Braun", Class: "1a"},
	{FirstName: "Ben", LastName: "Krüger", Class: "1a"},
	{FirstName: "Lena", LastName: "Hofmann", Class: "1a"},
	{FirstName: "Tim", LastName: "Hartmann", Class: "1a"},

	// Klasse 2b (15 students)
	{FirstName: "Maximilian", LastName: "Schmitt", Class: "2b"},
	{FirstName: "Laura", LastName: "Werner", Class: "2b"},
	{FirstName: "David", LastName: "Krause", Class: "2b"},
	{FirstName: "Anna", LastName: "Meier", Class: "2b"},
	{FirstName: "Simon", LastName: "Lange", Class: "2b"},
	{FirstName: "Julia", LastName: "Schulz", Class: "2b"},
	{FirstName: "Moritz", LastName: "König", Class: "2b"},
	{FirstName: "Marie", LastName: "Walter", Class: "2b"},
	{FirstName: "Niklas", LastName: "Huber", Class: "2b"},
	{FirstName: "Clara", LastName: "Herrmann", Class: "2b"},
	{FirstName: "Jan", LastName: "Peters", Class: "2b"},
	{FirstName: "Sophia", LastName: "Lang", Class: "2b"},
	{FirstName: "Erik", LastName: "Möller", Class: "2b"},
	{FirstName: "Lea", LastName: "Beck", Class: "2b"},
	{FirstName: "Finn", LastName: "Jung", Class: "2b"},

	// Klasse 3c (15 students)
	{FirstName: "Anton", LastName: "Keller", Class: "3c"},
	{FirstName: "Charlotte", LastName: "Berger", Class: "3c"},
	{FirstName: "Henri", LastName: "Fuchs", Class: "3c"},
	{FirstName: "Amelie", LastName: "Vogel", Class: "3c"},
	{FirstName: "Leonard", LastName: "Roth", Class: "3c"},
	{FirstName: "Johanna", LastName: "Frank", Class: "3c"},
	{FirstName: "Elias", LastName: "Baumann", Class: "3c"},
	{FirstName: "Isabella", LastName: "Graf", Class: "3c"},
	{FirstName: "Matteo", LastName: "Kaiser", Class: "3c"},
	{FirstName: "Nele", LastName: "Pfeiffer", Class: "3c"},
	{FirstName: "Theo", LastName: "Sommer", Class: "3c"},
	{FirstName: "Frieda", LastName: "Brandt", Class: "3c"},
	{FirstName: "Oscar", LastName: "Vogt", Class: "3c"},
	{FirstName: "Greta", LastName: "Engel", Class: "3c"},
	{FirstName: "Jakob", LastName: "Stein", Class: "3c"},
}

// DemoActivities defines the 10 activities with room assignments
var DemoActivities = []DemoActivity{
	{Name: "Hausaufgaben", DefaultRoom: "OGS-Raum 1", DurationMins: 60},
	{Name: "Fußball", DefaultRoom: "Sporthalle", DurationMins: 90},
	{Name: "Basteln", DefaultRoom: "Kreativraum", DurationMins: 75},
	{Name: "Kochen", DefaultRoom: "Mensa", DurationMins: 120},
	{Name: "Lesen", DefaultRoom: "OGS-Raum 2", DurationMins: 45},
	{Name: "Musik", DefaultRoom: "OGS-Raum 1", DurationMins: 60},
	{Name: "Tanzen", DefaultRoom: "Sporthalle", DurationMins: 60},
	{Name: "Schach", DefaultRoom: "OGS-Raum 2", DurationMins: 45},
	{Name: "Garten", DefaultRoom: "Schulhof", DurationMins: 90},
	{Name: "Freispiel", DefaultRoom: "Schulhof", DurationMins: 60},
}

// DemoDevices defines IoT devices for check-in/check-out
var DemoDevices = []DemoDevice{
	{DeviceID: "demo-device-001", Name: "Main Entrance Scanner"},
}

// DefaultRuntimeConfig provides default values for runtime snapshot creation
var DefaultRuntimeConfig = RuntimeConfig{
	ActiveSessions:    4,  // Start 4 activity sessions
	CheckedInStudents: 32, // Check in 32 of 45 students
	StudentsUnterwegs: 2,  // Leave 2 students "on the way"
}
