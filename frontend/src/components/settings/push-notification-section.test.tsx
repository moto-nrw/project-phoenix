import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PushNotificationSection } from "./push-notification-section";

const pushApi = vi.hoisted(() => ({
  ensurePushConfigured: vi.fn(),
  getExistingSubscription: vi.fn(),
  isPushConfigurationMissing: vi.fn(),
  isPushSupported: vi.fn(),
  needsIOSInstall: vi.fn(),
  subscribePush: vi.fn(),
  unsubscribePush: vi.fn(),
}));

vi.mock("~/lib/push-api", () => pushApi);

function stubNotificationPermission(permission: NotificationPermission) {
  vi.stubGlobal("Notification", { permission });
}

describe("PushNotificationSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    pushApi.isPushConfigurationMissing.mockReturnValue(false);
    pushApi.needsIOSInstall.mockReturnValue(false);
    pushApi.isPushSupported.mockReturnValue(true);
    pushApi.ensurePushConfigured.mockResolvedValue(undefined);
    pushApi.getExistingSubscription.mockResolvedValue(null);
    stubNotificationPermission("default");
  });

  it("shows the enable button when push is supported and not subscribed", async () => {
    render(<PushNotificationSection />);
    expect(
      await screen.findByRole("button", { name: "Aktivieren" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Erhalten Sie Erinnerungen als Systembenachrichtigung/),
    ).toBeInTheDocument();
  });

  it("shows the iOS install hint when the push API is missing in a Safari tab", async () => {
    pushApi.needsIOSInstall.mockReturnValue(true);
    render(<PushNotificationSection />);
    expect(await screen.findByText(/Zum Home-Bildschirm/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Aktivieren" }),
    ).not.toBeInTheDocument();
  });

  it("shows the unsupported message on browsers without push", async () => {
    pushApi.isPushSupported.mockReturnValue(false);
    render(<PushNotificationSection />);
    expect(
      await screen.findByText(
        "Dieser Browser unterstützt keine Push-Benachrichtigungen.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a disabled state when VAPID is not configured", async () => {
    pushApi.ensurePushConfigured.mockRejectedValue(
      new Error("web push is not configured"),
    );
    pushApi.isPushConfigurationMissing.mockReturnValue(true);

    render(<PushNotificationSection />);

    expect(
      await screen.findByText(
        "Push-Benachrichtigungen sind auf diesem Server nicht eingerichtet.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Aktivieren" }),
    ).not.toBeInTheDocument();
  });

  it("shows the blocked message when permission is denied", async () => {
    stubNotificationPermission("denied");
    render(<PushNotificationSection />);
    expect(
      await screen.findByText(/für diese Seite blockiert/),
    ).toBeInTheDocument();
  });

  it("subscribes via the push API and flips to the active state", async () => {
    pushApi.subscribePush.mockImplementation(() => {
      pushApi.getExistingSubscription.mockResolvedValue({
        endpoint: "https://push.example/e",
      });
      return Promise.resolve();
    });

    render(<PushNotificationSection portal="tenant" />);
    fireEvent.click(await screen.findByRole("button", { name: "Aktivieren" }));

    await waitFor(() =>
      expect(pushApi.subscribePush).toHaveBeenCalledWith("tenant"),
    );
    expect(
      await screen.findByText(
        "Benachrichtigungen sind auf diesem Gerät aktiviert.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Deaktivieren" }),
    ).toBeInTheDocument();
  });

  it("passes the parent portal through to the push API", async () => {
    render(<PushNotificationSection portal="parent" />);
    fireEvent.click(await screen.findByRole("button", { name: "Aktivieren" }));
    await waitFor(() =>
      expect(pushApi.subscribePush).toHaveBeenCalledWith("parent"),
    );
  });

  it("surfaces subscribe errors", async () => {
    pushApi.subscribePush.mockRejectedValue(
      new Error("Benachrichtigungen wurden nicht erlaubt."),
    );
    render(<PushNotificationSection />);
    fireEvent.click(await screen.findByRole("button", { name: "Aktivieren" }));
    expect(
      await screen.findByText("Benachrichtigungen wurden nicht erlaubt."),
    ).toBeInTheDocument();
  });

  it("unsubscribes and flips back to the inactive state", async () => {
    pushApi.getExistingSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });
    pushApi.unsubscribePush.mockImplementation(() => {
      pushApi.getExistingSubscription.mockResolvedValue(null);
      return Promise.resolve();
    });

    render(<PushNotificationSection />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Deaktivieren" }),
    );

    await waitFor(() =>
      expect(pushApi.unsubscribePush).toHaveBeenCalledWith("tenant"),
    );
    expect(
      await screen.findByRole("button", { name: "Aktivieren" }),
    ).toBeInTheDocument();
  });
});
