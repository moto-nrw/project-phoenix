import { describe, it, expect, vi } from "vitest";

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

describe("NextAuth Route Handlers", () => {
  it("exports GET handler from auth", () => {
    expect(GET).toBe(mockHandlers.GET);
  });

  it("exports POST handler from auth", () => {
    expect(POST).toBe(mockHandlers.POST);
  });
});
