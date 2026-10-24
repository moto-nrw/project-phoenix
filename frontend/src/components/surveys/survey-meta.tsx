"use client";

import { summarizeTargets } from "~/components/announcements/announcement-meta";
import { StatusBadge } from "~/components/ui/status-badge";
import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { berlinClockFromISO, formatBerlinDate } from "~/lib/date-helpers";
import type { Survey, SurveyStatus } from "~/lib/surveys-api";

/**
 * Wörter und Abzeichen der Umfragen (#4781), geteilt von Liste und
 * Objektseite, damit beide dasselbe sagen.
 */

const SURVEY_STATUS_LABEL: Record<SurveyStatus, string> = {
  draft: "Entwurf",
  open: "Läuft",
  closed: "Beendet",
};

const SURVEY_STATUS_TONE: Record<SurveyStatus, StatusBadgeTone> = {
  draft: "gray",
  open: "green",
  closed: "blue",
};

export function SurveyStatusBadge({
  status,
}: Readonly<{ status: SurveyStatus }>) {
  return (
    <StatusBadge
      label={SURVEY_STATUS_LABEL[status]}
      tone={SURVEY_STATUS_TONE[status]}
    />
  );
}

export function SurveyAnonymityBadge({
  anonymous,
}: Readonly<{ anonymous: boolean }>) {
  return anonymous ? (
    <StatusBadge
      label="Anonym"
      tone="blue"
      title="Niemand sieht, wer was geantwortet hat."
    />
  ) : (
    <StatusBadge
      label="Nicht anonym"
      tone="gray"
      title="Bei jeder Antwort steht der Name."
    />
  );
}

/** „12.11.2026, 18:00 Uhr“ — Zeitpunkt in der Zeitzone der Schule. */
export function formatSurveyClosesAt(closesAt: string): string {
  return `${formatBerlinDate(closesAt)}, ${berlinClockFromISO(closesAt)} Uhr`;
}

/** „Eltern: Ganze Schule“ oder „Mitarbeitende“. */
export function describeSurveyAudience(
  survey: Pick<Survey, "audience" | "targets">,
): string {
  return survey.audience === "staff"
    ? "Mitarbeitende"
    : `Eltern: ${summarizeTargets(survey.targets)}`;
}

/** „1 Teilnahme“, „12 Teilnahmen“. */
export function participantLabel(count: number): string {
  return `${count} ${count === 1 ? "Teilnahme" : "Teilnahmen"}`;
}

/** SWR-Schlüssel der Liste; die Objektseite lädt sie nach jeder Aktion neu. */
export const SURVEY_LIST_KEY = "surveys-list";

/** Der Erfolgstoast nach „Erinnern“: nur die Zahl, nie wer. */
export function remindedMessage(count: number): string {
  return count === 0
    ? "Es wurde niemand erinnert. Entweder haben alle teilgenommen, oder die übrigen bekommen keine Benachrichtigungen."
    : `${count} ${count === 1 ? "Person wurde" : "Personen wurden"} an die Umfrage erinnert.`;
}
