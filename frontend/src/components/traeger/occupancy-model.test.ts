import { describe, expect, it } from "vitest";
import type { TraegerSchoolOccupancy } from "~/lib/traeger-occupancy-api";
import {
  formatShare,
  isFull,
  matchesSchool,
  occupancyCareFormRows,
  occupancyCareFormSums,
  occupancyGroupRows,
  occupancyTotals,
} from "./occupancy-model";

function school(
  id: string,
  overrides: Partial<TraegerSchoolOccupancy> = {},
): TraegerSchoolOccupancy {
  return {
    id,
    name: `OGS ${id}`,
    slug: id,
    subdomain: id,
    city: "Melle",
    children: 0,
    withoutGroup: 0,
    groups: [],
    withoutCareForm: 0,
    careForms: [],
    ...overrides,
  };
}

const BACH = school("1", {
  name: "OGS Am Bach",
  city: "Bissendorf",
  children: 10,
  withoutGroup: 2,
  groups: [
    { id: "11", name: "Bären", children: 5 },
    { id: "12", name: "Eulen", children: 3 },
  ],
  withoutCareForm: 1,
  careForms: [
    { id: "21", name: "OGS bis 16 Uhr", children: 8, capacity: 8 },
    { id: "22", name: "Frühbetreuung", children: 2, capacity: null },
  ],
});
const LINDE = school("2", {
  name: "OGS Lindenweg",
  children: 6,
  groups: [{ id: "13", name: "Füchse", children: 6 }],
  careForms: [{ id: "23", name: " OGS bis 16 uhr", children: 6, capacity: 10 }],
});

describe("occupancy model", () => {
  it("adds up the children, those in a group and those with a care form", () => {
    expect(occupancyTotals([BACH, LINDE])).toEqual({
      children: 16,
      inGroup: 14,
      withCareForm: 15,
    });
    expect(occupancyTotals([])).toEqual({
      children: 0,
      inGroup: 0,
      withCareForm: 0,
    });
  });

  it("lists every group and closes a school with its children without a group", () => {
    const rows = occupancyGroupRows([BACH, LINDE]);
    expect(rows.map((row) => [row.school.id, row.name, row.children])).toEqual([
      ["1", "Bären", 5],
      ["1", "Eulen", 3],
      ["1", null, 2],
      ["2", "Füchse", 6],
    ]);
    expect(new Set(rows.map((row) => row.key)).size).toBe(rows.length);
  });

  it("lists every care form and the children without one", () => {
    const rows = occupancyCareFormRows([BACH, LINDE]);
    expect(
      rows.map((row) => [row.school.id, row.form?.name ?? null, row.children]),
    ).toEqual([
      ["1", "OGS bis 16 Uhr", 8],
      ["1", "Frühbetreuung", 2],
      ["1", null, 1],
      ["2", " OGS bis 16 uhr", 6],
    ]);
  });

  it("sums care forms of the same name over the schools, places only when every offer has a limit", () => {
    expect(occupancyCareFormSums([BACH, LINDE])).toEqual([
      { name: "OGS bis 16 Uhr", schools: 2, children: 14, capacity: 18 },
      { name: "Frühbetreuung", schools: 1, children: 2, capacity: null },
    ]);
  });

  it("finds a school by name or town, ignoring case and umlauts", () => {
    expect(matchesSchool(BACH, "bissen")).toBe(true);
    expect(matchesSchool(LINDE, "LINDEN")).toBe(true);
    expect(matchesSchool(BACH, "linden")).toBe(false);
    expect(matchesSchool(BACH, "  ")).toBe(true);
  });

  it("marks an offer full at its places and formats shares", () => {
    expect(isFull(8, 8)).toBe(true);
    expect(isFull(9, 8)).toBe(true);
    expect(isFull(7, 8)).toBe(false);
    expect(isFull(5, null)).toBe(false);
    expect(isFull(0, 0)).toBe(false);
    expect(formatShare(3, 4)).toMatch(/^75\s%$/);
    expect(formatShare(0, 0)).toBe("–");
  });
});
