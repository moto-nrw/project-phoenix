import { describe, expect, it } from "vitest";
import {
  HANDOVER_ABSENT_WORDING,
  HANDOVER_PRESENT_WORDING,
  handoverDetail,
  handoverLine,
} from "./class-handover";

describe("class handover in the OGS views (#4794)", () => {
  it("names the Berlin clock time and the Lehrkraft", () => {
    expect(
      handoverDetail({
        school_class: "2b",
        handed_over_at: "2026-09-09T09:45:00Z",
        handed_over_by: "Petra Lehrkraft",
        absent: true,
      }),
    ).toBe("11:45 Uhr, Petra Lehrkraft");
  });

  it("leaves out a removed Lehrkraft and falls back to the class", () => {
    expect(
      handoverDetail({
        school_class: "2b",
        handed_over_at: "2026-09-09T09:45:00Z",
        absent: false,
      }),
    ).toBe("11:45 Uhr");
    expect(
      handoverDetail({
        school_class: "2b",
        handed_over_at: "kaputt",
        absent: false,
      }),
    ).toBe("Klasse 2b");
  });

  it("says whether the child was there", () => {
    const at = "2026-09-09T09:45:00Z";
    expect(
      handoverLine({ school_class: "2b", handed_over_at: at, absent: false }),
    ).toBe("Nach dem Unterricht da (11:45 Uhr)");
    expect(
      handoverLine({ school_class: "2b", handed_over_at: at, absent: true }),
    ).toBe("Nach dem Unterricht nicht da (11:45 Uhr)");
  });

  it("avoids the word of the group handover in the OGS portal", () => {
    expect(HANDOVER_ABSENT_WORDING).not.toMatch(/Übergabe/);
    expect(HANDOVER_PRESENT_WORDING).not.toMatch(/Übergabe/);
  });
});
