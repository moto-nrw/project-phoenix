package test

import (
	"strings"
	"testing"
)

func TestArrivalSchoolNoticeMigrationIsExactAndConsumedOnce(t *testing.T) {
	t.Parallel()
	for _, pair := range arrivalSchoolNoticeMigration {
		t.Run(pair[0], func(t *testing.T) {
			t.Parallel()
			old := losableSideEffectNotice{Reason: "unchanged loss case", Fingerprint: strings.Split(strings.Split(pair[0], "@")[1], "#")[0]}
			base := map[string]losableSideEffectNotice{pair[0]: old}
			migrated := arrivalSchoolNoticeBase(base)
			expected := map[string]losableSideEffectNotice{pair[1]: {Reason: old.Reason, Fingerprint: strings.Split(strings.Split(pair[1], "@")[1], "#")[0]}}
			if err := sideEffectRegisterSubset(expected, migrated); err != nil {
				t.Fatal(err)
			}
			if len(migrated) != 1 || migrated[pair[1]].Reason != old.Reason {
				t.Fatal("migration must preserve count and loss case")
			}
			altered := migrated[pair[1]]
			altered.Fingerprint = "added durable write"
			if err := sideEffectRegisterSubset(map[string]losableSideEffectNotice{pair[1]: altered}, migrated); err == nil {
				t.Fatal("changed body accepted")
			}
			altered = migrated[pair[1]]
			altered.Reason = "changed loss case"
			if err := sideEffectRegisterSubset(map[string]losableSideEffectNotice{pair[1]: altered}, migrated); err == nil {
				t.Fatal("changed reason accepted")
			}
			migrated[pair[0]] = old
			if err := sideEffectRegisterSubset(migrated, arrivalSchoolNoticeBase(base)); err == nil {
				t.Fatal("both old and new allowances accepted")
			}
			old.Fingerprint = "wrong base body"
			if got := arrivalSchoolNoticeBase(map[string]losableSideEffectNotice{pair[0]: old}); got[pair[1]].Fingerprint != "" {
				t.Fatal("unreviewed base accepted")
			}
			if len(base) != 1 || base[pair[0]].Fingerprint == "" {
				t.Fatal("migration mutated fetched base")
			}
		})
	}
}
