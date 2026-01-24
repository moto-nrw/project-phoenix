import { describe, it, expect, beforeEach, vi } from "vitest";
import { GET } from "./route";

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/health", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2025-01-15T10:00:00.000Z"));
  });

  it("returns 200 OK status", async () => {
    const response = await GET();

    expect(response.status).toBe(200);
  });

  it("returns correct JSON structure", async () => {
    const response = await GET();
    const json = (await response.json()) as {
      status: string;
      service: string;
      timestamp: string;
    };

    expect(json).toEqual({
      status: "ok",
      service: "phoenix-frontend",
      timestamp: "2025-01-15T10:00:00.000Z",
    });
  });

  it("returns status 'ok'", async () => {
    const response = await GET();
    const json = (await response.json()) as { status: string };

    expect(json.status).toBe("ok");
  });

  it("returns service name 'phoenix-frontend'", async () => {
    const response = await GET();
    const json = (await response.json()) as { service: string };

    expect(json.service).toBe("phoenix-frontend");
  });

  it("returns valid ISO timestamp", async () => {
    vi.useRealTimers();

    const response = await GET();
    const json = (await response.json()) as { timestamp: string };

    // Check that timestamp is a valid ISO date string
    const parsedDate = new Date(json.timestamp);
    expect(parsedDate.toISOString()).toBe(json.timestamp);
  });
});
