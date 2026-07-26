import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  STUDENT_EXPORT_COLUMNS,
  STUDENT_EXPORT_PRESETS,
  buildStudentExportColumns,
  buildStudentExportPresets,
  exportStudents,
  type StudentExportRequest,
} from "./student-export-api";

const originalFetch = globalThis.fetch;
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

const request: StudentExportRequest = {
  format: "pdf",
  preset: "ogs_weekly",
  title: "Meine Liste",
  filters: { search: "klasse 1a" },
  columns: ["name", "school_class"],
};

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => "blob:export-url");
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  URL.createObjectURL = originalCreateObjectURL;
  URL.revokeObjectURL = originalRevokeObjectURL;
  vi.restoreAllMocks();
});

describe("student export metadata", () => {
  it("keeps blocked internal columns out of the catalog", () => {
    const ids = STUDENT_EXPORT_COLUMNS.map((column) => column.id);

    expect(ids).not.toContain("room");
    expect(ids).not.toContain("homeroom");
    expect(ids).not.toContain("identifier");
  });

  it("defines weekday presets with explicit weekday columns", () => {
    const weeklyPreset = STUDENT_EXPORT_PRESETS.find(
      (preset) => preset.id === "ogs_weekly",
    );

    expect(weeklyPreset?.columns).toEqual([
      "name",
      "school_class",
      "group",
      "weekly_monday",
      "weekly_tuesday",
      "weekly_wednesday",
      "weekly_thursday",
      "weekly_friday",
    ]);
  });

  it("offers the departure column and includes it in the pickup list (#1610)", () => {
    const ids = STUDENT_EXPORT_COLUMNS.map((column) => column.id);
    expect(ids).toContain("departure");

    const pickupPreset = STUDENT_EXPORT_PRESETS.find(
      (preset) => preset.id === "pickup_list",
    );
    expect(pickupPreset?.columns).toContain("departure");
  });

  it("offers Tagesstatus and includes it in the Tagesplanung preset", () => {
    const dailyStatus = STUDENT_EXPORT_COLUMNS.find(
      (column) => column.id === "daily_status",
    );
    expect(dailyStatus?.label).toBe("Tagesstatus");

    const dailyPreset = STUDENT_EXPORT_PRESETS.find(
      (preset) => preset.id === "daily_planning",
    );
    expect(dailyPreset?.columns).toContain("daily_status");
  });

  it("offers a class roster preset with enrollment status", () => {
    const ids = STUDENT_EXPORT_COLUMNS.map((column) => column.id);
    expect(ids).toContain("enrollment_summary");

    const classRoster = STUDENT_EXPORT_PRESETS.find(
      (preset) => preset.id === "class_roster",
    );
    expect(classRoster?.columns).toEqual([
      "name",
      "school_class",
      "group",
      "enrollment_summary",
      "care_days",
      "weekly_monday",
      "weekly_tuesday",
      "weekly_wednesday",
      "weekly_thursday",
      "weekly_friday",
      "departure",
    ]);
  });
});

describe("planning-date export copy (#1939)", () => {
  it("returns the today constants verbatim when no date is given", () => {
    expect(buildStudentExportColumns()).toBe(STUDENT_EXPORT_COLUMNS);
    expect(buildStudentExportPresets()).toBe(STUDENT_EXPORT_PRESETS);
  });

  it("names the selected day instead of 'heute' in day-scoped columns", () => {
    const columns = buildStudentExportColumns("2026-07-20");
    const dayScoped = new Set([
      "planned_arrival",
      "planned_pickup",
      "daily_status",
      "daily_notes",
      // Age is calculated against the selected day too, so its description must
      // name that day rather than claim "heute" (#1939).
      "age",
    ]);
    for (const column of columns) {
      if (dayScoped.has(column.id)) {
        expect(column.description).toContain("20.07.2026");
        expect(column.description.toLowerCase()).not.toContain("heut");
      }
    }
    // Order is preserved and only the copy changes — except the live-location
    // snapshot column, which cannot describe a future day and is dropped (#1939).
    expect(columns.map((column) => column.id)).toEqual(
      STUDENT_EXPORT_COLUMNS.filter(
        (column) => column.group !== "snapshot",
      ).map((column) => column.id),
    );
  });

  it("drops live-location snapshot columns and presets for a dated export", () => {
    const columns = buildStudentExportColumns("2026-07-20");
    const presets = buildStudentExportPresets("2026-07-20");

    // The current-location column reads from today's live snapshot even for a
    // dated request, so it must not be offered on a future planning day.
    expect(columns.some((column) => column.id === "current_location")).toBe(
      false,
    );
    // The snapshot preset is built on that column — it disappears too.
    expect(presets.some((preset) => preset.id === "attendance_snapshot")).toBe(
      false,
    );
    // Non-snapshot presets still expose all their (non-snapshot) columns.
    expect(presets.some((preset) => preset.id === "daily_planning")).toBe(true);
  });

  it("names the selected day in the day-scoped presets", () => {
    const presets = buildStudentExportPresets("2026-07-20");
    const daily = presets.find((preset) => preset.id === "daily_planning");
    const compact = presets.find((preset) => preset.id === "ogs_compact");

    expect(daily?.description).toContain("20.07.2026");
    expect(daily?.description.toLowerCase()).not.toContain("heut");
    expect(compact?.description).toContain("20.07.2026");
    expect(compact?.description.toLowerCase()).not.toContain("heut");

    // The date-neutral presets keep their base description.
    const weekly = presets.find((preset) => preset.id === "ogs_weekly");
    expect(weekly?.description).toBe(
      STUDENT_EXPORT_PRESETS.find((preset) => preset.id === "ogs_weekly")
        ?.description,
    );
  });
});

describe("exportStudents", () => {
  it("posts the export request and downloads the returned blob", async () => {
    const click = vi.fn();
    const remove = vi.fn();
    const append = vi.spyOn(document.body, "append");
    const createElement = vi
      .spyOn(document, "createElement")
      .mockReturnValue({ click, remove } as unknown as HTMLAnchorElement);

    globalThis.fetch = vi.fn(async () => {
      return new Response(new Blob(["pdf"]), {
        status: 200,
        headers: {
          "content-disposition": 'attachment; filename="meine-liste.pdf"',
        },
      });
    });

    await exportStudents(request);

    expect(globalThis.fetch).toHaveBeenCalledWith("/api/students/export", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
    expect(createElement).toHaveBeenCalledWith("a");
    expect(append).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
    expect(remove).toHaveBeenCalled();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:export-url");
  });

  it("uses a local filename when the response has no disposition header", async () => {
    let download = "";
    const link = {
      click: vi.fn(),
      remove: vi.fn(),
      set download(value: string) {
        download = value;
      },
      set href(_value: string) {},
    } as unknown as HTMLAnchorElement;

    vi.spyOn(document, "createElement").mockReturnValue(link);
    vi.spyOn(document.body, "append").mockImplementation(() => undefined);
    globalThis.fetch = vi.fn(async () => new Response(new Blob(["pdf"])));

    await exportStudents({ ...request, title: "  " });

    expect(download).toBe("kindersuche-export.pdf");
  });

  it("throws the backend error body when the export fails", async () => {
    globalThis.fetch = vi.fn(async () => {
      return new Response("keine Berechtigung", { status: 403 });
    });

    await expect(exportStudents(request)).rejects.toThrow("keine Berechtigung");
  });

  it("unwraps the JSON error envelope from the export proxy", async () => {
    globalThis.fetch = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            error:
              "die Auswahl umfasst 6000 Kinder, ein Export ist auf höchstens 5000 Kinder begrenzt",
          }),
          { status: 400, headers: { "Content-Type": "application/json" } },
        ),
    );

    await expect(exportStudents(request)).rejects.toThrow(
      "die Auswahl umfasst 6000 Kinder",
    );
  });
});
