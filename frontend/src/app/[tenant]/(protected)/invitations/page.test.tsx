import { describe, it, expect, vi } from "vitest";

const { mockRedirect } = vi.hoisted(() => ({
  mockRedirect: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

import InvitationsRedirectPage from "./page";

describe("InvitationsRedirectPage", () => {
  it("redirects to the personal database page", () => {
    InvitationsRedirectPage();

    expect(mockRedirect).toHaveBeenCalledWith("database/personal");
  });
});
