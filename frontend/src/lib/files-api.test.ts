import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { mockSessionData } from "~/test/mocks/next-auth";

// Mock session-cache before importing the module under test.
vi.mock("./session-cache", () => {
  const getCachedSession = vi.fn();
  return {
    getCachedSession,
    clearSessionCache: vi.fn(),
    sessionFetch: vi.fn(async (url: string, init?: RequestInit) => {
      const session = (await getCachedSession()) as {
        user?: { token?: string };
      } | null;
      const token = session?.user?.token;
      if (!token) throw new Error("No authentication token available");
      return fetch(url, {
        ...init,
        headers: {
          "Content-Type": "application/json",
          ...(init?.headers as Record<string, string> | undefined),
          ...{ Authorization: `Bearer ${token}` },
        },
      });
    }),
  };
});

import { getCachedSession } from "./session-cache";
import { filesService } from "./files-api";

const mockedGetSession = vi.mocked(getCachedSession);

describe("files-api error wording", () => {
  let originalFetch: typeof fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn();
    mockedGetSession.mockResolvedValue(mockSessionData());
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  const mockFetch = () => globalThis.fetch as ReturnType<typeof vi.fn>;

  const errorResponse = (status: number, body: unknown) =>
    ({
      ok: false,
      status,
      json: () => Promise.resolve(body),
    }) as unknown as Response;

  it("never shows the backend sentinel text of a rejected payload", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(400, {
        error: "invalid file storage request: name is required",
      }),
    );

    await expect(
      filesService.createFolder({
        name: "",
        visibility: "all_staff",
        roleIds: [],
        accountIds: [],
      }),
    ).rejects.toThrow("Ordner konnte nicht angelegt werden.");
  });

  it("names the duplicate folder by its error code", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(409, {
        error: "folder name already exists",
        code: "folder_name_taken",
      }),
    );

    await expect(
      filesService.createFolder({
        name: "Elternbriefe",
        visibility: "all_staff",
        roleIds: [],
        accountIds: [],
      }),
    ).rejects.toThrow("Es gibt schon einen Ordner mit diesem Namen.");
  });

  it("explains a full quota instead of repeating the backend wording", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(409, {
        error: "file storage quota exceeded",
        code: "quota_exceeded",
      }),
    );

    await expect(
      filesService.upload("3", new File(["x"], "Brief.pdf")),
    ).rejects.toThrow(
      "Der Speicherplatz der Dateiablage ist voll. Bitte erst Dateien löschen.",
    );
  });

  it("turns a missing permission into one German sentence", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(403, { error: "file storage action not permitted" }),
    );

    await expect(filesService.deleteFolder("3")).rejects.toThrow(
      "Dafür fehlt Ihnen die Berechtigung.",
    );
  });

  it("tells the user which files an upload accepts", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(400, {
        error:
          "Diese Datei ist nicht erlaubt. Erlaubt sind PDF, DOCX, XLSX, PPTX, PNG und JPEG.",
      }),
    );

    await expect(
      filesService.upload("3", new File(["x"], "Liste.zip")),
    ).rejects.toThrow(
      "Die Datei konnte nicht hochgeladen werden. Erlaubt sind PDF, DOCX, XLSX, PPTX, PNG und JPEG bis 25 MB.",
    );
  });
});
