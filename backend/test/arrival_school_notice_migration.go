package test

import "strings"

// ADR 0147: a finite contract migration, not an unchanged relocation. Each pair
// changes only the school address of the same losable after-commit hint.
var arrivalSchoolNoticeMigration = [][2]string{
	{"modules/careplan/http/arrival_schedule_handlers.go:Resource.bulkUpsertArrivalSchedules:RegisterAfterCommit@2142bede6047950437325f320c454140#1", "modules/careplan/http/arrival_schedule_handlers.go:Resource.bulkUpsertArrivalSchedules:RegisterAfterCommit@2f0d4e6b4b962a4855baff7789cbdfcc#1"},
	{"modules/careplan/http/arrival_schedule_handlers.go:Resource.createStudentArrivalException:RegisterAfterCommit@e1ef3c3b8c3a621df2c83eb1ba59b53b#1", "modules/careplan/http/arrival_schedule_handlers.go:Resource.createStudentArrivalException:RegisterAfterCommit@4944679c73047e107c76d83c1bee45e9#1"},
	{"modules/careplan/http/arrival_schedule_handlers.go:Resource.createStudentArrivalNote:RegisterAfterCommit@15bf7b090c9012f2394fe6a3349172b3#1", "modules/careplan/http/arrival_schedule_handlers.go:Resource.createStudentArrivalNote:RegisterAfterCommit@f8ac0d6c44dfc7415d775d3d9baa9cde#1"},
	{"modules/careplan/http/arrival_schedule_handlers.go:Resource.deleteStudentArrivalException:RegisterAfterCommit@e1ef3c3b8c3a621df2c83eb1ba59b53b#1", "modules/careplan/http/arrival_schedule_handlers.go:Resource.deleteStudentArrivalException:RegisterAfterCommit@4944679c73047e107c76d83c1bee45e9#1"},
	{"modules/careplan/http/arrival_schedule_handlers.go:Resource.deleteStudentArrivalNote:RegisterAfterCommit@15bf7b090c9012f2394fe6a3349172b3#1", "modules/careplan/http/arrival_schedule_handlers.go:Resource.deleteStudentArrivalNote:RegisterAfterCommit@f8ac0d6c44dfc7415d775d3d9baa9cde#1"},
	{"modules/careplan/http/arrival_schedule_handlers.go:Resource.updateStudentArrivalException:RegisterAfterCommit@e1ef3c3b8c3a621df2c83eb1ba59b53b#1", "modules/careplan/http/arrival_schedule_handlers.go:Resource.updateStudentArrivalException:RegisterAfterCommit@4944679c73047e107c76d83c1bee45e9#1"},
	{"modules/careplan/http/arrival_schedule_handlers.go:Resource.updateStudentArrivalNote:RegisterAfterCommit@15bf7b090c9012f2394fe6a3349172b3#1", "modules/careplan/http/arrival_schedule_handlers.go:Resource.updateStudentArrivalNote:RegisterAfterCommit@f8ac0d6c44dfc7415d775d3d9baa9cde#1"},
	{"modules/careplan/http/arrival_schedule_handlers.go:Resource.updateStudentArrivalSchedules:RegisterAfterCommit@e1ef3c3b8c3a621df2c83eb1ba59b53b#1", "modules/careplan/http/arrival_schedule_handlers.go:Resource.updateStudentArrivalSchedules:RegisterAfterCommit@4944679c73047e107c76d83c1bee45e9#1"},
	{"modules/careplan/http/class_arrival_exception_handlers.go:Resource.deleteClassArrivalException:RegisterAfterCommit@90f5a59ac4a3f1867d090f3a6fd15ad4#1", "modules/careplan/http/class_arrival_exception_handlers.go:Resource.deleteClassArrivalException:RegisterAfterCommit@302ee7a29691de34727118d6ee41a904#1"},
	{"modules/careplan/http/class_arrival_exception_handlers.go:Resource.putClassArrivalException:RegisterAfterCommit@90f5a59ac4a3f1867d090f3a6fd15ad4#1", "modules/careplan/http/class_arrival_exception_handlers.go:Resource.putClassArrivalException:RegisterAfterCommit@302ee7a29691de34727118d6ee41a904#1"},
	{"modules/careplan/http/pickup_adjustment_handlers.go:Resource.applyStudentPickupAdjustment:RegisterAfterCommit@59cb70050ac1a3b29cb62b0a0273fc4c#1", "modules/careplan/http/pickup_adjustment_handlers.go:Resource.applyStudentPickupAdjustment:RegisterAfterCommit@7ab43b5f508d9ea67a5b553532cdca7f#1"},
	{"modules/careplan/internal/application/care_request_effects.go:CareRequestEffects.broadcastCareScheduleChanges:AfterCommit@ebb47aca05e7275a279183ba56d2fb3f#1", "modules/careplan/internal/application/care_request_effects.go:CareRequestEffects.broadcastCareScheduleChanges:AfterCommit@c1914ed0e54f7fa6f546259ffc26bae6#1"},
	{"modules/delivery/internal/application/realtimeevents/arrival_schedule.go:QueueArrivalScheduleChanged:RegisterAfterCommit@db38e6fbacbf4eee93ba329fc304d0c5#1", "modules/delivery/internal/application/realtimeevents/arrival_schedule.go:QueueArrivalScheduleChanged:RegisterAfterCommit@291c46f59a7cfdf1cfdfa9669b75e065#1"},
}

// arrivalSchoolNoticeBase consumes each exact old allowance once. Reasons are
// inherited from the fetched base, never granted by the changed register.
// An already migrated base is unchanged; keeping both versions is rejected.
func arrivalSchoolNoticeBase(previous map[string]losableSideEffectNotice) map[string]losableSideEffectNotice {
	result := make(map[string]losableSideEffectNotice, len(previous))
	for key, notice := range previous {
		result[key] = notice
	}
	for _, pair := range arrivalSchoolNoticeMigration {
		old, exists := result[pair[0]]
		if !exists {
			continue
		}
		oldFingerprint := strings.Split(strings.Split(pair[0], "@")[1], "#")[0]
		if old.Fingerprint != oldFingerprint {
			continue
		}
		if _, exists := result[pair[1]]; exists {
			continue
		}
		delete(result, pair[0])
		old.Fingerprint = strings.Split(strings.Split(pair[1], "@")[1], "#")[0]
		result[pair[1]] = old
	}
	return result
}
