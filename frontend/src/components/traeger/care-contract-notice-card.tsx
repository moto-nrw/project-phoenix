"use client";

// Vertrag beenden (#4682): was ein abgeschlossener Vertrag heute erlaubt und
// die Kündigung oder den Widerruf, den die Eltern abgegeben haben. Eltern
// kündigen per Mail, Brief oder selbst in der Eltern-App; das Träger-Büro
// trägt eine Erklärung ein, die es per Mail oder Brief bekommen hat. Danach
// zeigt die Karte das Vertragsende und die Bestätigung, wie sie verschickt
// wurde.

import { CalendarX2 } from "lucide-react";
import dynamic from "next/dynamic";
import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import type { CareContractStatus } from "~/lib/traeger-care-contracts-api";
import { TERMINATION_KIND_LABELS } from "./care-contract-format";
import { useApiLoadError } from "~/contexts/ToastContext";
import type { ContractNoticeOptions } from "~/lib/care-contract-notices";
import { fetchContractNoticeOptions } from "~/lib/care-contract-notices-api";
import {
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareContractNoticeCard" });

// The entry form carries the date picker; it loads when the office opens it,
// so the contract page stays as light as before (bundle budget).
const RecordNoticeModal = dynamic(
  () =>
    import("./care-contract-notice-modal").then(
      (module) => module.RecordNoticeModal,
    ),
  { ssr: false },
);

const RejectReasonModal = dynamic(
  () =>
    import("./care-contract-notice-modal").then(
      (module) => module.RejectReasonModal,
    ),
  { ssr: false },
);

const SetNoticeEndModal = dynamic(
  () =>
    import("./care-contract-notice-modal").then(
      (module) => module.SetNoticeEndModal,
    ),
  { ssr: false },
);

/** How the office decided the end of an extraordinary termination. */
const DECISION_LABELS = {
  festgelegt: { label: "Grund anerkannt", tone: "green" },
  umgedeutet: { label: "Grund nicht anerkannt", tone: "warning" },
} as const;

/** Who declared, as the office reads it. */
const CHANNEL_LABELS = {
  eltern_app: "Von den Eltern in der Eltern-App",
  traeger_buero: "Vom Träger-Büro eingetragen",
} as const;

export function CareContractNoticeCard({
  contractId,
  status,
  concludedAt,
  onChanged,
}: Readonly<{
  contractId: string;
  status: CareContractStatus;
  /** When the contract was concluded: no declaration can be older. */
  concludedAt: string;
  onChanged: () => void;
}>) {
  const [options, setOptions] = useState<ContractNoticeOptions | null>(null);
  const [open, setOpen] = useState(false);
  const [endOpen, setEndOpen] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const { error, show, clear } = useApiLoadError();

  const load = useCallback(async () => {
    try {
      setOptions(await fetchContractNoticeOptions(contractId));
      clear();
    } catch (err) {
      logger.error("traeger_care_contract_notice_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await show(err, { object: "die Kündigung", retry: () => void load() });
    }
  }, [contractId, clear, show]);

  // The status of the contract changes when the parents declare in the
  // parents app; the card reads again with it.
  useEffect(() => {
    void load();
  }, [load, status]);

  const notice = options?.notice ?? null;
  const active = options?.status === "aktiv";
  const endPending =
    notice?.terminationKind === "ausserordentlich" && notice.endsOn === null;

  return (
    <SectionCard
      icon={CalendarX2}
      title="Vertrag beenden"
      description={
        endPending
          ? "Die Eltern haben außerordentlich gekündigt. Prüfen Sie ihren Grund."
          : notice
            ? "So haben die Eltern den Vertrag beendet."
            : "Kündigung oder Widerruf der Eltern."
      }
      loading={options === null && !error}
      loadingRows={3}
      footer={
        options && active && !notice ? (
          <div className="space-y-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              className="w-full"
              onClick={() => setOpen(true)}
            >
              Kündigung eintragen
            </Button>
            <p className="text-center text-sm text-gray-500">
              Für eine Kündigung per Mail oder Brief. In der Eltern-App kündigen
              Eltern selbst.
            </p>
          </div>
        ) : endPending ? (
          <EndDecisionActions
            reinterpretedEndsOn={options?.reinterpretedEndsOn ?? null}
            onSetEnd={() => setEndOpen(true)}
            onReject={() => setRejectOpen(true)}
          />
        ) : null
      }
    >
      {error ? <LoadErrorAlert error={error} /> : null}
      {options && notice ? <NoticeFacts notice={notice} /> : null}
      {options && !notice && active ? <ActiveFacts options={options} /> : null}
      {options && active && open ? (
        <RecordNoticeModal
          isOpen={open}
          onClose={() => setOpen(false)}
          contractId={contractId}
          options={options}
          concludedAt={concludedAt}
          onRecorded={(next: ContractNoticeOptions) => {
            setOptions(next);
            setOpen(false);
            onChanged();
          }}
        />
      ) : null}
      {notice && endPending && endOpen ? (
        <SetNoticeEndModal
          isOpen={endOpen}
          onClose={() => setEndOpen(false)}
          contractId={contractId}
          receivedOn={notice.receivedOn}
          onSet={(next: ContractNoticeOptions) => {
            setOptions(next);
            setEndOpen(false);
            onChanged();
          }}
        />
      ) : null}
      {notice && endPending && rejectOpen && options?.reinterpretedEndsOn ? (
        <RejectReasonModal
          isOpen={rejectOpen}
          onClose={() => setRejectOpen(false)}
          contractId={contractId}
          reason={notice.reason ?? ""}
          receivedOn={notice.receivedOn}
          endsOn={options.reinterpretedEndsOn}
          onRejected={(next: ContractNoticeOptions) => {
            setOptions(next);
            setRejectOpen(false);
            onChanged();
          }}
        />
      ) : null}
    </SectionCard>
  );
}

/**
 * The two ways the office decides an extraordinary termination, each with
 * what follows from it. Both reach the parents in Textform.
 */
function EndDecisionActions({
  reinterpretedEndsOn,
  onSetEnd,
  onReject,
}: Readonly<{
  reinterpretedEndsOn: string | null;
  onSetEnd: () => void;
  onReject: () => void;
}>) {
  return (
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Button
          type="button"
          variant="primary"
          size="md"
          className="w-full"
          onClick={onSetEnd}
        >
          Vertragsende festlegen
        </Button>
        <p className="text-center text-sm text-gray-500">
          Sie erkennen den Grund an und wählen den letzten Tag.
        </p>
      </div>
      <div className="space-y-1.5">
        <Button
          type="button"
          variant="outline"
          size="md"
          className="w-full"
          onClick={onReject}
          disabled={!reinterpretedEndsOn}
        >
          Grund nicht anerkennen
        </Button>
        <p className="text-center text-sm text-gray-500">
          {reinterpretedEndsOn
            ? `Dann gilt sie als ordentliche Kündigung zum ${formatDate(reinterpretedEndsOn)}.`
            : "Dann gilt sie als ordentliche Kündigung."}
        </p>
      </div>
      <p className="border-t border-gray-100 pt-3 text-center text-sm text-gray-500">
        Die Eltern bekommen Ihre Entscheidung per E-Mail und in der Eltern-App.
      </p>
    </div>
  );
}

/** What an active contract allows today. */
function ActiveFacts({
  options,
}: Readonly<{ options: ContractNoticeOptions }>) {
  return (
    <DataGrid columns={1}>
      <DataField label="Bei Kündigung heute endet er am">
        {options.ordinaryEndsOn ? formatDate(options.ordinaryEndsOn) : "–"}
        <span className="block text-sm font-normal text-gray-500">
          {options.terms}
        </span>
      </DataField>
      {options.specialEndsOn ? (
        <DataField label="Sonderkündigung zum Halbjahr">
          {formatDate(options.specialEndsOn)}
        </DataField>
      ) : null}
      <DataField label="Widerruf">
        {options.withdrawalUntil ? (
          `Absendefrist bis ${formatDate(options.withdrawalUntil)}`
        ) : (
          <span className="font-normal text-gray-500">Nicht mehr möglich</span>
        )}
      </DataField>
      <DataField label="Betreuung vor Ende der Widerrufsfrist">
        {options.earlyStart ? "Von den Eltern verlangt" : "Nicht verlangt"}
      </DataField>
    </DataGrid>
  );
}

function NoticeFacts({
  notice,
}: Readonly<{ notice: NonNullable<ContractNoticeOptions["notice"]> }>) {
  const withdrawn = notice.kind === "widerruf";
  return (
    <div className="space-y-4">
      <DataGrid columns={1}>
        <DataField label={withdrawn ? "Widerrufen" : "Gekündigt zum"}>
          {withdrawn ? (
            "Aufgehoben vom ersten Tag an"
          ) : notice.endsOn ? (
            formatDate(notice.endsOn)
          ) : (
            <StatusBadge label="Ende noch offen" tone="warning" />
          )}
        </DataField>
        {notice.terminationKind ? (
          <DataField label="Art der Kündigung">
            {TERMINATION_KIND_LABELS[notice.terminationKind]}
          </DataField>
        ) : null}
        {notice.endDecision ? (
          <DataField label="Entscheidung des Träger-Büros">
            <StatusBadge
              label={DECISION_LABELS[notice.endDecision].label}
              tone={DECISION_LABELS[notice.endDecision].tone}
            />
            <span className="mt-1 block text-sm font-normal text-gray-500">
              {notice.endDecision === "umgedeutet"
                ? "Gilt als ordentliche Kündigung zum nächsten möglichen Termin."
                : "Das Träger-Büro hat den letzten Tag festgelegt."}
              {notice.endDecidedAt
                ? ` Entschieden am ${formatBerlinDate(notice.endDecidedAt)}.`
                : ""}
            </span>
          </DataField>
        ) : null}
        {notice.reason ? (
          <DataField label="Grund der Eltern">
            <span className="wrap-anywhere whitespace-pre-line">
              {notice.reason}
            </span>
          </DataField>
        ) : null}
        <DataField label="Eingegangen am">
          {formatDate(notice.receivedOn)}
          <span className="block text-sm font-normal text-gray-500">
            {CHANNEL_LABELS[notice.channel]},{" "}
            {formatBerlinDate(notice.declaredAt)} um{" "}
            {formatBerlinTime(notice.declaredAt)} Uhr
          </span>
        </DataField>
        {notice.dispatchedOn ? (
          <DataField label="Abgesendet am">
            {formatDate(notice.dispatchedOn)}
          </DataField>
        ) : null}
        <DataField label="Bestätigung an">
          <span className="wrap-anywhere">{notice.recipient}</span>
        </DataField>
      </DataGrid>
      {notice.endNoticeText && !notice.endNoticeSent ? (
        <Alert
          type="warning"
          message="Die Mitteilung zum Vertragsende ging nicht raus. Keine Adresse des Vertrags gehört noch einer Bezugsperson, die für das Kind handeln darf. Teilen Sie den Eltern das Vertragsende selbst mit, zum Beispiel per Brief."
        />
      ) : null}
      {notice.endNoticeText ? (
        <figure
          aria-label="Mitteilung zum Vertragsende, wie sie verschickt wurde"
          className="moto-content-surface rounded-xl border p-4"
        >
          <figcaption className="mb-2 text-sm font-medium text-gray-900">
            Mitteilung zum Vertragsende, wie sie verschickt wurde
          </figcaption>
          <p className="text-sm wrap-anywhere whitespace-pre-line text-gray-700">
            {notice.endNoticeText}
          </p>
        </figure>
      ) : null}
      <figure
        aria-label="Bestätigung, wie sie verschickt wurde"
        className="moto-content-surface rounded-xl border p-4"
      >
        <figcaption className="mb-2 text-sm font-medium text-gray-900">
          Bestätigung, wie sie verschickt wurde
        </figcaption>
        <p className="text-sm wrap-anywhere whitespace-pre-line text-gray-700">
          {notice.confirmationText}
        </p>
      </figure>
    </div>
  );
}
