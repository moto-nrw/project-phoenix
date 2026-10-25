package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// seedEnrollmentPlacementStep places the children the online enrollment
// admitted. Approval creates them with their grade as the class ("1") and
// without a group; a school in its running year completes both. The demo
// school shows them like every other child: in a class of their grade and the
// group of that class.
type seedEnrollmentPlacementStep struct{}

func (seedEnrollmentPlacementStep) Name() string { return "Placing admitted enrollment children" }

func (seedEnrollmentPlacementStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	students, err := listSeedStudentClasses(rt)
	if err != nil {
		return err
	}
	classes := demoClassesByGrade()
	placed := map[string]int{}
	for _, student := range students {
		options := classes[student.SchoolClass]
		if len(options) == 0 {
			continue // a real class already, or a grade the demo has no class for
		}
		option := options[placed[student.SchoolClass]%len(options)]
		placed[student.SchoolClass]++
		groupID := rt.FixedSeeder.groupIDs[option.groupKey]
		if groupID == 0 {
			return fmt.Errorf("group %s not available for class %s", option.groupKey, option.class)
		}
		if _, err := rt.Client.Put("/api/students/"+student.ID.String(), map[string]any{
			"school_class": option.class, "group_id": groupID,
		}); err != nil {
			return fmt.Errorf("place admitted child %s in %s: %w", student.ID, option.class, err)
		}
	}
	return nil
}

type seedStudentClass struct {
	ID          json.Number `json:"id"`
	SchoolClass string      `json:"school_class"`
}

func listSeedStudentClasses(rt *Runtime) ([]seedStudentClass, error) {
	var all []seedStudentClass
	for page := 1; ; page++ {
		raw, err := rt.Client.Get(fmt.Sprintf("/api/students?page=%d&page_size=100", page))
		if err != nil {
			return nil, fmt.Errorf("read students for placement: %w", err)
		}
		var envelope struct {
			Data       []seedStudentClass `json:"data"`
			Pagination struct {
				TotalPages int `json:"total_pages"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, fmt.Errorf("decode students for placement: %w", err)
		}
		all = append(all, envelope.Data...)
		if len(envelope.Data) == 0 || page >= envelope.Pagination.TotalPages {
			return all, nil
		}
	}
}

type demoClassOption struct{ class, groupKey string }

// demoClassesByGrade maps a bare grade ("1") to the demo classes of that grade
// ("Klasse 1a", "Klasse 1b") with the group their children belong to.
func demoClassesByGrade() map[string][]demoClassOption {
	byGrade := map[string][]demoClassOption{}
	seen := map[string]bool{}
	for _, student := range DemoStudents {
		grade, ok := strings.CutPrefix(student.Class, "Klasse ")
		if !ok || len(grade) < 2 || seen[student.Class] {
			continue
		}
		seen[student.Class] = true
		byGrade[grade[:1]] = append(byGrade[grade[:1]], demoClassOption{class: student.Class, groupKey: student.GroupKey})
	}
	return byGrade
}
