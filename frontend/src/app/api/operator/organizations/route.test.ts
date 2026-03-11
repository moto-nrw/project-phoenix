/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { NextRequest } from "next/server";

const { mockOperatorApiGet, mockOperatorApiPost } = vi.hoisted(() => ({
  mockOperatorApiGet: vi.fn(),
  mockOperatorApiPost: vi.fn(),
}));

vi.mock("~/lib/operator/route-wrapper", () => ({
  createOperatorGetHandler: (
    handler: (request: Request, token: string) => Promise<unknown>,
  ) => {
    return async (request: Request) => {
      const data = await handler(request, "test-token");
      return new Response(JSON.stringify({ success: true, data }));
    };
  },
  createOperatorPostHandler: (
    handler: (
      request: Request,
      body: unknown,
      token: string,
    ) => Promise<unknown>,
  ) => {
    return async (request: Request) => {
      const body = await request.json();
      const data = await handler(request, body, "test-token");
      return new Response(JSON.stringify({ success: true, data }));
    };
  },
  operatorApiGet: mockOperatorApiGet,
  operatorApiPost: mockOperatorApiPost,
}));

import { GET, POST } from "./route";

describe("operator/organizations route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("GET", () => {
    it("calls operatorApiGet with correct endpoint", async () => {
      const mockOrgs = [{ id: 1, name: "Org A" }];
      mockOperatorApiGet.mockResolvedValue(mockOrgs);

      const request = new Request(
        "http://localhost/api/operator/organizations",
      ) as unknown as NextRequest;
      const response = await GET(request, {} as never);
      const json = (await response.json()) as { data: unknown };

      expect(mockOperatorApiGet).toHaveBeenCalledWith(
        "/operator/organizations",
        "test-token",
      );
      expect(json.data).toEqual(mockOrgs);
    });
  });

  describe("POST", () => {
    it("calls operatorApiPost with correct endpoint and body", async () => {
      const mockOrg = { id: 1, name: "New Org" };
      mockOperatorApiPost.mockResolvedValue(mockOrg);

      const body = { name: "New Org", slug: "new-org" };
      const request = new Request(
        "http://localhost/api/operator/organizations",
        {
          method: "POST",
          body: JSON.stringify(body),
          headers: { "Content-Type": "application/json" },
        },
      ) as unknown as NextRequest;

      const response = await POST(request, {} as never);
      const json = (await response.json()) as { data: unknown };

      expect(mockOperatorApiPost).toHaveBeenCalledWith(
        "/operator/organizations",
        "test-token",
        body,
      );
      expect(json.data).toEqual(mockOrg);
    });
  });
});
