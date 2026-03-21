import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { GET } from "./route";

// ============================================================================
// Mocks
// ============================================================================

const { mockGetServerApiUrl } = vi.hoisted(() => ({
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/tenant/list", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("proxies backend response on success", async () => {
    const backendData = {
      status: "success",
      data: [
        {
          slug: "school-a",
          name: "School A",
          subdomain: "school-a",
          organization_name: "Org",
        },
      ],
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendData), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const response = await GET();

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/tenants",
    );
    expect(response.status).toBe(200);

    const json = (await response.json()) as { data: { slug: string }[] };
    expect(json.data).toHaveLength(1);
    expect(json.data[0]?.slug).toBe("school-a");
  });

  it("forwards non-200 status from backend", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "not found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const response = await GET();

    expect(response.status).toBe(404);
  });

  it("handles non-JSON response (e.g. rate limit)", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Rate limit exceeded. Please try again later.", {
        status: 429,
        headers: { "Content-Type": "text/plain; charset=utf-8" },
      }),
    );

    const response = await GET();

    expect(response.status).toBe(429);
    const json = (await response.json()) as { error: string };
    expect(json.error).toBe("Rate limit exceeded. Please try again later.");
  });

  it("returns 500 on network error", async () => {
    vi.mocked(global.fetch).mockRejectedValueOnce(
      new Error("Connection refused"),
    );

    const response = await GET();

    expect(response.status).toBe(500);
    const json = (await response.json()) as { error: string };
    expect(json.error).toBe("Internal Server Error");
  });
});
