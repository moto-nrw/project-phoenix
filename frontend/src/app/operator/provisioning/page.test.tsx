/**
 * Tests for Operator Provisioning Redirect.
 *
 * The legacy /operator/provisioning page was split into five dedicated
 * routes in issue #1282. The old path is kept alive as a server redirect to
 * /operator/organizations so bookmarks and external links continue to
 * resolve.
 */
import { describe, it, expect, vi } from "vitest";

const { mockRedirect } = vi.hoisted(() => ({
  mockRedirect: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

import OperatorProvisioningRedirect from "./page";

describe("OperatorProvisioningRedirect", () => {
  it("redirects to /operator/organizations", () => {
    OperatorProvisioningRedirect();

    expect(mockRedirect).toHaveBeenCalledWith("/operator/organizations");
  });
});
