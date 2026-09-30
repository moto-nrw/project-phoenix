package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	staff "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffannouncements"
)

// ParentDeclarationStatus is the staff view of an Erklärung (#3430).
func (p *parentAnnouncements) ParentDeclarationStatus(ctx context.Context, id int64) (*communication.ParentDeclarationStatus, error) {
	status, err := p.service.ParentDeclarationStatus(ctx, id)
	if err != nil {
		return nil, mapParentAnnouncementError(err)
	}
	return mapDeclarationStatus(status), nil
}

func mapDeclarationStatus(s *staff.DeclarationStatus) *communication.ParentDeclarationStatus {
	out := &communication.ParentDeclarationStatus{
		Title: s.Title, Settings: communication.ParentDeclarationSettings(s.Settings), Deadline: s.Deadline,
		Summary:     communication.DeclarationSummary{ChildrenTotal: s.ChildrenTotal, ByState: s.Summary},
		GeneratedAt: s.GeneratedAt, IntegrityAllGood: s.IntegrityAllGood,
	}
	for _, v := range s.Versions {
		out.Versions = append(out.Versions, mapDeclarationVersion(v))
	}
	if s.CurrentVersion != nil {
		current := mapDeclarationVersion(*s.CurrentVersion)
		out.CurrentVersion = &current
	}
	for _, c := range s.Children {
		child := communication.DeclarationChildStatus{
			StudentID: c.StudentID, FirstName: c.FirstName, LastName: c.LastName, SchoolClass: c.SchoolClass,
			State: c.State, Signers: make([]communication.DeclarationSignerStatus, 0, len(c.Signers)),
		}
		for _, signer := range c.Signers {
			child.Signers = append(child.Signers, communication.DeclarationSignerStatus(signer))
		}
		out.Children = append(out.Children, child)
	}
	for _, sub := range s.Submissions {
		out.Submissions = append(out.Submissions, communication.DeclarationSubmissionRecord(sub))
	}
	return out
}

func mapDeclarationVersion(v staff.DeclarationVersionView) communication.DeclarationVersion {
	version := communication.DeclarationVersion{
		ID: v.ID, VersionNo: v.VersionNo, Title: v.Title, Body: v.Body, Kind: v.Kind,
		ContentHash: v.ContentHash, PublishedAt: v.PublishedAt, IntegrityOK: v.IntegrityOK,
		Attachments: make([]communication.DeclarationAttachment, 0, len(v.Attachments)),
	}
	for _, a := range v.Attachments {
		version.Attachments = append(version.Attachments, communication.DeclarationAttachment(a))
	}
	return version
}
