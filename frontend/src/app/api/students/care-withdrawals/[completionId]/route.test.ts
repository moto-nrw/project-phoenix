import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

import { ApiResponseError } from "~/lib/api-helpers.server";
import { DELETE } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/api-helpers.server")>();
  return {
    ...actual,
    apiDelete: mockApiDelete,
  };
});

const session: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

describe("DELETE /api/students/care-withdrawals/[completionId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(session);
  });

  it("forwards the guarded deletion confirmation body", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);
    const body = {
      expected_fingerprint: "abc123",
      confirmation_name: "Mia Muster",
      reason: "test_data",
      acknowledged: true,
    };

    const response = await DELETE(
      new NextRequest(
        "http://localhost:3000/api/students/care-withdrawals/73",
        {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
      ),
      { params: Promise.resolve({ completionId: "73" }) },
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/students/care-withdrawals/73",
      "test-token",
      body,
    );
    expect(response.status).toBe(204);
  });

  it("preserves the backend error code as a structured proxy field", async () => {
    mockApiDelete.mockRejectedValueOnce(
      new ApiResponseError(
        409,
        JSON.stringify({
          status: "error",
          error: "Vorschau veraltet",
          code: "students.deletion_preview_changed",
        }),
      ),
    );

    const response = await DELETE(
      new NextRequest(
        "http://localhost:3000/api/students/care-withdrawals/73",
        {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({}),
        },
      ),
      { params: Promise.resolve({ completionId: "73" }) },
    );

    expect(response.status).toBe(409);
    await expect(response.json()).resolves.toEqual({
      error: "Vorschau veraltet",
      code: "students.deletion_preview_changed",
    });
  });
});
