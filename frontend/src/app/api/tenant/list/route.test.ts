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
});
