import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  STUDENT_EXPORT_COLUMNS,
  STUDENT_EXPORT_PRESETS,
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
});
