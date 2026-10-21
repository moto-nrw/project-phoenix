import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import {
  type ExtendedSession,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

describe("GET /api/time-tracking/export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/export");
    const response = await GET(request);

    expect(response.status).toBe(401);
    const text = await response.text();
    expect(JSON.parse(text)).toMatchObject({ detail: "Unauthorized" });
  });

  it("streams export file from backend with correct headers", async () => {
    const mockStream = new ReadableStream({
      start(controller) {
        controller.enqueue(new TextEncoder().encode("CSV data"));
        controller.close();
      },
    });

    const mockBackendResponse = new Response(mockStream, {
      headers: {
        "Content-Type": "text/csv",
        "Content-Disposition": "attachment; filename=export.csv",
        "Content-Length": "8",
      },
    });

    global.fetch = vi.fn().mockResolvedValue(mockBackendResponse);

    const request = createMockRequest(
      "/api/time-tracking/export?format=csv&from=2024-01-01&to=2024-01-31",
    );
    const response = await GET(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://server:8080/api/time-tracking/export?format=csv&from=2024-01-01&to=2024-01-31",
      {
        headers: {
          Authorization: "Bearer test-token",
        },
        cache: "no-store",
      },
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("text/csv");
    expect(response.headers.get("Content-Disposition")).toBe(
      "attachment; filename=export.csv",
    );
    expect(response.headers.get("Content-Length")).toBe("8");
  });

  it("replaces a backend server error with a safe problem envelope", async () => {
    const mockErrorResponse = new Response("Export failed", { status: 500 });
    global.fetch = vi.fn().mockResolvedValue(mockErrorResponse);

    const request = createMockRequest("/api/time-tracking/export");
    const response = await GET(request);

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
  });

  it("handles missing response body", async () => {
    const mockResponse = new Response(null, { status: 200 });
    global.fetch = vi.fn().mockResolvedValue(mockResponse);

    const request = createMockRequest("/api/time-tracking/export");
    const response = await GET(request);

    expect(response.status).toBe(502);
    const text = await response.text();
    expect(JSON.parse(text)).toMatchObject({
      detail: "Bad Gateway",
    });
  });

  it("handles network errors", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const request = createMockRequest("/api/time-tracking/export");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const text = await response.text();
    expect(JSON.parse(text)).toMatchObject({ detail: "Internal Server Error" });
  });
});
