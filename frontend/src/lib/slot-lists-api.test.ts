import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  exportSlotList,
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
  vi.useRealTimers();
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
