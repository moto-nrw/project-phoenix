import { describe, expect, it } from "vitest";
import type {
  TraegerCatalogSchool,
  TraegerOffering,
  TraegerPhaseDetail,
  TraegerPhaseFields,
} from "~/lib/traeger-catalog-api";
import {
  formatFieldValue,
  hasPendingRollout,
  schoolStandings,
  shownChanges,
} from "./catalog-format";
import { blankOfferingDraft } from "./catalog-offering-form";

const fields: TraegerPhaseFields = {
  name: "Schuljahr 2027/28",
  kind: "school_year",
  serviceStartDate: "2027-08-01",
  serviceEndDate: "2028-07-31",
  enrollmentOpenAt: null,
  enrollmentCloseAt: null,
  audience: "open",
  careOverflowMode: "waitlist",
  careOfferingSelectionMode: "optional",
};

function school(
  schoolId: string,
  patch: Partial<TraegerCatalogSchool> = {},
): TraegerCatalogSchool {
  return {
    schoolId,
    selected: true,
    appliedVersion: 1,
    outcome: "applied",
    errorCode: null,
    recordedAt: "2026-11-11T10:00:00Z",
    ...patch,
  };
}

function offering(patch: Partial<TraegerOffering> = {}): TraegerOffering {
  const draft = {
    fields: blankOfferingDraft().fields,
    prices: [],
    contractTemplate: null,
  };
  return {
    id: "31",
    traegerPhaseId: "7",
    draft,
    releasedVersion: 1,
    released: draft,
    withdrawnAt: null,
    schools: [
      {
        schoolId: "11",
        appliedVersion: 1,
        outcome: "applied",
        errorCode: null,
      },
    ],
    ...patch,
  };
}

function phase(patch: Partial<TraegerPhaseDetail> = {}): TraegerPhaseDetail {
  return {
    id: "7",
    draft: fields,
    allSchools: false,
    releasedVersion: 1,
    released: fields,
    withdrawnAt: null,
    schools: [school("11")],
    updatedAt: "2026-11-11T10:00:00Z",
    offerings: [offering()],
    ...patch,
  };
}

describe("hasPendingRollout (#4704)", () => {
  it("is quiet when every chosen school holds the released version", () => {
    expect(hasPendingRollout(phase(), ["11", "12"])).toBe(false);
  });

  it("reports a draft the office changed after the release", () => {
    expect(
      hasPendingRollout(phase({ draft: { ...fields, name: "Neu" } }), ["11"]),
    ).toBe(true);
    const changed = offering();
    expect(
      hasPendingRollout(
        phase({
          offerings: [
            {
              ...changed,
              draft: {
                ...changed.draft,
                fields: { ...changed.draft.fields, capacity: 20 },
              },
            },
          ],
        }),
        ["11"],
      ),
    ).toBe(true);
  });

  it("reports a school of „Alle Schulen“ that has no copy yet", () => {
    expect(hasPendingRollout(phase({ allSchools: true }), ["11", "12"])).toBe(
      true,
    );
  });

  it("reports a refused school and a deselected school that still holds a copy", () => {
    expect(
      hasPendingRollout(
        phase({ schools: [school("11", { outcome: "rejected" })] }),
        ["11"],
      ),
    ).toBe(true);
    expect(
      hasPendingRollout(
        phase({ schools: [school("11"), school("12", { selected: false })] }),
        ["11", "12"],
      ),
    ).toBe(true);
  });

  it("reports a withdrawal until no school holds a copy any more", () => {
    const withdrawn = { withdrawnAt: "2026-11-11T11:00:00Z" };
    expect(hasPendingRollout(phase(withdrawn), ["11"])).toBe(true);
    expect(
      hasPendingRollout(
        phase({
          ...withdrawn,
          schools: [school("11", { outcome: "withdrawn" })],
        }),
        ["11"],
      ),
    ).toBe(false);
  });
});

describe("schoolStandings", () => {
  it("lists every school of „Alle Schulen“ and marks a deselected copy as leaving", () => {
    const standings = schoolStandings(
      phase({
        allSchools: true,
        schools: [school("11")],
      }),
      ["11", "12"],
    );
    expect(
      standings.map((row) => [row.schoolId, row.row?.outcome ?? null]),
    ).toEqual([
      ["11", "applied"],
      ["12", null],
    ]);

    const leaving = schoolStandings(
      phase({ schools: [school("11"), school("12", { selected: false })] }),
      ["11", "12"],
    );
    expect(leaving.find((row) => row.schoolId === "12")?.leaving).toBe(true);
  });
});

describe("formatFieldValue", () => {
  it("shows the values in the words of the form", () => {
    expect(formatFieldValue("audience", "new_students")).toBe(
      "Nur neue Kinder",
    );
    expect(formatFieldValue("service_start_date", "2027-08-01")).toBe(
      "01.08.2027",
    );
    expect(formatFieldValue("available_days", ["mon", "fri"])).toBe("Mo, Fr");
    expect(
      formatFieldValue("pickup_times", { tue: "15:00", mon: "14:30" }),
    ).toBe("Mo 14:30, Di 15:00");
    expect(formatFieldValue("is_active", false)).toBe("Nein");
    expect(formatFieldValue("capacity", null)).toBe("–");
    expect(formatFieldValue("price_cents", 650)).toContain("6,50");
  });

  it("shows changed fields and those the school keeps, never unchanged ones", () => {
    expect(
      shownChanges([
        { field: "name", state: "unchanged", traeger: "a", school: "a" },
        { field: "capacity", state: "keeps", traeger: 20, school: 12 },
        {
          field: "kind",
          state: "changes",
          traeger: "holiday",
          school: "custom",
        },
      ]).map((change) => change.field),
    ).toEqual(["capacity", "kind"]);
  });
});
