import type { ResolvedSetting } from "~/lib/settings-api";
import type { SettingsAudience } from "./settings-origin";

/**
 * Löschfristen trägt das moto-Team je Schule ein, die Schule liest sie nur
 * (ADR 0048). Ihre Stufe ist "operator_managed".
 */
export function isOperatorManaged(setting: ResolvedSetting): boolean {
  return setting.access_policy === "operator_managed";
}

function toDays(value: unknown): number | null {
  const days = typeof value === "string" ? Number(value) : value;
  return typeof days === "number" && Number.isFinite(days) ? days : null;
}

/**
 * Returns the new window in days when saving next would shorten a retention
 * window in the operator dashboard, else null. The next nightly run then
 * deletes everything older for good, so the save needs a confirmation;
 * lengthening a window needs none.
 */
export function shortenedRetentionDays(
  setting: ResolvedSetting,
  next: unknown,
  audience: SettingsAudience,
): number | null {
  if (audience !== "operator" || !isOperatorManaged(setting)) return null;
  const current = toDays(setting.value);
  const nextDays = toDays(next);
  if (current === null || nextDays === null) return null;
  return nextDays < current ? nextDays : null;
}

export function retentionShorteningText(
  setting: ResolvedSetting,
  days: number,
): string {
  if (setting.key === "gdpr.parent_message_retention_days") {
    return `Beim nächsten nächtlichen Lauf löscht moto bei dieser Schule Elternnachrichten, die älter als ${days} Tage sind. Offene Elternanfragen bleiben erhalten. Gelöschte Daten lassen sich nicht wiederherstellen.`;
  }
  return `Beim nächsten nächtlichen Lauf löscht moto bei dieser Schule alle Daten dieser Art, die älter als ${days} Tage sind. Gelöschte Daten lassen sich nicht wiederherstellen.`;
}
