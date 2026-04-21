/**
 * Tests for Operator Root Page (Redirect).
 *
 * The operator root redirects to the first Verwaltung page — see issue
 * #1282 (operator sidebar restructure) where Schulverwaltung's tabs were
 * split into dedicated /operator/{organizations,schools,accounts,devices,persons}
 * routes and Träger became the default landing.
 */
import { describe, it, expect, vi } from "vitest";

const { mockRedirect } = vi.hoisted(() => ({
  mockRedirect: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

import OperatorRootPage from "./page";

describe("OperatorRootPage", () => {
  it("redirects to /operator/organizations", () => {
    OperatorRootPage();

    expect(mockRedirect).toHaveBeenCalledWith("/operator/organizations");
  });
});
