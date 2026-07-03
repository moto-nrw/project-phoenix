import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";

const { mockReplace } = vi.hoisted(() => ({
  mockReplace: vi.fn(),
}));

// useTenantRouter prefixes the tenant slug in path-based routing mode, so the
// redirect must go through it instead of a bare next/navigation redirect.
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: mockReplace }),
}));

import ActivityDetailRedirect from "./page";

describe("ActivityDetailRedirect (legacy /activities/[id])", () => {
  it("redirects to the canonical activities page via the tenant router", () => {
    render(<ActivityDetailRedirect />);

    expect(mockReplace).toHaveBeenCalledWith("/activities");
  });
});
