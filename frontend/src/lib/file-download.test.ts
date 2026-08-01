import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  downloadBlob,
  filenameFromDisposition,
  openBlobForPrint,
} from "./file-download";

// The shared plumbing behind every export button. The popup-blocker dance in
// openBlobForPrint is exactly the kind of fix that used to be applied in one
// export client and forgotten in the other three, which is why it lives here
// once and is pinned here once.

const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;
const originalOpen = globalThis.open;

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => "blob:export");
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  URL.createObjectURL = originalCreateObjectURL;
  URL.revokeObjectURL = originalRevokeObjectURL;
  globalThis.open = originalOpen;
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("filenameFromDisposition", () => {
  it("reads the filename the backend chose", () => {
    const response = new Response(null, {
      headers: {
        "content-disposition":
          'attachment; filename="Dienstplan 2026-07-27.pdf"',
      },
    });
    expect(filenameFromDisposition(response)).toBe("Dienstplan 2026-07-27.pdf");
  });

  it("returns null when the header is absent or unquoted", () => {
    expect(filenameFromDisposition(new Response(null))).toBeNull();
    expect(
      filenameFromDisposition(
        new Response(null, {
          headers: { "content-disposition": "attachment" },
        }),
      ),
    ).toBeNull();
  });
});

describe("downloadBlob", () => {
  it("saves under the given name and releases the object URL", () => {
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    downloadBlob(new Blob(["%PDF"]), "Dienstplan.pdf");

    expect(click).toHaveBeenCalledOnce();
    const link = click.mock.instances[0] as HTMLAnchorElement;
    expect(link.download).toBe("Dienstplan.pdf");
    expect(link.href).toContain("blob:export");
    // The anchor is a means, not a leftover — and the URL must not leak.
    expect(document.body.contains(link)).toBe(false);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:export");
  });
});

describe("openBlobForPrint", () => {
  it("prints into the tab the caller opened before the fetch", () => {
    vi.useFakeTimers();
    const target = { focus: vi.fn(), print: vi.fn(), location: { href: "" } };
    const open = vi.fn();
    globalThis.open = open as unknown as typeof globalThis.open;

    openBlobForPrint(new Blob(["%PDF"]), target as unknown as Window);

    // Opening a second tab here is what popup blockers refuse after an await.
    expect(open).not.toHaveBeenCalled();
    expect(target.location.href).toBe("blob:export");
    expect(target.print).not.toHaveBeenCalled();

    vi.runAllTimers();
    expect(target.focus).toHaveBeenCalledOnce();
    expect(target.print).toHaveBeenCalledOnce();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:export");
  });

  it("opens the tab itself when the caller did no async work first", () => {
    vi.useFakeTimers();
    const tab = { focus: vi.fn(), print: vi.fn() };
    globalThis.open = vi
      .fn()
      .mockReturnValue(tab) as unknown as typeof globalThis.open;

    openBlobForPrint(new Blob(["%PDF"]));

    expect(globalThis.open).toHaveBeenCalledWith("blob:export", "_blank");
    vi.runAllTimers();
    expect(tab.print).toHaveBeenCalledOnce();
  });

  // A blocked popup must surface as an error the caller can show, not as a
  // silent no-op that looks like a broken export.
  it("reports a blocked popup and releases the URL", () => {
    globalThis.open = vi
      .fn()
      .mockReturnValue(null) as unknown as typeof globalThis.open;

    expect(() => openBlobForPrint(new Blob(["%PDF"]))).toThrow(/Druckdialog/);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:export");
  });
});
