import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";

const { mockRedirect } = vi.hoisted(() => ({
  mockRedirect: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

import InvitationsRedirectPage from "./page";

describe("InvitationsRedirectPage", () => {
  it("redirects to the tenant-aware personal database page", () => {
    render(<InvitationsRedirectPage />);

    expect(mockRedirect).toHaveBeenCalledWith("/test-tenant/database/personal");
  });
});
