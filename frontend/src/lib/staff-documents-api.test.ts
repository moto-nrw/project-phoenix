import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { mockSessionData } from "~/test/mocks/next-auth";

// Mock session-cache before importing the module under test. sessionFetch is
// reproduced faithfully enough to assert the headers the client relies on.
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
import {
  formatFileSize,
  staffDocumentCategoryLabels,
  staffDocumentsService,
  type StaffDocumentCategory,
} from "./staff-documents-api";

const mockedGetSession = vi.mocked(getCachedSession);

function backendDocument(overrides: Record<string, unknown> = {}) {
  return {
    id: 42,
    staff_id: 7,
    category: "arbeitsvertrag" as StaffDocumentCategory,
    filename: "Vertrag.pdf",
    size_bytes: 2048,
    content_type: "application/pdf",
    uploaded_at: "2026-08-01T10:00:00Z",
    retain_until: "2030-12-31",
    review_due: "2027-01-31",
    ...overrides,
  };
}

describe("staff-documents-api", () => {
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

  describe("list", () => {
    it("maps backend documents and returns the visible categories", async () => {
      mockFetch().mockResolvedValue({
        ok: true,
        statusText: "OK",
        json: () =>
          Promise.resolve({
            data: {
              staff_id: 7,
              documents: [backendDocument()],
              visible_categories: ["arbeitsvertrag", "zeugnis"],
            },
          }),
      } as unknown as Response);

      const result = await staffDocumentsService.list("7");

      expect(result).toEqual({
        documents: [
          {
            id: "42",
            staffId: "7",
            category: "arbeitsvertrag",
            filename: "Vertrag.pdf",
            sizeBytes: 2048,
            contentType: "application/pdf",
            uploadedAt: "2026-08-01T10:00:00Z",
            retainUntil: "2030-12-31",
            reviewDue: "2027-01-31",
          },
        ],
        visibleCategories: ["arbeitsvertrag", "zeugnis"],
      });
      expect(mockFetch()).toHaveBeenCalledWith(
        "/api/staff/7/documents",
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer test-token",
          }),
        }),
      );
    });

    it("normalises missing and null optional dates to null", async () => {
      mockFetch().mockResolvedValue({
        ok: true,
        statusText: "OK",
        json: () =>
          Promise.resolve({
            data: {
              staff_id: 7,
              documents: [
                backendDocument({
                  id: 1,
                  retain_until: null,
                  review_due: null,
                }),
                backendDocument({
                  id: 2,
                  retain_until: undefined,
                  review_due: undefined,
                }),
              ],
              visible_categories: [],
            },
          }),
      } as unknown as Response);

      const result = await staffDocumentsService.list("7");

      expect(result.documents.map((d) => d.retainUntil)).toEqual([null, null]);
      expect(result.documents.map((d) => d.reviewDue)).toEqual([null, null]);
      expect(result.visibleCategories).toEqual([]);
    });

    it("throws with the status text when the request fails", async () => {
      mockFetch().mockResolvedValue({
        ok: false,
        statusText: "Internal Server Error",
        json: () => Promise.resolve({}),
      } as unknown as Response);

      await expect(staffDocumentsService.list("7")).rejects.toThrow(
        "Failed to fetch staff documents: Internal Server Error",
      );
    });
  });

  describe("upload", () => {
    const file = new File(["content"], "Lohn.pdf", {
      type: "application/pdf",
    });

    it("posts multipart form data with a bearer token and no JSON content type", async () => {
      mockFetch().mockResolvedValue({ ok: true } as unknown as Response);

      await staffDocumentsService.upload("7", file, "lohnabrechnung");

      expect(mockFetch()).toHaveBeenCalledTimes(1);
      const [url, init] = mockFetch().mock.calls[0] as [string, RequestInit];
      expect(url).toBe("/api/staff/7/documents");
      expect(init.method).toBe("POST");
      expect(init.headers).toEqual({ Authorization: "Bearer test-token" });
      expect(init.headers).not.toHaveProperty("Content-Type");

      const body = init.body as FormData;
      expect(body).toBeInstanceOf(FormData);
      expect(body.get("category")).toBe("lohnabrechnung");
      expect(body.get("file")).toBe(file);
    });

    it("rejects without sending when there is no session at all", async () => {
      mockedGetSession.mockResolvedValue(null);

      await expect(
        staffDocumentsService.upload("7", file, "sonstiges"),
      ).rejects.toThrow("Authentifizierung erforderlich");
      expect(mockFetch()).not.toHaveBeenCalled();
    });

    it("rejects when the session carries no token", async () => {
      mockedGetSession.mockResolvedValue({
        ...mockSessionData(),
        user: { id: "1", name: "Test User" },
      });

      await expect(
        staffDocumentsService.upload("7", file, "sonstiges"),
      ).rejects.toThrow("Authentifizierung erforderlich");
      expect(mockFetch()).not.toHaveBeenCalled();
    });

    it("surfaces the backend error message", async () => {
      mockFetch().mockResolvedValue({
        ok: false,
        status: 400,
        json: () => Promise.resolve({ error: "Dateityp nicht erlaubt." }),
      } as unknown as Response);

      await expect(
        staffDocumentsService.upload("7", file, "sonstiges"),
      ).rejects.toThrow("Dateityp nicht erlaubt.");
    });

    it("keeps the fallback message when the error body has no error field", async () => {
      mockFetch().mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.resolve({}),
      } as unknown as Response);

      await expect(
        staffDocumentsService.upload("7", file, "sonstiges"),
      ).rejects.toThrow("Dokument konnte nicht hochgeladen werden.");
    });

    it("keeps the fallback message when the body is not JSON", async () => {
      mockFetch().mockResolvedValue({
        ok: false,
        status: 502,
        json: () => Promise.reject(new Error("Unexpected token < in JSON")),
      } as unknown as Response);

      await expect(
        staffDocumentsService.upload("7", file, "sonstiges"),
      ).rejects.toThrow("Dokument konnte nicht hochgeladen werden.");
    });

    it("replaces the message with the category hint on 403", async () => {
      mockFetch().mockResolvedValue({
        ok: false,
        status: 403,
        json: () => Promise.resolve({ error: "forbidden" }),
      } as unknown as Response);

      await expect(
        staffDocumentsService.upload("7", file, "lohnabrechnung"),
      ).rejects.toThrow("Keine Berechtigung für diese Dokument-Kategorie.");
    });
  });

  describe("delete", () => {
    it("issues a DELETE against the document route", async () => {
      mockFetch().mockResolvedValue({ ok: true } as unknown as Response);

      await staffDocumentsService.delete("7", "42");

      expect(mockFetch()).toHaveBeenCalledWith(
        "/api/staff/7/documents/42",
        expect.objectContaining({
          method: "DELETE",
          headers: expect.objectContaining({
            Authorization: "Bearer test-token",
          }),
        }),
      );
    });

    it("surfaces the backend error message", async () => {
      mockFetch().mockResolvedValue({
        ok: false,
        status: 409,
        json: () => Promise.resolve({ error: "Dokument bereits entfernt." }),
      } as unknown as Response);

      await expect(staffDocumentsService.delete("7", "42")).rejects.toThrow(
        "Dokument bereits entfernt.",
      );
    });

    it("uses the delete fallback message for non-JSON bodies", async () => {
      mockFetch().mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error("not json")),
      } as unknown as Response);

      await expect(staffDocumentsService.delete("7", "42")).rejects.toThrow(
        "Dokument konnte nicht gelöscht werden.",
      );
    });

    it("reports a missing permission on 403", async () => {
      mockFetch().mockResolvedValue({
        ok: false,
        status: 403,
        json: () => Promise.resolve({}),
      } as unknown as Response);

      await expect(staffDocumentsService.delete("7", "42")).rejects.toThrow(
        "Keine Berechtigung für diese Dokument-Kategorie.",
      );
    });
  });

  describe("downloadUrl", () => {
    it("points at the same-origin proxy route", () => {
      expect(staffDocumentsService.downloadUrl("7", "42")).toBe(
        "/api/staff/7/documents/42/download",
      );
    });
  });

  describe("formatFileSize", () => {
    it.each([
      [0, "0 B"],
      [512, "512 B"],
      [1023, "1023 B"],
      [1024, "1 KB"],
      [1536, "2 KB"],
      [1024 * 1024 - 1, "1024 KB"],
      [1024 * 1024, "1,0 MB"],
      [1_468_006, "1,4 MB"],
      [5 * 1024 * 1024, "5,0 MB"],
    ])("formats %i bytes as %s", (bytes, expected) => {
      expect(formatFileSize(bytes)).toBe(expected);
    });
  });

  describe("staffDocumentCategoryLabels", () => {
    it("labels every category the client accepts", () => {
      expect(Object.keys(staffDocumentCategoryLabels)).toEqual([
        "arbeitsvertrag",
        "zeugnis",
        "lohnabrechnung",
        "bewerbung",
        "au_bescheinigung",
        "sonstiges",
      ]);
      expect(staffDocumentCategoryLabels.au_bescheinigung).toBe(
        "AU-Bescheinigung",
      );
    });
  });
});
