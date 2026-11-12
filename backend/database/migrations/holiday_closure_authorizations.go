package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// holidayClosureAuthorizationsUpSQL is the actor ledger of ADR 0196. The
// event anchors its lifecycle; renewing a command preserves the original
// operation, revision, actor, channel and time of each earlier authorization.
const holidayClosureAuthorizationsUpSQL = `
CREATE UNIQUE INDEX uq_holiday_closure_events_tenant_id ON calendar.holiday_closure_events (tenant_id, id);
CREATE TABLE calendar.holiday_closure_authorizations (
 id BIGSERIAL PRIMARY KEY,
 tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
 event_id BIGINT NOT NULL,
 operation_id UUID NOT NULL,
 revision BIGINT NOT NULL,
 account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
 channel TEXT NOT NULL CHECK (channel IN ('school', 'traeger')),
 occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT fk_holiday_closure_authorizations_event FOREIGN KEY (tenant_id, event_id)
  REFERENCES calendar.holiday_closure_events (tenant_id, id) ON DELETE CASCADE
);
ALTER TABLE calendar.holiday_closure_authorizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE calendar.holiday_closure_authorizations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_holiday_closure_authorizations ON calendar.holiday_closure_authorizations FOR ALL
 USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
 WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
GRANT SELECT, INSERT ON calendar.holiday_closure_authorizations TO phoenix_tenant;
GRANT UPDATE (event_id) ON calendar.holiday_closure_authorizations TO phoenix_tenant;
GRANT USAGE ON SEQUENCE calendar.holiday_closure_authorizations_id_seq TO phoenix_tenant;
COMMENT ON TABLE calendar.holiday_closure_authorizations IS 'Atomic approval history: operation and revision identify each authorization; event_id anchors retention across renewed commands. Actor is anonymized on account erasure (ADR 0196).';
`

func installHolidayClosureAuthorizations(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayClosureAuthorizationsUpSQL); err != nil {
		return fmt.Errorf("create holiday closure authorization history: %w", err)
	}
	return nil
}
