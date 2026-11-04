import { describe, expect, it } from "vitest";
import {
  accidentReportDraftErrors,
  draftFromReport,
  emptyAccidentReportDraft,
  fieldsFromDraft,
} from "./accident-report-form";

describe("accident report draft", () => {
  it("names every missing required field of the official form", () => {
    expect(
      Object.keys(accidentReportDraftErrors(emptyAccidentReportDraft())).sort(),
    ).toEqual(
      [
        "description",
        "injuryKind",
        "location",
        "occurredAt",
        "supervisor",
      ].sort(),
    );
  });

  it("asks for the doctor only when the child saw one", () => {
    const draft = {
      ...emptyAccidentReportDraft(),
      occurredAt: "2026-10-20T10:40",
      location: "Schulhof",
      description: "Gestürzt",
      injuryKind: "Prellung",
      supervisor: "Frau Berger",
    };
    expect(accidentReportDraftErrors(draft)).toEqual({});
    expect(
      accidentReportDraftErrors({ ...draft, doctorVisited: true }),
    ).toHaveProperty("doctor");
  });

  it("reads and writes the times as Berlin wall-clock values", () => {
    const fields = fieldsFromDraft({
      ...emptyAccidentReportDraft(),
      occurredAt: "2026-10-20T10:40",
      parentsNotified: "Mutter",
      parentsNotifiedAt: "2026-10-20T11:05",
    });
    expect(fields.occurredAt).toBe("2026-10-20T08:40:00.000Z");
    expect(fields.parentsNotifiedAt).toBe("2026-10-20T09:05:00.000Z");

    const draft = draftFromReport(fields);
    expect(draft.occurredAt).toBe("2026-10-20T10:40");
    expect(draft.parentsNotifiedAt).toBe("2026-10-20T11:05");
  });

  it("drops the notification time when no parent was told", () => {
    const fields = fieldsFromDraft({
      ...emptyAccidentReportDraft(),
      occurredAt: "2026-10-20T10:40",
      parentsNotifiedAt: "2026-10-20T11:05",
    });
    expect(fields.parentsNotifiedAt).toBeNull();
  });
});
