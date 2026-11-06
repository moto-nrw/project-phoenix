/**
 * Rows and sums of the occupancy page (#4676). The backend counts per school;
 * this module only lays the counts out for the three views and adds them up.
 */

import { foldForSearch } from "~/lib/search-fold";
import type {
  TraegerOccupancyCareForm,
  TraegerSchoolOccupancy,
} from "~/lib/traeger-occupancy-api";

export interface OccupancyTotals {
  readonly children: number;
  readonly inGroup: number;
  readonly withCareForm: number;
}

/** One OGS group of a school, or its children without a group. */
export interface OccupancyGroupRow {
  readonly key: string;
  readonly school: TraegerSchoolOccupancy;
  /** `null` for the children of the school without a group. */
  readonly name: string | null;
  readonly children: number;
}

/** One care form of a school, or its children without one. */
export interface OccupancyCareFormRow {
  readonly key: string;
  readonly school: TraegerSchoolOccupancy;
  /** `null` for the children of the school without a care form. */
  readonly form: TraegerOccupancyCareForm | null;
  readonly children: number;
}

/** A care form summed over the schools that name it the same. */
export interface OccupancyCareFormSum {
  readonly name: string;
  readonly schools: number;
  readonly children: number;
  /** The places of every offer; `null` when one of them has no limit. */
  readonly capacity: number | null;
}

export function occupancyTotals(
  schools: readonly TraegerSchoolOccupancy[],
): OccupancyTotals {
  return schools.reduce<OccupancyTotals>(
    (sum, school) => ({
      children: sum.children + school.children,
      inGroup: sum.inGroup + school.children - school.withoutGroup,
      withCareForm: sum.withCareForm + school.children - school.withoutCareForm,
    }),
    { children: 0, inGroup: 0, withCareForm: 0 },
  );
}

export function matchesSchool(
  school: TraegerSchoolOccupancy,
  search: string,
): boolean {
  const needle = foldForSearch(search.trim());
  if (needle === "") return true;
  return foldForSearch(`${school.name} ${school.city}`).includes(needle);
}

/** Every group of every school; children without a group close each school. */
export function occupancyGroupRows(
  schools: readonly TraegerSchoolOccupancy[],
): OccupancyGroupRow[] {
  return schools.flatMap((school) => [
    ...school.groups.map((group) => ({
      key: `${school.id}-${group.id}`,
      school,
      name: group.name,
      children: group.children,
    })),
    ...(school.withoutGroup > 0
      ? [
          {
            key: `${school.id}-ohne`,
            school,
            name: null,
            children: school.withoutGroup,
          },
        ]
      : []),
  ]);
}

/** Every care form of every school; children without one close each school. */
export function occupancyCareFormRows(
  schools: readonly TraegerSchoolOccupancy[],
): OccupancyCareFormRow[] {
  return schools.flatMap((school) => [
    ...school.careForms.map((form) => ({
      key: `${school.id}-${form.id}`,
      school,
      form,
      children: form.children,
    })),
    ...(school.withoutCareForm > 0
      ? [
          {
            key: `${school.id}-ohne`,
            school,
            form: null,
            children: school.withoutCareForm,
          },
        ]
      : []),
  ]);
}

/**
 * The care forms over all schools, by name, most children first. Offers of
 * several schools with the same name („OGS bis 16 Uhr“) add up; the places
 * only when every one of them has a limit.
 */
export function occupancyCareFormSums(
  schools: readonly TraegerSchoolOccupancy[],
): OccupancyCareFormSum[] {
  const sums = new Map<
    string,
    {
      name: string;
      schools: Set<string>;
      children: number;
      capacity: number | null;
      open: boolean;
    }
  >();
  for (const school of schools) {
    for (const form of school.careForms) {
      const key = foldForSearch(form.name.trim());
      const sum = sums.get(key) ?? {
        name: form.name.trim(),
        schools: new Set<string>(),
        children: 0,
        capacity: 0,
        open: false,
      };
      sum.schools.add(school.id);
      sum.children += form.children;
      if (form.capacity === null) sum.open = true;
      else sum.capacity = (sum.capacity ?? 0) + form.capacity;
      sums.set(key, sum);
    }
  }
  return [...sums.values()]
    .map((sum) => ({
      name: sum.name,
      schools: sum.schools.size,
      children: sum.children,
      capacity: sum.open ? null : sum.capacity,
    }))
    .sort(
      (a, b) => b.children - a.children || a.name.localeCompare(b.name, "de"),
    );
}

/** Whether an offer has as many children as places, or more. */
export function isFull(children: number, capacity: number | null): boolean {
  return capacity !== null && capacity > 0 && children >= capacity;
}

const percentFormat = new Intl.NumberFormat("de-DE", {
  style: "percent",
  maximumFractionDigits: 0,
});

/** „82 %“; „–“ without a whole. */
export function formatShare(part: number, whole: number): string {
  return whole > 0 ? percentFormat.format(part / whole) : "–";
}
