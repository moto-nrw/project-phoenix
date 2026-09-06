import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PortalNotificationSetup } from "./portal-notification-setup";

const shellAuth = vi.hoisted(() => ({ useShellAuthSafe: vi.fn() }));
const dialog = vi.hoisted(() => ({ NotificationSetupDialog: vi.fn() }));

vi.mock("~/lib/shell-auth-context", () => shellAuth);
vi.mock("~/components/notifications/notification-setup-dialog", () => ({
  NotificationSetupDialog: (props: Record<string, unknown>) => {
    dialog.NotificationSetupDialog(props);
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

    expect(await screen.findByTestId("setup-dialog")).toBeInTheDocument();
    expect(dialog.NotificationSetupDialog).toHaveBeenCalledWith(
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
