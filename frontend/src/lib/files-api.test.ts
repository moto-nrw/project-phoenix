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
import { ApiError } from "./api-error";
import { filesService } from "./files-api";

const mockedGetSession = vi.mocked(getCachedSession);

// #2517: the client keeps the error identity (code, status, request ID); the
// screen turns it into the catalog text. No wording is decided here.
describe("files-api errors", () => {
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
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });

  const folderInput = {
    name: "Elternbriefe",
    visibility: "all_staff" as const,
    roleIds: [],
    accountIds: [],
  };

  it("keeps the code and request ID of a duplicate folder name", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(409, {
        error: "folder name already exists",
        code: "files.folder_name_taken",
        instance: "req-7",
      }),
    );

    const error: unknown = await filesService
      .createFolder(folderInput)
      .catch((err: unknown) => err);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 409,
      code: "files.folder_name_taken",
      requestId: "req-7",
    });
  });

  it("keeps the quota code of a rejected upload", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(409, {
        error: "file storage quota exceeded",
        code: "files.quota_exceeded",
      }),
    );

    await expect(
      filesService.upload("3", new File(["x"], "Brief.pdf")),
    ).rejects.toMatchObject({ status: 409, code: "files.quota_exceeded" });
  });

  it("classifies a missing permission by its status", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(403, { error: "file storage action not permitted" }),
    );

    await expect(filesService.deleteFolder("3")).rejects.toMatchObject({
      status: 403,
      code: "general.permission",
    });
  });

  it("keeps the rejected field of an invalid payload", async () => {
    mockFetch().mockResolvedValue(
      errorResponse(400, {
        error: "invalid file storage request: name is required",
        code: "general.input",
        errors: [{ field: "name", reason: "required" }],
      }),
    );

    await expect(
      filesService.createFolder({ ...folderInput, name: "" }),
    ).rejects.toMatchObject({
      code: "general.input",
      errors: [{ field: "name", reason: "required" }],
    });
  });

  it("turns an upload that never reaches the API into unavailable", async () => {
    mockFetch().mockRejectedValue(new TypeError("Failed to fetch"));

    await expect(
      filesService.upload("3", new File(["x"], "Liste.pdf")),
    ).rejects.toMatchObject({ code: "general.unavailable" });
  });
});
