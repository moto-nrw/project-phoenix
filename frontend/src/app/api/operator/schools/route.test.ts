/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockOperatorApiGet, mockOperatorApiPost } = vi.hoisted(() => ({
  mockOperatorApiGet: vi.fn(),
  mockOperatorApiPost: vi.fn(),
  mockGetOperatorToken: vi.fn(() => Promise.resolve("test-token")),
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

describe("operator/schools route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("GET", () => {
    it("calls operatorApiGet with correct endpoint", async () => {
      const mockSchools = [{ id: 1, name: "School A" }];
      mockOperatorApiGet.mockResolvedValue(mockSchools);

      const request = new Request("http://localhost/api/operator/schools");
      const response = await GET(request, {} as never);
      const json = (await response.json()) as {
        data: unknown;
        success: boolean;
      };

      expect(mockOperatorApiGet).toHaveBeenCalledWith(
        "/operator/schools",
        "test-token",
      );
      expect(json.data).toEqual(mockSchools);
    });
  });

  describe("POST", () => {
    it("calls operatorApiPost with correct endpoint and body", async () => {
      const mockSchool = { id: 1, name: "New School" };
      mockOperatorApiPost.mockResolvedValue(mockSchool);

      const body = { name: "New School", slug: "new-school" };
      const request = new Request("http://localhost/api/operator/schools", {
        method: "POST",
        body: JSON.stringify(body),
        headers: { "Content-Type": "application/json" },
      });

      const response = await POST(request, {} as never);
      const json = (await response.json()) as {
        data: unknown;
        success: boolean;
      };

      expect(mockOperatorApiPost).toHaveBeenCalledWith(
        "/operator/schools",
        "test-token",
        body,
      );
      expect(json.data).toEqual(mockSchool);
    });
  });
});
