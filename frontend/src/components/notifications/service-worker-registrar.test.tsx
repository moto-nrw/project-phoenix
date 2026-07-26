import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PushSubscriptionSync } from "./service-worker-registrar";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  isPushSupported: vi.fn(),
  syncExistingPushSubscription: vi.fn(),
  warn: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mocks.useSession,
}));

vi.mock("~/lib/push-api", () => ({
  isPushSupported: mocks.isPushSupported,
  syncExistingPushSubscription: mocks.syncExistingPushSubscription,
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ warn: mocks.warn }),
}));

describe("PushSubscriptionSync", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.isPushSupported.mockReturnValue(true);
    mocks.syncExistingPushSubscription.mockResolvedValue(null);
  });

  it("rebinds an existing subscription after authentication", async () => {
    mocks.useSession.mockReturnValue({ status: "authenticated" });

    render(<PushSubscriptionSync portal="parent" />);

    await waitFor(() =>
      expect(mocks.syncExistingPushSubscription).toHaveBeenCalledWith("parent"),
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
});
