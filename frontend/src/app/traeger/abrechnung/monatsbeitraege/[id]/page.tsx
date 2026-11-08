"use client";

// Ein Monatslauf (#4684): die Vorschau aller Rechnungen eines Monats nach
// dem gültigen Vertrag und der Preisliste, und die Freigabe, die jede
// Rechnung mit ihrer Nummer ausstellt. Danach ändern sich die Rechnungen
// nicht mehr. Ein freigegebener Lauf erstellt die Schreiben an die
// Familien, sendet sie per E-Mail und in die Eltern-App, druckt sie und
// storniert eine Rechnung; ein neuer Lauf für denselben Monat stellt eine
// stornierte Rechnung neu aus. Jedes Öffnen steht im Protokoll des Laufs.

import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  BillingCorrectionModal,
  type BillingCorrectionTarget,
} from "~/components/traeger/billing-correction-modal";
import { CareLetterPanel } from "~/components/traeger/care-letter-panel";
import {
  CareLettersCard,
  familyCount,
} from "~/components/traeger/care-letters-card";
import {
  CARE_RUN_STATUS_LABELS,
  CARE_RUN_STATUS_TONES,
  MONTH_RULE_LABELS,
  formatMonth,
  invoiceCountLabel,
} from "~/components/traeger/care-billing-format";
import {
  CareRunInvoices,
  CareDuplicatesCard,
  CareUnpricedCard,
  careLetterKey,
} from "~/components/traeger/care-run-invoices";
import { formatEuro } from "~/components/traeger/contribution-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type CareLetterMailing,
  type CareLetterSeries,
  type CareLetters,
  type CareRunDetail,
  fetchCareLetters,
  fetchCareRun,
  fetchCareRuns,
  mailCareLetter,
  mailCareLetters,
  printCareLetters,
  releaseCareRun,
  startCareRun,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCareRunPage" });

/**
 * "1 Kind mit doppeltem Vertrag wird nicht abgerechnet.", after the release
 * "… ist nicht abgerechnet."
 */
function notBilledLabel(children: number, released = false): string {
  if (children === 1) {
    return `1 Kind mit doppeltem Vertrag ${released ? "ist" : "wird"} nicht abgerechnet.`;
  }
  return `${children} Kinder mit doppeltem Vertrag ${released ? "sind" : "werden"} nicht abgerechnet.`;
}

// While a mail waits, the page looks again after this many milliseconds.
const REFRESH_WHILE_QUEUED_MS = 15_000;

/** The success toast of „An alle senden“. */
function seriesMessage(series: CareLetterSeries): string {
  const parts = [
    series.queued === 0
      ? "Es gab kein neues Schreiben zu senden."
      : series.queued === 1
        ? "1 Schreiben wird jetzt gesendet."
        : `${series.queued} Schreiben werden jetzt gesendet.`,
  ];
  if (series.withoutEmail > 0) {
    parts.push(
      `${familyCount(series.withoutEmail)} ohne E-Mail-Adresse: Bitte drucken Sie deren Schreiben.`,
    );
  }
  if (series.recipientChanged > 0) {
    parts.push(
      `${series.recipientChanged === 1 ? "1 Schreiben ging" : `${series.recipientChanged} Schreiben gingen`} nicht raus: Die Person steht nicht mehr im Vertrag. Stornieren Sie diese Rechnung. Ein neuer Lauf stellt sie neu aus.`,
    );
  }
  return parts.join(" ");
}

export default function TraegerCareRunPage() {
  const params = useParams<{ id: string }>();
  const runId = params?.id ?? "";
  const [run, setRun] = useState<CareRunDetail | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [releasing, setReleasing] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const releaseErrors = useApiErrorDisplay();
  const actionErrors = useApiErrorDisplay();
  const { success } = useToast();
  const router = useRouter();
  const [letters, setLetters] = useState<CareLetters | null>(null);
  const lettersLoad = useApiLoadError();
  const { show: showLettersError, clear: clearLettersError } = lettersLoad;
  const [sendingAll, setSendingAll] = useState(false);
  const [sendingKey, setSendingKey] = useState<string | null>(null);
  const [printing, setPrinting] = useState<"all" | "without_email" | null>(
    null,
  );
  const [shownLetter, setShownLetter] = useState<CareLetterMailing | null>(
    null,
  );
  const [cancelling, setCancelling] = useState<BillingCorrectionTarget | null>(
    null,
  );
  const [reissuing, setReissuing] = useState(false);

  const load = useCallback(async () => {
    try {
      setRun(await fetchCareRun(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_run_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Monatslauf",
        retry: () => void load(),
      });
    }
  }, [runId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const released = run?.status === "released";
  const loadLetters = useCallback(async () => {
    try {
      setLetters(await fetchCareLetters(runId));
      clearLettersError();
    } catch (err) {
      logger.error("traeger_care_letters_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLettersError(err, {
        object: "die Schreiben",
        retry: () => void loadLetters(),
      });
    }
  }, [runId, clearLettersError, showLettersError]);

  useEffect(() => {
    if (released) void loadLetters();
  }, [released, loadLetters]);

  // A mail on its way: look again until it is sent or failed.
  const waiting = (letters?.summary.queued ?? 0) > 0;
  useEffect(() => {
    if (!waiting) return;
    const timer = setTimeout(() => void loadLetters(), REFRESH_WHILE_QUEUED_MS);
    return () => clearTimeout(timer);
  }, [waiting, letters, loadLetters]);

  const sendAll = async () => {
    setSendingAll(true);
    try {
      success(seriesMessage(await mailCareLetters(runId)));
    } catch (err) {
      logger.error("traeger_care_letters_mail_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionErrors.show(err, { object: "die Schreiben" });
    } finally {
      setSendingAll(false);
      void loadLetters();
    }
  };

  const sendOne = async (letter: CareLetterMailing) => {
    setSendingKey(careLetterKey(letter));
    try {
      await mailCareLetter(runId, letter.invoiceId, letter.kind);
      success(
        `Das Schreiben an ${letter.recipientFirstName} ${letter.recipientLastName} wird jetzt gesendet.`,
      );
    } catch (err) {
      logger.error("traeger_care_letter_mail_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionErrors.show(err, { object: "das Schreiben" });
    } finally {
      setSendingKey(null);
      void loadLetters();
    }
  };

  const print = async (withoutEmailOnly: boolean) => {
    setPrinting(withoutEmailOnly ? "without_email" : "all");
    try {
      const file = await printCareLetters(runId, withoutEmailOnly);
      downloadBlob(file.blob, file.filename);
      await loadLetters();
    } catch (err) {
      logger.error("traeger_care_letters_print_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionErrors.show(err, { object: "die Schreiben" });
    } finally {
      setPrinting(null);
    }
  };

  // A cancelled invoice is issued again by the run of its month that is
  // still a preview; without one, a new run starts.
  const reissue = async () => {
    if (!run) return;
    setReissuing(true);
    try {
      // Another tab may already have released the replacement.
      const latest = await fetchCareLetters(run.id);
      setLetters(latest);
      if (
        !latest.documents.some(
          (document) => document.kind === "cancellation" && !document.replaced,
        )
      )
        return;
      const open = (await fetchCareRuns()).runs.find(
        (other) =>
          other.status === "calculated" &&
          other.billingMonth === run.billingMonth,
      );
      const target = open ?? (await startCareRun(run.billingMonth));
      router.push(
        traegerPath(`/traeger/abrechnung/monatsbeitraege/${target.id}`),
      );
    } catch (err) {
      logger.error("traeger_care_reissue_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (
        err instanceof ApiError &&
        err.code === "parent_billing.care_run_open"
      ) {
        router.push(traegerPath("/traeger/abrechnung/monatsbeitraege"));
        return;
      }
      void actionErrors.show(err, { object: "der neue Lauf" });
    } finally {
      setReissuing(false);
    }
  };

  const release = async () => {
    if (!run) return;
    setReleasing(true);
    try {
      const released = await releaseCareRun(run);
      setRun(released);
      setConfirming(false);
      success(
        released.duplicateChildren > 0
          ? `Der Lauf ist freigegeben. ${notBilledLabel(released.duplicateChildren, true)}`
          : "Der Lauf ist freigegeben. Die Rechnungen sind ausgestellt.",
      );
    } catch (err) {
      logger.error("traeger_care_run_release_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      void releaseErrors.show(err, { object: "der Monatslauf" });
      void load();
    } finally {
      setReleasing(false);
    }
  };

  const preview = run?.status === "calculated";
  const cancelledCount =
    letters?.documents.filter(
      (document) => document.kind === "cancellation" && !document.replaced,
    ).length ?? 0;
  const canRelease = preview && run.numberRangeSet && run.previewCount > 0;

  return (
    <TenantPage
      title={
        run ? `Monatsbeitrag ${formatMonth(run.billingMonth)}` : "Monatslauf"
      }
      back
      backHref={traegerPath("/traeger/abrechnung/monatsbeitraege")}
      backLabel="Zurück zu den Monatsbeiträgen"
      stats={
        run ? (
          <HolidayHeadFacts>
            <StatusBadge
              label={CARE_RUN_STATUS_LABELS[run.status]}
              tone={CARE_RUN_STATUS_TONES[run.status]}
            />
            <span className="tabular-nums">
              {invoiceCountLabel(run.previewCount)} ·{" "}
              {formatEuro(run.previewCents)}
            </span>
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!run && !pageLoad.error}
      actions={
        preview ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={!canRelease}
            onClick={() => setConfirming(true)}
          >
            Lauf freigeben
          </Button>
        ) : undefined
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !run}
      overlays={
        run ? (
          <>
            <CareLetterPanel
              runId={run.id}
              letter={shownLetter}
              onClose={() => setShownLetter(null)}
            />
            <BillingCorrectionModal
              runId={run.id}
              target={cancelling}
              onClose={() => setCancelling(null)}
              onCareCancelled={(next) => {
                setCancelling(null);
                setLetters(next);
              }}
            />
            <ConfirmationModal
              isOpen={confirming}
              title="Monatslauf freigeben?"
              confirmText="Freigeben"
              isConfirmLoading={releasing}
              loadingText="Wird freigegeben …"
              onClose={() => setConfirming(false)}
              onConfirm={() => void release()}
            >
              <p className="text-sm text-gray-700">
                {invoiceCountLabel(run.previewCount)} über{" "}
                {formatEuro(run.previewCents)} für{" "}
                {formatMonth(run.billingMonth)} bekommen ihre Nummer. Danach
                lassen sie sich nicht mehr ändern.
              </p>
              {run.duplicateChildren > 0 ? (
                <p className="mt-3 text-sm text-gray-700">
                  {notBilledLabel(run.duplicateChildren)} Das holen Sie mit
                  einem neuen Lauf nach, wenn ein Vertrag beendet ist.
                </p>
              ) : null}
            </ConfirmationModal>
          </>
        ) : null
      }
    >
      {run ? (
        <>
          <SectionCard
            title="Monatslauf"
            description={
              preview
                ? "Vorschau nach den Verträgen, die in diesem Monat gelten. Neue Verträge und Preise kommen bis zur Freigabe dazu."
                : "Die Rechnungen sind ausgestellt und lassen sich nicht mehr ändern."
            }
          >
            <DataGrid columns={2}>
              <DataField label="Monat">
                {formatMonth(run.billingMonth)}
              </DataField>
              <DataField label="Rechnungen">
                <span className="tabular-nums">
                  {invoiceCountLabel(run.previewCount)} ·{" "}
                  {formatEuro(run.previewCents)}
                </span>
              </DataField>
              <DataField label="Abgerechnet wird">
                Nach Vertrag, nicht nach Anwesenheit
              </DataField>
              <DataField label="Vertrag ändert sich im Monat">
                {run.monthRule ? MONTH_RULE_LABELS[run.monthRule] : "–"}
              </DataField>
              <DataField label="Rechnungsnummern">
                {run.issuedOn ? (
                  `Ausgestellt am ${formatDate(run.issuedOn)}`
                ) : run.numberRangeSet ? (
                  <span className="font-mono">{run.numberPattern}</span>
                ) : (
                  "Noch kein Muster"
                )}
              </DataField>
              {run.billed > 0 ? (
                <DataField label="Schon abgerechnet">
                  {run.billed === 1
                    ? "1 Kind aus einem früheren Lauf"
                    : `${run.billed} Kinder aus früheren Läufen`}
                </DataField>
              ) : null}
            </DataGrid>
          </SectionCard>
          {preview && !run.numberRangeSet ? (
            <Alert
              type="warning"
              message="Bevor Sie freigeben: Legen Sie das Muster der Rechnungsnummern für Monatsbeiträge fest."
              action={
                <ButtonLink
                  href={traegerPath("/traeger/einstellungen/abrechnung")}
                  variant="surface"
                  size="compact"
                >
                  Abrechnungsvorgaben öffnen
                </ButtonLink>
              }
            />
          ) : null}
          <CareDuplicatesCard duplicates={run.duplicates} released={!preview} />
          {preview ? <CareUnpricedCard unpriced={run.unpriced} /> : null}
          {released ? (
            <CareLettersCard
              runId={run.id}
              letters={letters}
              loadError={lettersLoad.error}
              sendingAll={sendingAll}
              printing={printing}
              onCreated={setLetters}
              onSendAll={sendAll}
              onPrint={(withoutEmailOnly) => void print(withoutEmailOnly)}
            />
          ) : null}
          {cancelledCount > 0 ? (
            <Alert
              type="info"
              title={
                cancelledCount === 1
                  ? "1 Rechnung ist storniert"
                  : `${cancelledCount} Rechnungen sind storniert`
              }
              message={`Ein neuer Monatslauf für ${formatMonth(run.billingMonth)} stellt sie neu aus. Er zeigt sie zuerst als Vorschau.`}
              action={
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  isLoading={reissuing}
                  loadingText="Wird geöffnet …"
                  onClick={() => void reissue()}
                >
                  Neu ausstellen
                </Button>
              }
            />
          ) : null}
          <CareRunInvoices
            invoices={run.invoices}
            month={run.billingMonth}
            released={!preview}
            letterActions={
              released && letters
                ? {
                    letters: letters.documents,
                    created: letters.letters !== null,
                    sendingKey,
                    onShowLetter: setShownLetter,
                    onSend: (letter) => void sendOne(letter),
                    onCancel: (invoice) =>
                      setCancelling({ kind: "care-cancellation", invoice }),
                  }
                : undefined
            }
          />
        </>
      ) : null}
    </TenantPage>
  );
}
