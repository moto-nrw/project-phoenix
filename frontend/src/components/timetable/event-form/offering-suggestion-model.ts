import type { DraftRoster } from "~/lib/offering-suggestion";
import { schoolClassLabel } from "~/lib/school-class-label";

import { rosterForWeekday, type EventFormState } from "./form-model";

const COHORT_TARGETS = new Set(["klasse", "jahrgang", "gruppe"]);

/**
 * The children the editor plans per weekday: the current members of the
 * Klasse, Jahrgang or Gruppe on every weekday, plus the children picked by
 * hand (the shared list, or each weekday's own list).
 */
export function buildDraftRoster(
  form: EventFormState,
  cohortMemberIds: readonly string[],
): DraftRoster {
  const cohort = COHORT_TARGETS.has(form.targetGroupType)
    ? cohortMemberIds
    : [];
  const roster: DraftRoster = {};
  for (const weekday of [...form.weekdays].sort((a, b) => a - b)) {
    const picked = rosterForWeekday(form, weekday).studentIds;
    roster[weekday] = [...new Set([...cohort, ...picked])].sort((a, b) =>
      a.localeCompare(b, "de", { numeric: true }),
    );
  }
  return roster;
}

/** Number of different children in the roster. */
export function draftRosterSize(roster: DraftRoster): number {
  return new Set(Object.values(roster).flat()).size;
}

/** The Klassen and Jahrgänge of the Zielgruppe, which narrow the bookings. */
export function draftTargetFilter(form: EventFormState): {
  schoolClasses: string[];
  gradeLevels: number[];
} {
  if (form.targetGroupType === "klasse") {
    return { schoolClasses: form.targetSchoolClasses, gradeLevels: [] };
  }
  if (form.targetGroupType === "jahrgang") {
    return {
      schoolClasses: [],
      gradeLevels: form.targetGradeLevels
        .map(Number)
        .filter((grade) => Number.isFinite(grade)),
    };
  }
  return { schoolClasses: [], gradeLevels: [] };
}

const NO_BOOKINGS_SENTENCE =
  "unabhängig von Anmeldungen. Spätere An- und Abmeldungen werden nicht übernommen.";

/**
 * What a Klasse, Jahrgang or Gruppe target really means (#5104): every child
 * of it, whether booked or not, and later bookings never arrive. Null for
 * every other Zielgruppe.
 */
export function targetAudienceText(
  form: EventFormState,
  groups: ReadonlyArray<{ id: string; name: string }>,
): string | null {
  let audience: string | null = null;
  if (form.targetGroupType === "klasse") {
    const classes = form.targetSchoolClasses;
    audience =
      classes.length === 1
        ? `der ${schoolClassLabel(classes[0] ?? "")}`
        : "der gewählten Klassen";
  } else if (form.targetGroupType === "jahrgang") {
    const grades = form.targetGradeLevels;
    audience =
      grades.length === 1
        ? `der Jahrgangsstufe ${grades[0]}`
        : "der gewählten Jahrgangsstufen";
  } else if (form.targetGroupType === "gruppe") {
    const ids = form.educationGroupIds;
    const name =
      ids.length === 1
        ? groups.find((group) => group.id === ids[0])?.name
        : undefined;
    audience = name ? `der Gruppe ${name}` : "der gewählten Gruppen";
  }
  return audience
    ? `Es kommen alle Kinder ${audience}, ${NO_BOOKINGS_SENTENCE}`
    : null;
}
