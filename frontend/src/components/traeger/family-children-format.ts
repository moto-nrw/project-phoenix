/** Display helpers of the children of the families (#5918). */

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  FamilyChildCount,
  FamilyChildren,
  FamilyChildrenStatus,
} from "~/lib/traeger-family-children-api";

/** States the Abrechnung has to look at. */
export const NEEDS_CHECK: readonly FamilyChildrenStatus[] = [
  "stated",
  "changed",
  "expired",
  "expiring",
];

export const STATUS_LABELS: Record<FamilyChildrenStatus, string> = {
  missing: "Keine Angabe",
  stated: "Neu von Eltern",
  changed: "Eltern melden Änderung",
  confirmed: "Bestätigt",
  expiring: "Läuft bald ab",
  expired: "Abgelaufen",
  upcoming: "Gilt später",
};

export const STATUS_TONES: Record<FamilyChildrenStatus, StatusBadgeTone> = {
  missing: "gray",
  stated: "orange",
  changed: "orange",
  confirmed: "green",
  expiring: "orange",
  expired: "red",
  upcoming: "blue",
};

/** "1 Kind", "3 Kinder". */
export function childrenLabel(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

/** "Familie Beispiel" or the address when the name is missing. */
export function familyName(family: FamilyChildren): string {
  const name =
    `${family.recipientFirstName} ${family.recipientLastName}`.trim();
  return name || family.recipientEmail;
}

/** "ab 01.09.2026" or "01.09.2026 bis 31.07.2027". */
export function validityLabel(count: FamilyChildCount): string {
  if (!count.validFrom) return "";
  return count.validUntil
    ? `${formatDate(count.validFrom)} bis ${formatDate(count.validUntil)}`
    : `ab ${formatDate(count.validFrom)}`;
}

/** The calendar day of an instant the parents stated their number on. */
export function statedOn(count: FamilyChildCount): string {
  return count.statedAt ? formatDate(count.statedAt) : "";
}

/** What the Abrechnung has to check, in one short line. */
export function checkReason(family: FamilyChildren, today: string): string {
  const count = family.count;
  if (!count) return "";
  // A later confirmation is stored as the current one: the state then
  // belongs to the earlier confirmation, whose end the page does not know.
  const later =
    count.validFrom !== null && count.validFrom > today
      ? formatDate(count.validFrom)
      : null;
  if (later && family.status === "expired") {
    return `Die bisherige Kinderzahl ist abgelaufen. Die neue gilt erst ab ${later}.`;
  }
  if (later && family.status === "expiring") {
    return `Die bisherige Kinderzahl läuft bald ab. Die neue gilt erst ab ${later}.`;
  }
  switch (family.status) {
    case "stated":
      return `Eltern geben ${childrenLabel(count.statedChildren ?? 0)} an (am ${statedOn(count)}).`;
    case "changed":
      return `Eltern geben jetzt ${childrenLabel(count.statedChildren ?? 0)} an. Bestätigt sind ${childrenLabel(count.confirmedChildren ?? 0)}.`;
    case "expired":
      return `Die Kinderzahl galt bis ${formatDate(count.validUntil ?? "")}. Bitte neu prüfen.`;
    case "expiring":
      return `Die Kinderzahl gilt nur bis ${formatDate(count.validUntil ?? "")}.`;
    default:
      return "";
  }
}
