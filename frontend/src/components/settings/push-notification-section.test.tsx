import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PushNotificationSection } from "./push-notification-section";

const pushApi = vi.hoisted(() => ({
  isPushConfigurationMissing: vi.fn(),
  isPushSupported: vi.fn(),
  isStandaloneApp: vi.fn(),
  needsIOSInstall: vi.fn(),
  syncExistingPushSubscription: vi.fn(),
  subscribePush: vi.fn(),
  unsubscribePush: vi.fn(),
  verifyPushConfiguration: vi.fn(),
}));
const pwaInstall = vi.hoisted(() => ({
  canPromptInstall: vi.fn(),
  isAndroidDevice: vi.fn(),
  isInstallationCompleted: vi.fn(),
  subscribeInstallPrompt: vi.fn(() => () => undefined),
  triggerInstallPrompt: vi.fn(),
}));
const notificationApi = vi.hoisted(() => ({
  sendTestNotification: vi.fn(),
}));

vi.mock("~/lib/push-api", () => pushApi);
vi.mock("~/lib/pwa-install-prompt", () => pwaInstall);
vi.mock("~/lib/notification-api", () => notificationApi);

function stubNotificationPermission(permission: NotificationPermission) {
  vi.stubGlobal("Notification", { permission });
}

describe("PushNotificationSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    pushApi.isPushConfigurationMissing.mockReturnValue(false);
    pushApi.needsIOSInstall.mockReturnValue(false);
    pushApi.isPushSupported.mockReturnValue(true);
    pushApi.isStandaloneApp.mockReturnValue(false);
    pushApi.syncExistingPushSubscription.mockResolvedValue(null);
    pushApi.verifyPushConfiguration.mockResolvedValue(undefined);
    pwaInstall.canPromptInstall.mockReturnValue(false);
    pwaInstall.isAndroidDevice.mockReturnValue(false);
    pwaInstall.isInstallationCompleted.mockReturnValue(false);
    pwaInstall.triggerInstallPrompt.mockResolvedValue("accepted");
    notificationApi.sendTestNotification.mockResolvedValue(undefined);
    stubNotificationPermission("default");
  });

  it("reserves the complete device card while push state is loading", () => {
    pushApi.syncExistingPushSubscription.mockReturnValue(new Promise(() => {}));

    render(<PushNotificationSection portal="parent" />);

    expect(
      screen.getByTestId("push-notification-skeleton"),
    ).toBeInTheDocument();
  });

  it("shows the enable button when push is supported and not subscribed", async () => {
    render(<PushNotificationSection />);
    expect(
      await screen.findByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/moto informiert Sie über wichtige Neuigkeiten/),
    ).toHaveClass("text-sm", "leading-6");
  });

  it("shows the iOS install hint when the push API is missing in a Safari tab", async () => {
    pushApi.needsIOSInstall.mockReturnValue(true);
    render(<PushNotificationSection />);
    expect(await screen.findByText(/Zum Home-Bildschirm/)).toBeInTheDocument();
    expect(screen.getByText(/Auf iPhone und iPad funktionieren/)).toHaveClass(
      "text-sm",
      "leading-6",
    );
    expect(pushApi.verifyPushConfiguration).toHaveBeenCalledWith("tenant");
    expect(
      screen.queryByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    ).not.toBeInTheDocument();
  });

  it("shows the unsupported message on browsers without push", async () => {
    pushApi.isPushSupported.mockReturnValue(false);
    render(<PushNotificationSection />);
    expect(
      await screen.findByText(
        "Öffnen Sie moto in Safari, Chrome, Edge oder Firefox und versuchen Sie es dort erneut.",
      ),
    ).toBeInTheDocument();
  });

  it("guides parent users through Android installation before push", async () => {
    pwaInstall.isAndroidDevice.mockReturnValue(true);
    pwaInstall.canPromptInstall.mockReturnValue(true);

    render(<PushNotificationSection portal="parent" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "App installieren" }),
    );
    await waitFor(() =>
      expect(pwaInstall.triggerInstallPrompt).toHaveBeenCalledOnce(),
    );
    expect(
      await screen.findByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    ).toBeInTheDocument();
  });

  it("shows the Android browser-menu fallback when no prompt is available", async () => {
    pwaInstall.isAndroidDevice.mockReturnValue(true);

    render(<PushNotificationSection portal="parent" />);

    expect(
      await screen.findByText(/Öffnen Sie das Browser-Menü/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "App installieren" }),
    ).not.toBeInTheDocument();
  });

  it("hides the card when VAPID is not configured", async () => {
    pushApi.syncExistingPushSubscription.mockRejectedValue(
      new Error("web push is not configured"),
    );
    pushApi.isPushConfigurationMissing.mockReturnValue(true);

    render(<PushNotificationSection />);

    await waitFor(() =>
      expect(pushApi.syncExistingPushSubscription).toHaveBeenCalled(),
    );
    expect(
      screen.queryByRole("heading", {
        name: "Benachrichtigungen auf diesem Gerät",
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    ).not.toBeInTheDocument();
  });

  it("hides the iOS guide when VAPID is not configured", async () => {
    pushApi.needsIOSInstall.mockReturnValue(true);
    pushApi.verifyPushConfiguration.mockRejectedValue(
      new Error("web push is not configured"),
    );
    pushApi.isPushConfigurationMissing.mockReturnValue(true);

    render(<PushNotificationSection />);

    await waitFor(() =>
      expect(pushApi.verifyPushConfiguration).toHaveBeenCalledWith("tenant"),
    );
    expect(screen.queryByText(/Zum Home-Bildschirm/)).not.toBeInTheDocument();
  });

  it("shows the blocked message when permission is denied", async () => {
    stubNotificationPermission("denied");
    render(<PushNotificationSection />);
    expect(
      await screen.findByText(/Öffnen Sie die Einstellungen Ihres Geräts/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Status erneut prüfen" }),
    ).toBeInTheDocument();
  });

  it("subscribes via the push API and flips to the active state", async () => {
    pushApi.subscribePush.mockImplementation(() => {
      pushApi.syncExistingPushSubscription.mockResolvedValue({
        endpoint: "https://push.example/e",
      });
      return Promise.resolve();
    });

    render(<PushNotificationSection portal="tenant" />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    );

    await waitFor(() =>
      expect(pushApi.subscribePush).toHaveBeenCalledWith("tenant"),
    );
    expect(
      await screen.findByText(
        "Benachrichtigungen sind auf diesem Gerät eingeschaltet.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Benachrichtigungen ausschalten",
      }),
    ).toBeInTheDocument();
  });

  it("passes the parent portal through to the push API", async () => {
    render(<PushNotificationSection portal="parent" />);
    const enableButton = await screen.findByRole("button", {
      name: "Benachrichtigungen einschalten",
    });
    expect(enableButton).toHaveTextContent("Aktivieren");
    expect(enableButton).toHaveClass("rounded-lg", "px-4", "py-2", "text-sm");
    expect(enableButton).not.toHaveClass("min-h-12", "text-[17px]");

    fireEvent.click(enableButton);
    await waitFor(() =>
      expect(pushApi.subscribePush).toHaveBeenCalledWith("parent"),
    );
  });

  it("surfaces subscribe errors", async () => {
    pushApi.subscribePush.mockRejectedValue(
      new Error("Benachrichtigungen wurden nicht erlaubt."),
    );
    render(<PushNotificationSection />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    );
    expect(
      await screen.findByText(
        "Die Benachrichtigungen konnten nicht eingeschaltet werden. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
  });

  it("unsubscribes and flips back to the inactive state", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });
    pushApi.unsubscribePush.mockImplementation(() => {
      pushApi.syncExistingPushSubscription.mockResolvedValue(null);
      return Promise.resolve();
    });

    render(<PushNotificationSection />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Benachrichtigungen ausschalten",
      }),
    );

    await waitFor(() =>
      expect(pushApi.unsubscribePush).toHaveBeenCalledWith("tenant"),
    );
    expect(
      await screen.findByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    ).toBeInTheDocument();
  });

  it("offers a test notification only for an active tenant subscription", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });

    const { rerender } = render(<PushNotificationSection portal="tenant" />);
    const testButton = await screen.findByRole("button", {
      name: "Testbenachrichtigung senden",
    });
    expect(testButton).toHaveClass("h-8", "text-xs", "bg-transparent");
    expect(testButton).not.toHaveClass("ring-1", "shadow-md");

    rerender(<PushNotificationSection portal="parent" />);
    await waitFor(() =>
      expect(
        screen.queryByRole("button", {
          name: "Testbenachrichtigung senden",
        }),
      ).not.toBeInTheDocument(),
    );
  });

  it("sends a test notification and confirms success", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });

    render(<PushNotificationSection />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Testbenachrichtigung senden",
      }),
    );

    await waitFor(() =>
      expect(notificationApi.sendTestNotification).toHaveBeenCalledOnce(),
    );
    expect(
      await screen.findByText("Testbenachrichtigung wurde gesendet."),
    ).toBeInTheDocument();
  });

  it("disables competing actions while the test request is pending", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });
    let finishRequest: (() => void) | undefined;
    notificationApi.sendTestNotification.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          finishRequest = resolve;
        }),
    );

    render(<PushNotificationSection />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Testbenachrichtigung senden",
      }),
    );

    expect(
      await screen.findByRole("button", { name: "Wird gesendet …" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", {
        name: "Benachrichtigungen ausschalten",
      }),
    ).toBeDisabled();

    finishRequest?.();
    await screen.findByText("Testbenachrichtigung wurde gesendet.");
  });

  it("surfaces test notification errors", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });
    notificationApi.sendTestNotification.mockRejectedValue(
      new Error("Ihre Schule hat Benachrichtigungen derzeit deaktiviert."),
    );

    render(<PushNotificationSection />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Testbenachrichtigung senden",
      }),
    );

    expect(
      await screen.findByText(
        "Ihre Schule hat Benachrichtigungen derzeit deaktiviert.",
      ),
    ).toBeInTheDocument();
  });
});
