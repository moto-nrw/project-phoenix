import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import {
  listCareScheduleChangeRequests,
  decideCareScheduleChangeRequest,
  CareRequestApiError,
  type StaffCareRequest,
} from "./care-request-review-api";
import type { RequestDiffEntry } from "~/lib/messaging-status";

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

function mkDiff(overrides: Partial<RequestDiffEntry> = {}): RequestDiffEntry {
  return { label: "Montag Ankunft", old: "07:30", new: "08:00", ...overrides };
}

function mkRequest(
  overrides: Partial<StaffCareRequest> = {},
): StaffCareRequest {
  return {
    id: "r1",
    student_id: "42",
    first_name: "Max",
    last_name: "M.",
    status: "pending",
    request_kind: "weekly_schedule",
    diff: [mkDiff()],
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
// listCareScheduleChangeRequests
// ---------------------------------------------------------------------------

describe("listCareScheduleChangeRequests", () => {
  it("GETs the care-schedule-change-requests proxy route with no-store", async () => {
    let seenURL = "";
    let seenInit: RequestInit | undefined;
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenInit = init;
      return jsonResponse({ data: [mkRequest()] });
    });

    const out = await listCareScheduleChangeRequests();

    expect(seenURL).toBe("/api/students/care-schedule-change-requests");
    expect(seenInit?.method).toBe("GET");
    expect(seenInit?.cache).toBe("no-store");
    expect(out).toHaveLength(1);
    expect(out[0]!.id).toBe("r1");
  });

  it("unwraps the {data} envelope", async () => {
    mockFetch(async () =>
      jsonResponse({ data: [mkRequest({ id: "a" }), mkRequest({ id: "b" })] }),
    );
    const out = await listCareScheduleChangeRequests();
    expect(out.map((r) => r.id)).toEqual(["a", "b"]);
  });

  it("returns a bare array when the response is not enveloped", async () => {
    // unwrap falls through to the raw JSON when no `data` key is present.
    mockFetch(async () => jsonResponse([mkRequest({ id: "bare" })]));
    const out = await listCareScheduleChangeRequests();
    expect(out[0]!.id).toBe("bare");
  });

  it("preserves the diff and structured discriminators on each request", async () => {
    mockFetch(async () =>
      jsonResponse({
        data: [
          mkRequest({
            diff: [
              mkDiff({
                label: "Montag Abholart",
                care_kind: "departure_mode",
                weekday: 1,
                old_modes: ["bus"],
                new_mode: "pickup",
              }),
            ],
          }),
        ],
      }),
    );
    const out = await listCareScheduleChangeRequests();
    const entry = out[0]!.diff[0]!;
    expect(entry.care_kind).toBe("departure_mode");
    expect(entry.old_modes).toEqual(["bus"]);
    expect(entry.new_mode).toBe("pickup");
  });

  it("throws CareRequestApiError carrying the backend code on a 409", async () => {
    mockFetch(async () =>
      jsonResponse(
        { error: "Nachrichten sind deaktiviert", code: "messaging_disabled" },
        409,
      ),
    );

    const err = await listCareScheduleChangeRequests().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(CareRequestApiError);
    expect((err as CareRequestApiError).message).toBe(
      "Nachrichten sind deaktiviert",
    );
    expect((err as CareRequestApiError).code).toBe("messaging_disabled");
  });

  it("falls back to the German message with no code when the body is not JSON", async () => {
    mockFetch(async () => new Response("<html>500</html>", { status: 500 }));
    const err = await listCareScheduleChangeRequests().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(CareRequestApiError);
    expect((err as CareRequestApiError).message).toBe(
      "Betreuungszeit-Anfragen konnten nicht geladen werden",
    );
    expect((err as CareRequestApiError).code).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// decideCareScheduleChangeRequest
// ---------------------------------------------------------------------------

describe("decideCareScheduleChangeRequest", () => {
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

    const out = await decideCareScheduleChangeRequest("r1", true);

    expect(seenMethod).toBe("POST");
    expect(seenURL).toBe(
      "/api/students/care-schedule-change-requests/r1/decide",
    );
    expect(JSON.parse(seenBody)).toEqual({ approve: true, reason: "" });
    expect(out.status).toBe("approved");
  });

  it("forwards the rejection reason verbatim", async () => {
    let seenBody = "";
    mockFetch(async (_input, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse({ data: mkRequest({ status: "rejected" }) });
    });

    await decideCareScheduleChangeRequest("r1", false, "Zeiten kollidieren");

    expect(JSON.parse(seenBody)).toEqual({
      approve: false,
      reason: "Zeiten kollidieren",
    });
  });

  it("URL-encodes the request id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkRequest() });
    });
    await decideCareScheduleChangeRequest("a/b 1", true);
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

    const err = await decideCareScheduleChangeRequest("r1", true).catch(
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(CareRequestApiError);
    expect((err as CareRequestApiError).code).toBe(
      "change_request_not_pending",
    );
  });

  it("falls back to the German decision message on a non-JSON error", async () => {
    mockFetch(async () => new Response("", { status: 500 }));
    const err = await decideCareScheduleChangeRequest("r1", false).catch(
      (e: unknown) => e,
    );
    expect((err as CareRequestApiError).message).toBe(
      "Entscheidung konnte nicht gespeichert werden",
    );
  });
});
