import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: vi.fn(),
  }),
}));

const mockFetch = vi.fn();

describe("enrollment submit proxy", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetAllMocks();
  });

  it("forwards only the canonical client IP", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "ok" }), {
        status: 201,
        headers: { "content-type": "application/json" },
      }),
    );

    const request = new NextRequest(
      "http://school-a.localhost:3000/api/enrollment/school-a/submit",
      {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "x-forwarded-for": "203.0.113.10, 172.20.0.4",
          "x-real-ip": "198.51.100.25",
        },
        body: JSON.stringify({ child: "Ada" }),
      },
    );

    const response = await POST(request, {
      params: Promise.resolve({ tenantSlug: "school-a" }),
    });

    expect(response.status).toBe(201);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend.test/api/enrollment/school-a/submit",
      expect.objectContaining({
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          "X-Forwarded-For": "172.20.0.4",
        }) as HeadersInit,
      }),
    );
  });
});
