/**
 * Hints at siblings in separate holiday registrations (#5628): the same
 * e-mail address of the parent (ignoring case) or the same last name. A
 * hint changes nothing; only the Träger-Büro merges. Registrations already
 * in one group and cancelled ones get no hint about each other.
 */

import type { HolidaySiblingGroup } from "./traeger-sibling-groups-api";

export type SiblingHintReason = "email" | "last_name";

export interface SiblingHint {
  readonly reason: SiblingHintReason;
  /** The other registrations of the hint, by number. */
  readonly registrationIds: readonly string[];
}

interface HintedRegistration {
  readonly id: string;
  readonly status: string;
  readonly guardianEmail: string;
  readonly guardianLastName: string;
}

export const SIBLING_HINT_LABELS: Readonly<Record<SiblingHintReason, string>> =
  {
    email: "Vielleicht Geschwister: gleiche E-Mail",
    last_name: "Vielleicht Geschwister: gleicher Nachname",
  };

/** The group of every grouped registration, by registration. */
export function siblingGroupOf(
  groups: readonly HolidaySiblingGroup[],
): ReadonlyMap<string, HolidaySiblingGroup> {
  const byRegistration = new Map<string, HolidaySiblingGroup>();
  for (const group of groups) {
    for (const id of group.registrationIds) byRegistration.set(id, group);
  }
  return byRegistration;
}

function byKey(
  registrations: readonly HintedRegistration[],
  key: (registration: HintedRegistration) => string,
): Map<string, string[]> {
  const ids = new Map<string, string[]>();
  for (const registration of registrations) {
    const value = key(registration).trim().toLocaleLowerCase("de-DE");
    if (!value) continue;
    ids.set(value, [...(ids.get(value) ?? []), registration.id]);
  }
  return ids;
}

/**
 * The hint of every registration that may have siblings in another
 * registration of the list; the same e-mail address wins over the same
 * last name.
 */
export function siblingHints(
  registrations: readonly HintedRegistration[],
  groups: readonly HolidaySiblingGroup[],
): ReadonlyMap<string, SiblingHint> {
  const open = registrations.filter(
    (registration) => registration.status !== "cancelled",
  );
  const groupOf = siblingGroupOf(groups);
  const hints = new Map<string, SiblingHint>();
  const keys: [SiblingHintReason, Map<string, string[]>][] = [
    ["email", byKey(open, (registration) => registration.guardianEmail)],
    ["last_name", byKey(open, (registration) => registration.guardianLastName)],
  ];
  for (const [reason, ids] of keys) {
    for (const same of ids.values()) {
      for (const id of same) {
        if (hints.has(id)) continue;
        const group = groupOf.get(id);
        const others = same.filter(
          (other) => other !== id && !group?.registrationIds.includes(other),
        );
        if (others.length > 0) {
          hints.set(id, { reason, registrationIds: others });
        }
      }
    }
  }
  return hints;
}
