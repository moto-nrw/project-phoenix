import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PushSubscriptionSync } from "./service-worker-registrar";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  isPushConfigurationMissing: vi.fn(),
  isPushSupported: vi.fn(),
  syncExistingPushSubscription: vi.fn(),
  reportStandaloneUsage: vi.fn(),
  warn: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mocks.useSession,
}));

vi.mock("~/lib/push-api", () => ({
  isPushConfigurationMissing: mocks.isPushConfigurationMissing,
  isPushSupported: mocks.isPushSupported,
  syncExistingPushSubscription: mocks.syncExistingPushSubscription,
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ warn: mocks.warn }),
}));

vi.mock("~/lib/pwa-usage-api", () => ({
  reportStandaloneUsage: mocks.reportStandaloneUsage,
}));

describe("PushSubscriptionSync", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.isPushConfigurationMissing.mockReturnValue(false);
    mocks.isPushSupported.mockReturnValue(true);
    mocks.syncExistingPushSubscription.mockResolvedValue(null);
    mocks.reportStandaloneUsage.mockResolvedValue(undefined);
  });

  it("rebinds an existing subscription after authentication", async () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "account-1", tenantId: 1 } },
    });

    render(<PushSubscriptionSync portal="parent" />);

    await waitFor(() =>
      expect(mocks.syncExistingPushSubscription).toHaveBeenCalledWith("parent"),
    );
    expect(mocks.reportStandaloneUsage).toHaveBeenCalledWith(
      "parent",
      "account-1",
      1,
    );
  });

  it("does not sync before authentication or on unsupported browsers", () => {
    mocks.useSession.mockReturnValue({ status: "unauthenticated" });
    const { rerender } = render(<PushSubscriptionSync portal="tenant" />);
    expect(mocks.syncExistingPushSubscription).not.toHaveBeenCalled();

    mocks.useSession.mockReturnValue({ status: "authenticated" });
    mocks.isPushSupported.mockReturnValue(false);
    rerender(<PushSubscriptionSync portal="tenant" />);
    expect(mocks.syncExistingPushSubscription).not.toHaveBeenCalled();
  });

  it("reports standalone usage when push is unsupported", async () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "account-1", tenantId: 1 } },
    });
    mocks.isPushSupported.mockReturnValue(false);

    render(<PushSubscriptionSync portal="tenant" />);

    await waitFor(() =>
      expect(mocks.reportStandaloneUsage).toHaveBeenCalledWith(
        "tenant",
        "account-1",
        1,
      ),
    );
    expect(mocks.syncExistingPushSubscription).not.toHaveBeenCalled();
  });

  it("does not warn when push is intentionally unconfigured", async () => {
    mocks.useSession.mockReturnValue({ status: "authenticated" });
    mocks.isPushConfigurationMissing.mockReturnValue(true);
    mocks.syncExistingPushSubscription.mockRejectedValue(
      new Error("not configured"),
    );

    render(<PushSubscriptionSync portal="tenant" />);

    await waitFor(() =>
      expect(mocks.syncExistingPushSubscription).toHaveBeenCalled(),
    );
    expect(mocks.warn).not.toHaveBeenCalled();
  });
});
