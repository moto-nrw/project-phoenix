import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import {
  listExcusedAbsenceRequestHistory,
  listExcusedAbsenceRequests,
  decideExcusedAbsenceRequest,
  ExcusedRequestApiError,
  type StaffExcusedRequest,
} from "./excused-request-review-api";

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

const originalFetch = globalThis.fetch;

function mockFetch(
  impl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  globalThis.fetch = vi.fn(impl) as typeof globalThis.fetch;
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function mkRequest(
  overrides: Partial<StaffExcusedRequest> = {},
): StaffExcusedRequest {
  return {
    id: "r1",
    student_id: "42",
    first_name: "Max",
    last_name: "M.",
    status: "pending",
    dates: ["2026-07-10", "2026-07-11"],
    note: "Familienfeier",
    created_at: "2026-07-01T10:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  globalThis.fetch = originalFetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

// ---------------------------------------------------------------------------
// listExcusedAbsenceRequests
// ---------------------------------------------------------------------------

describe("listExcusedAbsenceRequests", () => {
  it("GETs the excused-absence-requests proxy route with no-store", async () => {
    let seenURL = "";
    let seenInit: RequestInit | undefined;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenInit = init;
      return jsonResponse({ data: [mkRequest()] });
    });

    const out = await listExcusedAbsenceRequests();

    expect(seenURL).toBe("/api/students/excused-absence-requests");
    expect(seenInit?.method).toBe("GET");
    expect(seenInit?.cache).toBe("no-store");
    expect(out).toHaveLength(1);
    expect(out[0]!.id).toBe("r1");
    expect(out[0]!.note).toBe("Familienfeier");
    expect(out[0]!.dates).toEqual(["2026-07-10", "2026-07-11"]);
  });

  it("unwraps the {data} envelope", async () => {
    mockFetch(async () =>
      jsonResponse({ data: [mkRequest({ id: "a" }), mkRequest({ id: "b" })] }),
    );
    const out = await listExcusedAbsenceRequests();
    expect(out.map((r) => r.id)).toEqual(["a", "b"]);
  });

  it("returns a bare array when the response is not enveloped", async () => {
    // unwrap falls through to the raw JSON when no `data` key is present.
    mockFetch(async () => jsonResponse([mkRequest({ id: "bare" })]));
    const out = await listExcusedAbsenceRequests();
    expect(out[0]!.id).toBe("bare");
  });

  it("throws ExcusedRequestApiError carrying the backend code on a 409", async () => {
    mockFetch(async () =>
      jsonResponse(
        { error: "Nachrichten sind deaktiviert", code: "messaging_disabled" },
        409,
      ),
    );

    const err = await listExcusedAbsenceRequests().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ExcusedRequestApiError);
    expect((err as ExcusedRequestApiError).message).toBe(
      "Nachrichten sind deaktiviert",
    );
    expect((err as ExcusedRequestApiError).code).toBe("messaging_disabled");
  });

  it("falls back to the German message with no code when the body is not JSON", async () => {
    mockFetch(async () => new Response("<html>500</html>", { status: 500 }));
    const err = await listExcusedAbsenceRequests().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ExcusedRequestApiError);
    expect((err as ExcusedRequestApiError).message).toBe(
      "Entschuldigungs-Anfragen konnten nicht geladen werden",
    );
    expect((err as ExcusedRequestApiError).code).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// decideExcusedAbsenceRequest
// ---------------------------------------------------------------------------

describe("decideExcusedAbsenceRequest", () => {
  it("POSTs approve=true with an empty reason string by default", async () => {
    let seenURL = "";
    let seenBody = "";
    let seenMethod = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      seenMethod = init?.method ?? "";
      return jsonResponse({ data: mkRequest({ status: "approved" }) });
    });

    const out = await decideExcusedAbsenceRequest("r1", true);

    expect(seenMethod).toBe("POST");
    expect(seenURL).toBe("/api/students/excused-absence-requests/r1/decide");
    expect(JSON.parse(seenBody)).toEqual({ approve: true, reason: "" });
    expect(out.status).toBe("approved");
  });

  it("forwards the rejection reason verbatim", async () => {
    let seenBody = "";
    mockFetch(async (_input, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse({ data: mkRequest({ status: "rejected" }) });
    });

    await decideExcusedAbsenceRequest("r1", false, "Bitte telefonisch klären");

    expect(JSON.parse(seenBody)).toEqual({
      approve: false,
      reason: "Bitte telefonisch klären",
    });
  });

  it("URL-encodes the request id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkRequest() });
    });
    await decideExcusedAbsenceRequest("a/b 1", true);
    expect(seenURL).toContain("a%2Fb%201");
  });

  it("surfaces the backend code so the UI can name the recovery action", async () => {
    mockFetch(async () =>
      jsonResponse(
        {
          error: "Die Anfrage ist nicht mehr offen",
          code: "change_request_not_pending",
        },
        409,
      ),
    );

    const err = await decideExcusedAbsenceRequest("r1", true).catch(
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(ExcusedRequestApiError);
    expect((err as ExcusedRequestApiError).code).toBe(
      "change_request_not_pending",
    );
  });

  it("falls back to the German decision message on a non-JSON error", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    const err = await decideExcusedAbsenceRequest("r1", false).catch(
      (e: unknown) => e,
    );
    expect((err as ExcusedRequestApiError).message).toBe(
      "Entscheidung konnte nicht gespeichert werden",
    );
  });

  it("lädt die Historie mit URL-kodiertem Cursor", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({
        data: { items: [], next_cursor: "abc" },
      });
    });

    const out = await listExcusedAbsenceRequestHistory("cur+1");

    expect(seenURL).toBe(
      "/api/students/excused-absence-requests/history?cursor=cur%2B1",
    );
    expect(out.next_cursor).toBe("abc");
  });
});
