import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: vi.fn(),
    warn: vi.fn(),
  }),
}));

import {
  exportCareUsageReport,
  exportPhaseClassRoster,
  getCareUsageReport,
  type CareUsageFilters,
  type CareUsageReport,
} from "./enrollment-report-api";

const originalFetch = globalThis.fetch;
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

function report(overrides: Partial<CareUsageReport> = {}): CareUsageReport {
  return {
    phase: { id: "42", name: "Schuljahr 2026/27" },
    filters: {
      phase_id: "42",
      status: "all",
      care_offering_ids: ["7"],
    },
    totals: {
      children: 2,
      by_day_count: { "2": 1, "5": 1 },
      by_weekday_pickup_time: {
        mon: { "14:30": 1, "16:00": 1 },
        tue: { "14:30": 1 },
      },
    },
    by_offering: [
      {
        offering_id: "7",
        offering_name: "OGS",
        children: 2,
        by_day_count: { "2": 1, "5": 1 },
      },
    ],
    filter_options: {
      offerings: [{ id: "7", name: "OGS", counts_as_care: true }],
      grade_levels: [1, 2],
      pickup_times: ["14:30", "16:00"],
    },
    rows: [
      {
        request_id: "100",
        child_id: "200",
        child_first_name: "Ada",
        child_last_name: "Lovelace",
        date_of_birth: "2019-01-02",
        target_grade_level: 1,
        status: "approved",
        offerings: [
          {
            id: "7",
            name: "OGS",
            days: ["monday", "tuesday"],
            days_source: "selected",
            days_of_week_mode: "selected",
          },
        ],
        effective_days: ["monday", "tuesday"],
        day_count: 2,
        pickup_by_day: { mon: "14:30", tue: "16:00" },
        guardian_first_name: "Grace",
        guardian_last_name: "Hopper",
        guardian_email: "grace@example.test",
        guardian_phone: null,
        submitted_at: "2026-04-12T08:30:00Z",
      },
    ],
    ...overrides,
  };
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

function mockFetch(
  impl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  globalThis.fetch = vi.fn(impl) as typeof globalThis.fetch;
  return globalThis.fetch as ReturnType<typeof vi.fn>;
}

describe("getCareUsageReport", () => {
  beforeEach(() => {
    globalThis.fetch = originalFetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("builds the full query string and unwraps backend data envelopes", async () => {
    let seenURL = "";
    let seenInit: RequestInit | undefined;
    const expected = report();
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenInit = init;
      return jsonResponse({ data: expected });
    });

    const filters: CareUsageFilters = {
      phase_id: "42",
      status: "approved",
      care_offering_ids: ["7", "8"],
      day_count: 0,
      grade_level: 1,
      weekday: "mon",
      pickup_time: "14:30",
      search: "  Ada  ",
    };
    const actual = await getCareUsageReport(filters);

    expect(actual).toStrictEqual(expected);
    expect(seenInit).toEqual({ cache: "no-store" });
    expect(seenURL).toBe(
      "/api/enrollment/admin/reports/care-usage?phase_id=42&status=approved&care_offering_ids=7&care_offering_ids=8&day_count=0&grade_level=1&weekday=mon&pickup_time=14%3A30&search=Ada",
    );
  });

  it("sends an explicit empty care offering filter", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: report() });
    });

    await getCareUsageReport({
      phase_id: "42",
      care_offering_ids: [],
    });

    expect(seenURL).toBe(
      "/api/enrollment/admin/reports/care-usage?phase_id=42&care_offering_ids=",
    );
  });

  it("omits optional filters that are unset or falsy", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: report() });
    });

    await getCareUsageReport({
      phase_id: "42",
      status: undefined,
      care_offering_ids: undefined,
      day_count: undefined,
      grade_level: 0,
      search: "   ",
    });

    expect(seenURL).toBe(
      "/api/enrollment/admin/reports/care-usage?phase_id=42",
    );
  });

  it("returns raw report payloads when the backend does not wrap data", async () => {
    const expected = report({ rows: [] });
    mockFetch(async () => jsonResponse(expected));

    await expect(getCareUsageReport({ phase_id: "42" })).resolves.toStrictEqual(
      expected,
    );
  });

  it("uses the localhost base URL when no browser window exists", async () => {
    let seenURL = "";
    vi.stubGlobal("window", undefined);
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: report() });
    });

    await getCareUsageReport({ phase_id: "42" });

    expect(seenURL).toBe(
      "/api/enrollment/admin/reports/care-usage?phase_id=42",
    );
  });

  it("falls back to the operation message when the backend error is not translatable", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "backend exploded in english" }, { status: 500 }),
    );

    await expect(getCareUsageReport({ phase_id: "42" })).rejects.toMatchObject({
      message: "Auswertung konnte nicht geladen werden (HTTP 500)",
      status: 500,
      rawMessage: "backend exploded in english",
    });
  });

  it("uses translated enrollment validation errors on non-ok responses", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "phase_id is required" }, { status: 400 }),
    );

    await expect(getCareUsageReport({ phase_id: "" })).rejects.toThrow(
      "Bitte wähle eine Anmeldephase aus.",
    );
  });
});

describe("exportCareUsageReport", () => {
  let anchor: {
    href: string;
    download: string;
    click: ReturnType<typeof vi.fn>;
    remove: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    globalThis.fetch = originalFetch;
    anchor = {
      href: "",
      download: "",
      click: vi.fn(),
      remove: vi.fn(),
    };
    vi.spyOn(document, "createElement").mockReturnValue(
      anchor as unknown as HTMLAnchorElement,
    );
    vi.spyOn(document.body, "append").mockImplementation(() => undefined);
    URL.createObjectURL = vi.fn(() => "blob:care-usage-report");
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    URL.createObjectURL = originalCreateObjectURL;
    URL.revokeObjectURL = originalRevokeObjectURL;
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("posts format and filters, then downloads with the filename from content disposition", async () => {
    const headers = new Headers({
      "content-disposition": 'attachment; filename="care-usage.xlsx"',
    });
    const fetchFn = mockFetch(
      async () =>
        ({
          ok: true,
          headers,
          blob: async () => new Blob(["XLSX"]),
        }) as unknown as Response,
    );

    await exportCareUsageReport(
      {
        phase_id: "42",
        status: "approved",
        care_offering_ids: ["7"],
        day_count: 5,
        grade_level: 2,
        search: "Ada",
      },
      "xlsx",
    );

    expect(fetchFn).toHaveBeenCalledTimes(1);
    const [url, init] = fetchFn.mock.calls[0]!;
    expect(url).toBe("/api/enrollment/admin/reports/care-usage/export");
    expect(init).toMatchObject({
      method: "POST",
      headers: { "Content-Type": "application/json" },
    });
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      format: "xlsx",
      filters: {
        phase_id: "42",
        status: "approved",
        care_offering_ids: ["7"],
        day_count: 5,
        grade_level: 2,
        search: "Ada",
      },
    });
    expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
    expect(anchor.href).toBe("blob:care-usage-report");
    expect(anchor.download).toBe("care-usage.xlsx");
    expect(document.body.append).toHaveBeenCalledWith(anchor);
    expect(anchor.click).toHaveBeenCalledTimes(1);
    expect(anchor.remove).toHaveBeenCalledTimes(1);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:care-usage-report");
  });

  it("sends the compact layout in the export body", async () => {
    const fetchFn = mockFetch(
      async () =>
        ({
          ok: true,
          headers: new Headers(),
          blob: async () => new Blob(["PDF"]),
        }) as unknown as Response,
    );

    await exportCareUsageReport(
      { phase_id: "42", pickup_time: "14:30" },
      "pdf",
      "compact",
    );

    const [, init] = fetchFn.mock.calls[0]!;
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      format: "pdf",
      layout: "compact",
      filters: {
        phase_id: "42",
        pickup_time: "14:30",
      },
    });
  });

  it("keeps an explicit empty care offering filter in the export body", async () => {
    const fetchFn = mockFetch(
      async () =>
        ({
          ok: true,
          headers: new Headers(),
          blob: async () => new Blob(["XLSX"]),
        }) as unknown as Response,
    );

    await exportCareUsageReport(
      {
        phase_id: "42",
        care_offering_ids: [],
      },
      "xlsx",
    );

    const [, init] = fetchFn.mock.calls[0]!;
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      format: "xlsx",
      filters: {
        phase_id: "42",
        care_offering_ids: [],
      },
    });
  });

  it("uses the format-specific fallback filename when no quoted filename is present", async () => {
    const headers = new Headers({
      "content-disposition": "attachment; filename=care-usage.pdf",
    });
    mockFetch(
      async () =>
        ({
          ok: true,
          headers,
          blob: async () => new Blob(["PDF"]),
        }) as unknown as Response,
    );

    await exportCareUsageReport({ phase_id: "42" }, "pdf");

    expect(anchor.download).toBe("anmelde-auswertung.pdf");
  });

  it("supports docx exports and falls back to the docx filename", async () => {
    mockFetch(
      async () =>
        ({
          ok: true,
          headers: new Headers(),
          blob: async () => new Blob(["DOCX"]),
        }) as unknown as Response,
    );

    await exportCareUsageReport({ phase_id: "42", search: undefined }, "docx");

    expect(anchor.download).toBe("anmelde-auswertung.docx");
  });

  it("throws translated backend errors and skips the browser download", async () => {
    mockFetch(async () =>
      jsonResponse(
        { code: "enrollment.window_closed", error: "window closed" },
        { status: 403 },
      ),
    );

    await expect(
      exportCareUsageReport({ phase_id: "42" }, "xlsx"),
    ).rejects.toMatchObject({
      message:
        "Die Anmeldefrist für diese Anmeldephase ist abgelaufen oder noch nicht geöffnet. Bitte wende dich an die Schule.",
      status: 403,
      code: "enrollment.window_closed",
      rawMessage: "window closed",
    });
    expect(anchor.click).not.toHaveBeenCalled();
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });
});

describe("exportPhaseClassRoster", () => {
  let anchor: {
    href: string;
    download: string;
    click: ReturnType<typeof vi.fn>;
    remove: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    globalThis.fetch = originalFetch;
    anchor = {
      href: "",
      download: "",
      click: vi.fn(),
      remove: vi.fn(),
    };
    vi.spyOn(document, "createElement").mockReturnValue(
      anchor as unknown as HTMLAnchorElement,
    );
    vi.spyOn(document.body, "append").mockImplementation(() => undefined);
    URL.createObjectURL = vi.fn(() => "blob:class-roster");
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    URL.createObjectURL = originalCreateObjectURL;
    URL.revokeObjectURL = originalRevokeObjectURL;
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("posts phase and class filters, then downloads the roster", async () => {
    const fetchFn = mockFetch(
      async () =>
        ({
          ok: true,
          headers: new Headers(),
          blob: async () => new Blob(["PDF"]),
        }) as unknown as Response,
    );

    await exportPhaseClassRoster("42", "1a", "pdf");

    expect(fetchFn).toHaveBeenCalledTimes(1);
    const [url, init] = fetchFn.mock.calls[0]!;
    expect(url).toBe("/api/enrollment/admin/reports/class-roster/export");
    expect(init).toMatchObject({
      method: "POST",
      headers: { "Content-Type": "application/json" },
    });
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      format: "pdf",
      filters: {
        phase_id: "42",
        school_class: "1a",
      },
    });
    expect(anchor.href).toBe("blob:class-roster");
    expect(anchor.download).toBe("klassenliste.pdf");
    expect(anchor.click).toHaveBeenCalledTimes(1);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:class-roster");
  });

  it("posts all_classes when no class is selected", async () => {
    const fetchFn = mockFetch(
      async () =>
        ({
          ok: true,
          headers: new Headers(),
          blob: async () => new Blob(["PDF"]),
        }) as unknown as Response,
    );

    await exportPhaseClassRoster("42", null, "pdf");

    expect(fetchFn).toHaveBeenCalledTimes(1);
    const [, init] = fetchFn.mock.calls[0]!;
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      format: "pdf",
      filters: {
        phase_id: "42",
        all_classes: true,
      },
    });
    expect(anchor.download).toBe("klassenlisten.pdf");
    expect(anchor.click).toHaveBeenCalledTimes(1);
  });
});
