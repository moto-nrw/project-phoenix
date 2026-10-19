package messaging

// Poll and declaration mode are facts supplied by the canonical owner.
func (v ParentAnnouncement) IsPoll() bool        { return v.Poll }
func (v ParentAnnouncement) IsDeclaration() bool { return v.DeclarationMode }
func (v ParentAnnouncement) GetTenantID() int64  { return v.TenantID }
func (v AnnouncementFeedItem) IsPoll() bool      { return v.Poll }
func (v AnnouncementFeedScope) IsEmpty() bool {
	return len(v.TenantIDs) == 0 && len(v.SystemOnlyTenantIDs) == 0
}
