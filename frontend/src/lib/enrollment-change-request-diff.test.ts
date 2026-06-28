import { describe, expect, it } from "vitest";
import {
  buildEnrollmentChangeRequestDiffGroups,
  formatEnrollmentChangeRequestValue,
} from "./enrollment-change-request-diff";

describe("buildEnrollmentChangeRequestDiffGroups", () => {
  it("renders top-level guardian fields as changed rows", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: { guardian_first_name: "Daniela" },
      proposedSnapshot: { guardian_first_name: "Danielaaaa" },
      diff: { changed: ["guardian_first_name"] },
    });

    expect(groups).toEqual([
      {
        key: "guardian_first_name",
        label: "Vorname",
        rows: [
          {
            id: "guardian_first_name",
            label: "Vorname",
            before: "Daniela",
            after: "Danielaaaa",
          },
        ],
      },
    ]);
  });

  it("renders additional guardians per person and field", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        additional_guardians: [
          { first_name: "Coco", last_name: "Sommer", email: "a@example.test" },
        ],
      },
      proposedSnapshot: {
        additional_guardians: [
          {
            first_name: "Cocoa",
            last_name: "Sommer",
            email: "b@example.test",
          },
        ],
      },
      diff: { changed: ["additional_guardians"] },
    });

    expect(groups[0]?.label).toBe("Weitere erziehungsberechtigte Personen");
    expect(groups[0]?.rows).toEqual([
      {
        id: "Weitere Person-0-email",
        label: "Weitere Person 1 · E-Mail",
        before: "a@example.test",
        after: "b@example.test",
      },
      {
        id: "Weitere Person-0-first_name",
        label: "Weitere Person 1 · Vorname",
        before: "Coco",
        after: "Cocoa",
      },
    ]);
  });

  it("renders child and child custom-data changes per field", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        children: [
          {
            id: "99",
            status: "approved",
            first_name: "Lea",
            last_name: "Sommer",
            custom_data: { allergies: "keine" },
          },
        ],
      },
      proposedSnapshot: {
        children: [
          {
            first_name: "Lea-Marie",
            last_name: "Sommer",
            custom_data: { allergies: "Nuesse" },
          },
        ],
      },
      diff: { changed: ["children"] },
    });

    expect(groups[0]?.rows).toEqual([
      {
        id: "Kind 1 · Zusatzangaben · allergies",
        label: "Kind 1 · Zusatzangaben · allergies",
        before: "keine",
        after: "Nuesse",
      },
      {
        id: "Kind-0-first_name",
        label: "Kind 1 · Vorname",
        before: "Lea",
        after: "Lea-Marie",
      },
    ]);
  });

  it("renders added guardian entries with missing before values", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: { additional_guardians: [] },
      proposedSnapshot: {
        additional_guardians: [
          { first_name: "Coco", last_name: "Sommer", phone: "+49 221" },
        ],
      },
      diff: { changed: ["additional_guardians"] },
    });

    expect(groups[0]?.rows).toEqual([
      {
        id: "Weitere Person-0-first_name",
        label: "Weitere Person 1 · Vorname",
        before: undefined,
        after: "Coco",
      },
      {
        id: "Weitere Person-0-last_name",
        label: "Weitere Person 1 · Nachname",
        before: undefined,
        after: "Sommer",
      },
      {
        id: "Weitere Person-0-phone",
        label: "Weitere Person 1 · Telefon",
        before: undefined,
        after: "+49 221",
      },
    ]);
  });

  it("drops unchanged array entries", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        additional_guardians: [{ first_name: "Coco", last_name: "Sommer" }],
      },
      proposedSnapshot: {
        additional_guardians: [{ first_name: "Coco", last_name: "Sommer" }],
      },
      diff: { changed: ["additional_guardians"] },
    });

    expect(groups).toEqual([]);
  });

  it("drops child changes that only remove internal metadata", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        children: [
          {
            id: "99",
            status: "approved",
            first_name: "Lea",
            last_name: "Sommer",
          },
        ],
      },
      proposedSnapshot: {
        children: [
          {
            first_name: "Lea",
            last_name: "Sommer",
          },
        ],
      },
      diff: { changed: ["children"] },
    });

    expect(groups).toEqual([]);
  });
});

describe("formatEnrollmentChangeRequestValue", () => {
  it("formats primitive arrays as values instead of entry counts", () => {
    expect(formatEnrollmentChangeRequestValue(["mon", "wed", "fri"])).toBe(
      "Mo, Mi, Fr",
    );
  });

  it("formats offering day objects explicitly", () => {
    expect(
      formatEnrollmentChangeRequestValue([
        { offering_id: "12", selected_days: ["mon", "tue"] },
      ]),
    ).toBe("Betreuungsangebot: 12; Tage: Mo, Di");
  });
});
