// Texte zu den Kostenträger-Anteilen am Monatsbeitrag (#4687): Art des
// Anteils, wie viel er übernimmt und wo er gerade steht.

import type {
  CareCostShareKind,
  CareCostShareScope,
} from "~/lib/traeger-care-billing-api";
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

/** Wofür der Anteil gilt (#4686): Betreuung, Mensa oder beides. */
export const COST_SHARE_SCOPE_LABELS: Record<CareCostShareScope, string> = {
  care: "Betreuung",
  meal: "Mensa",
  both: "Betreuung und Mensa",
};

/** Ein Anteil für die Betreuung sagt nichts dazu, wie vor der Mensa. */
const COST_SHARE_SCOPE_SUFFIX: Record<CareCostShareScope, string> = {
  care: "",
  meal: " für die Mensa",
  both: " für Betreuung und Mensa",
};

/** "100 %", "45,00 € im Monat" oder "100 % für die Mensa". */
export function costShareAmountText(
  share: Readonly<{
    shareBasisPoints: number | null;
    shareCents: number | null;
    scope?: CareCostShareScope;
  }>,
): string {
  const suffix = COST_SHARE_SCOPE_SUFFIX[share.scope ?? "care"];
  if (share.shareBasisPoints !== null) {
    return `${(share.shareBasisPoints / 100).toLocaleString("de-DE", { maximumFractionDigits: 2 })} %${suffix}`;
  }
  if (share.shareCents !== null) {
    return `${formatEuro(share.shareCents)} im Monat${suffix}`;
  }
  return "";
}
