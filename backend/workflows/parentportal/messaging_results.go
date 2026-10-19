package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

type MessageThreadView struct {
	ThreadID    int64
	StudentID   int64
	StudentName string
	SchoolName  string
	Messages    []*ParentMessage
}
type DeclarationProof struct {
	Title       string
	SchoolName  string
	ChildName   string
	Kind        string
	Versions    map[int64]*DeclarationVersion
	Submissions []*DeclarationSubmission
	IntegrityOK bool
}

func messagingFromAppMessageThreadView(v messaging.MessageThreadView) MessageThreadView {
	return MessageThreadView{ThreadID: v.ThreadID, StudentID: v.StudentID, StudentName: v.StudentName, SchoolName: v.SchoolName, Messages: messagingValueSlice(v.Messages, func(row *messaging.ParentMessage) *ParentMessage {
		return messagingValuePointer(row, messagingFromAppParentMessage)
	})}
}
func messagingFromAppDeclarationProof(v messaging.DeclarationProof) DeclarationProof {
	return DeclarationProof{Title: v.Title, SchoolName: v.SchoolName, ChildName: v.ChildName, Kind: v.Kind, Versions: messagingValueMap(v.Versions, func(row *messaging.DeclarationVersion) *DeclarationVersion {
		return messagingValuePointer(row, messagingFromAppDeclarationVersion)
	}), Submissions: messagingValueSlice(v.Submissions, func(row *messaging.DeclarationSubmission) *DeclarationSubmission {
		return messagingValuePointer(row, messagingFromAppDeclarationSubmission)
	}), IntegrityOK: v.IntegrityOK}
}
