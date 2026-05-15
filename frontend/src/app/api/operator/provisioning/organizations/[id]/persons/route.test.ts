import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

const { mockAuth, mockFetch, mockGetServerApiUrl } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

function contextWith(id: string | string[] | undefined): RouteContext {
  return { params: Promise.resolve({ id }) };
}

describe("GET /api/operator/provisioning/organizations/[id]/persons", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards the org id to the backend and returns the persons list", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const persons = [
      {
        id: 1,
        first_name: "Anna",
        last_name: "Beispiel",
        is_staff: true,
        is_student: false,
        has_account: true,
        has_rfid_card: false,
        school_id: 7,
        school_name: "Schule A",
        organization_id: 42,
        organization_name: "Träger A",
        created_at: "2026-01-01T00:00:00Z",
      },
    ];
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: persons }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/42/persons",
    );
    const response = await GET(request, contextWith("42"));

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: typeof persons };
    expect(json.data).toEqual(persons);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations/42/persons",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/42/persons",
    );
    const response = await GET(request, contextWith("42"));

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns an error response when the id param is missing", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations//persons",
    );
    const response = await GET(request, contextWith(undefined));

    expect(response.status).toBeGreaterThanOrEqual(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("propagates a backend 500 as an error response", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => "internal error",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/42/persons",
    );
    const response = await GET(request, contextWith("42"));

    expect(response.status).toBeGreaterThanOrEqual(400);
  });
});
