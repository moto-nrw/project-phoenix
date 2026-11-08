"use client";

// Schreiben und Versand eines freigegebenen Monatslaufs (#4684): die
// Schreiben an die Familien erstellen, per E-Mail und in die Eltern-App
// senden, drucken und eine Rechnung stornieren. Ein neuer Lauf für denselben
// Monat stellt eine stornierte Rechnung neu aus. Die Seite des Laufs zeigt
// davon nur den Stand, damit sie diesen Code nicht lädt.

import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  BillingCorrectionModal,
  type BillingCorrectionTarget,
} from "~/components/traeger/billing-correction-modal";
import {
  CARE_RUN_STATUS_LABELS,
  CARE_RUN_STATUS_TONES,
  formatMonth,
  invoiceCountLabel,
} from "~/components/traeger/care-billing-format";
import { CareLetterPanel } from "~/components/traeger/care-letter-panel";
import {
  CareLettersCard,
  familyCount,
} from "~/components/traeger/care-letters-card";
import { careLetterKey } from "~/components/traeger/care-run-invoices";
import { CareRunLetterInvoices } from "~/components/traeger/care-run-letter-invoices";
import { formatEuro } from "~/components/traeger/contribution-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
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
  startCareRun,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCareRunLettersPage" });

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

export default function TraegerCareRunLettersPage() {
  const params = useParams<{ id: string }>();
  const runId = params?.id ?? "";
  const runPath = traegerPath(`/traeger/abrechnung/monatsbeitraege/${runId}`);
  const [run, setRun] = useState<CareRunDetail | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
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
      const cancelled = new Set(
        latest.documents
          .filter(
            (document) =>
              document.kind === "cancellation" && !document.replaced,
          )
          .map((document) => document.invoiceId),
      );
      if (cancelled.size === 0) return;
      const schoolIds = [
        ...new Set(
          run.invoices
            .filter((invoice) => cancelled.has(invoice.id))
            .map((invoice) => invoice.schoolId),
        ),
      ];
      if (schoolIds.length === 0)
        throw new Error("Cancelled invoices have no school in the run");
      // Work through existing previews first, even if they cover only
      // part of the outstanding cancellations. The remaining schools
      // keep offering reissue after that preview has been released.
      const findOpen = async () =>
        (await fetchCareRuns()).runs.find(
          (other) =>
            other.status === "calculated" &&
            other.billingMonth === run.billingMonth &&
            schoolIds.some((id) => other.schoolIds.includes(id)),
        );
      let target = await findOpen();
      if (!target) {
        try {
          target = await startCareRun({
            billingMonth: run.billingMonth,
            schoolIds,
            invoiceDate: null,
          });
        } catch (err) {
          if (
            !(err instanceof ApiError) ||
            err.code !== "parent_billing.care_run_open"
          )
            throw err;
          // A parallel start may have reserved one of these schools.
          target = await findOpen();
          if (!target) throw err;
        }
      }
      router.push(
        traegerPath(`/traeger/abrechnung/monatsbeitraege/${target.id}`),
      );
    } catch (err) {
      logger.error("traeger_care_reissue_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionErrors.show(err, { object: "der neue Lauf" });
    } finally {
      setReissuing(false);
    }
  };

  const cancelledCount =
    letters?.documents.filter(
      (document) => document.kind === "cancellation" && !document.replaced,
    ).length ?? 0;

  return (
    <TenantPage
      title={
        run ? `Schreiben für ${formatMonth(run.billingMonth)}` : "Schreiben"
      }
      back
      backHref={runPath}
      backLabel="Zurück zum Monatslauf"
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
      error={pageLoad.error}
      loading={!pageLoad.error && !run}
      overlays={
        run && released ? (
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
          </>
        ) : null
      }
    >
      {run && !released ? (
        <SectionCard
          title="Noch nicht freigegeben"
          description="Schreiben gibt es erst, wenn der Monatslauf freigegeben ist. Geben Sie ihn auf der Seite des Laufs frei."
        />
      ) : null}
      {run && released ? (
        <>
          <CareLettersCard
            runId={run.id}
            payments={run.invoices.map((invoice) => invoice.payment)}
            letters={letters}
            loadError={lettersLoad.error}
            sendingAll={sendingAll}
            printing={printing}
            onCreated={setLetters}
            onSendAll={sendAll}
            onPrint={(withoutEmailOnly) => void print(withoutEmailOnly)}
          />
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
          {letters ? (
            <CareRunLetterInvoices
              invoices={run.invoices}
              month={run.billingMonth}
              letterActions={{
                letters: letters.documents,
                created: letters.letters !== null,
                sendingKey,
                onShowLetter: setShownLetter,
                onSend: (letter) => void sendOne(letter),
                onCancel: (invoice) =>
                  setCancelling({ kind: "care-cancellation", invoice }),
              }}
            />
          ) : null}
        </>
      ) : null}
    </TenantPage>
  );
}
