"use client";

// Der DATEV-Buchungsstapel auf der Seite „Exporte“ (#4694): welche
// DATEV-Werte noch fehlen, bevor es eine Datei gibt, und welche Rechnungen
// DATEV ablehnen würde, wenn ein Download sie nennt. Beide Hinweise sagen,
// was als Nächstes zu tun ist.

import { ButtonLink } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
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
export function datevSettingLines(readiness: DatevReadiness): string[] {
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
export function datevProblemText(problem: DatevBookingProblem): string {
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
export function datevProblemLines(
  problems: readonly DatevBookingProblem[],
): string[] {
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
          ? "DATEV nimmt diese Rechnungen so nicht an. Eine Debitorennummer hat nur Ziffern, eine mehr als Ihre Sachkonten. Tragen Sie sie bei der Familie ein. Eine ausgestellte Rechnung behält ihre Nummer: Stornieren Sie sie und stellen Sie sie neu aus."
          : "DATEV nimmt diese Rechnungen so nicht an. Es wurde keine Datei erstellt."
      }
      details={<BulletList lines={datevProblemLines(problems)} />}
      onDismiss={onDismiss}
      dismissLabel="Hinweis schließen"
    />
  );
}
