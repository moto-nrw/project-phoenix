package api

import (
	"errors"
	"fmt"
	"slices"
)

// knownHolidayChildStudentIndex is the demo child (Leon Koch, Klasse 1a)
// whose family registers him for the holidays as a holiday guest of another
// school (#6683). The hosting school manages him, so the Träger-Büro sees
// the hint „Bekannt an …“ and checks his school.
const knownHolidayChildStudentIndex = 2

// seedHolidayKnownChild registers the demo child as a holiday guest for the
// last week of the siblings' guest and lets the Träger-Büro compare the
// registered children with the children of its schools, which records the
// comparison. A run without the fixed seeder, or a server that refuses the
// registration, keeps the period without the hint.
func seedHolidayKnownChild(rt *Runtime, auth AuthRef, token string, periodID int64, registration map[string]any) error {
	if rt.FixedSeeder == nil || knownHolidayChildStudentIndex >= len(DemoStudents) {
		return nil
	}
	student := DemoStudents[knownHolidayChildStudentIndex]
	children, _ := registration["children"].([]map[string]any)
	if len(children) < 2 {
		return fmt.Errorf("the seeded holiday registration has no holiday guest")
	}
	guestBookings, _ := children[1]["bookings"].([]map[string]any)
	if len(guestBookings) == 0 {
		return fmt.Errorf("the seeded holiday guest has no booking")
	}
	known := map[string]any{}
	for key, value := range registration {
		known[key] = value
	}
	known["guardian"] = map[string]any{"first_name": "Nadine", "last_name": student.LastName, "email": "nadine.koch@example.org", "phone": "0151 0000004"}
	known["guardian_address"] = map[string]any{"street": "Lindenstraße 3", "postal_code": "49143", "city": "Bissendorf"}
	known["declaration_name"] = "Nadine " + student.LastName
	delete(known, "second_guardian")
	delete(known, "benefits_declared")
	delete(known, "benefits_proof_tokens")
	known["children"] = []map[string]any{{
		"first_name": student.FirstName, "last_name": student.LastName,
		"birth_date":     demoStudentBirthday(knownHolidayChildStudentIndex, student, rt.FixedSeeder.seedDay),
		"home_school_id": 0, "guest_school_name": "Grundschule am Park",
		"bookings":    guestBookings[len(guestBookings)-1:],
		"custom_data": children[1]["custom_data"],
	}}
	_, err := rt.Client.PostPublic("/traeger/holiday-registrations/"+token, known)
	var refused *APIError
	if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
		fmt.Printf("  WARNING: the server refuses the known child's registration (%s); no hint on a known child\n", refused.Code)
		return nil
	}
	if err != nil {
		return fmt.Errorf("register the known holiday child: %w", err)
	}
	if _, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/registrations/known-children", periodID)); err != nil {
		return fmt.Errorf("compare the holiday children with the children of the schools: %w", err)
	}
	fmt.Printf("  Träger-Büro: %s %s registered as a holiday guest, known at the hosting school\n", student.FirstName, student.LastName)
	return nil
}
