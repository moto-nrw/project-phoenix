import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockSessionFetch } = vi.hoisted(() => ({
  mockSessionFetch:
    vi.fn<(url: string, init?: RequestInit) => Promise<Response>>(),
}));

vi.mock("./session-cache", () => ({ sessionFetch: mockSessionFetch }));

import { ApiError } from "./api-error";
import { bffJson } from "./bff-fetch";

beforeEach(() => {
  mockSessionFetch.mockReset();
});

describe("bffJson", () => {
  it("returns the data of the route wrapper's answer", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ success: true, message: "Success", data: { id: "7" } }),
    );

    await expect(bffJson("/api/x")).resolves.toEqual({ id: "7" });
    expect(mockSessionFetch).toHaveBeenCalledWith("/api/x", {
      method: "GET",
      signal: undefined,
    });
  });

  it("sends a JSON body serialized and a FormData body as is", async () => {
    mockSessionFetch
      .mockResolvedValueOnce(Response.json({ success: true, data: null }))
      .mockResolvedValueOnce(Response.json({ success: true, data: null }));
    const form = new FormData();

    await bffJson("/api/x", { method: "PUT", body: { a: 1 } });
    await bffJson("/api/x", { method: "POST", body: form });

    expect(mockSessionFetch.mock.calls[0]?.[1]?.body).toBe('{"a":1}');
    expect(mockSessionFetch.mock.calls[1]?.[1]?.body).toBe(form);
  });

  it("resolves a 204 to undefined", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(
      bffJson("/api/x", { method: "DELETE" }),
    ).resolves.toBeUndefined();
  });

  it("turns a refusal into an ApiError with the envelope's code", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        {
          status: 409,
          detail: "stale",
          code: "students.companions_changed",
          instance: "req-1",
          errors: [{ field: "x", reason: "stale" }],
        },
        { status: 409 },
      ),
    );

    const error = await bffJson("/api/x").catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 409,
      code: "students.companions_changed",
      detail: "stale",
      requestId: "req-1",
      errors: [{ field: "x", reason: "stale" }],
    });
  });

  it("forwards the abort signal", async () => {
    const controller = new AbortController();
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ success: true, data: 1 }),
    );

    await bffJson("/api/x", { signal: controller.signal });

    expect(mockSessionFetch.mock.calls[0]?.[1]?.signal).toBe(controller.signal);
  });
});
