import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockUseSession, mockSupervision } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockSupervision: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("~/lib/auth-utils", () => ({
  hasEffectiveAdminScope: () => false,
  hasPermission: () => false,
  isCaregiver: () => true,
}));
vi.mock("~/lib/change-request-access", () => ({
  canOpenRequestsPage: () => false,
}));
vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: mockSupervision,
}));

import { useHomeBlockAccess } from "./use-home-block-access";

describe("useHomeBlockAccess", () => {
  beforeEach(() => {
    mockUseSession.mockReturnValue({
      data: {
        user: { roles: ["user"], permissions: ["groups:read"] },
        expires: "2099-01-01",
      },
      status: "authenticated",
      update: vi.fn(),
    });
  });

  it("does not treat a visible tenant group as an own assignment", () => {
    mockSupervision.mockReturnValue({
      hasGroups: true,
      groups: [
        {
          id: "7",
          name: "Sonnengruppe",
          is_personal: false,
        },
      ],
    });

    const { result } = renderHook(() => useHomeBlockAccess());

    expect(result.current.hasOwnGroups).toBe(false);
  });

  it("enables the card for a personally assigned group", () => {
    mockSupervision.mockReturnValue({
      hasGroups: true,
      groups: [
        {
          id: "8",
          name: "Sternengruppe",
          is_personal: true,
        },
      ],
    });

    const { result } = renderHook(() => useHomeBlockAccess());

    expect(result.current.hasOwnGroups).toBe(true);
  });

  it("keeps the card hidden until the server confirms the assignment", () => {
    mockSupervision.mockReturnValue({
      hasGroups: true,
      groups: [{ id: "9", name: "Mondgruppe" }],
    });

    const { result } = renderHook(() => useHomeBlockAccess());

    expect(result.current.hasOwnGroups).toBe(false);
  });
});
