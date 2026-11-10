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
  "parent_billing.datev_revenue_account": "Erlöskonto für Elternbeiträge",
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
  }
}

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
  runLabel,
  problems,
  onDismiss,
}: Readonly<{
  runLabel: string;
  problems: readonly DatevBookingProblem[];
  onDismiss: () => void;
}>) {
  const debtor = problems.some((problem) => problem.field === "debtor_number");
  return (
    <Alert
      type="warning"
      title={`Keine DATEV-Datei für ${runLabel}`}
      message={
        debtor
          ? "DATEV nimmt diese Rechnungen so nicht an. Es wurde keine Datei erstellt. So geht es weiter: Tragen Sie bei der Familie die richtige Debitorennummer ein. Stornieren Sie dann die Rechnung und stellen Sie sie neu aus. Danach ist der Lauf wieder frei."
          : "DATEV nimmt diese Rechnungen so nicht an. Es wurde keine Datei erstellt."
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
export function fiscalYearLabel(start: string): string {
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
  onConfirm,
  onClose,
}: Readonly<{
  runLabel: string | null;
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
      title="Alle Buchungen erneut herunterladen?"
      confirmText="Erneut herunterladen"
      confirmVariant="warning"
      isConfirmLoading={loading}
      loadingText="Wird erstellt …"
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
          message="Geben Sie die Datei nur weiter, wenn Ihr Steuerbüro die frühere Datei nicht hat oder nicht einlesen konnte."
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
