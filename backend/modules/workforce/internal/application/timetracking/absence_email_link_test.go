package timetracking

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAbsenceEmailLinkRequiresSchoolSubdomain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		school absenceEmailSchoolFinderStub
		want   string
	}{
		{"found", absenceEmailSchoolFinderStub{subdomain: "school", found: true}, "http://school.localhost:3000/staff"},
		{"missing school", absenceEmailSchoolFinderStub{}, ""},
		{"lookup failure", absenceEmailSchoolFinderStub{err: errors.New("school lookup unavailable")}, ""},
		{"missing subdomain", absenceEmailSchoolFinderStub{found: true}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &staffAbsenceService{emailDeps: &AbsenceEmailDeps{SchoolRepo: tc.school, FrontendURL: "http://localhost:3000"}}
			absence := notificationTestAbsence(AbsenceStatusRequested)
			link, ok := svc.absenceEmailLink(context.Background(), absence, "/staff")
			require.Equal(t, tc.want, link)
			require.Equal(t, tc.want != "", ok)
		})
	}
}
