/** Display helpers of the Elternabrechnung in the Träger portal (#4664). */

import { formatOfferingPrice } from "~/lib/care-offering-format";
import { formatDate } from "~/lib/date-helpers";
import type {
  ContributionLine,
  ContributionRule,
} from "~/lib/traeger-contributions-api";
import { formatDateRange } from "./holiday-period-format";

/** Cents → "12,50 €"; a discount keeps its minus sign. */
export function formatEuro(cents: number): string {
  return formatOfferingPrice(cents) ?? "";
}

/** 5000 → "50 %". */
export function formatShare(basisPoints: number): string {
  return `${new Intl.NumberFormat("de-DE", { maximumFractionDigits: 2 }).format(basisPoints / 100)} %`;
}

/** "50 % weniger" or "10,00 € weniger". */
function discountText(
  basisPoints: number | null,
  cents: number | null,
): string {
  if (basisPoints !== null) return `${formatShare(basisPoints)} weniger`;
  if (cents !== null) return `${formatEuro(cents)} weniger`;
  return "";
}

/** What one line of a contribution stands for. */
export function contributionLineLabel(line: ContributionLine): string {
  switch (line.kind) {
    case "base":
      return line.weekStartsOn && line.weekEndsOn
        ? `${formatDateRange(line.weekStartsOn, line.weekEndsOn)} · ${line.timeModelName}`
        : line.timeModelName;
    case "sibling":
      return `Geschwister: ${line.childRank ?? ""}. Kind, ${discountText(line.discountBasisPoints, line.discountCents)}`;
    case "benefits_exemption":
      return `Befreiung bei Leistungsbezug: ${line.note}`;
  }
}

/** "ab dem 2. Kind 50 % weniger". */
export function siblingRuleText(rule: ContributionRule): string {
  return `ab dem ${rule.siblingFromChild ?? 2}. Kind ${discountText(rule.discountBasisPoints, rule.discountCents)}`;
}

/** "ab 01.08.2026" or "01.08.2026 bis 31.07.2027". */
export function ruleValidity(rule: ContributionRule): string {
  return rule.validUntil
    ? formatDateRange(rule.validFrom, rule.validUntil)
    : `ab ${formatDate(rule.validFrom)}`;
}
