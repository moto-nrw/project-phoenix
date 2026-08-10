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

/**
 * Formats a sorted, de-duplicated grade list the way an admin reads it:
 * contiguous runs collapse to a range ("1–3"), gaps stay explicit
 * ("1–2 und 5"). Returns "" for an empty list so callers can treat a rule
 * with no grades as undescribable.
 */
export function formatGradeLevelList(values: readonly number[]): string {
  const grades = [...new Set(values)]
    .filter((grade) => Number.isInteger(grade))
    .sort((a, b) => a - b);
  if (grades.length === 0) return "";

  const runs: string[] = [];
  let start = grades[0]!;
  let previous = start;
  for (const grade of grades.slice(1)) {
    if (grade === previous + 1) {
      previous = grade;
      continue;
    }
    runs.push(start === previous ? `${start}` : `${start}–${previous}`);
    start = grade;
    previous = grade;
  }
  runs.push(start === previous ? `${start}` : `${start}–${previous}`);

  if (runs.length === 1) return runs[0]!;
  return `${runs.slice(0, -1).join(", ")} und ${runs.at(-1)}`;
}

function gradeNoun(values: readonly number[]): string {
  return new Set(values).size === 1 ? "Klasse" : "Klassen";
}

/**
 * Turns an availability rule into the short German phrase the catalog shows
 * as a pill and the admin pickers show as a reason. Returns null when the
 * rule imposes no restriction, so callers render nothing.
 *
 * The wording deliberately mirrors the rule editor's own vocabulary ("ist
 * eine von" / "ist keine von") so an admin can connect the pill back to the
 * setting that produced it.
 */
export function describeCareOfferingAvailabilityRule(
  rule: CareOfferingAvailabilityRule | null | undefined,
): string | null {
  if (!rule || rule.conditions.length === 0) return null;

  const phrases: string[] = [];
  for (const condition of rule.conditions) {
    if (condition.source !== "grade_level") continue;
    const list = formatGradeLevelList(condition.value);
    if (!list) continue;
    const noun = gradeNoun(condition.value);
    phrases.push(
      condition.operator === "not_in"
        ? `nicht ${noun} ${list}`
        : `${noun} ${list}`,
    );
  }
  if (phrases.length === 0) return null;

  if (phrases.length === 1) {
    const only = phrases[0]!;
    return only.startsWith("nicht ")
      ? `Nicht für ${only.slice("nicht ".length)}`
      : `Nur für ${only}`;
  }
  const joiner = rule.match === "any" ? " oder " : " und ";
  return `Nur für ${phrases.join(joiner)}`;
}

/**
 * Explains, in the admin's words, why this offering cannot be picked for a
 * child of this grade — or null when it can. Parent-facing forms keep hiding
 * blocked offerings; admins need the reason instead of a gap in the list
 * (#2186).
 */
export function careOfferingAvailabilityReason(
  offering: AvailabilityAwareOffering & { name?: string },
  gradeLevel: string | number | null | undefined,
): string | null {
  if (careOfferingIsAvailable(offering, gradeLevel)) return null;

  const description = describeCareOfferingAvailabilityRule(
    offering.availability_rule,
  );
  const grade =
    typeof gradeLevel === "number" ? gradeLevel : Number(gradeLevel || 0);
  const childPart =
    Number.isInteger(grade) && grade >= 1
      ? `Kind: Klasse ${grade}`
      : "Klassenstufe des Kindes fehlt";

  if (!description) {
    return `Nicht wählbar (${childPart})`;
  }
  return `Nicht wählbar: ${lowercaseFirst(description)} (${childPart})`;
}

function lowercaseFirst(value: string): string {
  return value.charAt(0).toLowerCase() + value.slice(1);
}

/** Grade distribution of an offering's current bookings (backend aggregate). */
export interface CareOfferingBookingGradeCounts {
  /** Keyed by grade level as a string, because JSON object keys are strings. */
  grade_levels: Record<string, number>;
  /** Bookings whose child has no target grade level recorded. */
  unknown_grade_count: number;
}

/**
 * Counts the existing bookings a rule would contradict. Nothing is blocked by
 * this — existing bookings keep Bestandsschutz — but an admin tightening a
 * rule should see that the catalog and the booked reality now disagree
 * (#2186).
 *
 * A booking whose grade is unknown counts as contradicting any rule: the
 * backend evaluator (`CareOfferingAvailabilityRule.MatchesGradeLevel`) never
 * matches a missing grade, not even for a negative condition.
 */
export function countCareOfferingRuleConflicts(
  rule: CareOfferingAvailabilityRule | null | undefined,
  counts: CareOfferingBookingGradeCounts | null | undefined,
): number {
  if (!counts) return 0;
  if (!rule || rule.conditions.length === 0) return 0;

  let conflicts = counts.unknown_grade_count;
  for (const [grade, count] of Object.entries(counts.grade_levels)) {
    if (!careOfferingIsAvailable({ availability_rule: rule }, grade)) {
      conflicts += count;
    }
  }
  return conflicts;
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
