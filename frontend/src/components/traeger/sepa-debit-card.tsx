"use client";

// Die Lastschrift eines ausgestellten Laufs (#4697): die SEPA-Datei
// (PAIN.008), die das Träger-Büro bei seiner Bank einreicht, und die
// Bestätigung, dass es das getan hat. moto bucht nichts ab. Der Download
// ändert nichts; erst die Bestätigung zählt die Abbuchung und macht aus der
// ersten Lastschrift eines Mandats eine Folgelastschrift. Lastschriften, die
// nicht in die Datei dürfen, stehen mit Grund darunter.

import { DownloadIcon, LandmarkIcon } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { formatEuro } from "~/components/traeger/contribution-format";
import {
  SEPA_DEBIT_LEFT_LABELS,
  SEPA_SEQUENCE_BADGES,
  collectDatesText,
  debitCountLabel,
} from "~/components/traeger/sepa-debit-format";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
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
      <span className="tabular-nums">{debit.invoiceNumber}</span>
    ),
    sortValue: (debit) => debit.invoiceNumber,
  },
  {
    key: "due",
    header: "Fällig",
    render: (debit) => (
      <span className="tabular-nums">{formatDate(debit.dueOn)}</span>
    ),
    sortValue: (debit) => debit.dueOn,
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

export function SepaDebitCard({
  kind,
  runId,
}: Readonly<{ kind: SepaDebitRunKind; runId: string }>) {
  const [run, setRun] = useState<SepaDebitRun | null>(null);
  const [downloaded, setDownloaded] = useState(false);
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

  const download = async () => {
    setDownloading(true);
    try {
      const file = await downloadSepaDebitFile(kind, runId);
      downloadBlob(file.blob, file.filename);
      setDownloaded(true);
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
    if (!run) return;
    setSubmitting(true);
    try {
      setRun(await submitSepaDebits(kind, runId, run.messageId));
      setConfirming(false);
      success("Die Lastschriften sind als eingereicht vermerkt.");
    } catch (err) {
      logger.error("traeger_sepa_debits_submit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      if (wireErrorCode(err) === "parent_billing.sepa_debits_changed") {
        setDownloaded(false);
      }
      void errors.show(err, { object: "die Lastschriftdatei" });
      void reload();
    } finally {
      setSubmitting(false);
    }
  };

  if (load.error) {
    return (
      <SectionCard title="Lastschrift">
        <LoadErrorAlert error={load.error} actionLayout="responsive" />
      </SectionCard>
    );
  }
  if (!run) return <SectionCard title="Lastschrift" loading />;
  // A run in which no family pays by direct debit has no file.
  if (run.count + run.submitted.length + run.left.length === 0) return null;

  const pending = run.count > 0;
  const fileBlocked = run.accountMissing;
  return (
    <>
      <SectionCard
        title="Lastschrift"
        icon={LandmarkIcon}
        titleBadge={
          pending ? (
            <StatusBadge label="Noch nicht eingereicht" tone="orange" />
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
                disabled={fileBlocked}
                onClick={() => setConfirming(true)}
              >
                Bei der Bank eingereicht
              </Button>
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
          {pending ? (
            <WizardStepper steps={STEPS} current={downloaded ? 1 : 0} />
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
              columns={columns}
              rows={[...(pending ? run.debits : run.submitted)]}
              getRowKey={(debit) => debit.id}
              stackedOnMobile
            />
          ) : null}
        </div>
      </SectionCard>
      {run.left.length > 0 ? <SepaDebitsLeftCard left={run.left} /> : null}
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
          Datei <span className="font-mono break-all">{run.messageId}</span>
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
                  {debit.invoiceNumber}
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
