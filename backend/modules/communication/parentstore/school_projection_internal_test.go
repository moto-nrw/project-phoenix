package parentstore

import (
	"context"
	"errors"
	"testing"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
)

type announcementSchoolDirectory struct {
	recordports.SchoolDirectory
	lookup func(context.Context, int64) (string, error)
}

func (d announcementSchoolDirectory) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	return d.lookup(ctx, tenantID)
}

func TestNativeAnnouncementSchoolNameUsesSchoolDirectory(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const tenantID = 9007199254740993
	cause := errors.New("school lookup failed")
	for _, lookupErr := range []error{nil, cause} {
		stores := (MessagingRepositories{ParentAnnouncement: recordports.ParentAnnouncementRepository(nil)}).WithSchoolNames(announcementSchoolDirectory{
			lookup: func(gotCtx context.Context, gotTenantID int64) (string, error) {
				if gotCtx != ctx || gotTenantID != tenantID {
					t.Fatalf("school lookup received context %v and tenant %d", gotCtx, gotTenantID)
				}
				if lookupErr != nil {
					return "", lookupErr
				}
				return "OGS Testschule", nil
			},
		})
		name, err := stores.ParentAnnouncement.AnnouncementSchoolName(ctx, tenantID)
		if lookupErr != nil {
			if name != "" || !errors.Is(err, cause) || err.Error() != "resolve school name: school lookup failed" {
				t.Fatalf("school lookup failure = (%q, %v), want wrapped original error", name, err)
			}
		} else if err != nil || name != "OGS Testschule" {
			t.Fatalf("school lookup = (%q, %v), want directory name", name, err)
		}
	}
}
