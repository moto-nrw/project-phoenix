// Per-child rules of the parent enrollment form. Validation, payload, render
// and server-error mapping read a child's grade, offerings, care days and
// class rule from deriveChild, so a rule changes in one place.

import { availableCareOfferings } from "~/lib/care-offering-availability";
import { getSchoolYear } from "~/lib/student-helpers";
import {
  materializeCareSelection,
  type MaterializedCareSelection,
} from "~/lib/care-offering-selection";
import type { ConditionContext } from "~/lib/enrollment-field-visibility";
import type {
  PublicCareOffering,
  PublicSchoolClassConfig,
  SubmitChildPayload,
} from "~/lib/enrollment-submission-api";
import { WEEKDAYS, type FormField } from "./enrollment-form-shared";

export interface ChildDraft {
  // Client-only identity for React list keys, never sent. Keyed by index, a
  // removed middle slot would hand its local view state to the next child.
  clientId: string;
  id?: string;
  first_name: string;
  last_name: string;
  date_of_birth: string;
  target_grade_level: string;
  /** Concrete future class (#1833); "" = none chosen or "Klasse offen". */
  target_school_class: string;
  /** Holiday care wanted (#4703); undefined until the parent answers. */
  holiday_care_wish?: boolean;
  offering_ids: Set<string>;
  /** Picked days of parent_choice offerings, keyed by offering id. */
  offering_days: Record<string, Set<string>>;
  custom: Record<string, unknown>;
  /**
   * Taken over into care (ADR 0003): the card renders read-only and the
   * values travel back unchanged, because the backend refuses a change
   * request that touches this child.
   */
  locked?: boolean;
  /**
   * The bootstrap wire representation of a taken-over child. It bypasses
   * normal form serialization so catalog changes cannot turn an unchanged,
   * read-only child into a rejected change request.
   */
  lockedPayload?: SubmitChildPayload;
}

export interface ChildDerivationContext {
  /** The catalog as displayed (localized). */
  readonly offerings: readonly PublicCareOffering[];
  /** The loaded German catalog; offering conditions match its names. */
  readonly canonicalOfferings: readonly PublicCareOffering[];
  readonly collectGradeLevel: boolean;
  readonly careOfferingsEnabled: boolean;
  readonly schoolClass: PublicSchoolClassConfig;
  readonly previewMode: boolean;
  readonly guardianAnswers: Record<string, unknown>;
  readonly fieldsByKey: ConditionContext["fieldsByKey"];
}

export interface ChildDerivation {
  /** The grade the rules see: undefined while the form collects none. */
  readonly grade: string | undefined;
  /** Offerings the child's grade may book. */
  readonly offerings: PublicCareOffering[];
  readonly requiredOfferingIds: ReadonlySet<string>;
  /** The selection with automatic additions applied. */
  readonly care: MaterializedCareSelection;
  /** Weekdays day-scoped fields ask about. */
  readonly careDays: string[];
  /**
   * True when careDays come from booked offerings, not the every-weekday
   * fallback; only then does each shown day need an answer.
   */
  readonly careDaysConstrained: boolean;
  /** Selected offerings the parent chose (required ones never count). */
  readonly choosableSelectedCount: number;
  /** False when the grade leaves only required offerings to book. */
  readonly selectionModeApplies: boolean;
  /** Whether the concrete-class field is shown and sent for this grade. */
  readonly collectsSchoolClass: boolean;
  /** The phase's classes that match the child's grade. */
  readonly schoolClasses: string[];
  readonly conditionContext: ConditionContext;
}

export function deriveChild(
  child: ChildDraft,
  ctx: ChildDerivationContext,
): ChildDerivation {
  const grade = ctx.collectGradeLevel ? child.target_grade_level : undefined;
  const offerings = availableCareOfferings(ctx.offerings, grade);
  const requiredOfferingIds = new Set(
    offerings.filter((o) => o.is_required).map((o) => o.id),
  );
  const care = materializeChildCare(child, offerings, grade);
  // Without a catalog (or in a template preview) every weekday stays
  // relevant so day-scoped fields remain usable. With a catalog but no
  // booking the list is empty on purpose: the parent picks care days first.
  const careDaysConstrained = !ctx.previewMode && ctx.offerings.length > 0;
  const choosableSelectedCount = [...child.offering_ids].filter(
    (id) => offerings.some((o) => o.id === id) && !requiredOfferingIds.has(id),
  ).length;
  const collectsSchoolClass =
    ctx.collectGradeLevel &&
    collectsConcreteClassForGrade(ctx.schoolClass, child.target_grade_level);
  return {
    grade,
    offerings,
    requiredOfferingIds,
    care,
    careDays: careDaysConstrained
      ? bookedCareDays(care, offerings)
      : [...WEEKDAYS],
    careDaysConstrained,
    choosableSelectedCount,
    selectionModeApplies:
      ctx.offerings.length === 0 || offerings.some((o) => !o.is_required),
    collectsSchoolClass,
    schoolClasses: ctx.schoolClass.available_classes.filter((c) =>
      classMatchesGrade(c, child.target_grade_level),
    ),
    conditionContext: {
      guardianAnswers: ctx.guardianAnswers,
      childAnswers: child.custom,
      gradeLevel: grade,
      offeringNames: new Set(
        Array.from(ctx.careOfferingsEnabled ? care.offeringIds : [])
          .map((id) => ctx.canonicalOfferings.find((o) => o.id === id)?.name)
          .filter((name): name is string => Boolean(name))
          .map((name) => name.trim().toLowerCase()),
      ),
      fieldsByKey: ctx.fieldsByKey,
    },
  };
}

/**
 * Adds the mandatory offerings the child's grade can book. A conditional
 * required offering stays out until the grade that unlocks it is chosen.
 * Locked children keep their booking unchanged.
 */
export function withRequiredOfferings(
  child: ChildDraft,
  offerings: readonly PublicCareOffering[],
  collectGradeLevel: boolean,
): ChildDraft {
  if (child.locked) return child;
  const missing = availableCareOfferings(
    offerings,
    collectGradeLevel ? child.target_grade_level : undefined,
  ).filter(
    (offering) => offering.is_required && !child.offering_ids.has(offering.id),
  );
  if (missing.length === 0) return child;
  const nextIDs = new Set(child.offering_ids);
  missing.forEach((offering) => nextIDs.add(offering.id));
  return { ...child, offering_ids: nextIDs };
}

/**
 * Heimweg-Beschränkung (#2381): whether the field allows one way home per
 * day for this grade. A missing or unparseable grade is never restricted,
 * like the backend's SingleModeAppliesTo.
 */
export function singleModeApplies(
  field: FormField,
  grade: string | undefined,
): boolean {
  const value = Number(grade);
  if (!Number.isInteger(value)) return false;
  return (field.single_mode_grades ?? []).includes(value);
}

/**
 * The grades the grade select offers: every grade up to the tenant cap,
 * narrowed by the phase restriction (#1663). A prefilled grade must be in
 * this list, or the form submits a value the parent cannot see. A
 * restriction naming only grades above the cap is broken config; the full
 * range shows and the server rejects with the authoritative error.
 */
export function selectableGradeLevels(
  max: number,
  allowedGrades: readonly number[],
): number[] {
  const everyGrade = Array.from({ length: max }, (_, n) => n + 1);
  const eligible = everyGrade.filter((grade) => allowedGrades.includes(grade));
  return allowedGrades.length > 0 && eligible.length > 0
    ? eligible
    : everyGrade;
}

/**
 * Whether a class may be offered for a grade. The phase list mixes all
 * grades and the backend rejects a mismatch; classes without a numeric
 * prefix fit every grade (#1833).
 */
export function classMatchesGrade(
  schoolClass: string,
  gradeLevel: string,
): boolean {
  const grade = getSchoolYear(schoolClass);
  return grade === null || grade === gradeLevel.trim();
}

// Grade 2 and up collect a class whenever the phase offers a matching one
// (#1833). Grade 1 is opt-in (#1663): the backend's `collect_grade_1` decides,
// because a phase with only a prefixless class ("Bienen") collects grade 1
// too. Without the field a grade-1 class prefix decides, as before #1663.
function collectsConcreteClassForGrade(
  config: PublicSchoolClassConfig,
  gradeLevel: string,
): boolean {
  if (!config.collect) return false;
  const grade = Number(gradeLevel);
  if (!Number.isFinite(grade) || grade < 1) return false;
  if (grade >= 2) {
    return config.available_classes.some((cls) =>
      classMatchesGrade(cls, gradeLevel),
    );
  }
  if (config.collect_grade_1 === true) return true;
  return config.available_classes.some((cls) => getSchoolYear(cls) === "1");
}

function materializeChildCare(
  child: ChildDraft,
  offerings: readonly PublicCareOffering[],
  gradeLevel: string | undefined,
): MaterializedCareSelection {
  const grade = Number(gradeLevel);
  return materializeCareSelection(
    offerings,
    { offeringIds: child.offering_ids, offeringDays: child.offering_days },
    Number.isFinite(grade) && grade > 0 ? grade : undefined,
  );
}

function bookedCareDays(
  care: MaterializedCareSelection,
  offerings: readonly PublicCareOffering[],
): string[] {
  const days = new Set<string>();
  for (const id of care.offeringIds) {
    const offering = offerings.find((o) => o.id === id);
    if (!offering) continue;
    const selected =
      offering.days_of_week_mode === "parent_choice"
        ? Array.from(care.offeringDays[id] ?? new Set<string>())
        : offering.available_days;
    for (const day of selected) days.add(day);
  }
  return WEEKDAYS.filter((day) => days.has(day));
}
