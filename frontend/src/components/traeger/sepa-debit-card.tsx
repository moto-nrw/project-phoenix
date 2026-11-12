"use client";

// Die Lastschrift eines ausgestellten Laufs (#4697): die SEPA-Datei
// (PAIN.008), die das Träger-Büro bei seiner Bank einreicht, und die
// Bestätigung, dass es das getan hat. moto bucht nichts ab. Der Download
// ändert nichts; erst die Bestätigung zählt die Abbuchung und macht aus der
// ersten Lastschrift eines Mandats eine Folgelastschrift. Lastschriften, die
// nicht in die Datei dürfen, stehen mit Grund darunter.

import { DownloadIcon, LandmarkIcon } from "lucide-react";
import { type ReactNode, useCallback, useEffect, useState } from "react";
import { formatEuro } from "~/components/traeger/contribution-format";
import {
  HandoverMenu,
  useFinanceHandover,
} from "~/components/traeger/finance-export-handover";
import {
  SEPA_DEBIT_LEFT_LABELS,
  SEPA_SEQUENCE_BADGES,
  collectDatesText,
  debitDocumentLabel,
  debitCountLabel,
} from "~/components/traeger/sepa-debit-format";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { wireErrorCode } from "~/lib/api-error";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type SepaDebit,
  type SepaDebitRun,
  type SepaDebitRunKind,
  downloadSepaDebitFile,
  messageIdOfDebitFile,
  fetchSepaDebitRun,
  submitSepaDebits,
} from "~/lib/traeger-sepa-debits-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "SepaDebitCard" });

/** The three steps of the office; the bank collects, not moto. */
const STEPS = [
  "Datei herunterladen",
  "Im Online-Banking hochladen",
  "Hier bestätigen",
] as const;

/** The mandate page of a family. */
function mandateHref(debit: SepaDebit): string {
  return traegerPath(`/traeger/abrechnung/lastschrift/${debit.contractId}`);
}

const columns: DataTableColumn<SepaDebit>[] = [
  {
    key: "family",
    header: "Familie",
    stacked: "title",
    render: (debit) => (
      <span className="font-medium text-gray-900">{debit.recipientName}</span>
    ),
    sortValue: (debit) => debit.recipientName,
  },
  {
    key: "amount",
    header: "Betrag",
    align: "right",
    stacked: "meta",
    render: (debit) => (
      <span className="tabular-nums">{formatEuro(debit.amountCents)}</span>
    ),
    sortValue: (debit) => debit.amountCents,
  },
  {
    key: "invoice",
    header: "Rechnung",
    render: (debit) => (
      <span className="tabular-nums">{debitDocumentLabel(debit)}</span>
    ),
    sortValue: (debit) => debit.invoiceNumber,
  },
  {
    key: "due",
    header: "Abbuchung am",
    render: (debit) => (
      <span className="tabular-nums">
        {formatDate(debit.collectOn ?? debit.dueOn)}
      </span>
    ),
    sortValue: (debit) => debit.collectOn ?? debit.dueOn,
  },
  {
    key: "sequence",
    header: "Art",
    render: (debit) => {
      const badge = SEPA_SEQUENCE_BADGES[debit.sequenceType];
      return <StatusBadge label={badge.label} tone={badge.tone} size="xs" />;
    },
  },
  {
    key: "mandate",
    header: "Mandat",
    render: (debit) => (
      <span className="flex flex-col">
        <span className="break-all text-gray-900">
          {debit.mandateReference}
        </span>
        <span className="text-xs text-gray-500 tabular-nums">
          {debit.ibanMasked}
        </span>
      </span>
    ),
  },
];

// An installment of a Jahresbescheid has no invoice: its column names the
// notice.
const noticeRateColumns = columns.map((column) =>
  column.key === "invoice" ? { ...column, header: "Bescheid" } : column,
);

export function SepaDebitCard({
  kind,
  runId,
  title = "Lastschrift",
  control,
}: Readonly<{
  kind: SepaDebitRunKind;
  runId: string;
  title?: string;
  /** A choice the card shows on top, e.g. the month of the installments;
   * with it the card stays when nothing is to collect. */
  control?: ReactNode;
}>) {
  const [run, setRun] = useState<SepaDebitRun | null>(null);
  const [downloaded, setDownloaded] = useState<{
    key: string;
    messageId: string;
  } | null>(null);
  const runKey = `${kind}/${runId}`;
  const downloadedId = downloaded?.key === runKey ? downloaded.messageId : null;
  const [downloading, setDownloading] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const errors = useApiErrorDisplay();
  const { success } = useToast();

  const reload = useCallback(async () => {
    try {
      setRun(await fetchSepaDebitRun(kind, runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_sepa_debits_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Lastschrift",
        retry: () => void reload(),
      });
    }
  }, [kind, runId, clearLoadError, showLoadError]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const handoverRun = {
    pathKind: kind,
    runId,
    label: title === "Lastschrift" ? "diesem Lauf" : title,
  };
  // Handover of the file to the recipient of the Träger (ADR 0217): a file
  // that arrived counts like a download for the confirmation below.
  const handover = useFinanceHandover((arrived) => {
    if (arrived) {
      setDownloaded({
        key: runKey,
        messageId: messageIdOfDebitFile(arrived.fileName),
      });
    }
    void reload();
  });

  const download = async () => {
    setDownloading(true);
    try {
      const file = await downloadSepaDebitFile(kind, runId);
      downloadBlob(file.blob, file.filename);
      setDownloaded({
        key: runKey,
        messageId: messageIdOfDebitFile(file.filename),
      });
    } catch (err) {
      logger.error("traeger_sepa_debit_file_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, { object: "die Lastschriftdatei" });
      void reload();
    } finally {
      setDownloading(false);
    }
  };

  const submit = async () => {
    if (!run || !downloadedId) return;
    setSubmitting(true);
    try {
      setRun(await submitSepaDebits(kind, runId, downloadedId));
      setConfirming(false);
      success("Die Lastschriften sind als eingereicht vermerkt.");
    } catch (err) {
      logger.error("traeger_sepa_debits_submit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      if (wireErrorCode(err) === "parent_billing.sepa_debits_changed") {
        setDownloaded(null);
      }
      void errors.show(err, { object: "die Lastschriftdatei" });
      void reload();
    } finally {
      setSubmitting(false);
    }
  };

  if (load.error) {
    return (
      <SectionCard title={title}>
        <div className="flex flex-col gap-4">
          {control}
          <LoadErrorAlert error={load.error} actionLayout="responsive" />
        </div>
      </SectionCard>
    );
  }
  if (!run) return <SectionCard title={title} loading />;
  if (run.count + run.submitted.length + run.left.length === 0) {
    // A run in which no family pays by direct debit has no file.
    if (!control) return null;
    return (
      <SectionCard title={title} icon={LandmarkIcon}>
        <div className="flex flex-col gap-4">
          {control}
          <EmptyState
            variant="compact"
            title="Keine Lastschrift in diesem Monat"
            description="In diesem Monat zieht kein Bescheid eine Rate per Lastschrift ein."
          />
        </div>
      </SectionCard>
    );
  }

  const pending = run.count > 0;
  const fileBlocked = run.accountMissing;
  return (
    <>
      <SectionCard
        title={title}
        icon={LandmarkIcon}
        titleBadge={
          pending ? (
            <StatusBadge label="Noch nicht eingereicht" tone="warning" />
          ) : run.submitted.length > 0 ? (
            <StatusBadge label="Bei der Bank eingereicht" tone="green" />
          ) : null
        }
        description={
          pending
            ? "Die Bank bucht die Beiträge ab, nicht moto. Laden Sie die Datei herunter und im Online-Banking hoch. Bestätigen Sie es danach hier."
            : run.submitted.length > 0
              ? `Die Datei ist eingereicht${run.submittedAt ? ` (${formatDate(run.submittedAt)})` : ""}. Die Bank bucht am Fälligkeitstag ab.`
              : "In diesem Lauf darf keine Lastschrift in die Datei. Die Gründe stehen unten."
        }
        actions={
          pending ? (
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row">
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={fileBlocked || !downloadedId || downloading}
                onClick={() => setConfirming(true)}
              >
                Bei der Bank eingereicht
              </Button>
              {fileBlocked ? null : (
                <HandoverMenu
                  run={handoverRun}
                  recipient={handover.recipient}
                  formats={["pain008"]}
                  busy={downloading || handover.running !== null}
                  running={handover.running !== null}
                  onHandOver={(format) =>
                    handover.handOver(format, handoverRun)
                  }
                />
              )}
              <Button
                type="button"
                variant="primary"
                size="md"
                disabled={fileBlocked}
                className="gap-2"
                isLoading={downloading}
                loadingText="Wird erstellt …"
                onClick={() => void download()}
              >
                <DownloadIcon aria-hidden className="size-4" />
                Datei herunterladen
              </Button>
            </div>
          ) : undefined
        }
      >
        <div className="flex flex-col gap-5">
          {control}
          {handover.notice}
          {fileBlocked ? (
            <Alert
              type="warning"
              title="Konto des Trägers fehlt"
              message="Die Datei braucht das Konto, auf das die Beiträge gehen. Legen Sie unter Einstellungen → Abrechnung eine Bankverbindung als Standard fest. Das darf, wer das Recht „Einstellungen“ hat."
              action={
                <ButtonLink
                  href={traegerPath("/traeger/einstellungen/abrechnung")}
                  variant="surface"
                  size="compact"
                >
                  Bankverbindung öffnen
                </ButtonLink>
              }
            />
          ) : null}
          {pending && run.pastDue > 0 ? (
            <Alert
              type="info"
              message={`${debitCountLabel(run.pastDue)} ${run.pastDue === 1 ? "war" : "waren"} schon fällig. Die Bank bucht nicht rückwirkend ab. Die Datei bucht ${run.pastDue === 1 ? "sie" : "diese"} am ${formatDate(run.groups[0]?.collectOn ?? "")} ab.`}
            />
          ) : null}
          {pending && !downloadedId ? (
            <p className="text-sm text-gray-600">
              Laden Sie zuerst die Datei herunter. Danach können Sie das
              Einreichen bestätigen.
            </p>
          ) : null}
          {pending ? (
            <WizardStepper steps={STEPS} current={downloadedId ? 1 : 0} />
          ) : null}
          {pending ? (
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <StatCard
                variant="inline"
                label="Lastschriften in der Datei"
                value={<span className="tabular-nums">{run.count}</span>}
              />
              <StatCard
                variant="inline"
                label="Summe"
                value={
                  <span className="tabular-nums">
                    {formatEuro(run.totalCents)}
                  </span>
                }
              />
              <StatCard
                variant="inline"
                label="Abbuchung am"
                value={
                  <span className="tabular-nums">
                    {collectDatesText(run.groups)}
                  </span>
                }
              />
            </div>
          ) : null}
          {run.creditorName ? (
            <p className="text-sm text-gray-600">
              Gutschrift auf{" "}
              <span className="text-gray-900">{run.creditorName}</span>,{" "}
              <span className="tabular-nums">{run.creditorIbanMasked}</span> ·
              Gläubiger-ID{" "}
              <span className="tabular-nums">{run.creditorId || "–"}</span>
            </p>
          ) : null}
          {pending || run.submitted.length > 0 ? (
            <DataTable
              columns={kind === "notice-months" ? noticeRateColumns : columns}
              rows={[...(pending ? run.debits : run.submitted)]}
              getRowKey={(debit) => debit.id}
              stackedOnMobile
            />
          ) : null}
        </div>
      </SectionCard>
      {run.left.length > 0 ? <SepaDebitsLeftCard left={run.left} /> : null}
      {handover.dialog}
      <ConfirmationModal
        isOpen={confirming}
        title="Datei bei der Bank eingereicht?"
        confirmText="Ja, eingereicht"
        confirmVariant="success"
        isConfirmLoading={submitting}
        loadingText="Wird vermerkt …"
        onClose={() => setConfirming(false)}
        onConfirm={() => void submit()}
      >
        <p className="text-sm text-gray-700">
          Bestätigen Sie erst, wenn Ihre Bank die Datei angenommen hat. Danach
          gelten {debitCountLabel(run.count)} über {formatEuro(run.totalCents)}{" "}
          als eingereicht. Bei einem neuen Mandat ist die nächste Lastschrift
          dann eine Folgelastschrift.
        </p>
        <p className="mt-3 text-sm text-gray-600">
          Datei <span className="font-mono break-all">{downloadedId}</span>
        </p>
      </ConfirmationModal>
    </>
  );
}

/** Debits that may not go into the file, each with its reason. */
function SepaDebitsLeftCard({
  left,
}: Readonly<{ left: readonly SepaDebit[] }>) {
  return (
    <SectionCard
      title="Nicht in der Datei"
      titleBadge={
        <span className="text-sm font-normal text-gray-600 tabular-nums">
          {debitCountLabel(left.length)}
        </span>
      }
      description="Für diese Rechnungen darf die Bank nicht abbuchen. Die Familie zahlt anders, oder Sie klären das Mandat."
    >
      <ResponsiveActionList>
        {left.map((debit) => (
          <li
            key={debit.id}
            className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
          >
            <div className="min-w-0">
              <p className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <span className="font-semibold text-gray-900">
                  {debit.recipientName}
                </span>
                <span className="text-gray-600 tabular-nums">
                  {debitDocumentLabel(debit)}
                </span>
                <span className="text-gray-500 tabular-nums">
                  {formatEuro(debit.amountCents)}
                </span>
              </p>
              <p className="mt-0.5 text-sm text-gray-700">
                {debit.left ? SEPA_DEBIT_LEFT_LABELS[debit.left] : ""}
              </p>
            </div>
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
              <ButtonLink
                href={mandateHref(debit)}
                variant="surface"
                size="compact"
                aria-label={`Mandat prüfen: ${debit.recipientName}`}
              >
                Mandat prüfen
              </ButtonLink>
            </div>
          </li>
        ))}
      </ResponsiveActionList>
    </SectionCard>
  );
}
