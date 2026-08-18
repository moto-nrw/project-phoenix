import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { loggerError } = vi.hoisted(() => ({
  loggerError: vi.fn(),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: loggerError,
  }),
}));

import {
  decideOfferingChangeRequest,
  listOfferingChangeRequests,
  OfferingRequestApiError,
  previewOfferingChangeRequest,
  type StaffOfferingRequest,
} from "./offering-request-review-api";

const originalFetch = globalThis.fetch;

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

function requestRow(): StaffOfferingRequest {
  return {
    id: "77",
    student_id: "42",
    student_name: "Lara Beispiel",
    status: "pending",
    effective_from: "2027-02-01",
    diff: [
      { offering_id: "3", label: "AG", old: "nicht gebucht", new: "Montag" },
    ],
    created_at: "2026-07-30T12:00:00Z",
  };
}

beforeEach(() => {
  loggerError.mockReset();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("listOfferingChangeRequests", () => {
  it("returns an enveloped queue and sends a non-cached GET", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse({ data: [requestRow()] }));
    globalThis.fetch = fetchMock;

    await expect(listOfferingChangeRequests()).resolves.toEqual([requestRow()]);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/offering-change-requests",
      {
        method: "GET",
        headers: { "Content-Type": "application/json" },
        cache: "no-store",
      },
    );
  });

  it("accepts a flat response for compatibility", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(jsonResponse([requestRow()]));

    await expect(listOfferingChangeRequests()).resolves.toEqual([requestRow()]);
  });

  it("preserves the backend error message and stable conflict code", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          error: "Das Angebot ist inzwischen voll",
          code: "offering_change_capacity_full",
        },
        { status: 409 },
      ),
    );

    const error = await listOfferingChangeRequests().catch(
      (reason: unknown) => reason,
    );

    expect(error).toBeInstanceOf(OfferingRequestApiError);
    expect(error).toMatchObject({
      message: "Das Angebot ist inzwischen voll",
      code: "offering_change_capacity_full",
    });
    expect(loggerError).toHaveBeenCalledWith(
      "offering_request_review_request_failed",
      {
        status: 409,
        message: "Das Angebot ist inzwischen voll",
        code: "offering_change_capacity_full",
      },
    );
  });

  it("uses the fallback when the error body is not JSON", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(new Response("proxy failure", { status: 502 }));

    await expect(listOfferingChangeRequests()).rejects.toMatchObject({
      name: "OfferingRequestApiError",
      message: "Angebots-Anfragen konnten nicht geladen werden",
      code: undefined,
    });
    expect(loggerError).toHaveBeenCalledWith(
      "offering_request_review_request_failed",
      {
        status: 502,
        message: "Angebots-Anfragen konnten nicht geladen werden",
      },
    );
  });
});

describe("decideOfferingChangeRequest", () => {
  it("posts an encoded request ID and normalizes an absent reason", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    globalThis.fetch = fetchMock;

    await expect(
      decideOfferingChangeRequest("request/77", true),
    ).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/offering-change-requests/request%2F77/decide",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ approve: true, reason: "" }),
      },
    );
  });

  it("throws the backend decision error", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          error: "Kein gültiger Betreuungsvertrag mehr",
          code: "offering_change_no_enrollment",
        },
        { status: 409 },
      ),
    );

    await expect(
      decideOfferingChangeRequest("77", false, "Zu spät"),
    ).rejects.toMatchObject({
      name: "OfferingRequestApiError",
      message: "Kein gültiger Betreuungsvertrag mehr",
      code: "offering_change_no_enrollment",
    });
  });
});

describe("previewOfferingChangeRequest", () => {
  it("posts all current exclusions and returns materialized selections", async () => {
    const preview = {
      selections: [{ offering_id: "11", new: "Mo, Mi" }],
    };
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse({ data: preview }));
    globalThis.fetch = fetchMock;

    await expect(
      previewOfferingChangeRequest("request/77", ["9", "12"]),
    ).resolves.toEqual(preview);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/offering-change-requests/request%2F77/preview",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ excluded_offering_ids: ["9", "12"] }),
      },
    );
  });
});
