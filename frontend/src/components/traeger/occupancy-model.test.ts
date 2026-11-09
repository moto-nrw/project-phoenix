import { describe, expect, it } from "vitest";
import type { TraegerSchoolOccupancy } from "~/lib/traeger-occupancy-api";
import {
  formatShare,
  occupancyRatio,
  matchesSchool,
  occupancyCareFormRows,
  occupancyCareFormSums,
  occupancyGroupRows,
  occupancyPlaces,
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
    {
      id: "21",
      name: "OGS bis 16 Uhr",
      children: 8,
      capacity: 8,
      active: true,
    },
    {
      id: "22",
      name: "Frühbetreuung",
      children: 2,
      capacity: null,
      active: true,
    },
  ],
});
const LINDE = school("2", {
  name: "OGS Lindenweg",
  children: 6,
  groups: [{ id: "13", name: "Füchse", children: 6 }],
  careForms: [
    {
      id: "23",
      name: " OGS bis 16 uhr",
      children: 6,
      capacity: 10,
      active: true,
    },
  ],
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

  it("adds up the places of the offers with a limit, an unbooked offer with all its places free", () => {
    const unbooked = school("3", {
      careForms: [
        {
          id: "24",
          name: "Betreuung bis 15 Uhr",
          children: 0,
          capacity: 12,
          active: true,
        },
        {
          id: "25",
          name: "Spätbetreuung",
          children: 5,
          capacity: 4,
          active: true,
        },
      ],
    });
    expect(occupancyPlaces([BACH, LINDE, unbooked])).toEqual({
      limited: 4,
      places: 34,
      taken: 18,
      free: 16,
      full: 2,
    });
    expect(occupancyPlaces([school("4")])).toEqual({
      limited: 0,
      places: 0,
      taken: 0,
      free: 0,
      full: 0,
    });
  });

  it("adds only the used places of an offer the school no longer offers to the sums", () => {
    const retired = school("6", {
      careForms: [
        {
          id: "27",
          name: "OGS bis 16 Uhr",
          children: 3,
          capacity: 20,
          active: false,
        },
      ],
    });
    expect(occupancyCareFormSums([LINDE, retired])).toEqual([
      { name: "OGS bis 16 uhr", schools: 2, children: 9, capacity: 13 },
    ]);
  });

  it("leaves the places of an offer the school no longer offers out of the free places", () => {
    const retired = school("5", {
      careForms: [
        {
          id: "26",
          name: "Alte Betreuung",
          children: 5,
          capacity: 20,
          active: false,
        },
      ],
    });
    expect(occupancyPlaces([retired])).toEqual({
      limited: 0,
      places: 0,
      taken: 0,
      free: 0,
      full: 0,
    });
  });

  it("sorts zero places as finite without dividing zero by zero", () => {
    expect(occupancyRatio(2, null)).toBeNull();
    expect(occupancyRatio(0, 0)).toBe(0);
    expect(occupancyRatio(2, 0)).toBe(Number.POSITIVE_INFINITY);
    expect(occupancyRatio(2, 4)).toBe(0.5);
  });

  it("finds a school by name or town, ignoring case and umlauts", () => {
    expect(matchesSchool(BACH, "bissen")).toBe(true);
    expect(matchesSchool(LINDE, "LINDEN")).toBe(true);
    expect(matchesSchool(BACH, "linden")).toBe(false);
    expect(matchesSchool(BACH, "  ")).toBe(true);
  });

  it("formats shares", () => {
    expect(formatShare(3, 4)).toMatch(/^75\s%$/);
    expect(formatShare(0, 0)).toBe("–");
  });
});
