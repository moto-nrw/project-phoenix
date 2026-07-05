import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";

const { mockReplace } = vi.hoisted(() => ({
  mockReplace: vi.fn(),
}));

// useTenantRouter prefixes the tenant slug in path-based routing mode, so the
// redirect must go through it instead of a bare next/navigation redirect.
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: mockReplace }),
}));

import InvitationsRedirectPage from "./page";

describe("InvitationsRedirectPage", () => {
  it("redirects to the personal database page via the tenant router", () => {
    render(<InvitationsRedirectPage />);

    expect(mockReplace).toHaveBeenCalledWith("/database/personal");
  });
});
