package enrollment

import (
	"context"
	"time"
)

// StudentAnswers reads what parents answered to the school's own questions of
// the registration form for a child that became a student (#6954). The
// answers stay on the registration (request_children.custom_data); the child
// record reads them by the link the approval stored (created_student_id), as
// the child's page does (GET /students/{id}/enrollment-extra-fields). Office
// matches are left out there and here: their answers belong to another
// child's registration.
//
// Only free questions count: a question with a target was copied onto the
// child record on approval and is read there, file questions carry no answer
// text, and the structured types belong to their targets. Every read runs in
// the caller's tenant transaction. Callers check users:read first.
type StudentAnswers interface {
	// StudentAnswerFilters lists the yes/no and choice questions with at
	// least one answer, each with the answers given and the children who gave
	// them. The newest form version names a question and its options. The
	// children are every linked child, also one who has left the school: the
	// caller counts the ones its list shows.
	StudentAnswerFilters(ctx context.Context) ([]StudentAnswerFilter, error)
	// StudentsWithAnswer returns the children whose newest answer to the
	// question key equals value ("true"/"false" for yes/no, the option value
	// for a choice). An unknown key or value finds none.
	StudentsWithAnswer(ctx context.Context, key, value string) ([]int64, error)
	// SearchStudentAnswers finds children whose answer or question matches
	// text, folded like a name search, newest registrations first. A
	// question matches only where the answer says something: yes, a choice,
	// a text, a number or a date, never no. A limit of zero or less returns
	// every child found; the children may include ones who have left the
	// school.
	SearchStudentAnswers(ctx context.Context, text string, limit int) ([]StudentAnswerMatch, error)
}

// StudentAnswerFilter is one question a list can filter by.
type StudentAnswerFilter struct {
	Key     string                `json:"key"`
	Label   string                `json:"label"`
	Type    FormFieldType         `json:"type"`
	Options []StudentAnswerOption `json:"options"`
}

// StudentAnswerOption is one answer of a filter question with the children
// whose newest answer it is.
type StudentAnswerOption struct {
	Value      string
	Label      string
	StudentIDs []int64
}

// StudentAnswerMatch is a child found through an answer: the question and the
// answer as the school reads them.
type StudentAnswerMatch struct {
	StudentID int64
	Label     string
	Answer    string
}

// LinkedChildAnswers is one registration child linked to a student: the raw
// answers and the form version they were given in, as the store reads them.
type LinkedChildAnswers struct {
	StudentID   int64
	SchemaID    int64
	SubmittedAt time.Time
	RequestID   int64
	CustomData  FormValues
}
