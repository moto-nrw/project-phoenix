"use client";

// Der DATEV-Buchungsstapel auf der Seite „Exporte“ (#4694, #6433): welche
// DATEV-Werte noch fehlen, bevor es eine Datei gibt, welche Rechnungen
// DATEV ablehnen würde, dass ein Lauf keine neuen Buchungen hat, und die
// ausdrückliche Wahl „Alle Buchungen erneut“ mit ihrer Warnung. Jeder
// Hinweis sagt, was als Nächstes zu tun ist.

import { useState } from "react";
import { ButtonLink } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionLabel } from "~/components/ui/section-label";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type {
  DatevBookingProblem,
  DatevReadiness,
  ExportRun,
} from "~/lib/traeger-finance-exports-api";
import { traegerPath } from "~/lib/traeger-url";

/** The DATEV area of the Träger settings. */
export const DATEV_SETTINGS_PATH = "/traeger/einstellungen?bereich=datev";

/** The id of the notice the disabled DATEV buttons point to. */
export const DATEV_READINESS_ID = "datev-readiness";

/** The name of each DATEV value as the settings page shows it. */
const DATEV_SETTING_LABELS: Record<string, string> = {
  "parent_billing.datev_consultant_number": "Beraternummer",
  "parent_billing.datev_client_number": "Mandantennummer",
  "parent_billing.datev_account_length": "Länge der Sachkonten",
  "parent_billing.datev_chart_of_accounts": "Kontenrahmen",
  "parent_billing.datev_fiscal_year_start": "Beginn des Wirtschaftsjahres",
  "parent_billing.datev_revenue_account": "Standard-Erlöskonto",
};

function settingLabel(key: string): string {
  return DATEV_SETTING_LABELS[key] ?? key;
}

/** What is wrong with each DATEV value, in the order of the settings. */
function datevSettingLines(readiness: DatevReadiness): string[] {
  const order = Object.keys(DATEV_SETTING_LABELS);
  const rank = (key: string) => {
    const index = order.indexOf(key);
    return index === -1 ? order.length : index;
  };
  return [
    ...readiness.missingSettings.map((key) => ({ key, state: "fehlt" })),
    ...readiness.invalidSettings.map((key) => ({ key, state: "passt nicht" })),
  ]
    .sort((a, b) => rank(a.key) - rank(b.key))
    .map(({ key, state }) => `${settingLabel(key)}: ${state}`);
}

function BulletList({ lines }: Readonly<{ lines: readonly string[] }>) {
  return (
    <ul className="mt-1 list-disc space-y-0.5 pl-5">
      {lines.map((line) => (
        <li key={line}>{line}</li>
      ))}
    </ul>
  );
}

/**
 * Names the DATEV values that are missing or wrong, with the way to the
 * settings. Shows nothing once every value fits.
 */
export function DatevReadinessNotice({
  readiness,
}: Readonly<{ readiness: DatevReadiness }>) {
  if (readiness.ready) return null;
  return (
    <Alert
      id={DATEV_READINESS_ID}
      type="warning"
      title="Die DATEV-Datei fehlt noch"
      message="Es fehlen Werte Ihres Steuerbüros. Danach können Sie die Datei herunterladen."
      details={<BulletList lines={datevSettingLines(readiness)} />}
      action={
        <ButtonLink
          href={traegerPath(DATEV_SETTINGS_PATH)}
          variant="ghost_tone"
          size="compact"
        >
          DATEV-Werte eintragen
        </ButtonLink>
      }
      announce="off"
    />
  );
}

/** Why DATEV would refuse an invoice, in the words of the screen. */
function datevProblemText(problem: DatevBookingProblem): string {
  switch (problem.field) {
    case "debtor_number":
      return problem.reason === "missing"
        ? "Debitorennummer fehlt"
        : "Debitorennummer passt nicht";
    case "document_date":
      return "Liegt in einem anderen Wirtschaftsjahr";
    case "document_number":
      return "Rechnungsnummer passt nicht zu DATEV";
    case "amount":
      return "Betrag passt nicht zu DATEV";
    case "revenue_account":
      return problem.reason === "missing"
        ? "Erlöskonto fehlt"
        : "Erlöskonto ist länger als Ihre Sachkonten";
    case "cost_center":
      return problem.reason === "missing"
        ? "Kostenstelle fehlt"
        : "Kostenstelle passt nicht zu DATEV";
    case "cost_center_2":
      return "Zweite Kostenstelle passt nicht zu DATEV";
  }
}

/** Where the Träger keeps its revenue accounts and cost centers (#4821). */
export const BOOKING_ACCOUNTS_PATH = "/traeger/abrechnung/exporte/konten";

/**
 * One line per reason with the invoices it names, so a reason that applies
 * to many invoices stands once.
 */
function datevProblemLines(problems: readonly DatevBookingProblem[]): string[] {
  const byReason = new Map<string, string[]>();
  for (const problem of problems) {
    const text = datevProblemText(problem);
    const numbers = byReason.get(text) ?? [];
    if (!numbers.includes(problem.documentNumber)) {
      numbers.push(problem.documentNumber);
    }
    byReason.set(text, numbers);
  }
  return [...byReason].map(
    ([text, numbers]) => `${text}: ${numbers.join(", ")}`,
  );
}

/**
 * The invoices a refused DATEV download named. No file left moto and no
 * number was used.
 */
export function DatevRefusedNotice({
  runKind,
  runLabel,
  problems,
  onDismiss,
}: Readonly<{
  runKind: ExportRun["runKind"];
  runLabel: string;
  problems: readonly DatevBookingProblem[];
  onDismiss: () => void;
}>) {
  const debtor = problems.some((problem) => problem.field === "debtor_number");
  const accounts = problems.some((problem) =>
    ["revenue_account", "cost_center", "cost_center_2"].includes(problem.field),
  );
  const correction = accounts
    ? "Schon festgehaltene Konten und Kostenstellen bleiben unverändert. Auch ein Storno übernimmt diese Werte. Laden Sie dann die Sollstellung dieses Laufs herunter. Geben Sie die Datei und diesen Hinweis an Ihr Steuerbüro. Es kann die genannten Werte berichtigen und manuell in DATEV buchen. Der DATEV-Download dieses Laufs bleibt dann gesperrt. Korrigieren Sie unter „Konten und Kostenstellen“ die Werte für neue Rechnungen."
    : runKind === "holiday"
      ? "Bei Rechnungen ohne Gutschrift: Stornieren Sie die Rechnung. Korrigieren Sie die Debitorennummer in der Ferienanmeldung. Stellen Sie dann die Rechnung neu aus. Danach ist der Lauf wieder frei. Hat die Rechnung schon eine Gutschrift, ist kein Storno möglich. Laden Sie dann die Sollstellung für Ihr Steuerbüro herunter. Es kann die Buchungen daraus manuell in DATEV erfassen. Der DATEV-Download dieses Laufs bleibt gesperrt."
      : "Tragen Sie bei der Familie die richtige Debitorennummer ein. Stornieren Sie dann die Rechnung und stellen Sie sie neu aus. Danach ist der Lauf wieder frei.";
  return (
    <Alert
      type="warning"
      title={`Keine DATEV-Datei für ${runLabel}`}
      message={
        accounts || debtor
          ? `DATEV nimmt diese Rechnungen so nicht an. Es wurde keine Datei erstellt. So geht es weiter: ${correction}`
          : "DATEV nimmt diese Rechnungen so nicht an. Es wurde keine Datei erstellt."
      }
      action={
        accounts ? (
          <ButtonLink
            href={traegerPath(BOOKING_ACCOUNTS_PATH)}
            variant="ghost_tone"
            size="compact"
          >
            Konten und Kostenstellen
          </ButtonLink>
        ) : undefined
      }
      details={<BulletList lines={datevProblemLines(problems)} />}
      onDismiss={onDismiss}
      dismissLabel="Hinweis schließen"
    />
  );
}

/**
 * A standard DATEV download found nothing new: every booking of the run is
 * in a DATEV file, or another download took them a moment earlier. No file
 * left moto and no number was used.
 */
export function DatevNothingNewNotice({
  runLabel,
  taken,
  onDismiss,
}: Readonly<{
  runLabel: string;
  taken: boolean;
  onDismiss: () => void;
}>) {
  return (
    <Alert
      type="info"
      title={`Keine neuen Buchungen für ${runLabel}`}
      message={
        taken
          ? "Jemand hat die neuen Buchungen eben heruntergeladen. Sie stehen im Exportprotokoll. Es wurde keine weitere Datei erstellt und keine Nummer verbraucht."
          : "Alle Buchungen dieses Laufs sind schon in einer DATEV-Datei. Es wurde keine Datei erstellt und keine Nummer verbraucht."
      }
      onDismiss={onDismiss}
      dismissLabel="Hinweis schließen"
    />
  );
}

/**
 * The name of a fiscal year by its first day: the year, or both years when
 * it does not start in January ("2026/27").
 */
function fiscalYearLabel(start: string): string {
  const [year, month] = start.split("-");
  if (!year || month === "01") return year ?? start;
  return `${year}/${String(Number(year) + 1).slice(-2)}`;
}

/**
 * Asks before a DATEV file of every booking of a run again (#6433). The
 * standard download never repeats a booking; this one does on purpose, for
 * a file the tax office lost, so it says what happens if the tax office
 * reads it in twice. With bookings in several fiscal years the person picks
 * one: a DATEV file holds one fiscal year.
 */
export function DatevRepeatDialog({
  runLabel,
  fiscalYears,
  loading,
  handover = false,
  onConfirm,
  onClose,
}: Readonly<{
  runLabel: string | null;
  /** The file goes to the recipient of the Träger instead of the browser. */
  handover?: boolean;
  fiscalYears: readonly string[];
  loading: boolean;
  onConfirm: (fiscalYear: string | undefined) => void;
  onClose: () => void;
}>) {
  const [chosen, setChosen] = useState<string | null>(null);
  const year =
    chosen !== null && fiscalYears.includes(chosen)
      ? chosen
      : (fiscalYears[0] ?? null);
  const close = () => {
    setChosen(null);
    onClose();
  };
  return (
    <ConfirmationModal
      isOpen={runLabel !== null}
      onClose={close}
      onConfirm={() => onConfirm(year ?? undefined)}
      title={
        handover
          ? "Alle Buchungen erneut übergeben?"
          : "Alle Buchungen erneut herunterladen?"
      }
      confirmText={handover ? "Erneut übergeben" : "Erneut herunterladen"}
      confirmVariant="warning"
      isConfirmLoading={loading}
      loadingText={handover ? "Wird übergeben …" : "Wird erstellt …"}
      mobileSheet
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-700">
          Die Datei enthält alle Buchungen von {runLabel}, auch die schon
          exportierten.
        </p>
        <Alert
          type="warning"
          title="Buchungen können doppelt in DATEV stehen"
          message={
            handover
              ? "Übergeben Sie die Datei nur, wenn der Empfänger die frühere Datei nicht hat oder nicht einlesen konnte."
              : "Geben Sie die Datei nur weiter, wenn Ihr Steuerbüro die frühere Datei nicht hat oder nicht einlesen konnte."
          }
          announce="off"
        />
        {fiscalYears.length > 1 && year !== null ? (
          <div className="space-y-2">
            <SectionLabel as="p">Wirtschaftsjahr</SectionLabel>
            <SegmentedControl
              items={fiscalYears.map((start) => ({
                value: start,
                label: fiscalYearLabel(start),
              }))}
              value={year}
              onChange={setChosen}
              ariaLabel="Wirtschaftsjahr der Datei"
              fullWidth
              wrap
            />
            <p className="text-sm text-gray-600">
              Eine DATEV-Datei gilt für ein Wirtschaftsjahr.
            </p>
          </div>
        ) : null}
        <p className="text-sm text-gray-600">
          Die Datei bekommt eine neue Nummer. Im Exportprotokoll steht sie als
          Wiederholung.
        </p>
      </div>
    </ConfirmationModal>
  );
}
