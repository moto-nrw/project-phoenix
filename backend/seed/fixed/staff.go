package fixed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// Staff notes for different roles
var staffNotes = []string{
	"Klassenleitung und Fachbereichsleitung",
	"Stellvertretende Klassenleitung",
	"Fachlehrkraft mit besonderen Aufgaben",
	"Betreuungskraft OGS",
	"Sozialpädagogische Fachkraft",
	"Verwaltungskraft",
	"Hausmeister",
	"Sekretariat",
	"IT-Beauftragte",
	"Sicherheitsbeauftragte",
}

// Teacher specializations and qualifications
var teacherData = []struct {
	specialization string
	role           string
	qualifications string
}{
	{"Mathematik", "Lehrerin für Mathematik", "Master of Education Mathematik, 10 Jahre Erfahrung"},
	{"Mathematik", "Mathematiklehrer", "Bachelor of Education Mathematik, 5 Jahre Erfahrung"},
	{"Naturwissenschaften", "Fachbereichsleiter Naturwissenschaften", "Promotion in Physik, 15 Jahre Erfahrung"},
	{"Naturwissenschaften", "Naturwissenschaftslehrerin", "Master Chemie, 7 Jahre Erfahrung"},
	{"Deutsch", "Fachbereichsleiter Deutsch", "Master Deutsche Literatur, 12 Jahre Erfahrung"},
	{"Deutsch", "Deutschlehrerin", "Bachelor Germanistik, 3 Jahre Erfahrung"},
	{"Geschichte", "Geschichtslehrer", "Master Geschichte, 8 Jahre Erfahrung"},
	{"Geografie", "Geografielehrerin", "Bachelor of Education Geografie, 6 Jahre Erfahrung"},
	{"Sport", "Sportkoordinator", "Bachelor of Education Sport, 10 Jahre Erfahrung"},
	{"Sport", "Sportlehrerin", "Sportwissenschaft, 4 Jahre Erfahrung"},
	{"Kunst", "Kunstlehrer", "Bachelor Bildende Kunst, 5 Jahre Erfahrung"},
	{"Musik", "Musiklehrerin", "Bachelor Musik, 7 Jahre Erfahrung"},
	{"Informatik", "IT-Lehrer", "Bachelor Informatik, 6 Jahre Erfahrung"},
	{"Fremdsprachen", "Spanischlehrerin", "Bachelor Spanisch, Muttersprachlerin"},
	{"Fremdsprachen", "Französischlehrer", "Master Französische Literatur, 9 Jahre Erfahrung"},
	{"Sonderpädagogik", "Sonderpädagogik-Koordinatorin", "Master Sonderpädagogik, 11 Jahre Erfahrung"},
	{"Bibliothek", "Bibliothekarin", "Master Bibliothekswissenschaft, 8 Jahre Erfahrung"},
	{"Beratung", "Schulberaterin", "Master Pädagogik, 13 Jahre Erfahrung"},
	{"Verwaltung", "Stellvertretende Schulleiterin", "Master Schulmanagement, 15 Jahre Erfahrung"},
	{"Verwaltung", "Schulleiter", "Promotion Pädagogik, 20 Jahre Erfahrung"},
}

// seedStaff creates staff records for the first 30 persons
func (s *Seeder) seedStaff(ctx context.Context) error {
	// First 30 persons become staff
	staffPersons := s.result.Persons[:30]

	for i, person := range staffPersons {
		staff := &users.Staff{
			PersonID:   person.ID,
			StaffNotes: staffNotes[i%len(staffNotes)],
		}
		staff.CreatedAt = time.Now()
		staff.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(staff).ModelTableExpr("users.staff").Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create staff for person %d: %w", person.ID, err)
		}

		s.result.Staff = append(s.result.Staff, staff)
	}

	if s.verbose {
		log.Printf("Created %d staff members", len(s.result.Staff))
	}

	return nil
}

// seedTeachers creates teacher records for the first 20 staff members
func (s *Seeder) seedTeachers(ctx context.Context) error {
	// First 20 staff members become teachers
	teacherStaff := s.result.Staff[:20]

	for i, staff := range teacherStaff {
		data := teacherData[i%len(teacherData)]

		teacher := &users.Teacher{
			StaffID:        staff.ID,
			Specialization: data.specialization,
			Role:           data.role,
			Qualifications: data.qualifications,
		}
		teacher.CreatedAt = time.Now()
		teacher.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(teacher).ModelTableExpr("users.teachers").Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create teacher for staff %d: %w", staff.ID, err)
		}

		s.result.Teachers = append(s.result.Teachers, teacher)
		s.result.TeacherByStaffID[staff.ID] = teacher
	}

	if s.verbose {
		log.Printf("Created %d teachers", len(s.result.Teachers))
	}

	return nil
}
