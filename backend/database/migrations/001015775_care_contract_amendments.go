package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractAmendmentsVersion = "1.15.775"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractAmendmentsVersion,
		Description: "Betreuungsvertrag: neue Version mit Wirkdatum bei geänderter Buchung (#4681)",
		DependsOn: []string{
			careContractConclusionVersion, // status aktiv and the version status guard (#4680)
		},
	})
	Migrations.MustRegister(careContractAmendmentsUp, careContractAmendmentsDown)
}

// careContractAmendmentsUpSQL lets an active contract follow a change of its
// booked care with a new version and its own effective date (#4681, ADR 0088
// section 2, ADR 0153 "Folgen"):
//
//   - contract_versions.source tells where a version came from: abschluss is
//     the first version of the contract, buchungsaenderung a later version
//     made from a changed booking that the school approved. #4816 decides by
//     it which version the parents sign again.
//   - The effective versions of a contract (wirksam and abgeloest) form one
//     timeline: a version holds from its effective date to the day before
//     the next one starts, or to its own end. At most one version is wirksam,
//     the newest; every earlier one is abgeloest. Two effective versions
//     never start on the same day.
//   - A new version is the newest (wirksam, starting after every effective
//     version) or goes before a planned one (abgeloest, with a later
//     effective version). A follow-up version (number 2 and up) never starts
//     before the day it becomes effective (Berlin date): no change reaches
//     back into a month that may be billed already. #4684 can tighten this
//     to the last issued invoice in the same trigger.
//   - A version that a change on its own first day replaces becomes ersetzt
//     (#4681 rule 3): it stays in the history and holds on no day. The status
//     guard of #4680 (1.15.773) is taken over and allows wirksam or abgeloest
//     to ersetzt besides its own steps.
const careContractAmendmentsUpSQL = `
ALTER TABLE care_contract.contract_versions
	ADD COLUMN source VARCHAR(24) NOT NULL DEFAULT 'abschluss';
ALTER TABLE care_contract.contract_versions
	ADD CONSTRAINT chk_contract_versions_source CHECK (source IN ('abschluss', 'buchungsaenderung'));
COMMENT ON COLUMN care_contract.contract_versions.source IS
	'Where the version came from: abschluss (first version) or buchungsaenderung (approved change of the booked care, #4681).';

CREATE UNIQUE INDEX uq_contract_versions_effective ON care_contract.contract_versions (contract_id)
	WHERE status = 'wirksam';
CREATE UNIQUE INDEX uq_contract_versions_timeline ON care_contract.contract_versions (contract_id, effective_from)
	WHERE status IN ('wirksam', 'abgeloest');

CREATE OR REPLACE FUNCTION care_contract.guard_version_timeline() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.status IN ('wirksam', 'abgeloest')
		AND (TG_OP = 'INSERT' OR OLD.status NOT IN ('wirksam', 'abgeloest')) THEN
		IF NEW.status = 'wirksam' AND EXISTS (SELECT 1 FROM care_contract.contract_versions AS v
			WHERE v.organization_id = NEW.organization_id AND v.contract_id = NEW.contract_id AND v.id <> NEW.id
				AND v.status IN ('wirksam', 'abgeloest') AND v.effective_from >= NEW.effective_from) THEN
			RAISE EXCEPTION 'care contract version of contract % must start after every effective version', NEW.contract_id
				USING ERRCODE = 'check_violation';
		END IF;
		IF NEW.status = 'abgeloest' AND NOT EXISTS (SELECT 1 FROM care_contract.contract_versions AS v
			WHERE v.organization_id = NEW.organization_id AND v.contract_id = NEW.contract_id AND v.id <> NEW.id
				AND v.status IN ('wirksam', 'abgeloest') AND v.effective_from > NEW.effective_from) THEN
			RAISE EXCEPTION 'care contract version of contract % inserted as superseded needs a later effective version', NEW.contract_id
				USING ERRCODE = 'check_violation';
		END IF;
		IF NEW.version_number > 1 AND NEW.effective_from < (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date THEN
			RAISE EXCEPTION 'care contract version of contract % cannot take effect in the past', NEW.contract_id
				USING ERRCODE = 'check_violation';
		END IF;
	END IF;
	RETURN NEW;
END;
$$;
CREATE OR REPLACE FUNCTION care_contract.guard_version_status() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.status IS DISTINCT FROM OLD.status AND NOT (
		(OLD.status = 'beantragt' AND NEW.status IN ('wirksam', 'abgelehnt'))
		OR (OLD.status = 'wirksam' AND NEW.status IN ('abgeloest', 'ersetzt'))
		OR (OLD.status = 'abgeloest' AND NEW.status = 'ersetzt')) THEN
		RAISE EXCEPTION 'care contract version % cannot move from % to %', OLD.id, OLD.status, NEW.status
			USING ERRCODE = 'check_violation';
	END IF;
	RETURN NEW;
END;
$$;
ALTER TABLE care_contract.contract_versions DROP CONSTRAINT chk_contract_versions_status;
ALTER TABLE care_contract.contract_versions ADD CONSTRAINT chk_contract_versions_status
	CHECK (status IN ('beantragt', 'wirksam', 'abgeloest', 'abgelehnt', 'ersetzt'));

CREATE TRIGGER guard_care_contract_version_timeline
	BEFORE INSERT OR UPDATE OF status ON care_contract.contract_versions
	FOR EACH ROW EXECUTE FUNCTION care_contract.guard_version_timeline();
`

// careContractAmendmentsDownSQL drops the later versions and makes the
// newest remaining effective version wirksam again; the status guard of
// #4680 refuses that step, so it is paused for the repair.
const careContractAmendmentsDownSQL = `
DROP TRIGGER IF EXISTS guard_care_contract_version_timeline ON care_contract.contract_versions;
DROP FUNCTION IF EXISTS care_contract.guard_version_timeline();
DELETE FROM care_contract.contract_versions WHERE source = 'buchungsaenderung';
ALTER TABLE care_contract.contract_versions DISABLE TRIGGER guard_care_contract_version_status;
UPDATE care_contract.contract_versions SET status = 'abgeloest' WHERE status = 'ersetzt';
UPDATE care_contract.contract_versions AS v SET status = 'wirksam'
	WHERE v.status = 'abgeloest' AND NOT EXISTS (SELECT 1 FROM care_contract.contract_versions AS o
		WHERE o.contract_id = v.contract_id AND o.status IN ('wirksam', 'abgeloest') AND o.effective_from > v.effective_from);
ALTER TABLE care_contract.contract_versions ENABLE TRIGGER guard_care_contract_version_status;
ALTER TABLE care_contract.contract_versions DROP CONSTRAINT chk_contract_versions_status;
ALTER TABLE care_contract.contract_versions ADD CONSTRAINT chk_contract_versions_status
	CHECK (status IN ('beantragt', 'wirksam', 'abgeloest', 'abgelehnt'));
CREATE OR REPLACE FUNCTION care_contract.guard_version_status() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.status IS DISTINCT FROM OLD.status AND NOT (
		(OLD.status = 'beantragt' AND NEW.status IN ('wirksam', 'abgelehnt'))
		OR (OLD.status = 'wirksam' AND NEW.status = 'abgeloest')) THEN
		RAISE EXCEPTION 'care contract version % cannot move from % to %', OLD.id, OLD.status, NEW.status
			USING ERRCODE = 'check_violation';
	END IF;
	RETURN NEW;
END;
$$;
DROP INDEX IF EXISTS care_contract.uq_contract_versions_timeline;
DROP INDEX IF EXISTS care_contract.uq_contract_versions_effective;
ALTER TABLE care_contract.contract_versions DROP CONSTRAINT IF EXISTS chk_contract_versions_source;
ALTER TABLE care_contract.contract_versions DROP COLUMN IF EXISTS source;
`

func careContractAmendmentsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractAmendmentsUpSQL); err != nil {
		return fmt.Errorf("add care contract amendments: %w", err)
	}
	return nil
}

func careContractAmendmentsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractAmendmentsDownSQL); err != nil {
		return fmt.Errorf("drop care contract amendments: %w", err)
	}
	return nil
}
