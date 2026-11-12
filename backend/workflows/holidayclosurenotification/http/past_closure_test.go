package http_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A change of a closing day that has passed proposes nothing new, but still
// resolves the commands of its source (#6806): a command that waits for
// renewed approval shows 0 families once its days are past, so nobody can
// approve it, and worker and retirement would pass it by forever.
func TestPastClosingDayChangeResolvesItsWaitingCommand(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t) // ProcessDue sweeps every Träger
	for _, mode := range []string{"deleted", "edited", "deleted_reaching_today"} {
		t.Run(mode, func(t *testing.T) {
			testpkg.OwnTenant(t)
			w := newClosureWorldIn(t, "Träger vergangener Schließtag "+mode, db)
			day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
			first := w.schoolNotices(t)[0]
			events := schoolCalendarCompose.NewHolidayClosureEvents()
			require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
				_, err := events.ConfirmHolidayClosureNotice(ctx, first.ID, first.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: first.Token})
				return err
			}))
			// An unrelated calendar revision makes the confirmed command await approval.
			w.createClosingDay(t, "2030-08-20", "2030-08-20")
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			require.Equal(t, []string{"needs_confirmation"}, w.events(t))

			// Time passes: the closing day, and unless the command still
			// reaches today its span too, lie years back.
			_, err := w.db.ExecContext(w.ctx, `UPDATE schedule.closing_days SET start_date = start_date - interval '10 years',
				end_date = end_date - interval '10 years' WHERE tenant_id = ? AND id = ?`, w.school, day.ID)
			require.NoError(t, err)
			if mode != "deleted_reaching_today" {
				// The command fields never change; only the clock moves them.
				require.NoError(t, w.db.RunInTx(w.ctx, nil, func(ctx context.Context, tx bun.Tx) error {
					if _, err := tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
						return err
					}
					_, err := tx.ExecContext(ctx, `UPDATE calendar.holiday_closure_events SET span_from = span_from - interval '10 years',
						span_to = span_to - interval '10 years' WHERE tenant_id = ?`, w.school)
					return err
				}))
				_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_proposals SET span_from = span_from - interval '10 years',
					span_to = span_to - interval '10 years', current_start = current_start - interval '10 years',
					current_end = current_end - interval '10 years' WHERE tenant_id = ? AND source_id = ?`, w.school, day.ID)
				require.NoError(t, err)
			}

			if mode == "edited" {
				_, err = w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{
					StartDate: "2020-07-09", EndDate: "2020-07-09", Reason: "Nachgetragen"}})
				require.NoError(t, err)
				assert.Zero(t, w.openProposals(t, day.ID), "a change of past days proposes nothing")
				require.NoError(t, w.notices.ProcessDue(w.ctx))
				assert.Empty(t, w.events(t), "a command that waits for approval of long past days is retired")
				assert.Empty(t, w.mails(t))
				return
			}
			require.NoError(t, w.records.DeleteClosingDay(w.schoolCtx(), day.ID))
			assert.Equal(t, []string{"no_mail"}, w.events(t), "the deletion resolves the command that waited for approval and told nobody")
			if mode == "deleted_reaching_today" {
				assert.Equal(t, 1, w.openProposals(t, day.ID), "days of the command that still lie ahead keep their cancellation proposal")
			} else {
				assert.Zero(t, w.openProposals(t, day.ID), "a deletion of past days proposes nothing")
			}
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Empty(t, w.mails(t))
		})
	}
}

func (w closureWorld) openProposals(t *testing.T, closingDayID int64) int {
	t.Helper()
	var count int
	require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM calendar.holiday_closure_proposals
		WHERE tenant_id = ? AND source_id = ? AND state = 'open'`, w.school, closingDayID).Scan(context.Background(), &count))
	return count
}
