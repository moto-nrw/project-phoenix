// Texte zu den Kostenträger-Anteilen am Monatsbeitrag (#4687): Art des
// Anteils, wie viel er übernimmt und wo er gerade steht.

import type { CareCostShareKind } from "~/lib/traeger-care-billing-api";
import { formatEuro } from "./contribution-format";

/** Art des Anteils, in der Reihenfolge, in der der Monatslauf rechnet. */
export const COST_SHARE_KINDS: readonly CareCostShareKind[] = [
  "education_participation",
  "youth_office",
  "other_subsidy",
];

export const COST_SHARE_KIND_LABELS: Record<CareCostShareKind, string> = {
  education_participation: "Bildung und Teilhabe",
  youth_office: "Jugendamt",
  other_subsidy: "Sonstiger Zuschuss",
};

/** Kurzform für enge Spalten. */
export const COST_SHARE_KIND_SHORT: Record<CareCostShareKind, string> = {
  education_participation: "BuT",
  youth_office: "Jugendamt",
  other_subsidy: "Zuschuss",
};

/** "100 %" oder "45,00 € im Monat". */
export function costShareAmountText(
  share: Readonly<{
    shareBasisPoints: number | null;
    shareCents: number | null;
  }>,
): string {
  if (share.shareBasisPoints !== null) {
    return `${(share.shareBasisPoints / 100).toLocaleString("de-DE", { maximumFractionDigits: 2 })} %`;
  }
  if (share.shareCents !== null) {
    return `${formatEuro(share.shareCents)} im Monat`;
  }
  return "";
}
