"use client";

// Änderungswunsch der Eltern (#5979): Eltern beantragen in der Eltern-App
// andere Tage oder Module. Das Träger-Büro sieht den Wunsch auf der
// Vertragsseite und gibt ihn selbst frei oder lehnt ihn ab, wie die Schule.
// Nach der Freigabe folgt der Vertrag mit einer neuen Version.

import { ArrowLeftRight, Check, Minus, Plus } from "lucide-react";
import dynamic from "next/dynamic";
import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import {
  type ChangeDayState,
  changeWeek,
  type ContractChangeOutcome,
  type ContractChangeRequest,
  type ContractChanges,
} from "~/lib/care-contract-changes";
import { fetchContractChanges } from "~/lib/care-contract-changes-api";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { Weekday } from "~/lib/traeger-care-contracts-api";

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

const WEEKDAY_SHORT: Record<Weekday, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

const WEEKDAY_LONG: Record<Weekday, string> = {
  mon: "Montag",
  tue: "Dienstag",
  wed: "Mittwoch",
  thu: "Donnerstag",
  fri: "Freitag",
  sat: "Samstag",
  sun: "Sonntag",
};

/** What a day tile says to a screen reader, and how it looks. */
const DAY_STATES: Record<
  ChangeDayState,
  { label: string; className: string; icon: typeof Check | null }
> = {
  kept: {
    label: "bleibt",
    className: "bg-gray-50 text-gray-700",
    icon: Check,
  },
  added: {
    label: "kommt dazu",
    className: "bg-moto-green-soft text-moto-green-strong",
    icon: Plus,
  },
  removed: {
    label: "fällt weg",
    className: "bg-moto-red-soft text-moto-red-strong",
    icon: Minus,
  },
  none: { label: "keine Betreuung", className: "text-gray-300", icon: null },
};

export function CareContractChangeCard({
  contractId,
  onDecided,
}: Readonly<{
  contractId: string;
  /** The contract page reads again: the next version follows the decision. */
  onDecided: () => void;
}>) {
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
      {changes && request ? <PendingChange request={request} /> : null}
      {changes && !request ? (
        <LastDecision outcome={changes.lastDecision} />
      ) : null}
      {deciding ? (
        <DecideChangeModal
          isOpen
          onClose={() => setDeciding(null)}
          contractId={contractId}
          request={deciding.request}
          approve={deciding.approve}
          reasonRequired={changes?.reasonRequired ?? true}
          onDecided={(next) => {
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
      <ChangeWeek request={request} />
      {request.parentNote ? (
        <InsetPanel tone="muted" padding="sm" as="section">
          <p className="text-xs font-medium text-gray-500">
            Nachricht der Eltern
          </p>
          <p className="mt-1 text-sm wrap-anywhere whitespace-pre-line text-gray-900">
            {request.parentNote}
          </p>
        </InsetPanel>
      ) : null}
    </div>
  );
}

/**
 * The week of the change, one module per row: each weekday says whether the
 * module stays, comes in or goes. Only a display, nothing to click.
 */
function ChangeWeek({ request }: Readonly<{ request: ContractChangeRequest }>) {
  const week = changeWeek(request.lines);
  if (week.rows.length === 0) return null;
  return (
    <div className="space-y-3">
      <ul className="space-y-3" aria-label="Änderung je Wochentag">
        {week.rows.map((row) => (
          <li key={row.careOfferingId} className="space-y-2">
            <p className="text-sm font-medium wrap-anywhere text-gray-900">
              {row.moduleName}
            </p>
            <ol
              className="grid gap-1.5"
              style={{
                gridTemplateColumns: `repeat(${week.weekdays.length}, minmax(0, 1fr))`,
              }}
            >
              {row.days.map((day) => {
                const state = DAY_STATES[day.state];
                const Icon = state.icon;
                return (
                  <li
                    key={day.weekday}
                    className={`flex h-12 flex-col items-center justify-center gap-0.5 rounded-lg ${state.className}`}
                  >
                    <span className="text-xs font-medium" aria-hidden="true">
                      {WEEKDAY_SHORT[day.weekday]}
                    </span>
                    {Icon ? (
                      <Icon className="size-4" strokeWidth={2.25} aria-hidden />
                    ) : (
                      <span className="text-xs leading-4" aria-hidden="true">
                        –
                      </span>
                    )}
                    <span className="sr-only">
                      {`${WEEKDAY_LONG[day.weekday]}: ${state.label}`}
                    </span>
                  </li>
                );
              })}
            </ol>
          </li>
        ))}
      </ul>
      <ul
        className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500"
        aria-label="Legende"
      >
        <li className="flex items-center gap-1.5">
          <Check className="size-3.5 text-gray-700" aria-hidden />
          Bleibt
        </li>
        <li className="flex items-center gap-1.5">
          <Plus className="text-moto-green-strong size-3.5" aria-hidden />
          Kommt dazu
        </li>
        <li className="flex items-center gap-1.5">
          <Minus className="text-moto-red-strong size-3.5" aria-hidden />
          Fällt weg
        </li>
      </ul>
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
