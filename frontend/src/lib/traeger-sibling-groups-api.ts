/**
 * Siblings from separate holiday registrations (#5628): the Träger-Büro
 * merges registrations of one period as siblings, and splits them again.
 * The Elternabrechnung ranks the children of a group together. Kept apart
 * from the contribution client, so the registration page does not load it.
 */

import { traegerRequest } from "./traeger-api";

export interface HolidaySiblingGroup {
  readonly id: string;
  readonly registrationIds: readonly string[];
}

interface BackendGroup {
  id: number;
  registration_ids?: number[] | null;
}

const groupsPath = (periodId: string) =>
  `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/sibling-groups`;

function groupFromBackend(group: BackendGroup): HolidaySiblingGroup {
  return {
    id: String(group.id),
    registrationIds: (group.registration_ids ?? []).map(String),
  };
}

/** The sibling groups of the period, by registration number. */
export async function fetchHolidaySiblingGroups(
  periodId: string,
): Promise<HolidaySiblingGroup[]> {
  const groups = await traegerRequest<BackendGroup[] | null>(
    groupsPath(periodId),
  );
  return (groups ?? []).map(groupFromBackend);
}

/**
 * Merges the registrations, with the groups they already belong to, into
 * one sibling group.
 */
export async function mergeHolidaySiblings(
  periodId: string,
  registrationIds: readonly string[],
): Promise<HolidaySiblingGroup> {
  return groupFromBackend(
    await traegerRequest<BackendGroup>(groupsPath(periodId), {
      method: "POST",
      body: { registration_ids: registrationIds.map(Number) },
    }),
  );
}

/** Takes the registration out of its sibling group. */
export async function splitHolidaySibling(
  periodId: string,
  registrationId: string,
): Promise<void> {
  await traegerRequest<unknown>(
    `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/registrations/${encodeURIComponent(registrationId)}/sibling-group`,
    { method: "DELETE" },
  );
}
