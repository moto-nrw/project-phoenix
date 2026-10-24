import { describe, expect, it } from "vitest";
import {
  HANDOVER_ABSENT_WORDING,
  handoverAbsenceDetail,
} from "./class-handover";

describe("handoverAbsenceDetail (#4794)", () => {
  it("names the Berlin clock time and the Lehrkraft", () => {
    expect(
      handoverAbsenceDetail({
        school_class: "2b",
        handed_over_at: "2026-09-09T09:45:00Z",
        handed_over_by: "Petra Lehrkraft",
      }),
    ).toBe("11:45 Uhr, Petra Lehrkraft");
  });

  it("leaves out a removed Lehrkraft and falls back to the class", () => {
    expect(
      handoverAbsenceDetail({
        school_class: "2b",
        handed_over_at: "2026-09-09T09:45:00Z",
      }),
    ).toBe("11:45 Uhr");
    expect(
      handoverAbsenceDetail({ school_class: "2b", handed_over_at: "kaputt" }),
    ).toBe("Klasse 2b");
  });

  it("avoids the word of the group handover in the OGS portal", () => {
    expect(HANDOVER_ABSENT_WORDING).not.toMatch(/Übergabe/);
  });
});
