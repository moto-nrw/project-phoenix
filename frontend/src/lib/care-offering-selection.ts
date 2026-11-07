/**
 * Automatic additions of a child's care-offering selection: trigger offerings
 * add days to their targets, and a required lunch offering follows the care
 * days of every other selected care offering. The backend applies the same
 * rules on every write (`backend/modules/careplan/selection/materialize.go`);
 * this copy only previews them while a form is being filled in.
 */

export interface SelectableOffering {
  readonly id: string;
  readonly days_of_week_mode: "fixed" | "parent_choice";
  readonly available_days: readonly string[];
  readonly is_required: boolean;
  readonly includes_lunch: boolean;
  readonly counts_as_care?: boolean;
  readonly auto_add_grade_levels?: readonly number[];
  readonly auto_add_trigger_offering_ids?: readonly string[];
}

export interface CareSelection {
  readonly offeringIds: ReadonlySet<string>;
  /** Picked days of parent_choice offerings. */
  readonly offeringDays: Readonly<Record<string, Iterable<string>>>;
}

export interface MaterializedCareSelection {
  readonly offeringIds: Set<string>;
  readonly offeringDays: Record<string, Set<string>>;
  /** Days the rules added, per target offering. */
  readonly automaticDays: Record<string, Set<string>>;
}

function isRequiredLunchOffering(offering: SelectableOffering): boolean {
  return (
    offering.is_required &&
    offering.includes_lunch &&
    offering.days_of_week_mode === "parent_choice"
  );
}

function canAutoAddDays(offering: SelectableOffering): boolean {
  return (
    (offering.auto_add_trigger_offering_ids?.length ?? 0) > 0 ||
    isRequiredLunchOffering(offering)
  );
}

function autoAddAppliesToGrade(
  offering: SelectableOffering,
  gradeLevel: number | undefined,
): boolean {
  const levels = offering.auto_add_grade_levels ?? [];
  if (levels.length === 0) return true;
  return gradeLevel !== undefined && levels.includes(gradeLevel);
}

function sameSet(left: Set<string> | undefined, right: Set<string>): boolean {
  if (left?.size !== right.size) return false;
  for (const value of left) {
    if (!right.has(value)) return false;
  }
  return true;
}

/**
 * Applies the automatic additions until the selection is stable, because one
 * addition can trigger another. `gradeLevel` is the child's effective grade;
 * undefined skips every target restricted to grades.
 */
export function materializeCareSelection(
  catalog: readonly SelectableOffering[],
  selection: CareSelection,
  gradeLevel: number | undefined,
): MaterializedCareSelection {
  const byID = new Map(catalog.map((offering) => [offering.id, offering]));
  const offeringIds = new Set(selection.offeringIds);
  const offeringDays: Record<string, Set<string>> = {};
  const automaticDays: Record<string, Set<string>> = {};
  for (const [id, days] of Object.entries(selection.offeringDays)) {
    offeringDays[id] = new Set(days);
  }
  const daysOf = (offering: SelectableOffering): Iterable<string> =>
    offering.days_of_week_mode === "parent_choice"
      ? (offeringDays[offering.id] ?? [])
      : offering.available_days;

  const targets = catalog.filter(
    (target) =>
      target.days_of_week_mode === "parent_choice" &&
      canAutoAddDays(target) &&
      autoAddAppliesToGrade(target, gradeLevel),
  );
  let changed = true;
  while (changed) {
    changed = false;
    for (const target of targets) {
      const targetDays = new Set(target.available_days);
      const auto = new Set<string>();
      const sources: SelectableOffering[] = [];
      for (const triggerID of target.auto_add_trigger_offering_ids ?? []) {
        const trigger = byID.get(triggerID);
        if (trigger && offeringIds.has(triggerID)) sources.push(trigger);
      }
      if (isRequiredLunchOffering(target)) {
        for (const source of catalog) {
          if (
            source.id !== target.id &&
            (source.counts_as_care ?? true) &&
            offeringIds.has(source.id)
          ) {
            sources.push(source);
          }
        }
      }
      for (const source of sources) {
        for (const day of daysOf(source)) {
          if (targetDays.has(day)) auto.add(day);
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
      const merged = new Set([...(offeringDays[target.id] ?? []), ...auto]);
      if (!sameSet(offeringDays[target.id], merged)) {
        offeringDays[target.id] = merged;
        changed = true;
      }
    }
  }
  return { offeringIds, offeringDays, automaticDays };
}
