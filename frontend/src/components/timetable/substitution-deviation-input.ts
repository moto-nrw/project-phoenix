import type {
  ApplyDeviationsInput,
  EnrichedInstance,
} from "~/lib/timetable-types";

export interface PersonForm {
  absent: boolean;
  wasAbsent: boolean;
  existingAbsentIds: string[];
  sickAbsenceIds: string[];
  sickLocked: boolean;
  scope: "" | "all" | "selected";
  selectedInstanceIds: string[];
  allDayAbsence: boolean;
  reason: string;
  substituteId: string;
  showReason: boolean;
}

export function isEditableAppointment(appointment: EnrichedInstance): boolean {
  return appointment.status === "planned" || appointment.status === "active";
}

type Absence = NonNullable<ApplyDeviationsInput["absences"]>[number];
type Presence = NonNullable<ApplyDeviationsInput["presences"]>[number];
type Substitution = NonNullable<ApplyDeviationsInput["substitutions"]>[number];

interface PersonChanges {
  absence?: Absence;
  presence?: Presence;
  substitution?: Substitution;
}

export function sameIds(
  left: readonly string[],
  right: readonly string[],
): boolean {
  if (left.length !== right.length) return false;
  const rightSet = new Set(right);
  return left.every((id) => rightSet.has(id));
}

export function desiredAbsenceIds(
  person: PersonForm,
  openIds: string[],
): string[] {
  let desired = person.existingAbsentIds;
  if (person.sickLocked && person.scope !== "selected") {
    desired = person.existingAbsentIds;
  } else if (!person.absent)
    desired = person.wasAbsent ? [] : person.existingAbsentIds;
  else if (person.scope === "all" || person.allDayAbsence) desired = openIds;
  else if (person.scope === "selected") desired = person.selectedInstanceIds;
  return [...new Set([...desired, ...person.sickAbsenceIds])];
}

export function coverageIds(person: PersonForm, openIds: string[]): string[] {
  if (person.scope === "all") {
    return person.sickLocked ? person.existingAbsentIds : openIds;
  }
  return person.scope === "selected" ? person.selectedInstanceIds : [];
}

function withoutIds(left: readonly string[], right: readonly string[]) {
  const rightSet = new Set(right);
  return left.filter((id) => !rightSet.has(id));
}

function substitutionChanges(
  staffId: string,
  person: PersonForm,
  presence: Presence | undefined,
  targets: string[],
): PersonChanges {
  const reason = person.reason.trim() || undefined;
  return {
    presence,
    substitution: {
      absentStaffId: staffId,
      substituteStaffId: person.substituteId,
      reason,
      instanceIds:
        person.scope === "all" && !person.sickLocked ? undefined : targets,
    },
    absence:
      !person.sickLocked && person.allDayAbsence
        ? { staffId, reason }
        : undefined,
  };
}

function changesForPerson(
  staffId: string,
  person: PersonForm,
  desired: string[],
  targets: string[],
): PersonChanges {
  const restored = withoutIds(person.existingAbsentIds, desired).filter(
    (instanceID) => !person.sickAbsenceIds.includes(instanceID),
  );
  const presence =
    restored.length > 0 ? { staffId, instanceIds: restored } : undefined;
  if (person.substituteId) {
    return substitutionChanges(staffId, person, presence, targets);
  }
  const newlyAbsent = withoutIds(desired, person.existingAbsentIds);
  const reason = person.reason.trim() || undefined;
  const absence =
    !person.sickLocked && newlyAbsent.length > 0
      ? {
          staffId,
          reason,
          instanceIds:
            person.scope === "all" || person.allDayAbsence
              ? undefined
              : newlyAbsent,
        }
      : undefined;
  return { absence, presence };
}

interface BuildStaffInputOptions {
  instanceId: string;
  people: Record<string, PersonForm>;
  removedSubs: ReadonlySet<string>;
  restoredSubs: ReadonlySet<string>;
  desiredIds: (staffId: string, person: PersonForm) => string[];
  targetIds: (staffId: string, person: PersonForm) => string[];
}

export function buildStaffDeviationInput({
  instanceId,
  people,
  removedSubs,
  restoredSubs,
  desiredIds,
  targetIds,
}: BuildStaffInputOptions): ApplyDeviationsInput {
  const absences: Absence[] = [];
  const presences: Presence[] = [...restoredSubs].map((staffId) => ({
    staffId,
    instanceIds: [instanceId],
  }));
  const substitutions: Substitution[] = [];
  for (const [staffId, person] of Object.entries(people)) {
    const change = changesForPerson(
      staffId,
      person,
      desiredIds(staffId, person),
      targetIds(staffId, person),
    );
    if (change.absence) absences.push(change.absence);
    if (change.presence) presences.push(change.presence);
    if (change.substitution) substitutions.push(change.substitution);
  }
  const input: ApplyDeviationsInput = {};
  if (absences.length > 0) input.absences = absences;
  if (presences.length > 0) input.presences = presences;
  if (substitutions.length > 0) input.substitutions = substitutions;
  if (removedSubs.size > 0) {
    input.substitutionRemovals = [...removedSubs].map((staffId) => ({
      staffId,
      instanceIds: [instanceId],
    }));
  }
  return input;
}
