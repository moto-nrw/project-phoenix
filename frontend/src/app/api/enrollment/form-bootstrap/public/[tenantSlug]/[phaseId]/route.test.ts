import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const { GET } = await import("./route");

function getRequest(query = ""): NextRequest {
  return new NextRequest(
    new URL(
      `http://localhost:3000/api/enrollment/form-bootstrap/public/demo/5${query}`,
    ),
  );
}

describe("GET /api/enrollment/form-bootstrap/public/[tenantSlug]/[phaseId]", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    mockAuth.mockReset();
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  it("returns anonymous bootstrap payload when optional auth enrichment fails", async () => {
    const payload = {
      data: {
        phase: { id: "5", name: "Schuljahr" },
        schema: null,
        offerings: [],
        legal_texts: { blocks: [] },
      },
    };
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify(payload), { status: 200 }),
    );
    mockAuth.mockRejectedValueOnce(new Error("session backend unavailable"));

    const response = await GET(getRequest(), {
      params: Promise.resolve({ tenantSlug: "demo", phaseId: "5" }),
    });

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual(payload);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("keeps tenant-session profile enrichment for the tenant public form", async () => {
    const payload = {
      data: {
        phase: { id: "5", name: "Schuljahr" },
        schema: null,
        offerings: [],
        legal_texts: { blocks: [] },
      },
    };
    const profile = {
      guardian: {
        first_name: "Tina",
        last_name: "Tenant",
        email: "tina@example.test",
      },
      children: [],
    };
    fetchMock
      .mockResolvedValueOnce(
        new Response(JSON.stringify(payload), { status: 200 }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: profile }), { status: 200 }),
      );
    mockAuth.mockResolvedValueOnce({ user: { token: "tenant-token" } });

    const response = await GET(getRequest(), {
      params: Promise.resolve({ tenantSlug: "demo", phaseId: "5" }),
    });

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({
      ...payload,
      data: { ...payload.data, profile },
    });
    expect(mockAuth).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock).toHaveBeenLastCalledWith(
      "http://backend.test/api/enrollment/me/profile",
      expect.objectContaining({
        headers: { Authorization: "Bearer tenant-token" },
      }),
    );
  });

  it("never invokes tenant auth when profile enrichment is explicitly disabled", async () => {
    const payload = {
      data: {
        phase: { id: "5", name: "Schuljahr" },
        schema: null,
        offerings: [],
        legal_texts: { blocks: [] },
      },
    };
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify(payload), { status: 200 }),
    );
    // Even if a domain-scoped tenant cookie/session reached this route, the
    // parent request's explicit opt-out must return before auth/profile fetch.
    mockAuth.mockResolvedValueOnce({ user: { token: "tenant-token" } });

    const response = await GET(getRequest("?prefetch_profile=0"), {
      params: Promise.resolve({ tenantSlug: "demo", phaseId: "5" }),
    });

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual(payload);
    expect(mockAuth).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
