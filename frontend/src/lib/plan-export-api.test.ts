import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  exportPlan,
  PLAN_EXPORT_TEMPLATES,
  PLAN_EXPORT_VARIANTS,
  type PlanExportRequest,
} from "./plan-export-api";

const originalFetch = globalThis.fetch;
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

const request: PlanExportRequest = {
  from: "2026-07-27",
  to: "2026-07-31",
  template: "persons",
  variant: "aushang",
};

function fileResponse(filename = "dienstplan-2026-07-27.pdf") {
  return new Response(new Blob(["%PDF"]), {
    status: 200,
    headers: {
      "content-type": "application/pdf",
      "content-disposition": `attachment; filename="${filename}"`,
    },
  });
}

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => "blob:plan-export");
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  URL.createObjectURL = originalCreateObjectURL;
  URL.revokeObjectURL = originalRevokeObjectURL;
  vi.restoreAllMocks();
});

describe("plan export metadata", () => {
  it("offers both row axes for the staff plan and one for the care plan", () => {
    expect(PLAN_EXPORT_TEMPLATES.dienstplan.map((item) => item.id)).toEqual([
      "persons",
      "areas",
    ]);
    expect(PLAN_EXPORT_TEMPLATES.betreuungsplan.map((item) => item.id)).toEqual(
      ["offerings"],
    );
  });

  it("defaults to the wall sheet, which is the variant without reasons", () => {
    expect(PLAN_EXPORT_VARIANTS[0]?.id).toBe("aushang");
  });
});

describe("exportPlan", () => {
  it("posts the request to the plan's own route", async () => {
    const fetchMock = vi.fn().mockResolvedValue(fileResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await exportPlan("dienstplan", request, "pdf", "download");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/staff-shifts/export",
      expect.objectContaining({ method: "POST" }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as { body: string } | undefined;
    const body = JSON.parse(init?.body ?? "{}") as Record<string, string>;
    expect(body).toEqual({ ...request, format: "pdf" });
  });

  it("routes the care plan to its own endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(fileResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await exportPlan(
      "betreuungsplan",
      { ...request, template: "offerings" },
      "xlsx",
      "download",
    );

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/timetable/betreuungsplan/export",
    );
  });

  // The proxy route wraps the backend body in {"error": "<json>"}; without
  // unwrapping, a useful limit ("range exceeds 8 weeks") reaches the user as
  // a blob of JSON.
  it("surfaces the backend message from the wrapped error body", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          error: JSON.stringify({ error: "range exceeds 8 weeks" }),
        }),
        { status: 400, headers: { "content-type": "application/json" } },
      ),
    ) as unknown as typeof fetch;

    await expect(
      exportPlan("dienstplan", request, "pdf", "download"),
    ).rejects.toThrow("range exceeds 8 weeks");
  });

  // The tab has to be opened by the caller inside the click handler; opening
  // it after the fetch is what popup blockers refuse.
  it("refuses to print without a caller-opened tab", async () => {
    const fetchMock = vi.fn();
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    await expect(
      exportPlan("dienstplan", request, "pdf", "print", null),
    ).rejects.toThrow(/Druckdialog/);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("closes the print tab when the export fails", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(
        new Response("nope", { status: 500 }),
      ) as unknown as typeof fetch;
    const close = vi.fn();
    const target = { close, focus: vi.fn(), print: vi.fn(), location: {} };

    await expect(
      exportPlan(
        "dienstplan",
        request,
        "pdf",
        "print",
        target as unknown as Window,
      ),
    ).rejects.toThrow();
    expect(close).toHaveBeenCalled();
  });

  it("prints into the caller's tab instead of downloading", async () => {
    vi.useFakeTimers();
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(fileResponse()) as unknown as typeof fetch;
    const target = {
      close: vi.fn(),
      focus: vi.fn(),
      print: vi.fn(),
      location: { href: "" },
    };
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    await exportPlan(
      "dienstplan",
      request,
      "pdf",
      "print",
      target as unknown as Window,
    );
    await vi.runAllTimersAsync();

    expect(target.print).toHaveBeenCalledOnce();
    expect(click).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  // Without a Content-Disposition the file still needs a name a user can find
  // again — never the browser's "download".
  it("names the file itself when the backend sends no filename", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(
        new Response(new Blob(["%PDF"]), { status: 200 }),
      ) as unknown as typeof fetch;
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    await exportPlan("dienstplan", request, "pdf", "download");

    const link = click.mock.instances[0] as HTMLAnchorElement;
    expect(link.download).toBe("dienstplan-2026-07-27.pdf");
  });

  it("falls back to a readable message for an unhelpful error body", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(
        new Response("<html>502</html>", { status: 502 }),
      ) as unknown as typeof fetch;

    await expect(
      exportPlan("dienstplan", request, "pdf", "download"),
    ).rejects.toThrow("Der Plan konnte nicht erstellt werden.");
  });

  it("uses the message field and a non-JSON inner body verbatim", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ message: "Keine Berechtigung." }), {
        status: 403,
        headers: { "content-type": "application/json" },
      }),
    ) as unknown as typeof fetch;

    await expect(
      exportPlan("dienstplan", request, "pdf", "download"),
    ).rejects.toThrow("Keine Berechtigung.");
  });
});
