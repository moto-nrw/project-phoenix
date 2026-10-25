package api

// DemoRoom represents a room to be created via API
type DemoRoom struct {
	Name     string
	Category string // German category name for display
	Capacity int
	Building string // Building name: "Hauptgebäude", "Sporthalle", "Außenbereich"
	Floor    *int   // Floor number: 0=EG, 1=1.OG, 2=2.OG, nil for outdoor
	// IsOpenRoom releases the room permanently ("offener Raum", #3064).
	IsOpenRoom bool
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

// Floor helper for creating floor pointers
func floor(f int) *int { return &f }

// DemoRooms defines the 12 rooms for the demo environment
// Categories MUST match frontend dropdown in rooms.config.tsx:
// "Normaler Raum", "Gruppenraum", "Themenraum", "Sport"
// Building layout:
//
//	Hauptgebäude: EG (Mensa, Aula, OGS-1), 1.OG (OGS-2, OGS-3, Kreativ, Musik), 2.OG (Werk, Lese)
//	Sporthalle: EG (Sporthalle, Bewegungsraum)
var DemoRooms = []DemoRoom{
	// Hauptgebäude - Erdgeschoss (Ground Floor)
	{Name: "OGS-Raum 1", Category: "Gruppenraum", Capacity: 25, Building: "Hauptgebäude", Floor: floor(0)},
	{Name: "Mensa", Category: "Normaler Raum", Capacity: 80, Building: "Hauptgebäude", Floor: floor(0)},
	{Name: "Aula", Category: "Normaler Raum", Capacity: 120, Building: "Hauptgebäude", Floor: floor(0)},
	// Hauptgebäude - 1. Obergeschoss (First Floor)
	{Name: "OGS-Raum 2", Category: "Gruppenraum", Capacity: 20, Building: "Hauptgebäude", Floor: floor(1)},
	{Name: "OGS-Raum 3", Category: "Gruppenraum", Capacity: 18, Building: "Hauptgebäude", Floor: floor(1)},
	// Released alongside the gym: #3062's own examples for an open room are a
	// gym and a craft area, and the demo has to show more than one so the
	// shared-room work has something to look at.
	{Name: "Kreativraum", Category: "Themenraum", Capacity: 20, Building: "Hauptgebäude", Floor: floor(1), IsOpenRoom: true},
	{Name: "Musikraum", Category: "Themenraum", Capacity: 15, Building: "Hauptgebäude", Floor: floor(1)},
	// Hauptgebäude - 2. Obergeschoss (Second Floor)
	{Name: "Werkraum", Category: "Themenraum", Capacity: 12, Building: "Hauptgebäude", Floor: floor(2)},
	{Name: "Leseecke", Category: "Themenraum", Capacity: 10, Building: "Hauptgebäude", Floor: floor(2)},
	// Sporthalle - Separate Building
	{Name: "Sporthalle", Category: "Sport", Capacity: 40, Building: "Sporthalle", Floor: floor(0), IsOpenRoom: true},
	{Name: "Bewegungsraum", Category: "Sport", Capacity: 15, Building: "Sporthalle", Floor: floor(0)},
	// Note: the Schulhof is a system room that moto creates, already released,
	// only when a school switches on the kiosk Schulhof button (ADR 0019). The
	// demo leaves that switch off, so its open rooms are the two above.
}

// DemoStaff defines staff members for the demo environment
// Position must match frontend dropdown values in teacher-form.tsx and invitation-form.tsx
// Demo accounts: 10 OGS-Büro, 9 Pädagogische Fachkraft and 1 externer Gast.
// All use email: demo{n}@mail.de and password: sdlXK26%
var DemoStaff = []DemoStaffMember{
	// 10 OGS-Büro accounts (admin role)
	{FirstName: "Anna", LastName: "Müller", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Katarzyna", LastName: "Schmidt", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Sabine", LastName: "Weber", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Svenja", LastName: "Fischer", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Claudia", LastName: "Wagner", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Stefanie", LastName: "Becker", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Monika", LastName: "Hoffmann", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Natalia", LastName: "Schulz", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Petra", LastName: "Koch", Position: "OGS-Büro", IsTeacher: true},
	{FirstName: "Irina", LastName: "Richter", Position: "OGS-Büro", IsTeacher: true},
	// 10 Pädagogische Fachkraft accounts (teacher/betreuer role)
	{FirstName: "Julia", LastName: "Klein", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Berken", LastName: "Hawleri", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Elif", LastName: "Arslan", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Christian", LastName: "Neumann", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Amara", LastName: "Okafor", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Tarek", LastName: "Haddad", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Daria", LastName: "Nowak", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Jana", LastName: "Krüger", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Mai", LastName: "Pham", Position: "Pädagogische Fachkraft", IsTeacher: true},
	{FirstName: "Ana", LastName: "Silva", Position: "Extern", IsTeacher: false},
}

// DemoStudents defines the 100 students across 10 groups (10 each)
// OGS groups contain students from multiple school classes (realistic scenario)
// Each group has a dedicated Betreuer (Pädagogische Fachkraft) assigned
var DemoStudents = []DemoStudent{
	// Sternengruppe (10 students) - Betreuer: Julia Klein - Mix of Klasse 1a, 1b
	{FirstName: "Felix", LastName: "Schneider", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Emma", LastName: "Meyer", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Leon", LastName: "Koch", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Aida", LastName: "Amini", Class: "Klasse 1b", GroupKey: "sternengruppe"},
	{FirstName: "Baran", LastName: "Amini", Class: "Klasse 1b", GroupKey: "sternengruppe"},
	{FirstName: "Hannah", LastName: "Klein", Class: "Klasse 1b", GroupKey: "sternengruppe"},
	{FirstName: "Paul", LastName: "Wolf", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Lina", LastName: "Schröder", Class: "Klasse 1b", GroupKey: "sternengruppe"},
	{FirstName: "Lukas", LastName: "Neumann", Class: "Klasse 1a", GroupKey: "sternengruppe"},
	{FirstName: "Bente", LastName: "Jansen", Class: "Klasse 1b", GroupKey: "sternengruppe"},

	// Bärengruppe (10 students) - Betreuer: Berken Hawleri - Mix of Klasse 1b, 2a
	{FirstName: "Jonas", LastName: "Zimmermann", Class: "Klasse 1b", GroupKey: "bärengruppe"},
	{FirstName: "Mariam", LastName: "Kumar", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Ben", LastName: "Krüger", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Lena", LastName: "Hofmann", Class: "Klasse 1b", GroupKey: "bärengruppe"},
	{FirstName: "Tim", LastName: "Hartmann", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Maximilian", LastName: "Schmitt", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Hilal", LastName: "Özdemir", Class: "Klasse 1b", GroupKey: "bärengruppe"},
	{FirstName: "Ivan", LastName: "Petrow", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Anna", LastName: "Meier", Class: "Klasse 2a", GroupKey: "bärengruppe"},
	{FirstName: "Simon", LastName: "Bader", Class: "Klasse 1b", GroupKey: "bärengruppe"},

	// Sonnengruppe (10 students) - Betreuer: Elif Arslan - Mix of Klasse 2a, 2b
	{FirstName: "Julia", LastName: "Schulz", Class: "Klasse 2a", GroupKey: "sonnengruppe"},
	{FirstName: "Emir", LastName: "Yilmaz", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Marie", LastName: "Walter", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "William", LastName: "Baker", Class: "Klasse 2a", GroupKey: "sonnengruppe"},
	{FirstName: "Maria", LastName: "Fernández", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Jan", LastName: "Peters", Class: "Klasse 2a", GroupKey: "sonnengruppe"},
	{FirstName: "Sophia", LastName: "Lang", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Erik", LastName: "Li", Class: "Klasse 2a", GroupKey: "sonnengruppe"},
	{FirstName: "Lea", LastName: "Beck", Class: "Klasse 2b", GroupKey: "sonnengruppe"},
	{FirstName: "Finn", LastName: "Jung", Class: "Klasse 2a", GroupKey: "sonnengruppe"},

	// Mondgruppe (10 students) - Betreuer: Christian Neumann - Mix of Klasse 2b, 3a
	{FirstName: "Anton", LastName: "Keller", Class: "Klasse 2b", GroupKey: "mondgruppe"},
	{FirstName: "Charlotte", LastName: "Berger", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "David", LastName: "Krause", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Amelie", LastName: "Vogel", Class: "Klasse 2b", GroupKey: "mondgruppe"},
	{FirstName: "Leonard", LastName: "Roth", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Johanna", LastName: "Mores", Class: "Klasse 2b", GroupKey: "mondgruppe"},
	{FirstName: "Dejan", LastName: "Baumann", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Isabella", LastName: "Graf", Class: "Klasse 3a", GroupKey: "mondgruppe"},
	{FirstName: "Mohammed", LastName: "Ahmed", Class: "Klasse 2b", GroupKey: "mondgruppe"},
	{FirstName: "Gesche", LastName: "Sjövold", Class: "Klasse 3a", GroupKey: "mondgruppe"},

	// Regenbogengruppe (10 students) - Betreuer: Amara Okafor - Mix of Klasse 3a, 3b
	{FirstName: "Theo", LastName: "Sommer", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Frieda", LastName: "Brandt", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},
	{FirstName: "Oscar", LastName: "Vogt", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Greta", LastName: "Engel", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},
	{FirstName: "Jakob", LastName: "Stein", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Mila", LastName: "Albrecht", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},
	{FirstName: "Mateus", LastName: "Oliveira", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Nambi", LastName: "Mukasa", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},
	{FirstName: "Can", LastName: "Demir", Class: "Klasse 3a", GroupKey: "regenbogengruppe"},
	{FirstName: "Ida", LastName: "Busch", Class: "Klasse 3b", GroupKey: "regenbogengruppe"},

	// Blumengruppe (10 students) - Betreuer: Tarek Haddad - Mix of Klasse 3b, 4a
	{FirstName: "Noah", LastName: "Richter", Class: "Klasse 3b", GroupKey: "blumengruppe"},
	{FirstName: "Sara", LastName: "Gómez", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Fabian", LastName: "Franke", Class: "Klasse 3b", GroupKey: "blumengruppe"},
	{FirstName: "Maja", LastName: "Friedrich", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Luca", LastName: "Nguyen", Class: "Klasse 3b", GroupKey: "blumengruppe"},
	{FirstName: "Samira", LastName: "Haas", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Julian", LastName: "Wang", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Carla", LastName: "Henkel", Class: "Klasse 3b", GroupKey: "blumengruppe"},
	{FirstName: "Hannes", LastName: "Hesse", Class: "Klasse 4a", GroupKey: "blumengruppe"},
	{FirstName: "Mathilda", LastName: "Horn", Class: "Klasse 4a", GroupKey: "blumengruppe"},

	// Schmetterlingsgruppe (10 students) - Betreuer: Daria Nowak - Mix of Klasse 4a, 4b
	{FirstName: "Shakur", LastName: "Mansour", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Victoria", LastName: "Kerner", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Pius", LastName: "Sango", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Helena", LastName: "Zhang", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Vincent", LastName: "Satō", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Neilla", LastName: "Lehmann", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Maximilian", LastName: "Lorenz", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Adelina", LastName: "Wolkow", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Tom", LastName: "Mayer", Class: "Klasse 4a", GroupKey: "schmetterlingsgruppe"},
	{FirstName: "Pauline", LastName: "Menzel", Class: "Klasse 4b", GroupKey: "schmetterlingsgruppe"},

	// Waldgruppe (10 students) - Betreuer: Jana Krüger - Mix of Klasse 1a, 2a
	{FirstName: "Samuel", LastName: "Naumann", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Luisa", LastName: "Otto", Class: "Klasse 2a", GroupKey: "waldgruppe"},
	{FirstName: "Jonathan", LastName: "Baker", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Emily", LastName: "Tran", Class: "Klasse 2a", GroupKey: "waldgruppe"},
	{FirstName: "Steven", LastName: "Smit", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Jina", LastName: "Sauer", Class: "Klasse 2a", GroupKey: "waldgruppe"},
	{FirstName: "Aaron", LastName: "Schäfer", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Vibe", LastName: "Sondre", Class: "Klasse 2a", GroupKey: "waldgruppe"},
	{FirstName: "Barjan", LastName: "Schubert", Class: "Klasse 1a", GroupKey: "waldgruppe"},
	{FirstName: "Yilida", LastName: "Özdemir", Class: "Klasse 2a", GroupKey: "waldgruppe"},

	// Meeresgruppe (10 students) - Betreuer: Mai Pham - Mix of Klasse 2b, 3b
	{FirstName: "Dominik", LastName: "Simon", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Chiara", LastName: "Rodríguez", Class: "Klasse 3b", GroupKey: "meeresgruppe"},
	{FirstName: "Benedikt", LastName: "Steiner", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Fatima", LastName: "Stock", Class: "Klasse 3b", GroupKey: "meeresgruppe"},
	{FirstName: "Valentin", LastName: "Suzuki", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Miriam", LastName: "Ulrich", Class: "Klasse 3b", GroupKey: "meeresgruppe"},
	{FirstName: "Constantin", LastName: "Gutiérrez", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Franziska", LastName: "Voigt", Class: "Klasse 3b", GroupKey: "meeresgruppe"},
	{FirstName: "Robin", LastName: "Walther", Class: "Klasse 2b", GroupKey: "meeresgruppe"},
	{FirstName: "Nina", LastName: "Takahashi", Class: "Klasse 3b", GroupKey: "meeresgruppe"},

	// Wiesengruppe (10 students) - Betreuerin: Anna Müller - Mix of Klasse 3a, 4b
	{FirstName: "Salam", LastName: "Wendt", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Annika", LastName: "Kumar", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
	{FirstName: "Tobias", LastName: "Winter", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Melina", LastName: "Wolff", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
	{FirstName: "Marvin", LastName: "Patel", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Malihe", LastName: "Ziegler", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
	{FirstName: "Thore", LastName: "Gritzmann", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Jessica", LastName: "Bader", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
	{FirstName: "Allen", LastName: "Bartsch", Class: "Klasse 3a", GroupKey: "wiesengruppe"},
	{FirstName: "Cynthia", LastName: "Bergmann", Class: "Klasse 4b", GroupKey: "wiesengruppe"},
}

// DemoActivities defines the 10 activities with room assignments.
//
// Fußball and Tanzen deliberately share the Sporthalle, which is a released
// room: that pairing is what makes the shared open-room view demonstrable
// (#3065) — several offerings in one open room, each child listed once. Moving
// either of them to a different room would quietly remove the only place in
// the demo where that can be seen. The full-day simulation also runs a planned
// block there, which gives the room page its block sections (#3281).
var DemoActivities = []DemoActivity{
	{Name: "Hausaufgaben", DefaultRoom: "OGS-Raum 1", DurationMins: 60},
	{Name: "Fußball", DefaultRoom: "Sporthalle", DurationMins: 90},
	{Name: "Basteln", DefaultRoom: "Kreativraum", DurationMins: 75},
	{Name: "Kochen", DefaultRoom: "Mensa", DurationMins: 120},
	{Name: "Lesen", DefaultRoom: "OGS-Raum 2", DurationMins: 45},
	{Name: "Musik", DefaultRoom: "OGS-Raum 1", DurationMins: 60},
	{Name: "Tanzen", DefaultRoom: "Sporthalle", DurationMins: 60},
	{Name: "Schach", DefaultRoom: "OGS-Raum 2", DurationMins: 45},
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
	{DeviceID: "demo-device-008", Name: "Aula Scanner"},
	{DeviceID: "demo-device-009", Name: "Musikraum Scanner"},
	{DeviceID: "demo-device-010", Name: "Bewegungsraum Scanner"},
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

// DemoGuardians defines guardians linked to demo students, one entry per
// guardian and child. Each student gets 1-2 guardians; siblings repeat the
// same guardian (same name and email), and the seeder links that one profile
// to every child instead of creating a second one.
var DemoGuardians = []DemoGuardian{
	// Sternengruppe
	{FirstName: "Sabine", LastName: "Schneider", Email: "sabine.schneider@email.de", MobilePhone: "+49 151 12345001", Relationship: "parent", StudentIndex: 0, IsPrimary: true},
	{FirstName: "Klaus", LastName: "Schneider", Email: "klaus.schneider@email.de", Phone: "+49 221 555001", Relationship: "parent", StudentIndex: 0, IsPrimary: false},
	{FirstName: "Petra", LastName: "Meyer", Email: "petra.meyer@email.de", MobilePhone: "+49 151 12345002", Relationship: "parent", StudentIndex: 1, IsPrimary: true},
	{FirstName: "Stefan", LastName: "Koch", Email: "stefan.koch@email.de", MobilePhone: "+49 151 12345003", Relationship: "parent", StudentIndex: 2, IsPrimary: true},
	{FirstName: "Shirin", LastName: "Amini", Email: "shirin.amini@email.de", MobilePhone: "+49 151 12345004", Relationship: "parent", StudentIndex: 3, IsPrimary: true},
	{FirstName: "Reza", LastName: "Amini", Email: "reza.amini@email.de", MobilePhone: "+49 160 98765004", Relationship: "parent", StudentIndex: 3, IsPrimary: false},
	{FirstName: "Shirin", LastName: "Amini", Email: "shirin.amini@email.de", MobilePhone: "+49 151 12345004", Relationship: "parent", StudentIndex: 4, IsPrimary: true},
	{FirstName: "Reza", LastName: "Amini", Email: "reza.amini@email.de", MobilePhone: "+49 160 98765004", Relationship: "parent", StudentIndex: 4, IsPrimary: false},
	{FirstName: "Karin", LastName: "Klein", Email: "karin.klein@email.de", MobilePhone: "+49 151 12345006", Relationship: "parent", StudentIndex: 5, IsPrimary: true},
	{FirstName: "Markus", LastName: "Wolf", Email: "markus.wolf@email.de", MobilePhone: "+49 151 12345007", Relationship: "parent", StudentIndex: 6, IsPrimary: true},
	{FirstName: "Julia", LastName: "Schröder", Email: "julia.schroeder@email.de", MobilePhone: "+49 151 12345008", Relationship: "parent", StudentIndex: 7, IsPrimary: true},
	{FirstName: "Christian", LastName: "Neumann", Email: "christian.neumann@email.de", Phone: "+49 221 555009", Relationship: "parent", StudentIndex: 8, IsPrimary: true},
	{FirstName: "Birgit", LastName: "Jansen", Email: "birgit.jansen@email.de", MobilePhone: "+49 151 12345010", Relationship: "parent", StudentIndex: 9, IsPrimary: true},

	// Bärengruppe
	{FirstName: "Ralf", LastName: "Zimmermann", Email: "ralf.zimmermann@email.de", MobilePhone: "+49 151 12345011", Relationship: "parent", StudentIndex: 10, IsPrimary: true},
	{FirstName: "Priya", LastName: "Kumar", Email: "priya.kumar@email.de", MobilePhone: "+49 151 12345012", Relationship: "parent", StudentIndex: 11, IsPrimary: true},
	{FirstName: "Martin", LastName: "Krüger", Email: "martin.krueger@email.de", Phone: "+49 221 555013", Relationship: "parent", StudentIndex: 12, IsPrimary: true},
	{FirstName: "Nicole", LastName: "Hofmann", Email: "nicole.hofmann@email.de", MobilePhone: "+49 151 12345014", Relationship: "parent", StudentIndex: 13, IsPrimary: true},
	{FirstName: "Uwe", LastName: "Hartmann", Email: "uwe.hartmann@email.de", MobilePhone: "+49 151 12345015", Relationship: "parent", StudentIndex: 14, IsPrimary: true},
	{FirstName: "Gabriele", LastName: "Schmitt", Email: "gabriele.schmitt@email.de", MobilePhone: "+49 151 12345016", Relationship: "parent", StudentIndex: 15, IsPrimary: true},
	{FirstName: "Serkan", LastName: "Özdemir", Email: "serkan.oezdemir@email.de", MobilePhone: "+49 151 12345017", Relationship: "parent", StudentIndex: 16, IsPrimary: true},
	{FirstName: "Olga", LastName: "Petrow", Email: "olga.petrow@email.de", MobilePhone: "+49 151 12345018", Relationship: "parent", StudentIndex: 17, IsPrimary: true},
	{FirstName: "Jörg", LastName: "Meier", Email: "joerg.meier@email.de", Phone: "+49 221 555019", Relationship: "parent", StudentIndex: 18, IsPrimary: true},
	{FirstName: "Claudia", LastName: "Bader", Email: "claudia.bader@email.de", MobilePhone: "+49 151 12345020", Relationship: "parent", StudentIndex: 19, IsPrimary: true},
	{FirstName: "Hartmut", LastName: "Bader", Email: "hartmut.bader@email.de", MobilePhone: "+49 151 12345098", Relationship: "parent", StudentIndex: 19, IsPrimary: false},

	// Sonnengruppe
	{FirstName: "Bernd", LastName: "Schulz", Email: "bernd.schulz@email.de", MobilePhone: "+49 151 12345021", Relationship: "parent", StudentIndex: 20, IsPrimary: true},
	{FirstName: "Ayşe", LastName: "Yilmaz", Email: "ayse.yilmaz@email.de", MobilePhone: "+49 151 12345022", Relationship: "parent", StudentIndex: 21, IsPrimary: true},
	{FirstName: "Wolfgang", LastName: "Walter", Email: "wolfgang.walter@email.de", Phone: "+49 221 555023", Relationship: "parent", StudentIndex: 22, IsPrimary: true},
	{FirstName: "Kathy", LastName: "Baker", Email: "kathy.baker@email.de", MobilePhone: "+49 151 12345024", Relationship: "parent", StudentIndex: 23, IsPrimary: true},
	{FirstName: "James", LastName: "Baker", Email: "james.baker@email.de", MobilePhone: "+49 160 98765024", Relationship: "parent", StudentIndex: 23, IsPrimary: false},
	{FirstName: "Javier", LastName: "Fernández", Email: "javier.fernandez@email.de", MobilePhone: "+49 151 12345025", Relationship: "parent", StudentIndex: 24, IsPrimary: true},
	{FirstName: "Renate", LastName: "Peters", Email: "renate.peters@email.de", MobilePhone: "+49 151 12345026", Relationship: "parent", StudentIndex: 25, IsPrimary: true},
	{FirstName: "Holger", LastName: "Lang", Email: "holger.lang@email.de", Phone: "+49 221 555027", Relationship: "parent", StudentIndex: 26, IsPrimary: true},
	{FirstName: "Martina", LastName: "Li", Email: "martina.li@email.de", MobilePhone: "+49 151 12345028", Relationship: "parent", StudentIndex: 27, IsPrimary: true},
	{FirstName: "Andreas", LastName: "Beck", Email: "andreas.beck@email.de", MobilePhone: "+49 151 12345029", Relationship: "parent", StudentIndex: 28, IsPrimary: true},
	{FirstName: "Elke", LastName: "Jung", Email: "elke.jung@email.de", MobilePhone: "+49 151 12345030", Relationship: "parent", StudentIndex: 29, IsPrimary: true},

	// Mondgruppe
	{FirstName: "Ingrid", LastName: "Keller", Email: "ingrid.keller@email.de", MobilePhone: "+49 151 12345031", Relationship: "parent", StudentIndex: 30, IsPrimary: true},
	{FirstName: "Peter", LastName: "Berger", Email: "peter.berger@email.de", Phone: "+49 221 555032", Relationship: "parent", StudentIndex: 31, IsPrimary: true},
	{FirstName: "Heike", LastName: "Krause", Email: "heike.krause@email.de", MobilePhone: "+49 151 12345018", Relationship: "parent", StudentIndex: 32, IsPrimary: true},
	{FirstName: "Manfred", LastName: "Vogel", Email: "manfred.vogel@email.de", MobilePhone: "+49 151 12345034", Relationship: "parent", StudentIndex: 33, IsPrimary: true},
	{FirstName: "Barbara", LastName: "Roth", Email: "barbara.roth@email.de", MobilePhone: "+49 151 12345035", Relationship: "parent", StudentIndex: 34, IsPrimary: true},
	{FirstName: "Daniel", LastName: "Mores", Email: "daniel.mores@email.de", MobilePhone: "+49 151 12345036", Relationship: "parent", StudentIndex: 35, IsPrimary: true},
	{FirstName: "Doris", LastName: "Baumann", Email: "doris.baumann@email.de", MobilePhone: "+49 151 12345037", Relationship: "parent", StudentIndex: 36, IsPrimary: true},
	{FirstName: "Milly", LastName: "Nikoliin", Email: "milly.nikoliin@email.de", MobilePhone: "+49 160 98765037", Relationship: "parent", StudentIndex: 36, IsPrimary: false},
	{FirstName: "Werner", LastName: "Graf", Email: "werner.graf@email.de", MobilePhone: "+49 151 12345038", Relationship: "parent", StudentIndex: 37, IsPrimary: true},
	{FirstName: "Amira", LastName: "Ahmed", Email: "amira.ahmed@email.de", MobilePhone: "+49 151 12345039", Relationship: "parent", StudentIndex: 38, IsPrimary: true},
	{FirstName: "Hans", LastName: "Sjövold", Email: "hans.sjoevold@email.de", MobilePhone: "+49 151 12345040", Relationship: "parent", StudentIndex: 39, IsPrimary: true},

	// Regenbogengruppe
	{FirstName: "Gisela", LastName: "Sommer", Email: "gisela.sommer@email.de", MobilePhone: "+49 151 12345041", Relationship: "parent", StudentIndex: 40, IsPrimary: true},
	{FirstName: "Kurt", LastName: "Brandt", Email: "kurt.brandt@email.de", MobilePhone: "+49 151 12345042", Relationship: "parent", StudentIndex: 41, IsPrimary: true},
	{FirstName: "Helga", LastName: "Vogt", Email: "helga.vogt@email.de", MobilePhone: "+49 151 12345043", Relationship: "parent", StudentIndex: 42, IsPrimary: true},
	{FirstName: "Hermann", LastName: "Engel", Email: "hermann.engel@email.de", Phone: "+49 221 555044", Relationship: "parent", StudentIndex: 43, IsPrimary: true},
	{FirstName: "Ursula", LastName: "Stein", Email: "ursula.stein@email.de", MobilePhone: "+49 151 12345045", Relationship: "parent", StudentIndex: 44, IsPrimary: true},
	{FirstName: "Gerhard", LastName: "Sommer", Email: "gerhard.sommer@email.de", MobilePhone: "+49 151 12345046", Relationship: "parent", StudentIndex: 45, IsPrimary: true},
	{FirstName: "Katja", LastName: "Albrecht", Email: "katja.albrecht@email.de", MobilePhone: "+49 160 98765046", Relationship: "parent", StudentIndex: 45, IsPrimary: false},
	{FirstName: "Camila", LastName: "Oliveira", Email: "camila.oliveira@email.de", MobilePhone: "+49 151 12345047", Relationship: "parent", StudentIndex: 46, IsPrimary: true},
	{FirstName: "Thiago", LastName: "Oliveira", Email: "thiago.oliveira@email.de", MobilePhone: "+49 160 98765047", Relationship: "parent", StudentIndex: 46, IsPrimary: false},
	{FirstName: "Sarah", LastName: "Nakato", Email: "sarah.nakato@email.de", MobilePhone: "+49 151 12345048", Relationship: "parent", StudentIndex: 47, IsPrimary: true},
	{FirstName: "Joseph", LastName: "Mukasa", Email: "joseph.mukasa@email.de", MobilePhone: "+49 160 98765048", Relationship: "parent", StudentIndex: 47, IsPrimary: false},
	{FirstName: "Hannelore", LastName: "Albrecht", Email: "hannelore.albrecht@email.de", MobilePhone: "+49 151 12345049", Relationship: "parent", StudentIndex: 48, IsPrimary: true},
	{FirstName: "Emre", LastName: "Demir", Email: "emre.demir@email.de", MobilePhone: "+49 160 98765049", Relationship: "parent", StudentIndex: 48, IsPrimary: false},
	{FirstName: "Ewald", LastName: "Busch", Email: "ewald.busch@email.de", MobilePhone: "+49 151 12345050", Relationship: "parent", StudentIndex: 49, IsPrimary: true},

	// Blumengruppe
	{FirstName: "Thomas", LastName: "Richter", Email: "thomas.richter@email.de", Phone: "+49 221 555005", Relationship: "parent", StudentIndex: 50, IsPrimary: true},
	{FirstName: "Monika", LastName: "Richter", Email: "monika.richter@email.de", MobilePhone: "+49 151 12345005", Relationship: "parent", StudentIndex: 50, IsPrimary: false},
	{FirstName: "Carlos", LastName: "Gómez", Email: "carlos.gomez@email.de", MobilePhone: "+49 151 12345052", Relationship: "parent", StudentIndex: 51, IsPrimary: true},
	{FirstName: "Elfriede", LastName: "Franke", Email: "elfriede.franke@email.de", MobilePhone: "+49 151 12345053", Relationship: "parent", StudentIndex: 52, IsPrimary: true},
	{FirstName: "Norbert", LastName: "Friedrich", Email: "norbert.friedrich@email.de", MobilePhone: "+49 151 12345054", Relationship: "parent", StudentIndex: 53, IsPrimary: true},
	{FirstName: "Linh", LastName: "Nguyen", Email: "linh.nguyen@email.de", MobilePhone: "+49 151 12345055", Relationship: "parent", StudentIndex: 54, IsPrimary: true},
	{FirstName: "Friedhelm", LastName: "Haas", Email: "friedhelm.haas@email.de", Phone: "+49 221 555056", Relationship: "parent", StudentIndex: 55, IsPrimary: true},
	{FirstName: "Mei", LastName: "Wang", Email: "mei.wang@email.de", MobilePhone: "+49 151 12345057", Relationship: "parent", StudentIndex: 56, IsPrimary: true},
	{FirstName: "Lothar", LastName: "Henkel", Email: "lothar.henkel@email.de", MobilePhone: "+49 151 12345058", Relationship: "parent", StudentIndex: 57, IsPrimary: true},
	{FirstName: "Brigitte", LastName: "Hesse", Email: "brigitte.hesse@email.de", MobilePhone: "+49 151 12345059", Relationship: "parent", StudentIndex: 58, IsPrimary: true},
	{FirstName: "Ottmar", LastName: "Horn", Email: "ottmar.horn@email.de", Phone: "+49 221 555060", Relationship: "parent", StudentIndex: 59, IsPrimary: true},

	// Schmetterlingsgruppe
	{FirstName: "Leila", LastName: "Mansour", Email: "leila.mansour@email.de", MobilePhone: "+49 151 12345061", Relationship: "parent", StudentIndex: 60, IsPrimary: true},
	{FirstName: "Karim", LastName: "Mansour", Email: "karim.mansour@email.de", MobilePhone: "+49 160 98765061", Relationship: "parent", StudentIndex: 60, IsPrimary: false},
	{FirstName: "Erwin", LastName: "Kerner", Email: "erwin.kerner@email.de", MobilePhone: "+49 151 12345062", Relationship: "parent", StudentIndex: 61, IsPrimary: true},
	{FirstName: "Grace", LastName: "Sango", Email: "grace.sango@email.de", MobilePhone: "+49 151 12345063", Relationship: "parent", StudentIndex: 62, IsPrimary: true},
	{FirstName: "Wei", LastName: "Zhang", Email: "wei.zhang@email.de", MobilePhone: "+49 151 12345064", Relationship: "parent", StudentIndex: 63, IsPrimary: true},
	{FirstName: "Yuki", LastName: "Satō", Email: "yuki.sato@email.de", MobilePhone: "+49 151 12345065", Relationship: "parent", StudentIndex: 64, IsPrimary: true},
	{FirstName: "Armin", LastName: "Lehmann", Email: "armin.lehmann@email.de", MobilePhone: "+49 151 12345066", Relationship: "parent", StudentIndex: 65, IsPrimary: true},
	{FirstName: "Rosemarie", LastName: "Lorenz", Email: "rosemarie.lorenz@email.de", Phone: "+49 221 555067", Relationship: "parent", StudentIndex: 66, IsPrimary: true},
	{FirstName: "Sergej", LastName: "Wolkow", Email: "sergej.wolkow@email.de", MobilePhone: "+49 151 12345068", Relationship: "parent", StudentIndex: 67, IsPrimary: true},
	{FirstName: "Christa", LastName: "Mayer", Email: "christa.mayer2@email.de", MobilePhone: "+49 151 12345069", Relationship: "parent", StudentIndex: 68, IsPrimary: true},
	{FirstName: "Klaus-Dieter", LastName: "Menzel", Email: "klausdieter.menzel@email.de", Phone: "+49 221 555070", Relationship: "parent", StudentIndex: 69, IsPrimary: true},

	// Waldgruppe
	{FirstName: "Anneliese", LastName: "Naumann", Email: "anneliese.naumann@email.de", MobilePhone: "+49 151 12345071", Relationship: "parent", StudentIndex: 70, IsPrimary: true},
	{FirstName: "Dietmar", LastName: "Otto", Email: "dietmar.otto@email.de", MobilePhone: "+49 151 12345072", Relationship: "parent", StudentIndex: 71, IsPrimary: true},
	{FirstName: "Kathy", LastName: "Baker", Email: "kathy.baker@email.de", MobilePhone: "+49 151 12345024", Relationship: "parent", StudentIndex: 72, IsPrimary: true},
	{FirstName: "James", LastName: "Baker", Email: "james.baker@email.de", MobilePhone: "+49 160 98765024", Relationship: "parent", StudentIndex: 72, IsPrimary: false},
	{FirstName: "Minh", LastName: "Tran", Email: "minh.tran@email.de", MobilePhone: "+49 151 12345074", Relationship: "parent", StudentIndex: 73, IsPrimary: true},
	{FirstName: "Anouk", LastName: "Smit", Email: "anouk.smit@email.de", MobilePhone: "+49 151 12345075", Relationship: "parent", StudentIndex: 74, IsPrimary: true},
	{FirstName: "Helmut", LastName: "Sauer", Email: "helmut.sauer@email.de", MobilePhone: "+49 151 12345076", Relationship: "parent", StudentIndex: 75, IsPrimary: true},
	{FirstName: "Edeltraud", LastName: "Schäfer", Email: "edeltraud.schaefer@email.de", Phone: "+49 221 555077", Relationship: "parent", StudentIndex: 76, IsPrimary: true},
	{FirstName: "Lars", LastName: "Sondre", Email: "lars.sondre@email.de", MobilePhone: "+49 151 12345078", Relationship: "parent", StudentIndex: 77, IsPrimary: true},
	{FirstName: "Johanna", LastName: "Schubert", Email: "johanna.schubert@email.de", MobilePhone: "+49 151 12345079", Relationship: "parent", StudentIndex: 78, IsPrimary: true},
	{FirstName: "Kerem", LastName: "Özdemir", Email: "kerem.oezdemir@email.de", MobilePhone: "+49 151 12345080", Relationship: "parent", StudentIndex: 79, IsPrimary: true},

	// Meeresgruppe
	{FirstName: "Isolde", LastName: "Simon", Email: "isolde.simon@email.de", MobilePhone: "+49 151 12345081", Relationship: "parent", StudentIndex: 80, IsPrimary: true},
	{FirstName: "Miguel", LastName: "Rodríguez", Email: "miguel.rodriguez@email.de", MobilePhone: "+49 151 12345082", Relationship: "parent", StudentIndex: 81, IsPrimary: true},
	{FirstName: "Irmgard", LastName: "Steiner", Email: "irmgard.steiner@email.de", Phone: "+49 221 555083", Relationship: "parent", StudentIndex: 82, IsPrimary: true},
	{FirstName: "Reinhard", LastName: "Stock", Email: "reinhard.stock@email.de", MobilePhone: "+49 151 12345084", Relationship: "parent", StudentIndex: 83, IsPrimary: true},
	{FirstName: "Akiko", LastName: "Suzuki", Email: "akiko.suzuki@email.de", MobilePhone: "+49 151 12345085", Relationship: "parent", StudentIndex: 84, IsPrimary: true},
	{FirstName: "Heinz-Georg", LastName: "Ulrich", Email: "heinzgeorg.ulrich@email.de", MobilePhone: "+49 151 12345086", Relationship: "parent", StudentIndex: 85, IsPrimary: true},
	{FirstName: "Lucía", LastName: "Gutiérrez", Email: "lucia.gutierrez@email.de", MobilePhone: "+49 151 12345087", Relationship: "parent", StudentIndex: 86, IsPrimary: true},
	{FirstName: "Konrad", LastName: "Voigt", Email: "konrad.voigt@email.de", MobilePhone: "+49 151 12345088", Relationship: "parent", StudentIndex: 87, IsPrimary: true},
	{FirstName: "Elfriede", LastName: "Walther", Email: "elfriede.walther@email.de", MobilePhone: "+49 151 12345089", Relationship: "parent", StudentIndex: 88, IsPrimary: true},
	{FirstName: "Kenji", LastName: "Takahashi", Email: "kenji.takahashi@email.de", MobilePhone: "+49 151 12345090", Relationship: "parent", StudentIndex: 89, IsPrimary: true},

	// Wiesengruppe
	{FirstName: "Gundula", LastName: "Wendt", Email: "gundula.wendt@email.de", MobilePhone: "+49 151 12345091", Relationship: "parent", StudentIndex: 90, IsPrimary: true},
	{FirstName: "Rahul", LastName: "Kumar", Email: "rahul.kumar@email.de", MobilePhone: "+49 151 12345092", Relationship: "parent", StudentIndex: 91, IsPrimary: true},
	{FirstName: "Renate", LastName: "Winter", Email: "renate.winter@email.de", Phone: "+49 221 555093", Relationship: "parent", StudentIndex: 92, IsPrimary: true},
	{FirstName: "Gerfried", LastName: "Wolff", Email: "gerfried.wolff@email.de", MobilePhone: "+49 151 12345094", Relationship: "parent", StudentIndex: 93, IsPrimary: true},
	{FirstName: "Anjali", LastName: "Patel", Email: "anjali.patel@email.de", MobilePhone: "+49 151 12345095", Relationship: "parent", StudentIndex: 94, IsPrimary: true},
	{FirstName: "Winfried", LastName: "Ziegler", Email: "winfried.ziegler@email.de", MobilePhone: "+49 151 12345096", Relationship: "parent", StudentIndex: 95, IsPrimary: true},
	{FirstName: "Wiebke", LastName: "Gritzmann", Email: "wiebke.gritzmann@email.de", MobilePhone: "+49 151 12345097", Relationship: "parent", StudentIndex: 96, IsPrimary: true},
	{FirstName: "Claudia", LastName: "Bader", Email: "claudia.bader@email.de", MobilePhone: "+49 151 12345020", Relationship: "parent", StudentIndex: 97, IsPrimary: true},
	{FirstName: "Hartmut", LastName: "Bader", Email: "hartmut.bader@email.de", MobilePhone: "+49 151 12345098", Relationship: "parent", StudentIndex: 97, IsPrimary: false},
	{FirstName: "Rosalinde", LastName: "Bartsch", Email: "rosalinde.bartsch@email.de", MobilePhone: "+49 151 12345099", Relationship: "parent", StudentIndex: 98, IsPrimary: true},
	{FirstName: "Bernhard", LastName: "Bergmann", Email: "bernhard.bergmann@email.de", Phone: "+49 221 555100", Relationship: "parent", StudentIndex: 99, IsPrimary: true},
	{FirstName: "Joyce", LastName: "Bergmann", Email: "joyce.bergmann@email.de", MobilePhone: "+49 160 98765100", Relationship: "parent", StudentIndex: 99, IsPrimary: false},
}
