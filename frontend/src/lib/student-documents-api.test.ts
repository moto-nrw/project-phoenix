import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({ sessionFetch: mockSessionFetch }));

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
  has_file: true,
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

    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/students/42/documents",
      expect.objectContaining({ method: "GET" }),
    );
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
        hasFile: true,
        accidentReport: null,
      },
    ]);
    // The caller's visible categories come straight from the backend: the tab
    // must not decide for itself which categories it may show.
    expect(result.visibleCategories).toEqual(visibleCategories);
  });

  it("maps an accident report with its status and version (#4771)", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        {
          data: {
            student_id: 42,
            documents: [
              {
                ...backendDocument,
                id: 92,
                category: "unfallanzeige",
                category_label: "Unfallanzeige",
                has_file: false,
                accident_report: {
                  status: "signed",
                  version: 2,
                  supersedes_id: "91",
                  signed_at: "2026-08-06T07:00:00Z",
                  signed_digitally: true,
                  occurred_at: "2026-08-05T08:40:00Z",
                  location: "Schulhof",
                  description: "Gestürzt",
                  injured_body_parts: "",
                  injury_kind: "Prellung",
                  witnesses: "",
                  first_aid: "",
                  doctor_visited: false,
                  doctor: "",
                  parents_notified: "",
                  supervisor: "Frau Berger",
                },
              },
            ],
            visible_categories: visibleCategories,
          },
        },
        { status: 200 },
      ),
    );

    const [report] = (await studentDocumentsService.list("42")).documents;

    expect(report?.hasFile).toBe(false);
    expect(report?.accidentReport).toMatchObject({
      status: "signed",
      version: 2,
      supersedesId: "91",
      supersededById: null,
      parentsNotifiedAt: null,
      signedDigitally: true,
    });
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

    await expect(studentDocumentsService.list("42")).rejects.toMatchObject({
      status: 500,
      code: "general.server",
    });
  });
});

describe("studentDocumentsService.upload", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // The route reads the session cookie; the browser holds no token (#5005).
  // Setting Content-Type here would clobber the multipart boundary the
  // browser generates for a FormData body.
  it("posts multipart form data without Authorization or Content-Type", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await studentDocumentsService.upload("42", pdf(), "attest");

    expect(mockSessionFetch).toHaveBeenCalledTimes(1);
    const [url, init] = mockSessionFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/students/42/documents");
    expect(init.method).toBe("POST");
    expect(init.headers).toBeUndefined();

    const body = init.body as FormData;
    expect(body).toBeInstanceOf(FormData);
    expect((body.get("file") as File).name).toBe("Attest.pdf");
    expect(body.get("category")).toBe("attest");
  });

  // ADR 0006: the backend sentence is diagnosis only; the code is what the
  // display path reads.
  it("carries the code and request ID of a refused upload", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { error: "Datei zu groß.", code: "general.input", instance: "req-1" },
        { status: 400 },
      ),
    );

    const error = await studentDocumentsService
      .upload("42", pdf(), "attest")
      .catch((e: unknown) => e);
    expect(error).toMatchObject({ code: "general.input", requestId: "req-1" });
    expect((error as Error).message).not.toContain("Datei zu groß");
  });

  it("classifies a non-JSON body by its status", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("<html>502</html>", { status: 502 }),
    );

    await expect(
      studentDocumentsService.upload("42", pdf(), "attest"),
    ).rejects.toMatchObject({ code: "general.unavailable" });
  });

  it("reports a missing category permission as the permission class", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ error: "forbidden" }, { status: 403 }),
    );

    await expect(
      studentDocumentsService.upload("42", pdf(), "attest"),
    ).rejects.toMatchObject({ status: 403, code: "general.permission" });
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
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("keeps the backend sentence out of the message", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ error: "Dokument nicht gefunden." }, { status: 404 }),
    );

    const error = await studentDocumentsService
      .delete("42", "91")
      .catch((e: unknown) => e);
    expect(error).toMatchObject({ status: 404 });
    expect((error as Error).message).not.toContain("Dokument nicht gefunden");
  });

  it("classifies a failed delete by its status", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("nope", { status: 500 }),
    );

    await expect(
      studentDocumentsService.delete("42", "91"),
    ).rejects.toMatchObject({ status: 500, code: "general.server" });
  });

  it("reports a missing category permission as such", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ error: "forbidden" }, { status: 403 }),
    );

    await expect(
      studentDocumentsService.delete("42", "91"),
    ).rejects.toMatchObject({ status: 403, code: "general.permission" });
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
