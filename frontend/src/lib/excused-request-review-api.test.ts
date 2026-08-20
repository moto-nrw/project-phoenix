import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import {
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
    absence_status: "excused",
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
});
