package enrollment

type DeletionRequestCounts struct {
	Requests                  int
	GuardianAccountID         *int64
	RequestChildren           int
	RequestChildOfferings     int
	RequestGuardians          int
	ChangeRequests            int
	ChangeRequestMessages     int
	LateInvites               int
	OfferingAdjustments       int
	EmailOutbox               int
	RolloverLinksCleared      int
	StudentSourceLinksCleared int
}
type DeletionChildCounts struct {
	Offerings             int
	ChangeRequests        int
	ChangeRequestMessages int
	OfferingAdjustments   int
	RolloverLinks         int
	StudentSourceLinks    int
}
type DeletionChildTarget struct {
	TargetChildren int
	AllChildren    int
}
