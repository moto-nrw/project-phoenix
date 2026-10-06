import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { releaseFakeTimers } from "~/test/clock";

import {
  exportSlotList,
  fetchSlotListOptions,
  fetchSlotListPreview,
  SlotListExportError,
  SlotListExportSupersededError,
  type SlotListRequest,
} from "./slot-lists-api";

// The export round trip is itself a window in which the user can change the
// date or the filters. What is pinned here is the handoff at the end of it:
// the file only leaves the building for the selection it was built for.

const originalFetch = globalThis.fetch;
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

const request: SlotListRequest = {
  date: "2026-07-27",
  target: "pickup_cohort",
  pickup_cohort: "long_day",
  source: "planned",
};

function fileResponse(disposition?: string) {
  const headers = new Headers({ "content-type": "application/pdf" });
  if (disposition) headers.set("content-disposition", disposition);
  return new Response(new Blob(["%PDF"]), { status: 200, headers });
}

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => "blob:slot-list");
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  URL.createObjectURL = originalCreateObjectURL;
  URL.revokeObjectURL = originalRevokeObjectURL;
  releaseFakeTimers();
  vi.restoreAllMocks();
});

describe("exportSlotList", () => {
  it("saves the file under the name the backend chose", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(
        fileResponse('attachment; filename="tagesliste.pdf"'),
      ) as unknown as typeof fetch;
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    await exportSlotList(request, "pdf", "download");

    const link = click.mock.instances[0] as HTMLAnchorElement;
    expect(link.download).toBe("tagesliste.pdf");
  });

  it("names the file from the selection when the backend sends none", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(fileResponse()) as unknown as typeof fetch;
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    await exportSlotList(request, "xlsx", "download");

    const link = click.mock.instances[0] as HTMLAnchorElement;
    expect(link.download).toBe("tagesliste-plan-ganztag-1600-2026-07-27.xlsx");
  });

  it("prints into the tab the caller opened during the click", async () => {
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

    await exportSlotList(request, "pdf", "print", target as unknown as Window);
    await vi.runAllTimersAsync();

    expect(target.print).toHaveBeenCalledOnce();
    expect(target.close).not.toHaveBeenCalled();
  });

  // The last gate: if the live selection moved on while the export ran, the
  // built file is stale and must not be handed out.
  it("aborts the handoff and closes the tab when the selection moved on", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(fileResponse()) as unknown as typeof fetch;
    const target = {
      close: vi.fn(),
      focus: vi.fn(),
      print: vi.fn(),
      location: { href: "" },
    };

    await expect(
      exportSlotList(
        request,
        "pdf",
        "print",
        target as unknown as Window,
        "signature-1",
        () => false,
      ),
    ).rejects.toBeInstanceOf(SlotListExportSupersededError);
    expect(target.print).not.toHaveBeenCalled();
    expect(target.close).toHaveBeenCalled();
  });
});

// Failures carry code, status and request ID from the error envelope (#2516);
// the page shows catalog text for them and never the server's sentence.
function errorResponse(status: number, body: Record<string, unknown>) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

async function rejectionOf(promise: Promise<unknown>): Promise<unknown> {
  try {
    await promise;
  } catch (error) {
    return error;
  }
  throw new Error("expected the call to reject");
}

describe("slot list errors", () => {
  it("keeps the envelope's code, status and request ID for the options", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      errorResponse(403, {
        status: "error",
        error: "Keine Berechtigung",
        code: "general.permission",
        instance: "req-options",
      }),
    ) as unknown as typeof fetch;

    const error = await rejectionOf(fetchSlotListOptions("2026-07-27"));

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 403,
      code: "general.permission",
      requestId: "req-options",
    });
  });

  it("classifies a preview refusal without a code by its status", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      errorResponse(400, {
        status: "error",
        error: "Ganztag-Listen sind nur für heute und künftige Tage verfügbar",
      }),
    ) as unknown as typeof fetch;

    const error = await rejectionOf(fetchSlotListPreview(request));

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 400, code: "general.input" });
  });

  it("turns a failed connection into general.unavailable", async () => {
    globalThis.fetch = vi
      .fn()
      .mockRejectedValue(
        new TypeError("Failed to fetch"),
      ) as unknown as typeof fetch;

    const options = await rejectionOf(fetchSlotListOptions("2026-07-27"));
    const preview = await rejectionOf(fetchSlotListPreview(request));

    for (const error of [options, preview]) {
      expect(error).toBeInstanceOf(ApiError);
      expect(error).toMatchObject({ code: "general.unavailable" });
    }
  });

  // The page branches on the 409 drift refusal by status, so the export error
  // must keep it next to the shared ApiError fields.
  it("rejects a drifted export as SlotListExportError with status 409", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      errorResponse(409, {
        status: "error",
        error: "Die Liste hat sich seit der Vorschau geändert.",
        instance: "req-export",
      }),
    ) as unknown as typeof fetch;
    const target = {
      close: vi.fn(),
      focus: vi.fn(),
      print: vi.fn(),
      location: { href: "" },
    };

    const error = await rejectionOf(
      exportSlotList(request, "pdf", "print", target as unknown as Window),
    );

    expect(error).toBeInstanceOf(SlotListExportError);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 409,
      code: "general.business_rejection",
      requestId: "req-export",
    });
    expect(target.print).not.toHaveBeenCalled();
    expect(target.close).toHaveBeenCalled();
  });

  it("closes the print tab and reports general.unavailable when the export cannot connect", async () => {
    globalThis.fetch = vi
      .fn()
      .mockRejectedValue(
        new TypeError("Failed to fetch"),
      ) as unknown as typeof fetch;
    const target = {
      close: vi.fn(),
      focus: vi.fn(),
      print: vi.fn(),
      location: { href: "" },
    };

    const error = await rejectionOf(
      exportSlotList(request, "pdf", "print", target as unknown as Window),
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).not.toBeInstanceOf(SlotListExportError);
    expect(error).toMatchObject({ code: "general.unavailable" });
    expect(target.close).toHaveBeenCalled();
  });
});
