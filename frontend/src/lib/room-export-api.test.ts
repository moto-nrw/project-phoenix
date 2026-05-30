import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  exportRoomSnapshot,
  type RoomSnapshotExportRequest,
} from "./room-export-api";

const originalFetch = globalThis.fetch;
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

const request: RoomSnapshotExportRequest = {
  format: "pdf",
  title: "Wer ist wo",
  room_ids: [1, 4, 10],
  include_transit: true,
};

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => "blob:room-export-url");
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  URL.createObjectURL = originalCreateObjectURL;
  URL.revokeObjectURL = originalRevokeObjectURL;
  vi.restoreAllMocks();
});

describe("exportRoomSnapshot", () => {
  it("posts the room snapshot request and downloads the returned blob", async () => {
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
          "content-disposition": 'attachment; filename="wer-ist-wo.pdf"',
        },
      });
    });

    await exportRoomSnapshot(request);

    expect(globalThis.fetch).toHaveBeenCalledWith("/api/rooms/export", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });
    expect(createElement).toHaveBeenCalledWith("a");
    expect(append).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
    expect(remove).toHaveBeenCalled();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:room-export-url");
  });

  it("uses a fallback filename when no content disposition is present", async () => {
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
    globalThis.fetch = vi.fn(async () => new Response(new Blob(["xlsx"])));

    await exportRoomSnapshot({ ...request, format: "xlsx" });

    expect(download).toBe("wer-ist-wo.xlsx");
  });

  it("throws the backend error body when the export fails", async () => {
    globalThis.fetch = vi.fn(async () => {
      return new Response("keine Räume", { status: 400 });
    });

    await expect(exportRoomSnapshot(request)).rejects.toThrow("keine Räume");
  });
});
