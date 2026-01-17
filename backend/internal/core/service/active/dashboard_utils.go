package active

import "github.com/moto-nrw/project-phoenix/internal/core/domain/active"

// extractUniqueStudentIDs extracts unique student IDs from visits
func extractUniqueStudentIDs(visits []*active.Visit) []int64 {
	studentIDSet := make(map[int64]struct{})
	studentIDs := make([]int64, 0, len(visits))

	for _, visit := range visits {
		if _, exists := studentIDSet[visit.StudentID]; !exists {
			studentIDs = append(studentIDs, visit.StudentID)
			studentIDSet[visit.StudentID] = struct{}{}
		}
	}

	return studentIDs
}

// calculateStudentsInTransit counts students with attendance but no active visit
func calculateStudentsInTransit(studentsWithAttendance, studentsWithActiveVisits map[int64]bool) int {
	count := 0
	for studentID := range studentsWithAttendance {
		if !studentsWithActiveVisits[studentID] {
			count++
		}
	}
	return count
}
