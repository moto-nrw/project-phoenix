package application

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func toApplicationStudentWrite(write peopledirectory.StudentWrite) StudentWrite {
	return StudentWrite{
		Record:        domain.StudentRecord(write.Record),
		Plan:          write.Plan,
		Baseline:      write.Baseline,
		CompanionNote: write.CompanionNote,
		NoteSupplied:  write.NoteSupplied,
	}
}
