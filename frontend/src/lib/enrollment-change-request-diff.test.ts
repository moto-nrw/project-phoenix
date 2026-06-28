import { describe, expect, it } from "vitest";
import {
  buildEnrollmentChangeRequestDiffGroups,
  changedEnrollmentChangeRequestLabels,
  DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY,
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

  it("uses injected labels for parent-visible localized diffs", () => {
    const copy = {
      ...DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY,
      snapshotLabels: {
        ...DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY.snapshotLabels,
        additional_guardians: "Additional guardians",
      },
      guardianSnapshotLabels: {
        ...DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY.guardianSnapshotLabels,
        first_name: "First name",
      },
      additionalGuardianEntryLabel: "Additional person",
    };
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        additional_guardians: [{ first_name: "Coco" }],
      },
      proposedSnapshot: {
        additional_guardians: [{ first_name: "Cocoa" }],
      },
      diff: { changed: ["additional_guardians"] },
      copy,
    });

    expect(groups[0]?.label).toBe("Additional guardians");
    expect(groups[0]?.rows[0]?.label).toBe("Additional person 1 · First name");
  });

  it("filters invalid and unchanged entries from changed arrays", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        guardian_first_name: "Daniela",
        guardian_last_name: "Sommer",
      },
      proposedSnapshot: {
        guardian_first_name: "Daniela",
        guardian_last_name: "Winter",
      },
      diff: { changed: ["guardian_first_name", 123, "guardian_last_name"] },
    });

    expect(groups).toEqual([
      {
        key: "guardian_last_name",
        label: "Nachname",
        rows: [
          {
            id: "guardian_last_name",
            label: "Nachname",
            before: "Sommer",
            after: "Winter",
          },
        ],
      },
    ]);
  });

  it("uses explicit diff keys when no changed array is present", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        guardian_phone: "111",
        guardian_email: "same@example.test",
      },
      proposedSnapshot: {
        guardian_phone: "222",
        guardian_email: "same@example.test",
      },
      diff: { guardian_phone: true, guardian_email: true, changed: "ignored" },
    });

    expect(groups.map((group) => group.key)).toEqual(["guardian_phone"]);
    expect(groups[0]?.rows[0]).toMatchObject({
      before: "111",
      after: "222",
    });
  });

  it("infers sorted changed keys when diff has no usable keys", () => {
    const labels = changedEnrollmentChangeRequestLabels({
      baseSnapshot: {
        guardian_last_name: "Sommer",
        guardian_first_name: "Daniela",
        guardian_phone: undefined,
      },
      proposedSnapshot: {
        guardian_last_name: "Winter",
        guardian_first_name: "Dani",
        guardian_phone: null,
      },
      diff: {},
    });

    expect(labels).toEqual(["Vorname", "Nachname"]);
  });

  it("renders top-level custom data and consent flag object diffs", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        custom_data: { allergies: "keine", pickup_note: "Oma" },
        consent_flags: { photo: true, newsletter: false },
      },
      proposedSnapshot: {
        custom_data: { allergies: "Nuesse", pickup_note: "Oma" },
        consent_flags: { photo: false, newsletter: false },
      },
      diff: { custom_data: true, consent_flags: true },
    });

    expect(groups).toEqual([
      {
        key: "custom_data",
        label: "Zusatzangaben",
        rows: [
          {
            id: "allergies",
            label: "allergies",
            before: "keine",
            after: "Nuesse",
          },
        ],
      },
      {
        key: "consent_flags",
        label: "Zustimmungen",
        rows: [
          {
            id: "photo",
            label: "photo",
            before: true,
            after: false,
          },
        ],
      },
    ]);
  });

  it("renders non-record array entry replacements as entry rows", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: { additional_guardians: ["legacy guardian"] },
      proposedSnapshot: { additional_guardians: [{ first_name: "Coco" }] },
      diff: { changed: ["additional_guardians"] },
    });

    expect(groups[0]?.rows).toEqual([
      {
        id: "Weitere Person-0-first_name",
        label: "Weitere Person 1 · Vorname",
        before: undefined,
        after: "Coco",
      },
    ]);
  });

  it("renders removed non-record array entries and ignores unchanged ones", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: { children: ["legacy child", "unchanged"] },
      proposedSnapshot: { children: [null, "unchanged"] },
      diff: { changed: ["children"] },
    });

    expect(groups[0]?.rows).toEqual([
      {
        id: "Kind-0",
        label: "Kind 1",
        before: "legacy child",
        after: null,
      },
    ]);
  });

  it("suppresses child custom-data rows that only touch metadata", () => {
    const groups = buildEnrollmentChangeRequestDiffGroups({
      baseSnapshot: {
        children: [
          {
            first_name: "Lea",
            custom_data: { id: "internal", status: "approved" },
          },
        ],
      },
      proposedSnapshot: {
        children: [
          {
            first_name: "Lea",
            custom_data: { id: "changed", status: "pending" },
          },
        ],
      },
      diff: { changed: ["children"] },
    });

    expect(groups).toEqual([]);
  });
});

describe("formatEnrollmentChangeRequestValue", () => {
  it("formats empty and scalar values", () => {
    expect(formatEnrollmentChangeRequestValue(null)).toBe("Nicht gesetzt");
    expect(formatEnrollmentChangeRequestValue(undefined)).toBe("Nicht gesetzt");
    expect(formatEnrollmentChangeRequestValue("")).toBe("Nicht gesetzt");
    expect(formatEnrollmentChangeRequestValue(false)).toBe("Nein");
    expect(formatEnrollmentChangeRequestValue(12)).toBe("12");
    expect(formatEnrollmentChangeRequestValue("mon")).toBe("Mo");
    expect(formatEnrollmentChangeRequestValue(Symbol("x"))).toBe("Symbol(x)");
  });

  it("formats empty arrays and records as empty", () => {
    expect(formatEnrollmentChangeRequestValue([])).toBe("Nicht gesetzt");
    expect(formatEnrollmentChangeRequestValue({})).toBe("Nicht gesetzt");
    expect(formatEnrollmentChangeRequestValue({ note: "" })).toBe(
      "Nicht gesetzt",
    );
  });

  it("formats primitive arrays as values instead of entry counts", () => {
    expect(formatEnrollmentChangeRequestValue(["mon", "wed", "fri"])).toBe(
      "Mo, Mi, Fr",
    );
  });

  it("formats mixed arrays with indexed fallback entries", () => {
    expect(formatEnrollmentChangeRequestValue(["mon", ["tue"], null])).toBe(
      "1. Mo; 2. Di; 3. Nicht gesetzt",
    );
  });

  it("formats offering day objects explicitly", () => {
    expect(
      formatEnrollmentChangeRequestValue([
        { offering_id: "12", selected_days: ["mon", "tue"] },
      ]),
    ).toBe("Betreuungsangebot: 12; Tage: Mo, Di");
  });

  it("uses injected value labels", () => {
    const copy = {
      ...DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY,
      recordValueLabels: {
        ...DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY.recordValueLabels,
        selected_days: "Days",
      },
      weekdayLabels: {
        ...DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY.weekdayLabels,
        mon: "Mon",
      },
      yesLabel: "Yes",
      noLabel: "No",
    };

    expect(formatEnrollmentChangeRequestValue(true, "Not provided", copy)).toBe(
      "Yes",
    );
    expect(
      formatEnrollmentChangeRequestValue(
        { selected_days: ["mon"] },
        "Not provided",
        copy,
      ),
    ).toBe("Days: Mon");
  });

  it("formats weekday records by booleans, strings, and arrays", () => {
    expect(formatEnrollmentChangeRequestValue({ mon: true, tue: false })).toBe(
      "Mo",
    );
    expect(formatEnrollmentChangeRequestValue({ mon: "", tue: "15:00" })).toBe(
      "Di: 15:00",
    );
    expect(formatEnrollmentChangeRequestValue({ mon: [], wed: ["fri"] })).toBe(
      "Mi: Fr",
    );
    expect(formatEnrollmentChangeRequestValue({ mon: false })).toBe(
      "mon: Nein",
    );
  });
});
