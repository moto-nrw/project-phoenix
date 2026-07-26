import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
}));
const parentsHostname = "parents.example.test";
const tenantHostname = "demo.localhost:3000";

vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", parentsHostname);

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const { GET } = await import("./route");

function getRequest({
  query = "",
  originalHost = tenantHostname,
}: {
  query?: string;
  originalHost?: string;
} = {}): NextRequest {
  return new NextRequest(
    new URL(
      `http://next-internal:3000/api/enrollment/form-bootstrap/public/demo/5${query}`,
    ),
    {
      headers: {
        host: "next-internal:3000",
        "x-moto-original-host": originalHost,
      },
    },
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

  it("never invokes tenant auth on the parent host without a query flag", async () => {
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
    // Even if a domain-scoped tenant session reaches this route, the trusted
    // original parent host must return before auth/profile enrichment.
    mockAuth.mockResolvedValueOnce({ user: { token: "tenant-token" } });

    const response = await GET(
      getRequest({ originalHost: `${parentsHostname}:443` }),
      {
        params: Promise.resolve({ tenantSlug: "demo", phaseId: "5" }),
      },
    );

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual(payload);
    expect(mockAuth).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("rejects a malformed original host before auth or backend fetch", async () => {
    mockAuth.mockResolvedValueOnce({ user: { token: "tenant-token" } });

    const response = await GET(
      getRequest({ originalHost: `user@${parentsHostname}` }),
      {
        params: Promise.resolve({ tenantSlug: "demo", phaseId: "5" }),
      },
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual({
      error: "Invalid request host",
    });
    expect(mockAuth).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("ignores the legacy profile query flag on a tenant host", async () => {
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

    const response = await GET(getRequest({ query: "?prefetch_profile=0" }), {
      params: Promise.resolve({ tenantSlug: "demo", phaseId: "5" }),
    });

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({
      ...payload,
      data: { ...payload.data, profile },
    });
    expect(mockAuth).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
