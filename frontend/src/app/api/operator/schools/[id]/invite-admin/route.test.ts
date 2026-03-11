/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { NextRequest } from "next/server";

const { mockOperatorApiPost } = vi.hoisted(() => ({
  mockOperatorApiPost: vi.fn(),
}));

vi.mock("~/lib/operator/route-wrapper", () => ({
  createOperatorPostHandler: (
    handler: (
      request: Request,
      body: unknown,
      token: string,
      params: Record<string, unknown>,
    ) => Promise<unknown>,
  ) => {
    return async (
      request: Request,
      context: { params: Promise<Record<string, unknown>> },
    ) => {
      const body = await request.json();
      const params = await context.params;
      const data = await handler(request, body, "test-token", params);
      return new Response(JSON.stringify({ success: true, data }));
    };
  },
  operatorApiPost: mockOperatorApiPost,
  isStringParam: (value: unknown): value is string => typeof value === "string",
}));

import { POST } from "./route";

describe("operator/schools/[id]/invite-admin route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls operatorApiPost with school id in path", async () => {
    const mockInvitation = { id: 1, email: "admin@test.de" };
    mockOperatorApiPost.mockResolvedValue(mockInvitation);

    const body = { email: "admin@test.de" };
    const request = new Request(
      "http://localhost/api/operator/schools/5/invite-admin",
      {
        method: "POST",
        body: JSON.stringify(body),
        headers: { "Content-Type": "application/json" },
      },
    ) as unknown as NextRequest;

    const response = await POST(request, {
      params: Promise.resolve({ id: "5" }),
    } as never);
    const json = (await response.json()) as { data: unknown };

    expect(mockOperatorApiPost).toHaveBeenCalledWith(
      "/operator/schools/5/invite-admin",
      "test-token",
      body,
    );
    expect(json.data).toEqual(mockInvitation);
  });

  it("throws error for invalid id parameter", async () => {
    const body = { email: "admin@test.de" };
    const request = new Request(
      "http://localhost/api/operator/schools/invalid/invite-admin",
      {
        method: "POST",
        body: JSON.stringify(body),
        headers: { "Content-Type": "application/json" },
      },
    ) as unknown as NextRequest;

    await expect(
      POST(request, {
        params: Promise.resolve({ id: 123 }),
      } as never),
    ).rejects.toThrow("Invalid id parameter");
  });
});
