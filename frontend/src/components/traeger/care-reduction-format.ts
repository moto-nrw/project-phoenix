// Texte zu den Regeln des Monatsbeitrags (#4685): wie eine Regel wirkt und
// wie eine Ermäßigung auf der Rechnung erklärt wird. Die Reihenfolge der
// Kinder folgt dem Grundbeitrag, das teuerste Kind zählt zuerst (ADR 0115).

import type { CareReduction } from "~/lib/traeger-care-billing-api";
import type { CareRule, CareRuleKind } from "~/lib/traeger-care-reductions-api";
import { formatEuro } from "./contribution-format";

/** Name of a rule kind, as card title and invoice label. */
export const CARE_RULE_KIND_LABELS: Record<CareRuleKind, string> = {
  income_level: "Einkommensstufen",
  sibling: "Geschwister",
  single_parent: "Alleinerziehende",
  cap: "Höchstbetrag",
};

/** One sentence per kind: what the rule does. */
export const CARE_RULE_KIND_PURPOSE: Record<CareRuleKind, string> = {
  income_level:
    "Stufen laut Satzung. Ein Kind bekommt die Stufe über einen Nachweis.",
  sibling: "Ab dem 2. oder 3. Kind einer Familie zahlt die Familie weniger.",
  single_parent: "Weniger für Kinder mit Nachweis „alleinerziehend“.",
  cap: "So viel zahlt eine Familie höchstens im Monat.",
};

function percent(basisPoints: number): string {
  return `${(basisPoints / 100).toLocaleString("de-DE", { maximumFractionDigits: 2 })} %`;
}

/** What a rule does to the contribution, in a few words. */
export function ruleEffect(
  rule: Pick<
    CareRule,
    "kind" | "discountBasisPoints" | "discountCents" | "priceCents" | "capCents"
  > & { readonly tariffSiblingPrice?: boolean },
): string {
  if (rule.kind === "cap" && rule.capCents !== null) {
    return `höchstens ${formatEuro(rule.capCents)} je Familie`;
  }
  if (rule.tariffSiblingPrice) return "Geschwisterpreis laut Preisliste";
  if (rule.priceCents !== null) return `Preis ${formatEuro(rule.priceCents)}`;
  if (rule.discountBasisPoints !== null) {
    return `${percent(rule.discountBasisPoints)} weniger`;
  }
  if (rule.discountCents !== null) {
    return `${formatEuro(rule.discountCents)} weniger`;
  }
  return "–";
}

/** The line under a reduction on the invoice. */
export function reductionExplanation(reduction: CareReduction): string {
  switch (reduction.kind) {
    case "sibling":
      return `${reduction.childRank ?? "?"}. Kind von ${reduction.familyChildren ?? "?"} · ${ruleEffect(reduction)} · zuerst zählt das Kind mit dem höchsten Beitrag`;
    case "cap":
      return `Höchstbetrag ${formatEuro(reduction.capCents ?? 0)} je Familie und Monat erreicht`;
    case "income_level":
      return `Einkommensstufe · ${ruleEffect(reduction)}`;
    case "single_parent":
      return `Alleinerziehend · ${ruleEffect(reduction)}`;
  }
}

/** "ab Jan. 2027" or "Jan. 2027 bis Juli 2027". */
export function validityLabel(
  validFrom: string,
  validUntil: string | null,
  format: (iso: string) => string,
): string {
  return validUntil
    ? `${format(validFrom)} bis ${format(validUntil.slice(0, 8) + "01")}`
    : `ab ${format(validFrom)}`;
}
