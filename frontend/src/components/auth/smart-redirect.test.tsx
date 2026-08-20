import { render, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SmartRedirect } from "./smart-redirect";

const { mockRedirect } = vi.hoisted(() => ({ mockRedirect: vi.fn() }));

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: { id: "1", token: "valid-token" },
    },
    status: "authenticated",
  })),
}));

vi.mock("~/lib/supervision-context", () => ({
  useSupervision: vi.fn(() => ({
    hasGroups: false,
    isLoadingGroups: false,
    isSupervising: false,
    isLoadingSupervision: false,
    adminOverviewEnabled: false,
    supervisedRooms: [],
    groups: [],
    refresh: vi.fn(),
  })),
}));

vi.mock("~/lib/redirect-utils", () => ({
  useSmartRedirectPath: vi.fn(() => ({
    redirectPath: "/dashboard",
    isReady: true,
  })),
}));

vi.mock("~/lib/tenant-context", () => ({
  usePresenceMode: vi.fn(() => "detailed"),
  useOpenCareGroupMode: vi.fn(() => false),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  useTenantRoutingModeSafe: vi.fn(() => "path"),
  useNFCEnabled: vi.fn(() => true),
}));

import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { useSmartRedirectPath } from "~/lib/redirect-utils";
import { usePresenceMode } from "~/lib/tenant-context";

describe("SmartRedirect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { id: "1", token: "valid-token" },
        expires: "",
      },
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useSupervision).mockReturnValue({
      hasGroups: false,
      isLoadingGroups: false,
      isSupervising: false,
      isLoadingSupervision: false,
      adminOverviewEnabled: false,
      supervisedRooms: [],
      groups: [],
      refresh: vi.fn(),
    });
    vi.mocked(useSmartRedirectPath).mockReturnValue({
      redirectPath: "/dashboard",
      isReady: true,
    });
    vi.mocked(usePresenceMode).mockReturnValue("detailed");
  });

  it("renders nothing (returns null)", () => {
    const { container } = render(<SmartRedirect />);

    expect(container.firstChild).toBeNull();
    expect(mockRedirect).toHaveBeenCalledWith("/test-tenant/dashboard");
  });

  it("redirects to dashboard when authenticated and ready", async () => {
    render(<SmartRedirect />);

    await waitFor(() => {
      expect(mockRedirect).toHaveBeenCalledWith("/test-tenant/dashboard");
    });
  });

  it("does not redirect when status is not authenticated", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });

    render(<SmartRedirect />);

    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("does not redirect when token is missing", () => {
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1", token: "" }, expires: "" },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<SmartRedirect />);

    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("does not redirect when not ready", () => {
    vi.mocked(useSmartRedirectPath).mockReturnValue({
      redirectPath: "/dashboard",
      isReady: false,
    });

    render(<SmartRedirect />);

    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("calls onRedirect callback instead of router.push when provided", async () => {
    const onRedirect = vi.fn();
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1", token: "valid-token" }, expires: "" },
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useSmartRedirectPath).mockReturnValue({
      redirectPath: "/ogs-groups",
      isReady: true,
    });

    render(<SmartRedirect onRedirect={onRedirect} />);

    await waitFor(() => {
      expect(onRedirect).toHaveBeenCalledWith("/ogs-groups");
      expect(mockRedirect).not.toHaveBeenCalled();
    });
  });

  it("uses redirect path from useSmartRedirectPath", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1", token: "valid-token" }, expires: "" },
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useSmartRedirectPath).mockReturnValue({
      redirectPath: "/active-supervisions",
      isReady: true,
    });

    render(<SmartRedirect />);

    await waitFor(() => {
      expect(mockRedirect).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions",
      );
    });
  });

  it("passes supervision context to useSmartRedirectPath", () => {
    vi.mocked(useSupervision).mockReturnValue({
      hasGroups: true,
      isLoadingGroups: false,
      isSupervising: true,
      isLoadingSupervision: false,
      adminOverviewEnabled: false,
      supervisedRooms: [],
      groups: [],
      refresh: vi.fn(),
    });

    render(<SmartRedirect />);

    expect(useSmartRedirectPath).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        hasGroups: true,
        isSupervising: true,
      }),
      "detailed",
      false,
    );
  });

  it("passes binary presence mode to useSmartRedirectPath", () => {
    vi.mocked(usePresenceMode).mockReturnValue("binary");

    render(<SmartRedirect />);

    expect(useSmartRedirectPath).toHaveBeenCalledWith(
      expect.anything(),
      expect.anything(),
      "binary",
      false,
    );
  });
});
