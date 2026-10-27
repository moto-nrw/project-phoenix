"use client";

// Der Versand der Bestätigungsschreiben eines Laufs (#4670): jede Familie
// bekommt ihr Schreiben als PDF im Anhang einer Mail, an alle auf einmal
// oder einzeln. Je Familie steht, ob die Mail raus ist; eine Mail, die nicht
// ankam, lässt sich erneut senden. Alle Schreiben lassen sich in einer Datei
// drucken, für Familien ohne Internet.

import { useCallback, useEffect, useState } from "react";
import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatBerlinDate, formatTime } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  fetchLetterMailings,
  type LetterMailing,
  type LetterMailStatus,
  type LetterSeries,
  mailBillingLetter,
  mailBillingLetters,
  printBillingLetters,
} from "~/lib/traeger-billing-api";

const logger = createLogger({ component: "BillingMailCard" });

// While a mail waits, the card looks again after this many milliseconds.
const REFRESH_WHILE_QUEUED_MS = 15_000;

const STATUS_TONES: Record<
  LetterMailStatus,
  "blue" | "green" | "red" | "gray"
> = {
  not_sent: "gray",
  queued: "blue",
  sent: "green",
  failed: "red",
  unknown: "gray",
};

const STATUS_LABELS: Record<LetterMailStatus, string> = {
  not_sent: "Nicht gesendet",
  queued: "Wird gesendet",
  sent: "Gesendet",
  failed: "Nicht angekommen",
  unknown: "Ergebnis unbekannt",
};

// The date beside the short status; it wraps on small screens.
function statusDetail(mailing: LetterMailing): string | null {
  const at = (iso: string) =>
    `${formatBerlinDate(iso)}, ${formatTime(iso)} Uhr`;
  if (mailing.status === "sent" && mailing.sentAt) {
    return `am ${at(mailing.sentAt)}`;
  }
  if (mailing.status === "unknown" && mailing.requestedAt) {
    return `Versand angefragt am ${at(mailing.requestedAt)}. Ob die Mail ankam, ist nicht bekannt.`;
  }
  return null;
}

function seriesMessage(series: LetterSeries): string {
  const parts: string[] = [];
  if (series.queued > 0) {
    parts.push(`${series.queued} Mail(s) werden jetzt gesendet.`);
  } else {
    parts.push("Es gab keine neue Mail zu senden.");
  }
  if (series.withoutEmail > 0) {
    parts.push(
      `${series.withoutEmail} Familie(n) ohne Mail-Adresse: Bitte drucken Sie deren Schreiben.`,
    );
  }
  return parts.join(" ");
}

export function BillingMailCard({ runId }: Readonly<{ runId: string }>) {
  const [mailings, setMailings] = useState<LetterMailing[] | null>(null);
  const [sendingAll, setSendingAll] = useState(false);
  const [sendingOne, setSendingOne] = useState<string | null>(null);
  const [printing, setPrinting] = useState(false);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const actionErrors = useApiErrorDisplay();
  const { success } = useToast();

  const refresh = useCallback(async () => {
    try {
      setMailings(await fetchLetterMailings(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_letter_mailings_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Versand per Mail",
        retry: () => void refresh(),
      });
    }
  }, [runId, clearLoadError, showLoadError]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const waiting = mailings?.some((mailing) => mailing.status === "queued");
  useEffect(() => {
    if (!waiting) return;
    const timer = setTimeout(() => void refresh(), REFRESH_WHILE_QUEUED_MS);
    return () => clearTimeout(timer);
  }, [waiting, mailings, refresh]);

  const sendAll = async () => {
    setSendingAll(true);
    try {
      success(seriesMessage(await mailBillingLetters(runId)));
    } catch (err) {
      logger.error("traeger_letter_series_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionErrors.show(err, { object: "die Mails" });
    } finally {
      setSendingAll(false);
      void refresh();
    }
  };

  const sendOne = async (mailing: LetterMailing) => {
    setSendingOne(mailing.documentId);
    try {
      await mailBillingLetter(runId, mailing.documentId);
      success(
        `Die Mail an Familie ${mailing.recipientLastName} wird jetzt gesendet.`,
      );
    } catch (err) {
      logger.error("traeger_letter_mail_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionErrors.show(err, { object: "die Mail" });
    } finally {
      setSendingOne(null);
      void refresh();
    }
  };

  const print = async () => {
    setPrinting(true);
    try {
      const file = await printBillingLetters(runId);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_letters_print_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionErrors.show(err, { object: "die Schreiben" });
    } finally {
      setPrinting(false);
    }
  };

  const failed = mailings?.some((mailing) => mailing.status === "failed");

  return (
    <SectionCard
      title="Versand per Mail"
      description="Jede Familie bekommt ihr Schreiben als PDF im Anhang einer Mail. Familien ohne Internet geben Sie das gedruckte Schreiben."
      actions={
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            isLoading={printing}
            loadingText="Wird erstellt …"
            onClick={() => void print()}
          >
            Alle Schreiben drucken
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            isLoading={sendingAll}
            loadingText="Wird gesendet …"
            disabled={!mailings}
            onClick={() => void sendAll()}
          >
            An alle Familien senden
          </Button>
        </div>
      }
    >
      {load.error ? (
        <LoadErrorAlert error={load.error} />
      ) : mailings ? (
        <div className="space-y-3">
          <p className="text-sm text-gray-700">
            „An alle Familien senden“ schickt nur Mails, die noch nicht gesendet
            sind.
            {failed
              ? " Kam eine Mail nicht an, senden Sie sie erneut. Stimmt die Adresse nicht, drucken Sie das Schreiben."
              : null}
          </p>
          <ul className="divide-y divide-gray-100">
            {mailings.map((mailing) => (
              <li
                key={mailing.documentId}
                className="flex flex-wrap items-center justify-between gap-2 py-3"
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium text-gray-900">
                    {mailing.recipientFirstName} {mailing.recipientLastName}
                    <span className="ml-2 font-normal text-gray-500">
                      {mailing.number}
                    </span>
                  </p>
                  <p className="truncate text-sm text-gray-600">
                    {mailing.recipientEmail ||
                      "Keine Mail-Adresse: Bitte drucken Sie das Schreiben."}
                  </p>
                </div>
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <StatusBadge
                    label={STATUS_LABELS[mailing.status]}
                    tone={STATUS_TONES[mailing.status]}
                  />
                  {statusDetail(mailing) ? (
                    <span className="text-sm text-gray-600">
                      {statusDetail(mailing)}
                    </span>
                  ) : null}
                  {mailing.recipientEmail && mailing.status !== "queued" ? (
                    <Button
                      type="button"
                      variant="ghost"
                      size="compact"
                      isLoading={sendingOne === mailing.documentId}
                      loadingText="Wird gesendet …"
                      disabled={sendingOne !== null || sendingAll}
                      onClick={() => void sendOne(mailing)}
                    >
                      {mailing.status === "not_sent"
                        ? "Senden"
                        : "Erneut senden"}
                    </Button>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <Loading fullPage={false} />
      )}
    </SectionCard>
  );
}
