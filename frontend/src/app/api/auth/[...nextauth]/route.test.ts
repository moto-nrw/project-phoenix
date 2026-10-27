import { describe, it, expect, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockHandlers } = vi.hoisted(() => ({
  mockHandlers: {
    GET: vi.fn(),
    POST: vi.fn(),
  },
}));

vi.mock("~/server/auth", () => ({
  handlers: mockHandlers,
}));

// Import after mocks are set up
const { GET, POST } = await import("./route");

// The routes delegate to Auth.js and add the BFF cache policy (#4999); the
// session endpoint answers without the session's tokens (#5005).
describe("NextAuth Route Handlers", () => {
  it.each([
    ["GET", GET, mockHandlers.GET],
    ["POST", POST, mockHandlers.POST],
  ] as const)(
    "delegates %s to the Auth.js handler with private, no-store",
    async (_method, route, handler) => {
      const request = new NextRequest("http://localhost:3000/api/auth/session");
      handler.mockResolvedValue(
        Response.json({
          user: { id: "1", token: "access", refreshToken: "refresh" },
          expires: "2099-01-01T00:00:00.000Z",
        }),
      );

      const response = await route(request);

      expect(handler).toHaveBeenCalledWith(request);
      expect(await response.json()).toEqual({
        user: { id: "1" },
        expires: "2099-01-01T00:00:00.000Z",
      });
      expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    },
  );
});
