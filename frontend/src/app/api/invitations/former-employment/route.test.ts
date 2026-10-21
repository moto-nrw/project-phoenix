import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { type ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: vi.fn(),
    apiPost: mockApiPost,
    apiPut: vi.fn(),
    apiDelete: vi.fn(),
    handleApiError: vi.fn(mockHandleApiError),
  };
});

const { POST } = await import("./route");

function createRequest(body: unknown): NextRequest {
  return new NextRequest(
    new URL("/api/invitations/former-employment", "http://localhost:3000"),
    {
      method: "POST",
      body: JSON.stringify(body),
      headers: { "Content-Type": "application/json" },
    },
  );
}

const context = { params: Promise.resolve({}) };

describe("POST /api/invitations/former-employment", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({
      user: { id: "1", token: "test-token", name: "Test User" },
      expires: "2099-01-01",
    });
  });

  it("asks the backend with the address in the body", async () => {
    const formerEmployment = {
      started_on: "2024-08-01",
      ended_on: "2026-07-31",
      resumes: ["master_data"],
      not_returning: ["staff_documents"],
    };
    mockApiPost.mockResolvedValueOnce({
      data: { former_employment: formerEmployment },
    });

    const response = await POST(
      createRequest({ email: "former@example.com", role_id: "7" }),
      context,
    );

    expect(mockApiPost).toHaveBeenCalledWith(
      "/auth/invitations/former-employment",
      "test-token",
      { email: "former@example.com" },
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data: { former_employment: unknown };
    };
    expect(json.data.former_employment).toEqual(formerEmployment);
  });
});
