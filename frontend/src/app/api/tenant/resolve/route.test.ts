import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";

// ============================================================================
// Mocks
// ============================================================================

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

const { GET } = await import("./route");

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/tenant/resolve", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("returns 400 when slug parameter is missing", async () => {
    const request = createMockRequest("/api/tenant/resolve");
    const response = await GET(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("slug parameter is required");
  });

  it("resolves tenant slug from backend", async () => {
    const tenantData = {
      tenant_id: 1,
      slug: "demo-school",
      name: "Demo School",
      subdomain: "demo",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(tenantData), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/tenant/resolve?slug=demo-school");
    const response = await GET(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/tenant/resolve?slug=demo-school",
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof tenantData>(response);
    expect(json.slug).toBe("demo-school");
  });

  it("forwards 404 from backend", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "not found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/tenant/resolve?slug=nonexistent");
    const response = await GET(request);

    expect(response.status).toBe(404);
  });

  it("encodes slug parameter", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({}), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/tenant/resolve?slug=school%20with%20spaces",
    );
    await GET(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/tenant/resolve?slug=school%20with%20spaces",
    );
  });

  it("returns 500 on fetch failure", async () => {
    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/tenant/resolve?slug=demo-school");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
