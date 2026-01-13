package api

// DemoRoom represents a room to be created via API
type DemoRoom struct {
	Name     string
	Category string // German category name for display
	Capacity int
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

// DemoRooms defines the 12 rooms for the demo environment
// Categories MUST match frontend dropdown in rooms.config.tsx:
// "Normaler Raum", "Gruppenraum", "Themenraum", "Sport"
var DemoRooms = []DemoRoom{
	// Primary OGS spaces
	{Name: "OGS-Raum 1", Category: "Gruppenraum", Capacity: 25},
	{Name: "OGS-Raum 2", Category: "Gruppenraum", Capacity: 20},
	{Name: "OGS-Raum 3", Category: "Gruppenraum", Capacity: 18},
	// Common areas
	{Name: "Mensa", Category: "Normaler Raum", Capacity: 80},
	{Name: "Schulhof", Category: "Normaler Raum", Capacity: 100},
	{Name: "Aula", Category: "Normaler Raum", Capacity: 120},
	// Sports facilities
	{Name: "Sporthalle", Category: "Sport", Capacity: 40},
	{Name: "Bewegungsraum", Category: "Sport", Capacity: 15},
	// Theme rooms for activities
	{Name: "Kreativraum", Category: "Themenraum", Capacity: 20},
	{Name: "Musikraum", Category: "Themenraum", Capacity: 15},
	{Name: "Werkraum", Category: "Themenraum", Capacity: 12},
	{Name: "Leseecke", Category: "Themenraum", Capacity: 10},
}

// DemoStaff defines the 7 staff members for the demo environment
// Position must match frontend dropdown values in teacher-form.tsx and invitation-form.tsx
var DemoStaff = []DemoStaffMember{
	{FirstName: "Anna", LastName: "Müller", Position: "OGS-Büro", IsTeacher: true},              // OGS Leader/Admin
	{FirstName: "Thomas", LastName: "Weber", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Sarah", LastName: "Schmidt", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Michael", LastName: "Hoffmann", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Lisa", LastName: "Wagner", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Jan", LastName: "Becker", Position: "Extern", IsTeacher: true}, // External helper
	{FirstName: "Maria", LastName: "Fischer", Position: "Pädagogische Fachkraft", IsTeacher: true},
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
	{DeviceID: "demo-device-002", Name: "OGS Room 1 Scanner"},
	{DeviceID: "demo-device-003", Name: "OGS Room 2 Scanner"},
	{DeviceID: "demo-device-004", Name: "Gymnasium Scanner"},
}

// DefaultRuntimeConfig provides default values for runtime snapshot creation
var DefaultRuntimeConfig = RuntimeConfig{
	ActiveSessions:    4,  // Start 4 activity sessions
	CheckedInStudents: 32, // Check in 32 of 45 students
	StudentsUnterwegs: 2,  // Leave 2 students "on the way"
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
