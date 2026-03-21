/**
 * Tests for Operator Root Page (Redirect)
 * Tests the redirect from /operator to /operator/suggestions
 */
import { describe, it, expect, vi } from "vitest";

// Hoisted mocks
const { mockRedirect } = vi.hoisted(() => ({
  mockRedirect: vi.fn(),
}));

// Mock next/navigation redirect
vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

import OperatorRootPage from "./page";

describe("OperatorRootPage", () => {
  it("redirects to /operator/suggestions", () => {
    OperatorRootPage();

    expect(mockRedirect).toHaveBeenCalledWith("/operator/suggestions");
  });
});
