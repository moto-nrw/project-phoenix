import { describe, it, expect, beforeEach, vi } from "vitest";
import { releaseFakeTimers } from "~/test/clock";
import { GET } from "./route";

describe("GET /api/health", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  it("returns health check response", async () => {
    const mockDate = new Date("2024-01-01T12:00:00.000Z");
    vi.setSystemTime(mockDate);

    const response = await GET();

    expect(response.status).toBe(200);

    const json = (await response.json()) as {
      status: string;
      service: string;
      timestamp: string;
    };
    expect(json).toEqual({
      status: "ok",
      service: "phoenix-frontend",
      timestamp: mockDate.toISOString(),
    });

    releaseFakeTimers();
  });

  it("always returns 200 status", async () => {
    const response = await GET();
    expect(response.status).toBe(200);

    releaseFakeTimers();
  });
});
