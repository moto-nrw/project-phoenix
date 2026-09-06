import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PortalNotificationSetup } from "./portal-notification-setup";

const shellAuth = vi.hoisted(() => ({ useShellAuthSafe: vi.fn() }));
const setup = vi.hoisted(() => ({ DeferredNotificationSetup: vi.fn() }));

vi.mock("~/lib/shell-auth-context", () => shellAuth);
vi.mock("~/components/notifications/deferred-notification-setup", () => ({
  DeferredNotificationSetup: (props: Record<string, unknown>) => {
    setup.DeferredNotificationSetup(props);
    return <div data-testid="setup-dialog" />;
  },
}));

describe("PortalNotificationSetup", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("binds the guided setup to the signed-in account of its portal", async () => {
    shellAuth.useShellAuthSafe.mockReturnValue({
      status: "authenticated",
      user: { id: "42" },
    });

    render(<PortalNotificationSetup portal="school" />);

    expect(screen.getByTestId("setup-dialog")).toBeInTheDocument();
    expect(setup.DeferredNotificationSetup).toHaveBeenCalledWith(
      expect.objectContaining({ portal: "school", accountId: "42" }),
    );
  });

  it("stays out of the way while no one is signed in", () => {
    shellAuth.useShellAuthSafe.mockReturnValue({
      status: "loading",
      user: null,
    });

    render(<PortalNotificationSetup portal="tenant" />);

    expect(screen.queryByTestId("setup-dialog")).not.toBeInTheDocument();
  });
});
