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
  previewOfferingChangeRequest,
} from "./offering-request-review-api";

const originalFetch = globalThis.fetch;

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

beforeEach(() => {
  loggerError.mockReset();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
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

  it("throws the backend preview error", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          error: "Diese Übersteuerung ist nicht mehr möglich",
          code: "offering_change_invalid",
        },
        { status: 422 },
      ),
    );

    await expect(
      previewOfferingChangeRequest("77", ["9"]),
    ).rejects.toMatchObject({
      name: "OfferingRequestApiError",
      message: "Diese Übersteuerung ist nicht mehr möglich",
      code: "offering_change_invalid",
    });
  });
});
