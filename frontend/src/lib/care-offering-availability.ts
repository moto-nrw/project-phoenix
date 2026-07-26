import type { CareOfferingAvailabilityRule } from "~/lib/care-offering-api";

export interface AvailabilityAwareOffering {
  availability_rule?: CareOfferingAvailabilityRule | null;
}

export function careOfferingIsAvailable(
  offering: AvailabilityAwareOffering,
  gradeLevel: string | number | null | undefined,
): boolean {
  const rule = offering.availability_rule;
  if (!rule || rule.conditions.length === 0) return true;
  const grade =
    typeof gradeLevel === "number" ? gradeLevel : Number(gradeLevel || 0);
  if (!Number.isInteger(grade) || grade < 1) return false;

  const matches = rule.conditions.map((condition) => {
    if (condition.source !== "grade_level") return false;
    const included = condition.value.includes(grade);
    return condition.operator === "in" ? included : !included;
  });
  return rule.match === "all" ? matches.every(Boolean) : matches.some(Boolean);
}

export function availableCareOfferings<T extends AvailabilityAwareOffering>(
  offerings: readonly T[],
  gradeLevel: string | number | null | undefined,
): T[] {
  return offerings.filter((offering) =>
    careOfferingIsAvailable(offering, gradeLevel),
  );
}

export function careOfferingAvailabilityRuleError(
  rule: CareOfferingAvailabilityRule | null | undefined,
  gradeLevelMax: number | null,
): string | null {
  if (!rule) return null;
  if (rule.conditions.length === 0)
    return "Füge mindestens eine Bedingung hinzu.";
  if (rule.match !== "all" && rule.match !== "any") {
    return "Wähle eine gültige Verknüpfung.";
  }
  for (const [index, condition] of rule.conditions.entries()) {
    if (condition.source !== "grade_level") {
      return `Bedingung ${index + 1}: unbekannte Quelle.`;
    }
    if (condition.operator !== "in" && condition.operator !== "not_in") {
      return `Bedingung ${index + 1}: unbekannter Operator.`;
    }
    if (condition.value.length === 0) {
      return `Bedingung ${index + 1}: Wähle mindestens eine Klassenstufe.`;
    }
    if (
      gradeLevelMax !== null &&
      condition.value.some(
        (grade) =>
          !Number.isInteger(grade) || grade < 1 || grade > gradeLevelMax,
      )
    ) {
      return `Bedingung ${index + 1}: Eine Klassenstufe liegt außerhalb des gültigen Bereichs.`;
    }
  }
  return null;
}
