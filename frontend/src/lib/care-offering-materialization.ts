/**
 * Client-side mirror of the backend care-offering materializer
 * (backend/services/enrollment/request_service.go, materializeOfferingSelections).
 * It answers, for the CURRENT form selection, which offerings a
 * Mitbuchungs-Regel or the required-lunch derivation would book
 * automatically, and on which days (#2366).
 *
 * Semantics kept in lockstep with the backend:
 * - a target offering with auto-add triggers receives the union of its
 *   selected triggers' days (fixed offerings contribute their available
 *   days), intersected with the target's own available days
 * - a required offering with lunch and parent-choice days receives the
 *   days of every other selected offering that counts as care
 * - the loop runs to a fixed point, so chained rules cascade
 * - a grade condition on the target must match the child's grade
 */

export interface MaterializableOffering {
  id: string;
  days_of_week_mode: string;
  available_days: string[];
  is_required: boolean;
  includes_lunch: boolean;
  counts_as_care?: boolean;
  auto_add_grade_levels?: number[];
  auto_add_trigger_offering_ids?: string[];
  /**
   * Server-evaluated grade gate. When present it overrides the client-side
   * check of auto_add_grade_levels — needed by the parent modal, whose
   * catalog carries no grade data (#2366).
   */
  auto_add_applies?: boolean;
}

export interface CareSelectionInput {
  /** Raw grade form value; irrelevant when no offering carries a grade condition. */
  gradeLevel: string;
  offeringIds: Iterable<string>;
  offeringDays: Readonly<Record<string, Iterable<string>>>;
}

export interface MaterializedCareSelection {
  offeringIds: Set<string>;
  offeringDays: Record<string, Set<string>>;
  automaticDays: Record<string, Set<string>>;
  /**
   * Per auto-add target: the selected trigger offerings that contributed at
   * least one day. Empty set = the days come only from the lunch derivation.
   */
  autoAddContributors: Record<string, Set<string>>;
}

export function materializeCareSelection(
  input: CareSelectionInput,
  offerings: readonly MaterializableOffering[],
): MaterializedCareSelection {
  const offeringIds = new Set(input.offeringIds);
  const offeringDays: Record<string, Set<string>> = {};
  const automaticDays: Record<string, Set<string>> = {};
  const autoAddContributors: Record<string, Set<string>> = {};
  for (const [id, days] of Object.entries(input.offeringDays)) {
    offeringDays[id] = new Set(days);
  }

  let changed = true;
  while (changed) {
    changed = false;
    for (const target of offerings) {
      const applies =
        target.auto_add_applies ??
        autoAddAppliesToGrade(input.gradeLevel, target);
      if (!applies) continue;
      const triggerIDs = target.auto_add_trigger_offering_ids ?? [];
      if (triggerIDs.length === 0 && !isRequiredLunchOffering(target)) continue;

      const auto = new Set<string>();
      const contributors = new Set<string>();
      const targetDays = new Set(target.available_days);
      for (const triggerID of triggerIDs) {
        if (!offeringIds.has(triggerID)) continue;
        const trigger = offerings.find((offering) => offering.id === triggerID);
        if (!trigger) continue;
        const triggerDays =
          trigger.days_of_week_mode === "parent_choice"
            ? Array.from(offeringDays[triggerID] ?? new Set<string>())
            : trigger.available_days;
        for (const day of triggerDays) {
          if (targetDays.has(day)) {
            auto.add(day);
            contributors.add(triggerID);
          }
        }
      }
      if (isRequiredLunchOffering(target)) {
        for (const source of offerings) {
          if (source.id === target.id || !(source.counts_as_care ?? true)) {
            continue;
          }
          if (!offeringIds.has(source.id)) {
            continue;
          }
          const sourceDays =
            source.days_of_week_mode === "parent_choice"
              ? Array.from(offeringDays[source.id] ?? new Set<string>())
              : source.available_days;
          for (const day of sourceDays) {
            if (targetDays.has(day)) auto.add(day);
          }
        }
      }
      if (auto.size === 0) continue;

      if (!offeringIds.has(target.id)) {
        offeringIds.add(target.id);
        changed = true;
      }
      if (!sameSet(automaticDays[target.id], auto)) {
        automaticDays[target.id] = auto;
        changed = true;
      }
      autoAddContributors[target.id] = contributors;
      const merged = new Set(offeringDays[target.id] ?? []);
      const beforeSize = merged.size;
      for (const day of auto) merged.add(day);
      if (
        merged.size !== beforeSize ||
        !sameSet(offeringDays[target.id], merged)
      ) {
        offeringDays[target.id] = merged;
        changed = true;
      }
    }
  }

  return { offeringIds, offeringDays, automaticDays, autoAddContributors };
}

function sameSet(left: Set<string> | undefined, right: Set<string>): boolean {
  if (!left || left.size !== right.size) return false;
  for (const value of left) {
    if (!right.has(value)) return false;
  }
  return true;
}

function isRequiredLunchOffering(offering: MaterializableOffering): boolean {
  return (
    offering.is_required &&
    offering.includes_lunch &&
    offering.days_of_week_mode === "parent_choice"
  );
}

function autoAddAppliesToGrade(
  gradeValue: string,
  offering: MaterializableOffering,
): boolean {
  const gradeLevels = offering.auto_add_grade_levels ?? [];
  if (gradeLevels.length === 0) return true;
  const grade = Number(gradeValue);
  if (!Number.isFinite(grade) || grade <= 0) return false;
  return gradeLevels.includes(grade);
}
