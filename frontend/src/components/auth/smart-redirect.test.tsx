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

import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { useSmartRedirectPath } from "~/lib/redirect-utils";

describe("SmartRedirect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

  it("does not redirect when status is not authenticated", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });

    render(<SmartRedirect />);

    expect(mockPush).not.toHaveBeenCalled();
  });

  it("does not redirect when token is missing", () => {
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1", token: "" }, expires: "" },
      status: "authenticated",
      update: vi.fn(),
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
      expect(mockPush).not.toHaveBeenCalled();
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

    expect(useSmartRedirectPath).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        hasGroups: true,
        isSupervising: true,
      }),
    );
  });
});
