import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockUncachedAuth,
}));

vi.mock("~/server/auth/operator-route", () => ({
  withOperatorAuth: (handler: unknown) => handler,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://server:8080",
}));

import { GET, POST } from "./route";

const session = { user: { token: "operator-token" } };

function context(accountId = "42") {
  return { params: Promise.resolve({ accountId }) };
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("/api/operator/accounts/[accountId]/tenants", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(session);
    vi.stubGlobal("fetch", mockFetch);
  });

  it("returns 401 without an operator session", async () => {
    mockAuth.mockResolvedValue(null);

    const response = await GET(
      new NextRequest("http://localhost:3000/api/operator/accounts/42/tenants"),
      context(),
    );

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("rejects a non-string account parameter", async () => {
    const response = await GET(
      new NextRequest("http://localhost:3000/api/operator/accounts/42/tenants"),
      { params: Promise.resolve({}) },
    );

    expect(response.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("forwards the list request with the bearer token", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ data: [] }));

    const response = await GET(
      new NextRequest("http://localhost:3000/api/operator/accounts/42/tenants"),
      context(),
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/operator/accounts/42/tenants",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer operator-token",
        }),
      }),
    );
  });

  it("forwards the grant body and preserves the backend status and message", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ message: "account already has access" }, 409),
    );

    const request = new NextRequest(
      "http://localhost:3000/api/operator/accounts/42/tenants",
      {
        method: "POST",
        body: JSON.stringify({ school_id: 3, role_id: 1 }),
        headers: { "Content-Type": "application/json" },
      },
    );
    const response = await POST(request, context());

    expect(response.status).toBe(409);
    expect(await response.json()).toEqual({
      message: "account already has access",
    });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/operator/accounts/42/tenants",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ school_id: 3, role_id: 1 }),
      }),
    );
  });

  it("rejects malformed grant JSON without contacting the backend", async () => {
    const response = await POST(
      new NextRequest(
        "http://localhost:3000/api/operator/accounts/42/tenants",
        {
          method: "POST",
          body: "{",
          headers: { "Content-Type": "application/json" },
        },
      ),
      context(),
    );

    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({ error: "Invalid JSON body" });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("retries once with a refreshed token on 401", async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse({ error: "expired" }, 401))
      .mockResolvedValueOnce(jsonResponse({ data: [] }));
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });

    const response = await GET(
      new NextRequest("http://localhost:3000/api/operator/accounts/42/tenants"),
      context(),
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch).toHaveBeenLastCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer fresh-token",
        }),
      }),
    );
  });
});
