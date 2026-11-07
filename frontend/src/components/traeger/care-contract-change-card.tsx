"use client";

// Änderungswunsch der Eltern (#5979): Eltern beantragen in der Eltern-App
// andere Tage oder Module. Das Träger-Büro sieht den Wunsch auf der
// Vertragsseite und gibt ihn selbst frei oder lehnt ihn ab, wie die Schule.
// Nach der Freigabe folgt der Vertrag mit einer neuen Version.

import { ArrowLeftRight } from "lucide-react";
import dynamic from "next/dynamic";
import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { OfferingRequestChanges } from "~/components/students/offering-request-review-item";
import { CarePlanImpact } from "~/components/students/care-plan-impact";
import type { RosterImpact } from "~/lib/roster-impact";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import {
  contractChangeReview,
  type ContractChangeOutcome,
  type ContractChangeRequest,
  type ContractChanges,
} from "~/lib/care-contract-changes";
import { fetchContractChanges } from "~/lib/care-contract-changes-api";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareContractChangeCard" });

// The decision form carries the date picker; it loads when the office
// opens it, so the contract page stays as light as before.
const DecideChangeModal = dynamic(
  () =>
    import("./care-contract-change-modal").then(
      (module) => module.DecideChangeModal,
    ),
  { ssr: false },
);

export function CareContractChangeCard({
  contractId,
  onDecided,
}: Readonly<{
  contractId: string;
  /** The contract page reads again: the next version follows the decision. */
  onDecided: () => void;
}>) {
  const [refreshed, setRefreshed] = useState(false);
  const [impact, setImpact] = useState<RosterImpact | null>(null);
  const [changes, setChanges] = useState<ContractChanges | null>(null);
  const [deciding, setDeciding] = useState<{
    request: ContractChangeRequest;
    approve: boolean;
  } | null>(null);
  const { error, show, clear } = useApiLoadError();

  const load = useCallback(async () => {
    try {
      setChanges(await fetchContractChanges(contractId));
      clear();
    } catch (err) {
      logger.error("traeger_care_contract_changes_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await show(err, {
        object: "der Änderungswunsch",
        retry: () => void load(),
      });
    }
  }, [contractId, clear, show]);

  useEffect(() => {
    void load();
  }, [load]);

  const onConflict = useCallback(() => {
    setDeciding(null);
    setChanges(null);
    setRefreshed(true);
    void load();
  }, [load]);
  const request = changes?.pending[0] ?? null;

  return (
    <SectionCard
      icon={ArrowLeftRight}
      title="Änderungswunsch der Eltern"
      titleBadge={
        request ? (
          <StatusBadge tone="orange" label="Wartet auf Entscheidung" />
        ) : undefined
      }
      description={
        request
          ? "Die Eltern möchten die gebuchte Betreuung ändern. Sie oder die Schule entscheiden."
          : "Eltern beantragen andere Tage oder Module in der Eltern-App. Sie oder die Schule entscheiden."
      }
      loading={changes === null && !error}
      loadingRows={3}
      footer={
        request && changes?.decidable ? (
          <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button
              type="button"
              variant="outline_danger"
              size="md"
              onClick={() => setDeciding({ request, approve: false })}
            >
              Ablehnen
            </Button>
            {request.endsAllCare ? null : (
              <Button
                type="button"
                variant="success"
                size="md"
                onClick={() => setDeciding({ request, approve: true })}
              >
                Freigeben
              </Button>
            )}
          </div>
        ) : null
      }
    >
      {error ? <LoadErrorAlert error={error} /> : null}
      {refreshed ? (
        <Alert
          type="info"
          message="Der Änderungswunsch wurde inzwischen geändert oder entschieden. Bitte prüfen Sie den aktuellen Stand."
        />
      ) : null}
      {changes && request ? <PendingChange request={request} /> : null}
      {changes && !request ? (
        <LastDecision outcome={changes.lastDecision} />
      ) : null}
      {impact ? (
        <div className="space-y-3">
          {impact.notAdjusted.length > 0 ||
          impact.pickupOverrides.length > 0 ? (
            <Alert
              type="warning"
              message="Bitte geben Sie diese Hinweise an die Schule weiter. Die Schule muss Regeltermine und Abholzeiten prüfen."
            />
          ) : null}
          <CarePlanImpact impact={impact} schoolFollowUp />
        </div>
      ) : null}
      {deciding ? (
        <DecideChangeModal
          isOpen
          onClose={() => setDeciding(null)}
          contractId={contractId}
          request={deciding.request}
          approve={deciding.approve}
          reasonRequired={changes?.reasonRequired ?? true}
          onConflict={onConflict}
          onDecided={(next) => {
            setImpact(next.rosterImpact ?? null);
            setRefreshed(false);
            setChanges(next);
            setDeciding(null);
            onDecided();
          }}
        />
      ) : null}
    </SectionCard>
  );
}

/** The open request: from when, what changes per module, the parents' note. */
function PendingChange({
  request,
}: Readonly<{ request: ContractChangeRequest }>) {
  return (
    <div className="space-y-5">
      <DataGrid>
        <DataField label="Gewünscht ab">
          {formatDate(request.effectiveFrom)}
        </DataField>
        <DataField label="Eingegangen am">
          {formatBerlinDate(request.submittedAt)}
        </DataField>
      </DataGrid>
      {request.endsAllCare ? (
        <Alert
          type="warning"
          message="Die Eltern melden damit die ganze Betreuung ab. Das ist eine Kündigung. Tragen Sie sie unter „Vertrag beenden“ ein. Hier können Sie den Wunsch nur ablehnen."
        />
      ) : null}
      <OfferingRequestChanges row={contractChangeReview(request)} />
    </div>
  );
}

/** No open request: the last decision, if any, so the office sees the end. */
function LastDecision({
  outcome,
}: Readonly<{ outcome: ContractChangeOutcome | null }>) {
  if (!outcome) {
    return (
      <p className="text-sm text-gray-500">Kein offener Änderungswunsch.</p>
    );
  }
  return (
    <DataGrid columns={1}>
      <DataField label="Zuletzt entschieden">
        <span className="flex flex-wrap items-center gap-2">
          <StatusBadge
            tone={outcome.approved ? "green" : "gray"}
            label={outcome.approved ? "Freigegeben" : "Abgelehnt"}
          />
          <span>
            {outcome.approved
              ? `Gilt ab ${formatDate(outcome.effectiveFrom)}`
              : `Am ${formatBerlinDate(outcome.decidedAt)}`}
          </span>
        </span>
        {outcome.reason ? (
          <span className="mt-1 block text-sm font-normal wrap-anywhere text-gray-500">
            {outcome.reason}
          </span>
        ) : null}
      </DataField>
    </DataGrid>
  );
}
