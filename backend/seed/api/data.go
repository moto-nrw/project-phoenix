package api

// DemoRoom represents a room to be created via API
type DemoRoom struct {
	Name     string
	Category string // German category name for display
	Capacity int
	Building string // Building name: "Hauptgebäude", "Sporthalle", "Außenbereich"
	Floor    *int   // Floor number: 0=EG, 1=1.OG, 2=2.OG, nil for outdoor
}

// DemoStaffMember represents a staff member to be created via API
type DemoStaffMember struct {
	FirstName string
	LastName  string
	Position  string // MUST match frontend dropdown: "Pädagogische Fachkraft", "OGS-Büro", "Extern"
	IsTeacher bool   // Whether to create a teacher record
}

// DemoStudent represents a student to be created via API
type DemoStudent struct {
	FirstName string
	LastName  string
	Class     string // School class like "Klasse 1a", "Klasse 2b"
	GroupKey  string // OGS group key for lookup: "sternengruppe", "bärengruppe", "sonnengruppe"
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

// Floor helper for creating floor pointers
func floor(f int) *int { return &f }

// DemoRooms defines the 12 rooms for the demo environment
// Categories MUST match frontend dropdown in rooms.config.tsx:
// "Normaler Raum", "Gruppenraum", "Themenraum", "Sport"
// Building layout:
//
//	Hauptgebäude: EG (Mensa, Aula, OGS-1), 1.OG (OGS-2, OGS-3, Kreativ, Musik), 2.OG (Werk, Lese)
//	Sporthalle: EG (Sporthalle, Bewegungsraum)
//	Außenbereich: Schulhof (no floor)
var DemoRooms = []DemoRoom{
	// Hauptgebäude - Erdgeschoss (Ground Floor)
	{Name: "OGS-Raum 1", Category: "Gruppenraum", Capacity: 25, Building: "Hauptgebäude", Floor: floor(0)},
	{Name: "Mensa", Category: "Normaler Raum", Capacity: 80, Building: "Hauptgebäude", Floor: floor(0)},
	{Name: "Aula", Category: "Normaler Raum", Capacity: 120, Building: "Hauptgebäude", Floor: floor(0)},
	// Hauptgebäude - 1. Obergeschoss (First Floor)
	{Name: "OGS-Raum 2", Category: "Gruppenraum", Capacity: 20, Building: "Hauptgebäude", Floor: floor(1)},
	{Name: "OGS-Raum 3", Category: "Gruppenraum", Capacity: 18, Building: "Hauptgebäude", Floor: floor(1)},
	{Name: "Kreativraum", Category: "Themenraum", Capacity: 20, Building: "Hauptgebäude", Floor: floor(1)},
	{Name: "Musikraum", Category: "Themenraum", Capacity: 15, Building: "Hauptgebäude", Floor: floor(1)},
	// Hauptgebäude - 2. Obergeschoss (Second Floor)
	{Name: "Werkraum", Category: "Themenraum", Capacity: 12, Building: "Hauptgebäude", Floor: floor(2)},
	{Name: "Leseecke", Category: "Themenraum", Capacity: 10, Building: "Hauptgebäude", Floor: floor(2)},
	// Sporthalle - Separate Building
	{Name: "Sporthalle", Category: "Sport", Capacity: 40, Building: "Sporthalle", Floor: floor(0)},
	{Name: "Bewegungsraum", Category: "Sport", Capacity: 15, Building: "Sporthalle", Floor: floor(0)},
	// Außenbereich - Outdoor (no floor)
	{Name: "Schulhof", Category: "Normaler Raum", Capacity: 100, Building: "Außenbereich", Floor: nil},
}

// DemoStaff defines staff members for the demo environment
// Position must match frontend dropdown values in teacher-form.tsx and invitation-form.tsx
// Demo accounts: 5 OGS-Büro (admin) + 10 Pädagogische Fachkraft (betreuer) + 5 Extern (extern)
// All use email: demo{n}@mail.de and password: sdlXK26%
var DemoStaff = []DemoStaffMember{
	// 5 OGS-Büro accounts (admin role) - Office/management staff
	{FirstName: "Anna", LastName: "Müller", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Thomas", LastName: "Schmidt", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Sabine", LastName: "Weber", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Michael", LastName: "Fischer", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Claudia", LastName: "Wagner", Position: "OGS-Büro", IsTeacher: true},
	// 10 Pädagogische Fachkraft accounts (betreuer role) - Childcare professionals
	{FirstName: "Julia", LastName: "Klein", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Markus", LastName: "Wolf", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Sandra", LastName: "Schröder", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Christian", LastName: "Neumann", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Nicole", LastName: "Schwarz", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Frank", LastName: "Zimmermann", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Birgit", LastName: "Braun", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Jörg", LastName: "Krüger", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Heike", LastName: "Hartmann", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Uwe", LastName: "Lange", Position: "Pädagogische Fachkraft", IsTeacher: true},
	// 5 Extern accounts (extern role) - External instructors/contractors
	{FirstName: "Stefan", LastName: "Becker", Position: "Extern", IsTeacher: true},
	{FirstName: "Monika", LastName: "Hoffmann", Position: "Extern", IsTeacher: true},
	{FirstName: "Andreas", LastName: "Schulz", Position: "Extern", IsTeacher: true},
	{FirstName: "Petra", LastName: "Koch", Position: "Extern", IsTeacher: true},
	{FirstName: "Martin", LastName: "Richter", Position: "Extern", IsTeacher: true},
}

// DemoStudents defines the 100 students across 10 groups (10 each)
// OGS groups contain students from multiple school classes (realistic scenario)
// Each group has a dedicated Betreuer (Pädagogische Fachkraft) assigned
var DemoStudents = []DemoStudent{
	// Sternengruppe (10 students) - Betreuer: Julia Klein - Mix of Klasse 1a, 1b
	{FirstName: "Felix", LastName: "Schneider", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Emma", LastName: "Meyer", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Leon", LastName: "Koch", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Mia", LastName: "Bauer", Class: "Klasse 1b", GroupKey: "sternengruppe"},
	{FirstName: "Noah", LastName: "Richter", Class: "Klasse 1b", GroupKey: "sternengruppe"},
	{FirstName: "Hannah", LastName: "Klein", Class: "Klasse 1b", GroupKey: "sternengruppe"},
	{FirstName: "Paul", LastName: "Wolf", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Lina", LastName: "Schröder", Class: "Klasse 1b", GroupKey: "sternengruppe"},
	{FirstName: "Lukas", LastName: "Neumann", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Sophie", LastName: "Schwarz", Class: "Klasse 1b", GroupKey: "sternengruppe"},

	// Bärengruppe (10 students) - Betreuer: Markus Wolf - Mix of Klasse 1b, 2a
	{FirstName: "Jonas", LastName: "Zimmermann", Class: "Klasse 1b", GroupKey: "bärengruppe"},
	{FirstName: "Emilia", LastName: "Braun", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Ben", LastName: "Krüger", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Lena", LastName: "Hofmann", Class: "Klasse 1b", GroupKey: "bärengruppe"},
	{FirstName: "Tim", LastName: "Hartmann", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Maximilian", LastName: "Schmitt", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Laura", LastName: "Werner", Class: "Klasse 1b", GroupKey: "bärengruppe"},
	{FirstName: "David", LastName: "Krause", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Anna", LastName: "Meier", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Simon", LastName: "Lange", Class: "Klasse 1b", GroupKey: "bärengruppe"},

	// Sonnengruppe (10 students) - Betreuer: Sandra Schröder - Mix of Klasse 2a, 2b
	{FirstName: "Julia", LastName: "Schulz", Class: "Klasse 2a", GroupKey: "sonnengruppe"},
	{FirstName: "Moritz", LastName: "König", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Marie", LastName: "Walter", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Niklas", LastName: "Huber", Class: "Klasse 2a", GroupKey: "sonnengruppe"},
	{FirstName: "Clara", LastName: "Herrmann", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Jan", LastName: "Peters", Class: "Klasse 2a", GroupKey: "sonnengruppe"},
	{FirstName: "Sophia", LastName: "Lang", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Erik", LastName: "Möller", Class: "Klasse 2a", GroupKey: "sonnengruppe"},
	{FirstName: "Lea", LastName: "Beck", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Finn", LastName: "Jung", Class: "Klasse 2a", GroupKey: "sonnengruppe"},

	// Mondgruppe (10 students) - Betreuer: Christian Neumann - Mix of Klasse 2b, 3a
	{FirstName: "Anton", LastName: "Keller", Class: "Klasse 2b", GroupKey: "mondgruppe"},
	{FirstName: "Charlotte", LastName: "Berger", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Henri", LastName: "Fuchs", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Amelie", LastName: "Vogel", Class: "Klasse 2b", GroupKey: "mondgruppe"},
	{FirstName: "Leonard", LastName: "Roth", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Johanna", LastName: "Frank", Class: "Klasse 2b", GroupKey: "mondgruppe"},
	{FirstName: "Elias", LastName: "Baumann", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Isabella", LastName: "Graf", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Matteo", LastName: "Kaiser", Class: "Klasse 2b", GroupKey: "mondgruppe"},
	{FirstName: "Nele", LastName: "Pfeiffer", Class: "Klasse 3a", GroupKey: "mondgruppe"},

	// Regenbogengruppe (10 students) - Betreuer: Nicole Schwarz - Mix of Klasse 3a, 3b
	{FirstName: "Theo", LastName: "Sommer", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Frieda", LastName: "Brandt", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},
	{FirstName: "Oscar", LastName: "Vogt", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Greta", LastName: "Engel", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},
	{FirstName: "Jakob", LastName: "Stein", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Mila", LastName: "Albrecht", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},
	{FirstName: "Luis", LastName: "Arnold", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Ella", LastName: "Bender", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},
	{FirstName: "Nico", LastName: "Böhm", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Ida", LastName: "Busch", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},

	// Blumengruppe (10 students) - Betreuer: Frank Zimmermann - Mix of Klasse 3b, 4a
	{FirstName: "Philipp", LastName: "Dietrich", Class: "Klasse 3b", GroupKey: "blumengruppe"},
	{FirstName: "Leni", LastName: "Ernst", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Fabian", LastName: "Franke", Class: "Klasse 3b", GroupKey: "blumengruppe"},
	{FirstName: "Maja", LastName: "Friedrich", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Luca", LastName: "Günther", Class: "Klasse 3b", GroupKey: "blumengruppe"},
	{FirstName: "Alina", LastName: "Haas", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Julian", LastName: "Heinrich", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Carla", LastName: "Henkel", Class: "Klasse 3b", GroupKey: "blumengruppe"},
	{FirstName: "Hannes", LastName: "Hesse", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Mathilda", LastName: "Horn", Class: "Klasse 4a", GroupKey: "blumengruppe"},

	// Schmetterlingsgruppe (10 students) - Betreuer: Birgit Braun - Mix of Klasse 4a, 4b
	{FirstName: "Alexander", LastName: "Jäger", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Victoria", LastName: "Kerner", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Florian", LastName: "Kraft", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Helena", LastName: "Kramer", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Vincent", LastName: "Kuhn", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Stella", LastName: "Lehmann", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Maximilian", LastName: "Lorenz", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Antonia", LastName: "Ludwig", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Tom", LastName: "Mayer", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Pauline", LastName: "Menzel", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},

	// Waldgruppe (10 students) - Betreuer: Jörg Krüger - Mix of Klasse 1a, 2a
	{FirstName: "Samuel", LastName: "Naumann", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Luisa", LastName: "Otto", Class: "Klasse 2a", GroupKey: "waldgruppe"},
	{FirstName: "Jonathan", LastName: "Paul", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Emily", LastName: "Pohl", Class: "Klasse 2a", GroupKey: "waldgruppe"},
	{FirstName: "Rafael", LastName: "Ritter", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Marlene", LastName: "Sauer", Class: "Klasse 2a", GroupKey: "waldgruppe"},
	{FirstName: "Aaron", LastName: "Schäfer", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Zoe", LastName: "Schenk", Class: "Klasse 2a", GroupKey: "waldgruppe"},
	{FirstName: "Till", LastName: "Schubert", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Romy", LastName: "Seifert", Class: "Klasse 2a", GroupKey: "waldgruppe"},

	// Meeresgruppe (10 students) - Betreuer: Heike Hartmann - Mix of Klasse 2b, 3b
	{FirstName: "Dominik", LastName: "Simon", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Chiara", LastName: "Stark", Class: "Klasse 3b", GroupKey: "meeresgruppe"},
	{FirstName: "Benedikt", LastName: "Steiner", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Katharina", LastName: "Stock", Class: "Klasse 3b", GroupKey: "meeresgruppe"},
	{FirstName: "Valentin", LastName: "Thiel", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Miriam", LastName: "Ulrich", Class: "Klasse 3b", GroupKey: "meeresgruppe"},
	{FirstName: "Constantin", LastName: "Vetter", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Franziska", LastName: "Voigt", Class: "Klasse 3b", GroupKey: "meeresgruppe"},
	{FirstName: "Robin", LastName: "Walther", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Nina", LastName: "Weiß", Class: "Klasse 3b", GroupKey: "meeresgruppe"},

	// Wiesengruppe (10 students) - Betreuer: Uwe Lange - Mix of Klasse 3a, 4b
	{FirstName: "Sebastian", LastName: "Wendt", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Annika", LastName: "Winkler", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
	{FirstName: "Tobias", LastName: "Winter", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Melina", LastName: "Wolff", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
	{FirstName: "Marvin", LastName: "Zander", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Selina", LastName: "Ziegler", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
	{FirstName: "Kevin", LastName: "Anders", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Jessica", LastName: "Bader", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
	{FirstName: "Dennis", LastName: "Bartsch", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Sabrina", LastName: "Bergmann", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
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
// Need enough devices to support 8+ concurrent activity sessions
var DemoDevices = []DemoDevice{
	{DeviceID: "demo-device-001", Name: "Haupteingang Scanner"},
	{DeviceID: "demo-device-002", Name: "OGS-Raum 1 Scanner"},
	{DeviceID: "demo-device-003", Name: "OGS-Raum 2 Scanner"},
	{DeviceID: "demo-device-004", Name: "OGS-Raum 3 Scanner"},
	{DeviceID: "demo-device-005", Name: "Sporthalle Scanner"},
	{DeviceID: "demo-device-006", Name: "Kreativraum Scanner"},
	{DeviceID: "demo-device-007", Name: "Mensa Scanner"},
	{DeviceID: "demo-device-008", Name: "Schulhof Scanner"},
	{DeviceID: "demo-device-009", Name: "Musikraum Scanner"},
	{DeviceID: "demo-device-010", Name: "Bewegungsraum Scanner"},
}

// DefaultRuntimeConfig provides default values for runtime snapshot creation
// Configured for 100 students across 10 groups, with realistic attendance
// - 85 checked in and in rooms
// - 5 "unterwegs" (moving between rooms)
// - 10 not checked in (sick at home, not yet arrived, etc.)
var DefaultRuntimeConfig = RuntimeConfig{
	ActiveSessions:    10, // Start 10 activity sessions (one per Betreuer)
	CheckedInStudents: 85, // 85 students in rooms
	StudentsUnterwegs: 5,  // 5 students "on the way" between rooms
	// Remaining 10 students: not checked in (sick/absent)
}

// DemoGuardian represents a guardian to be created via API
type DemoGuardian struct {
	FirstName    string
	LastName     string
	Email        string
	Phone        string
	MobilePhone  string
	Relationship string // Valid: "parent", "guardian", "relative", "other"
	StudentIndex int    // Index into DemoStudents array
	IsPrimary    bool
}

// DemoGuardians defines guardians linked to demo students
// Each student gets 1-2 guardians for realistic data
var DemoGuardians = []DemoGuardian{
	// Klasse 1a guardians
	{FirstName: "Sabine", LastName: "Schneider", Email: "sabine.schneider@email.de", MobilePhone: "+49 151 12345001", Relationship: "parent", StudentIndex: 0, IsPrimary: true},
	{FirstName: "Klaus", LastName: "Schneider", Email: "klaus.schneider@email.de", Phone: "+49 221 555001", Relationship: "parent", StudentIndex: 0, IsPrimary: false},
	{FirstName: "Petra", LastName: "Meyer", Email: "petra.meyer@email.de", MobilePhone: "+49 151 12345002", Relationship: "parent", StudentIndex: 1, IsPrimary: true},
	{FirstName: "Stefan", LastName: "Koch", Email: "stefan.koch@email.de", MobilePhone: "+49 151 12345003", Relationship: "parent", StudentIndex: 2, IsPrimary: true},
	{FirstName: "Andrea", LastName: "Bauer", Email: "andrea.bauer@email.de", MobilePhone: "+49 151 12345004", Relationship: "parent", StudentIndex: 3, IsPrimary: true},
	{FirstName: "Thomas", LastName: "Richter", Email: "thomas.richter@email.de", Phone: "+49 221 555005", Relationship: "parent", StudentIndex: 4, IsPrimary: true},
	{FirstName: "Monika", LastName: "Richter", Email: "monika.richter@email.de", MobilePhone: "+49 151 12345005", Relationship: "parent", StudentIndex: 4, IsPrimary: false},
	{FirstName: "Karin", LastName: "Klein", Email: "karin.klein@email.de", MobilePhone: "+49 151 12345006", Relationship: "parent", StudentIndex: 5, IsPrimary: true},
	{FirstName: "Markus", LastName: "Wolf", Email: "markus.wolf@email.de", MobilePhone: "+49 151 12345007", Relationship: "parent", StudentIndex: 6, IsPrimary: true},
	{FirstName: "Julia", LastName: "Schröder", Email: "julia.schroeder@email.de", MobilePhone: "+49 151 12345008", Relationship: "parent", StudentIndex: 7, IsPrimary: true},
	{FirstName: "Christian", LastName: "Neumann", Email: "christian.neumann@email.de", Phone: "+49 221 555009", Relationship: "parent", StudentIndex: 8, IsPrimary: true},
	{FirstName: "Birgit", LastName: "Schwarz", Email: "birgit.schwarz@email.de", MobilePhone: "+49 151 12345010", Relationship: "parent", StudentIndex: 9, IsPrimary: true},
	{FirstName: "Ralf", LastName: "Zimmermann", Email: "ralf.zimmermann@email.de", MobilePhone: "+49 151 12345011", Relationship: "parent", StudentIndex: 10, IsPrimary: true},
	{FirstName: "Susanne", LastName: "Braun", Email: "susanne.braun@email.de", MobilePhone: "+49 151 12345012", Relationship: "parent", StudentIndex: 11, IsPrimary: true},
	{FirstName: "Martin", LastName: "Krüger", Email: "martin.krueger@email.de", Phone: "+49 221 555013", Relationship: "parent", StudentIndex: 12, IsPrimary: true},
	{FirstName: "Nicole", LastName: "Hofmann", Email: "nicole.hofmann@email.de", MobilePhone: "+49 151 12345014", Relationship: "parent", StudentIndex: 13, IsPrimary: true},
	{FirstName: "Uwe", LastName: "Hartmann", Email: "uwe.hartmann@email.de", MobilePhone: "+49 151 12345015", Relationship: "parent", StudentIndex: 14, IsPrimary: true},

	// Klasse 2b guardians
	{FirstName: "Gabriele", LastName: "Schmitt", Email: "gabriele.schmitt@email.de", MobilePhone: "+49 151 12345016", Relationship: "parent", StudentIndex: 15, IsPrimary: true},
	{FirstName: "Frank", LastName: "Werner", Email: "frank.werner@email.de", MobilePhone: "+49 151 12345017", Relationship: "parent", StudentIndex: 16, IsPrimary: true},
	{FirstName: "Heike", LastName: "Krause", Email: "heike.krause@email.de", MobilePhone: "+49 151 12345018", Relationship: "parent", StudentIndex: 17, IsPrimary: true},
	{FirstName: "Jörg", LastName: "Meier", Email: "joerg.meier@email.de", Phone: "+49 221 555019", Relationship: "parent", StudentIndex: 18, IsPrimary: true},
	{FirstName: "Claudia", LastName: "Lange", Email: "claudia.lange@email.de", MobilePhone: "+49 151 12345020", Relationship: "parent", StudentIndex: 19, IsPrimary: true},
	{FirstName: "Bernd", LastName: "Schulz", Email: "bernd.schulz@email.de", MobilePhone: "+49 151 12345021", Relationship: "parent", StudentIndex: 20, IsPrimary: true},
	{FirstName: "Silke", LastName: "König", Email: "silke.koenig@email.de", MobilePhone: "+49 151 12345022", Relationship: "parent", StudentIndex: 21, IsPrimary: true},
	{FirstName: "Wolfgang", LastName: "Walter", Email: "wolfgang.walter@email.de", Phone: "+49 221 555023", Relationship: "parent", StudentIndex: 22, IsPrimary: true},
	{FirstName: "Anja", LastName: "Huber", Email: "anja.huber@email.de", MobilePhone: "+49 151 12345024", Relationship: "parent", StudentIndex: 23, IsPrimary: true},
	{FirstName: "Dieter", LastName: "Herrmann", Email: "dieter.herrmann@email.de", MobilePhone: "+49 151 12345025", Relationship: "parent", StudentIndex: 24, IsPrimary: true},
	{FirstName: "Renate", LastName: "Peters", Email: "renate.peters@email.de", MobilePhone: "+49 151 12345026", Relationship: "parent", StudentIndex: 25, IsPrimary: true},
	{FirstName: "Holger", LastName: "Lang", Email: "holger.lang@email.de", Phone: "+49 221 555027", Relationship: "parent", StudentIndex: 26, IsPrimary: true},
	{FirstName: "Martina", LastName: "Möller", Email: "martina.moeller@email.de", MobilePhone: "+49 151 12345028", Relationship: "parent", StudentIndex: 27, IsPrimary: true},
	{FirstName: "Andreas", LastName: "Beck", Email: "andreas.beck@email.de", MobilePhone: "+49 151 12345029", Relationship: "parent", StudentIndex: 28, IsPrimary: true},
	{FirstName: "Elke", LastName: "Jung", Email: "elke.jung@email.de", MobilePhone: "+49 151 12345030", Relationship: "parent", StudentIndex: 29, IsPrimary: true},

	// Klasse 3c guardians
	{FirstName: "Ingrid", LastName: "Keller", Email: "ingrid.keller@email.de", MobilePhone: "+49 151 12345031", Relationship: "parent", StudentIndex: 30, IsPrimary: true},
	{FirstName: "Peter", LastName: "Berger", Email: "peter.berger@email.de", Phone: "+49 221 555032", Relationship: "parent", StudentIndex: 31, IsPrimary: true},
	{FirstName: "Cornelia", LastName: "Fuchs", Email: "cornelia.fuchs@email.de", MobilePhone: "+49 151 12345033", Relationship: "parent", StudentIndex: 32, IsPrimary: true},
	{FirstName: "Manfred", LastName: "Vogel", Email: "manfred.vogel@email.de", MobilePhone: "+49 151 12345034", Relationship: "parent", StudentIndex: 33, IsPrimary: true},
	{FirstName: "Barbara", LastName: "Roth", Email: "barbara.roth@email.de", MobilePhone: "+49 151 12345035", Relationship: "parent", StudentIndex: 34, IsPrimary: true},
	{FirstName: "Heinz", LastName: "Frank", Email: "heinz.frank@email.de", Phone: "+49 221 555036", Relationship: "parent", StudentIndex: 35, IsPrimary: true},
	{FirstName: "Doris", LastName: "Baumann", Email: "doris.baumann@email.de", MobilePhone: "+49 151 12345037", Relationship: "parent", StudentIndex: 36, IsPrimary: true},
	{FirstName: "Werner", LastName: "Graf", Email: "werner.graf@email.de", MobilePhone: "+49 151 12345038", Relationship: "parent", StudentIndex: 37, IsPrimary: true},
	{FirstName: "Christa", LastName: "Kaiser", Email: "christa.kaiser@email.de", MobilePhone: "+49 151 12345039", Relationship: "parent", StudentIndex: 38, IsPrimary: true},
	{FirstName: "Hans", LastName: "Pfeiffer", Email: "hans.pfeiffer@email.de", Phone: "+49 221 555040", Relationship: "parent", StudentIndex: 39, IsPrimary: true},
	{FirstName: "Gisela", LastName: "Sommer", Email: "gisela.sommer@email.de", MobilePhone: "+49 151 12345041", Relationship: "parent", StudentIndex: 40, IsPrimary: true},
	{FirstName: "Kurt", LastName: "Brandt", Email: "kurt.brandt@email.de", MobilePhone: "+49 151 12345042", Relationship: "parent", StudentIndex: 41, IsPrimary: true},
	{FirstName: "Helga", LastName: "Vogt", Email: "helga.vogt@email.de", MobilePhone: "+49 151 12345043", Relationship: "parent", StudentIndex: 42, IsPrimary: true},
	{FirstName: "Hermann", LastName: "Engel", Email: "hermann.engel@email.de", Phone: "+49 221 555044", Relationship: "parent", StudentIndex: 43, IsPrimary: true},
	{FirstName: "Ursula", LastName: "Stein", Email: "ursula.stein@email.de", MobilePhone: "+49 151 12345045", Relationship: "parent", StudentIndex: 44, IsPrimary: true},
}
