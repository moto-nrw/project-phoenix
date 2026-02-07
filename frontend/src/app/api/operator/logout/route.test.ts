import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockClearOperatorTokens } = vi.hoisted(() => ({
  mockClearOperatorTokens: vi.fn(),
}));

vi.mock("~/lib/operator/cookies", () => ({
  clearOperatorTokens: mockClearOperatorTokens,
}));

import { POST } from "./route";

describe("POST /api/operator/logout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("clears operator tokens and returns success", async () => {
    const response = await POST();

    expect(response.status).toBe(200);
    const json = (await response.json()) as { success?: boolean };
    expect(json).toEqual({ success: true });
    expect(mockClearOperatorTokens).toHaveBeenCalledOnce();
  });
});
