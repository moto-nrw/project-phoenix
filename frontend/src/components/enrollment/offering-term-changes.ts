import type {
  AdminRequestChild,
  AdminRequestChildOffering,
} from "~/lib/enrollment-admin-api";
import { formatOfferingPrice } from "~/lib/care-offering-format";
import { weekdayShortList } from "~/lib/weekdays";

// The catalog terms of an offering can change after the parents submitted and
// signed (#6837). The detail names what the parents saw; the approval books
// the current terms only after the office confirms the change.

/** Whether the catalog changed price, days or activity since submission. */
export function offeringTermsChanged(
  offering: AdminRequestChildOffering,
): boolean {
  return (offering.changed_terms?.length ?? 0) > 0;
}

/** Current bookings of the child whose terms changed since submission. */
export function changedOfferings(
  child: AdminRequestChild,
): AdminRequestChildOffering[] {
  return (child.offerings ?? []).filter(offeringTermsChanged);
}

/**
 * One sentence per changed offering: what the parents chose at submission.
 * Empty when nothing changed.
 */
export function submittedTermsMessage(
  offering: AdminRequestChildOffering,
): string {
  const changed = offering.changed_terms ?? [];
  const submitted = offering.submitted_terms;
  if (changed.length === 0 || !submitted) return "";
  const parts: string[] = [];
  if (changed.includes("price")) {
    const price = formatOfferingPrice(submitted.price_cents);
    parts.push(price ? `${price} pro Monat` : "ohne Preis");
  }
  if (changed.includes("days")) {
    const days = weekdayShortList(submitted.available_days);
    if (submitted.days_of_week_mode === "parent_choice") {
      parts.push(days ? `Tage zur Wahl: ${days}` : "Tage zur Wahl");
    } else if (days) {
      parts.push(`Tage: ${days}`);
    }
  }
  const sentences = [];
  if (parts.length > 0) {
    sentences.push(
      `Seit der Anmeldung geändert. Die Eltern haben gewählt: ${parts.join(", ")}.`,
    );
  } else {
    sentences.push("Seit der Anmeldung geändert.");
  }
  if (changed.includes("activity")) {
    sentences.push("Das Angebot gehört jetzt zu einer anderen Aktivität.");
  }
  return sentences.join(" ");
}

export const APPROVAL_WITH_CHANGED_TERMS_COPY = {
  title: "Angebot wurde geändert",
  confirmText: "Neue Angaben bestätigen",
  message:
    "Ein Angebot hat sich nach der Anmeldung geändert. Beim Bestätigen gelten die neuen Angaben. Bitte informieren Sie die Familie.",
} as const;
