import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockRevalidatePath } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockRevalidatePath: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("next/cache", () => ({
  revalidatePath: mockRevalidatePath,
}));

import { POST } from "./route";

const emptyContext = { params: Promise.resolve({}) };

describe("POST /api/operator/provisioning/revalidate-tenant", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("revalidates paths for given slugs", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/revalidate-tenant",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ slugs: ["old-sub", "new-sub"] }),
      },
    );

    const response = await POST(request, emptyContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success: boolean;
      data: { status: string; revalidated: string[] };
    };
    expect(json.success).toBe(true);
    expect(json.data.status).toBe("ok");
    expect(json.data.revalidated).toEqual(["old-sub", "new-sub"]);
    expect(mockRevalidatePath).toHaveBeenCalledTimes(2);
    expect(mockRevalidatePath).toHaveBeenCalledWith("/old-sub", "layout");
    expect(mockRevalidatePath).toHaveBeenCalledWith("/new-sub", "layout");
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/revalidate-tenant",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ slugs: ["test"] }),
      },
    );

    const response = await POST(request, emptyContext);
    expect(response.status).toBe(401);
  });

  it("returns 400 when slugs array is missing", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/revalidate-tenant",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      },
    );

    const response = await POST(request, emptyContext);
    expect(response.status).toBe(400);
  });

  it("returns 400 when slugs array is empty", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/revalidate-tenant",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ slugs: [] }),
      },
    );

    const response = await POST(request, emptyContext);
    expect(response.status).toBe(400);
  });

  it("skips empty string slugs", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/revalidate-tenant",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ slugs: ["valid", "", "also-valid"] }),
      },
    );

    const response = await POST(request, emptyContext);
    expect(response.status).toBe(200);
    expect(mockRevalidatePath).toHaveBeenCalledTimes(2);
    expect(mockRevalidatePath).toHaveBeenCalledWith("/valid", "layout");
    expect(mockRevalidatePath).toHaveBeenCalledWith("/also-valid", "layout");
  });

  it("returns 500 when request body is invalid JSON", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/revalidate-tenant",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "not json",
      },
    );

    const response = await POST(request, emptyContext);
    expect(response.status).toBe(500);
  });
});
