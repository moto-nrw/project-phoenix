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

function getRequest(): NextRequest {
  return new NextRequest(
    new URL(
      "http://localhost:3000/api/enrollment/form-bootstrap/public/demo/5",
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
});
