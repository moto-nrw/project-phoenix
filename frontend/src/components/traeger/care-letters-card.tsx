"use client";

// Die Schreiben eines freigegebenen Monatslaufs (#4684): je Rechnung ein
// Schreiben im Briefkopf des Trägers. Erstellen ist nicht Senden: die
// Familien sehen nichts, bis ein Schreiben gesendet ist. Danach zeigt die
// Karte, wie weit der Versand ist, sendet an alle auf einmal und druckt die
// Schreiben, auch nur die der Familien ohne E-Mail. Die Mail und das Fach
// „Dokumente“ der Eltern-App bekommen dieselbe Datei.

import { ChevronDown } from "lucide-react";
import { useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ProgressBar } from "~/components/ui/progress-bar";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import type { useApiLoadError } from "~/contexts/ToastContext";
import type { InvoicePayment } from "~/lib/traeger-billing-terms-api";
import type { LetterSource } from "./billing-letters-modal";
import {
  BillingSettingsLink,
  LetterheadLink,
  LettersCreateCard,
  LettersFacts,
} from "./billing-letters-card";
import {
  type CareLetterSummary,
  type CareLetters,
  createCareLetters,
  previewCareLetter,
} from "~/lib/traeger-care-billing-api";

const TITLE = "Schreiben an die Familien";

/** The letters „An alle senden“ sends: open ones of families with an address. */
export function lettersToSend(summary: CareLetterSummary): number {
  return Math.max(0, summary.notSent - summary.withoutEmail);
}

/**
 * Why some letters stay out of the parents inbox, and how the family gets
 * them instead.
 */
export function withoutInboxNote(count: number): string {
  const letters = count === 1 ? "1 Schreiben ist" : `${count} Schreiben sind`;
  return `Nicht in der Eltern-App: ${letters} nicht unter „Dokumente“. Das Kind hat noch keinen Schülerdatensatz. Die Familie bekommt das Schreiben per E-Mail oder gedruckt.`;
}

/** „1 Familie“ / „3 Familien“. */
export function familyCount(count: number): string {
  return count === 1 ? "1 Familie" : `${count} Familien`;
}

/** The figures of the send state, each letter in exactly one of them. */
function SendState({ summary }: Readonly<{ summary: CareLetterSummary }>) {
  const open = lettersToSend(summary);
  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <p className="text-sm font-medium text-gray-900 tabular-nums">
          {summary.sent} von {summary.letters} Schreiben gesendet
        </p>
        <ProgressBar
          value={summary.sent}
          max={Math.max(summary.letters, 1)}
          tone="green"
          size="md"
          label={`${summary.sent} von ${summary.letters} Schreiben gesendet`}
        />
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
        <StatCard variant="tile" label="Gesendet" value={summary.sent} />
        <StatCard
          variant="tile"
          label="Wird gesendet"
          value={summary.queued}
          tone={summary.queued > 0 ? "blue" : undefined}
        />
        <StatCard
          variant="tile"
          label="Nicht angekommen"
          value={summary.failed}
          tone={summary.failed > 0 ? "red" : undefined}
        />
        <StatCard variant="tile" label="Noch nicht gesendet" value={open} />
        <StatCard
          variant="tile"
          label="Ohne E-Mail"
          value={summary.withoutEmail}
          tone={summary.withoutEmail > 0 ? "orange" : undefined}
        />
      </div>
    </div>
  );
}

export function CareLettersCard({
  runId,
  payments,
  letters,
  loadError,
  sendingAll,
  printing,
  onCreated,
  onSendAll,
  onPrint,
}: Readonly<{
  runId: string;
  payments: readonly (InvoicePayment | null | undefined)[];
  /** null while it loads. */
  letters: CareLetters | null;
  loadError: ReturnType<typeof useApiLoadError>["error"];
  sendingAll: boolean;
  /** The print that runs: all letters or those without an address. */
  printing: "all" | "without_email" | null;
  onCreated: (letters: CareLetters) => void;
  onSendAll: () => Promise<void>;
  onPrint: (withoutEmailOnly: boolean) => void;
}>) {
  const [confirming, setConfirming] = useState(false);

  if (loadError) {
    return (
      <SectionCard title={TITLE}>
        <LoadErrorAlert error={loadError} actionLayout="responsive" />
      </SectionCard>
    );
  }
  if (!letters) return <SectionCard title={TITLE} loading />;

  const source: LetterSource<CareLetters> = {
    create: (input) => createCareLetters(runId, input),
    preview: (input) => previewCareLetter(runId, input),
    intro:
      "Jede Familie bekommt ein Schreiben mit ihrer Rechnung. Ihre Hinweise stehen in allen Schreiben. Danach lässt sich nichts mehr ändern. Die Familien sehen die Schreiben erst, wenn Sie sie senden.",
    notesLabel: "Hinweis für die Familien (freiwillig)",
    notesPlaceholder:
      "Zum Beispiel: Ab Februar gilt eine neue Preisliste. Fragen beantwortet die Verwaltung.",
  };

  if (!letters.letters) {
    return (
      <LettersCreateCard<CareLetters>
        title={TITLE}
        description="Jede Familie bekommt ein Schreiben mit ihrer Rechnung. Erstellen sendet noch nichts: Die Familien sehen es erst, wenn Sie es senden."
        runId={runId}
        payments={payments}
        source={source}
        onCreated={onCreated}
      >
        Logo und Absender kommen aus <LetterheadLink />, die Bankverbindung aus
        den <BillingSettingsLink />. Ohne Bankverbindung lassen sich keine
        Schreiben erstellen.
      </LettersCreateCard>
    );
  }

  const { summary } = letters;
  const toSend = lettersToSend(summary);
  const printItems = [
    {
      label: "Alle Schreiben drucken",
      description: "Alle Schreiben, die noch gelten, in einer Datei",
      onClick: () => onPrint(false),
    },
    ...(summary.withoutEmail > 0
      ? [
          {
            label: "Nur Familien ohne E-Mail",
            description: `${familyCount(summary.withoutEmail)} ohne E-Mail-Adresse`,
            onClick: () => onPrint(true),
          },
        ]
      : []),
  ];

  return (
    <SectionCard
      title={TITLE}
      description="Jede Familie bekommt ihr Schreiben per E-Mail. Hat sie mit dieser Adresse ein Konto in der Eltern-App, liegt es auch dort unter „Dokumente“, wenn das Kind einen Schülerdatensatz hat. Familien ohne E-Mail geben Sie das gedruckte Schreiben."
      actions={
        <div className="flex flex-wrap gap-2">
          <OverflowMenu
            ariaLabel="Schreiben drucken"
            triggerButton={{ variant: "outline", size: "md" }}
            triggerClassName="gap-2 whitespace-nowrap"
            triggerContent={
              printing ? (
                "Wird erstellt …"
              ) : (
                <>
                  Drucken
                  <ChevronDown className="h-4 w-4" aria-hidden />
                </>
              )
            }
            disabled={printing !== null}
            items={printItems}
          />
          <Button
            type="button"
            variant="primary"
            size="md"
            isLoading={sendingAll}
            loadingText="Wird gesendet …"
            disabled={toSend === 0}
            onClick={() => setConfirming(true)}
          >
            An alle senden
          </Button>
        </div>
      }
    >
      <div className="space-y-5">
        <SendState summary={summary} />
        {summary.withoutInbox > 0 ? (
          <Alert type="info" message={withoutInboxNote(summary.withoutInbox)} />
        ) : null}
        {toSend === 0 && summary.letters > 0 ? (
          <p className="text-sm text-gray-600">
            Alle Schreiben mit E-Mail-Adresse sind gesendet. Einzelne Schreiben
            senden Sie erneut über das Menü ihrer Rechnung.
          </p>
        ) : null}
        <LettersFacts
          paymentDueOn={letters.letters.paymentDueOn}
          payments={payments}
          createdAt={letters.letters.createdAt}
          notes={letters.letters.notes}
          notesLabel="Hinweis für die Familien"
          emptyNotes="Keiner"
        />
      </div>
      <ConfirmationModal
        isOpen={confirming}
        title="An alle senden?"
        confirmText={
          toSend === 1 ? "1 Schreiben senden" : `${toSend} Schreiben senden`
        }
        isConfirmLoading={sendingAll}
        loadingText="Wird gesendet …"
        onClose={() => setConfirming(false)}
        onConfirm={() => {
          void onSendAll().finally(() => setConfirming(false));
        }}
      >
        <div className="space-y-3 text-sm text-gray-700">
          <p>
            {toSend === 1
              ? "1 Schreiben geht jetzt per E-Mail an die Familie."
              : `${toSend} Schreiben gehen jetzt per E-Mail an die Familien.`}{" "}
            Mit einem Konto in der Eltern-App liegt es auch dort unter
            „Dokumente“.
          </p>
          {summary.withoutEmail > 0 ? (
            <p>
              {familyCount(summary.withoutEmail)} ohne E-Mail-Adresse{" "}
              {summary.withoutEmail === 1 ? "bekommt" : "bekommen"} nichts.
              Drucken Sie deren Schreiben über „Drucken“.
            </p>
          ) : null}
        </div>
      </ConfirmationModal>
    </SectionCard>
  );
}
