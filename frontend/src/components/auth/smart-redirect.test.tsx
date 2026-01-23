import { render, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SmartRedirect } from "./smart-redirect";

const mockPush = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
  }),
}));

vi.mock("~/lib/supervision-context", () => ({
  useSupervision: vi.fn(() => ({
    hasGroups: false,
    isLoadingGroups: false,
    isSupervising: false,
    isLoadingSupervision: false,
    groups: [],
    refresh: vi.fn(),
  })),
}));

vi.mock("~/lib/redirect-utils", () => ({
  useSmartRedirectPath: vi.fn(() => ({
    redirectPath: "/dashboard",
    isReady: true,
  })),
  checkSaasAdminStatus: vi.fn(() => Promise.resolve(false)),
}));

import { useSession } from "~/lib/auth-client";
import { useSupervision } from "~/lib/supervision-context";
import { useSmartRedirectPath } from "~/lib/redirect-utils";

describe("SmartRedirect", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Ensure session mock is set for each test
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "test-user-id",
          email: "test@example.com",
          name: "Test User",
          emailVerified: true,
          image: null,
          createdAt: new Date(),
          updatedAt: new Date(),
        },
        session: {
          id: "test-session-id",
          userId: "test-user-id",
          expiresAt: new Date(Date.now() + 86400000),
          ipAddress: null,
          userAgent: null,
        },
        activeOrganizationId: "test-org-id",
      },
      isPending: false,
      error: null,
    });
  });

  it("renders nothing (returns null)", () => {
    const { container } = render(<SmartRedirect />);

    expect(container.firstChild).toBeNull();
  });

  it("redirects to dashboard when authenticated and ready", async () => {
    render(<SmartRedirect />);

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/dashboard");
      expect(mockRefresh).toHaveBeenCalled();
    });
  });

  it("does not redirect when not authenticated", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      isPending: false,
      error: null,
    });

    render(<SmartRedirect />);

    expect(mockPush).not.toHaveBeenCalled();
  });

  it("does not redirect when not ready", () => {
    vi.mocked(useSmartRedirectPath).mockReturnValue({
      redirectPath: "/dashboard",
      isReady: false,
    });

    render(<SmartRedirect />);

    expect(mockPush).not.toHaveBeenCalled();
  });

  it("calls onRedirect callback instead of router.push when provided", async () => {
    const onRedirect = vi.fn();
    vi.mocked(useSmartRedirectPath).mockReturnValue({
      redirectPath: "/ogs-groups",
      isReady: true,
    });

    render(<SmartRedirect onRedirect={onRedirect} />);

    await waitFor(() => {
      expect(onRedirect).toHaveBeenCalledWith("/ogs-groups");
      expect(mockPush).not.toHaveBeenCalled();
    });
  });

  it("uses redirect path from useSmartRedirectPath", async () => {
    vi.mocked(useSmartRedirectPath).mockReturnValue({
      redirectPath: "/active-supervisions",
      isReady: true,
    });

    render(<SmartRedirect />);

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/active-supervisions");
    });
  });

  it("passes supervision context to useSmartRedirectPath", () => {
    vi.mocked(useSupervision).mockReturnValue({
      hasGroups: true,
      isLoadingGroups: false,
      isSupervising: true,
      isLoadingSupervision: false,
      groups: [],
      refresh: vi.fn(),
    });

    render(<SmartRedirect />);

    // Check that useSmartRedirectPath was called with the correct supervision context
    expect(useSmartRedirectPath).toHaveBeenCalled();
    const call = vi.mocked(useSmartRedirectPath).mock.calls[0];
    expect(call?.[1]).toMatchObject({
      hasGroups: true,
      isSupervising: true,
    });
  });
});
