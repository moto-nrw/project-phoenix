import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());
const mockGetCachedSession = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
  getCachedSession: mockGetCachedSession,
}));

import { studentDocumentsService } from "./student-documents-api";

const backendDocument = {
  id: 91,
  student_id: 42,
  category: "attest",
  category_label: "Ärztliches Attest",
  filename: "Attest.pdf",
  size_bytes: 20_480,
  content_type: "application/pdf",
  uploaded_at: "2026-08-05T09:30:00Z",
  sensitive: true,
};

const visibleCategories = [
  { value: "attest", label: "Ärztliches Attest", sensitive: true },
  { value: "sonstiges", label: "Sonstiges", sensitive: false },
];

function pdf(): File {
  return new File(["%PDF-1.4"], "Attest.pdf", { type: "application/pdf" });
}

describe("studentDocumentsService.list", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps the backend row to the frontend shape", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        {
          data: {
            student_id: 42,
            documents: [backendDocument],
            visible_categories: visibleCategories,
          },
        },
        { status: 200 },
      ),
    );

    const result = await studentDocumentsService.list("42");

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/students/42/documents");
    // Backend int64 IDs arrive as numbers and have to reach the UI as strings.
    expect(result.documents).toEqual([
      {
        id: "91",
        studentId: "42",
        category: "attest",
        categoryLabel: "Ärztliches Attest",
        filename: "Attest.pdf",
        sizeBytes: 20_480,
        contentType: "application/pdf",
        uploadedAt: "2026-08-05T09:30:00Z",
        sensitive: true,
      },
    ]);
    // The caller's visible categories come straight from the backend: the tab
    // must not decide for itself which categories it may show.
    expect(result.visibleCategories).toEqual(visibleCategories);
  });

  it("returns an empty list when the child has no documents", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { data: { student_id: 42, documents: [], visible_categories: [] } },
        { status: 200 },
      ),
    );

    await expect(studentDocumentsService.list("42")).resolves.toEqual({
      documents: [],
      visibleCategories: [],
    });
  });

  it("throws when the list request fails", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(null, { status: 500, statusText: "Internal Server Error" }),
    );

    await expect(studentDocumentsService.list("42")).rejects.toThrow(
      "Failed to fetch student documents: Internal Server Error",
    );
  });
});

describe("studentDocumentsService.upload", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", fetchMock);
    mockGetCachedSession.mockResolvedValue({ user: { token: "jwt-token" } });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("posts multipart form data with a bearer token and no Content-Type", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 201 }));

    await studentDocumentsService.upload("42", pdf(), "attest");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/students/42/documents");
    expect(init.method).toBe("POST");
    // Setting Content-Type here would clobber the multipart boundary the
    // browser generates for a FormData body.
    expect(init.headers).toEqual({ Authorization: "Bearer jwt-token" });

    const body = init.body as FormData;
    expect(body).toBeInstanceOf(FormData);
    expect((body.get("file") as File).name).toBe("Attest.pdf");
    expect(body.get("category")).toBe("attest");
  });

  it("refuses to upload without a session token", async () => {
    mockGetCachedSession.mockResolvedValueOnce(null);

    await expect(
      studentDocumentsService.upload("42", pdf(), "attest"),
    ).rejects.toThrow("Authentifizierung erforderlich");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("refuses to upload when the session carries no token", async () => {
    mockGetCachedSession.mockResolvedValueOnce({ user: {} });

    await expect(
      studentDocumentsService.upload("42", pdf(), "attest"),
    ).rejects.toThrow("Authentifizierung erforderlich");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("surfaces the backend error message", async () => {
    fetchMock.mockResolvedValueOnce(
      Response.json({ error: "Datei zu groß." }, { status: 400 }),
    );

    await expect(
      studentDocumentsService.upload("42", pdf(), "attest"),
    ).rejects.toThrow("Datei zu groß.");
  });

  it("falls back to the generic message when the body carries no error", async () => {
    fetchMock.mockResolvedValueOnce(Response.json({}, { status: 400 }));

    await expect(
      studentDocumentsService.upload("42", pdf(), "attest"),
    ).rejects.toThrow("Dokument konnte nicht hochgeladen werden.");
  });

  it("falls back to the generic message on a non-JSON body", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("<html>502</html>", { status: 502 }),
    );

    await expect(
      studentDocumentsService.upload("42", pdf(), "attest"),
    ).rejects.toThrow("Dokument konnte nicht hochgeladen werden.");
  });

  it("reports a missing category permission as such, whatever the body says", async () => {
    fetchMock.mockResolvedValueOnce(
      Response.json({ error: "forbidden" }, { status: 403 }),
    );

    // 403 on this route means the caller lacks the category's permission, so
    // the raw backend wording would be less useful than the German hint.
    await expect(
      studentDocumentsService.upload("42", pdf(), "attest"),
    ).rejects.toThrow("Keine Berechtigung für diese Dokument-Kategorie.");
  });
});

describe("studentDocumentsService.delete", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("deletes through the proxy route", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(
      studentDocumentsService.delete("42", "91"),
    ).resolves.toBeUndefined();
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/students/42/documents/91",
      { method: "DELETE" },
    );
  });

  it("surfaces the backend error message", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ error: "Dokument nicht gefunden." }, { status: 404 }),
    );

    await expect(studentDocumentsService.delete("42", "91")).rejects.toThrow(
      "Dokument nicht gefunden.",
    );
  });

  it("falls back to the generic delete message", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("nope", { status: 500 }),
    );

    await expect(studentDocumentsService.delete("42", "91")).rejects.toThrow(
      "Dokument konnte nicht gelöscht werden.",
    );
  });

  it("reports a missing category permission as such", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ error: "forbidden" }, { status: 403 }),
    );

    await expect(studentDocumentsService.delete("42", "91")).rejects.toThrow(
      "Keine Berechtigung für diese Dokument-Kategorie.",
    );
  });
});

describe("studentDocumentsService.downloadUrl", () => {
  it("points at the same-origin proxy route", () => {
    // The bytes never sit behind a static mount — the download is gated by the
    // authenticated handler, which also writes the audit row.
    expect(studentDocumentsService.downloadUrl("42", "91")).toBe(
      "/api/students/42/documents/91/download",
    );
  });
});
