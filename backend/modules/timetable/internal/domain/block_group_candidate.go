package domain

// BlockGroupCandidate is the Timetable-owned data needed to resolve a
// block's OGS groups in one bounded query. A block can have several rows when
// it has several participant sources or planned children.
type BlockGroupCandidate struct {
	InstanceID               int64
	TemplateEducationGroupID *int64
	TargetGroupType          string
	TargetEducationGroupID   *int64
	StudentID                *int64
}
