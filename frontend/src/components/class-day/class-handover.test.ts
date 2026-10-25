import { describe, expect, it } from "vitest";
import type { ClassDayRow } from "~/lib/class-day-api";
import { handoverLine, handoverRows, initialAbsentIds } from "./class-handover";

function row(overrides: Partial<ClassDayRow>): ClassDayRow {
  return {
    student_id: 1,
    first_name: "Kind",
    last_name: "Test",
    registered: true,
    stays_today: true,
    offerings: [],
    ...overrides,
  };
}

describe("class handover helpers (#4794)", () => {
  const rows = [
    row({ student_id: 1 }),
    row({
      student_id: 2,
      morning_attendance: { status: "absent", source: "teacher" },
    }),
    row({ student_id: 3, stays_today: false }),
    row({ student_id: 0, list_entry: true, list_entry_id: "9" }),
    row({ student_id: 4, handover_absent: true }),
  ];

  it("hands over only the children going to the OGS", () => {
    expect(handoverRows(rows).map((r) => r.student_id)).toEqual([1, 2, 4]);
  });

  it("prefills from the morning, or from the stored handover", () => {
    expect([...initialAbsentIds(rows, false)]).toEqual([2]);
    expect([...initialAbsentIds(rows, true)]).toEqual([4]);
  });

  it("takes unsaved morning marks into the prefill", () => {
    expect([
      ...initialAbsentIds(rows, false, {
        "student-1": "absent",
        "student-2": "present",
      }),
    ]).toEqual([1]);
  });

  it("states moment, Lehrkraft and absences in one line", () => {
    expect(
      handoverLine({
        handed_over_at: "2026-09-09T09:45:00Z",
        handed_over_by: "Petra Lehrkraft",
        absent_count: 3,
      }),
    ).toBe("Übergeben um 11:45 Uhr von Petra Lehrkraft · 3 Kinder nicht da");
    expect(
      handoverLine({ handed_over_at: "2026-09-09T09:45:00Z", absent_count: 0 }),
    ).toBe("Übergeben um 11:45 Uhr · alle da");
    expect(handoverLine({ handed_over_at: "", absent_count: 1 })).toBe(
      "Übergeben · 1 Kind nicht da",
    );
    expect(
      handoverLine(
        { handed_over_at: "2026-09-09T09:45:00Z", absent_count: 0 },
        0,
      ),
    ).toBe("Übergeben um 11:45 Uhr");
  });
});
