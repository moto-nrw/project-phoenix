import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { exportEmergencySnapshot } from "./emergency-export-api";

const originalFetch = globalThis.fetch;
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;
const originalOpen = window.open;

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => "blob:emergency-export-url");
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  URL.createObjectURL = originalCreateObjectURL;
  URL.revokeObjectURL = originalRevokeObjectURL;
  window.open = originalOpen;
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("exportEmergencySnapshot", () => {
  it("downloads the emergency snapshot PDF", async () => {
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
          "content-disposition": 'attachment; filename="notfall.pdf"',
        },
      });
    });

    await exportEmergencySnapshot("download");

    expect(globalThis.fetch).toHaveBeenCalledWith(
      "/api/emergency/snapshot/export",
      { method: "POST" },
    );
    expect(createElement).toHaveBeenCalledWith("a");
    expect(append).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
    expect(remove).toHaveBeenCalled();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith(
      "blob:emergency-export-url",
    );
  });

  it("opens and prints the emergency snapshot PDF", async () => {
    vi.useFakeTimers();
    const print = vi.fn();
    const focus = vi.fn();
    window.open = vi.fn(
      () => ({ print, focus }) as unknown as Window,
    ) as typeof window.open;
    globalThis.fetch = vi.fn(async () => new Response(new Blob(["pdf"])));

    await exportEmergencySnapshot("print");
    await vi.runAllTimersAsync();

    expect(window.open).toHaveBeenCalledWith(
      "blob:emergency-export-url",
      "_blank",
    );
    expect(focus).toHaveBeenCalled();
    expect(print).toHaveBeenCalled();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith(
      "blob:emergency-export-url",
    );
  });

  // Without a Content-Disposition the snapshot still needs a name a user can
  // find again in their downloads folder.
  it("names the file itself when the backend sends no filename", async () => {
    const link = { click: vi.fn(), remove: vi.fn() } as unknown as {
      click: () => void;
      remove: () => void;
      download?: string;
    };
    vi.spyOn(document.body, "append").mockImplementation(
      () => undefined as unknown as void,
    );
    vi.spyOn(document, "createElement").mockReturnValue(
      link as unknown as HTMLAnchorElement,
    );
    globalThis.fetch = vi.fn(async () => new Response(new Blob(["pdf"])));

    await exportEmergencySnapshot("download");

    expect(link.download).toBe("notfallliste.pdf");
  });

  it("throws the backend error body when export fails", async () => {
    globalThis.fetch = vi.fn(async () => {
      return new Response("nicht erlaubt", { status: 403 });
    });

    await expect(exportEmergencySnapshot("download")).rejects.toThrow(
      "nicht erlaubt",
    );
  });
});
