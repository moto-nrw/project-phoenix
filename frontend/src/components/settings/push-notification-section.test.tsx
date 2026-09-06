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
  isDesktopDevice: vi.fn(),
  isSamsungInternet: vi.fn(),
  isInstallationCompleted: vi.fn(),
  subscribeInstallPrompt: vi.fn(() => () => undefined),
  triggerInstallPrompt: vi.fn(),
}));
const notificationApi = vi.hoisted(() => ({
  sendTestNotification: vi.fn(),
}));
const shellAuth = vi.hoisted(() => ({ useShellAuthSafe: vi.fn() }));

vi.mock("~/lib/push-api", () => pushApi);
vi.mock("~/lib/pwa-install-prompt", () => pwaInstall);
vi.mock("~/lib/notification-api", () => notificationApi);
vi.mock("~/components/notifications/notification-setup-dialog", () => ({
  NotificationSetupDialog: () => <div data-testid="setup-dialog" />,
}));
vi.mock("~/lib/shell-auth-context", () => shellAuth);

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
    pwaInstall.isDesktopDevice.mockReturnValue(false);
    pwaInstall.isSamsungInternet.mockReturnValue(false);
    pwaInstall.isInstallationCompleted.mockReturnValue(false);
    pwaInstall.triggerInstallPrompt.mockResolvedValue("accepted");
    notificationApi.sendTestNotification.mockResolvedValue(undefined);
    shellAuth.useShellAuthSafe.mockReturnValue({
      status: "authenticated",
      user: { id: "42" },
    });
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

  it("allows parent users to enable push directly in Samsung Internet", async () => {
    pwaInstall.isAndroidDevice.mockReturnValue(true);
    pwaInstall.isSamsungInternet.mockReturnValue(true);
    pwaInstall.canPromptInstall.mockReturnValue(true);

    render(<PushNotificationSection portal="parent" />);

    expect(
      await screen.findByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "App installieren" }),
    ).not.toBeInTheDocument();
    expect(pwaInstall.triggerInstallPrompt).not.toHaveBeenCalled();
  });

  it("shows the Android browser-menu fallback when no prompt is available", async () => {
    pwaInstall.isAndroidDevice.mockReturnValue(true);

    render(<PushNotificationSection portal="parent" />);

    // Seit #2831 dieselben nummerierten Schritte wie auf iPhone und iPad
    // statt eines Fließtextes.
    expect(
      await screen.findByText(/Tippen Sie oben rechts im Browser/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Zum Startbildschirm hinzufügen/),
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
    // Die Testaktion steht jetzt als zweite Kartenkopf-Aktion neben
    // "Ausschalten" und trägt daher die Kartenkopf-Höhe (size="md").
    expect(testButton).toHaveClass("rounded-lg", "px-4", "py-2", "text-sm");
    expect(testButton).not.toHaveClass("h-8", "text-xs", "shadow-md");

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
  // #2831: die Karte beantwortet die beiden Fragen, die niemand am Gerät
  // selbst beantworten kann, und startet die Einrichtung neu.
  it("shows install and permission state for this device", async () => {
    stubNotificationPermission("granted");
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });

    render(<PushNotificationSection portal="tenant" />);

    expect(
      await screen.findByText("moto als App geöffnet"),
    ).toBeInTheDocument();
    expect(screen.getByText("Nein")).toBeInTheDocument();
    expect(
      screen.getByText("moto darf Sie benachrichtigen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Ja")).toBeInTheDocument();
  });

  it("marks a blocked browser permission as blocked", async () => {
    stubNotificationPermission("denied");

    render(<PushNotificationSection portal="tenant" />);

    expect(await screen.findByText("Blockiert")).toBeInTheDocument();
  });

  it("guides staff through Android installation too, not only parents", async () => {
    pwaInstall.isAndroidDevice.mockReturnValue(true);
    pwaInstall.canPromptInstall.mockReturnValue(true);

    render(<PushNotificationSection portal="tenant" />);

    expect(
      await screen.findByRole("button", { name: "App installieren" }),
    ).toBeInTheDocument();
  });

  it("offers installation on a desktop browser that can install", async () => {
    pwaInstall.isDesktopDevice.mockReturnValue(true);
    pwaInstall.canPromptInstall.mockReturnValue(true);

    render(<PushNotificationSection portal="tenant" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "moto installieren" }),
    );
    await waitFor(() =>
      expect(pwaInstall.triggerInstallPrompt).toHaveBeenCalledOnce(),
    );
  });

  it("keeps the desktop offer away from an installed app", async () => {
    pwaInstall.isDesktopDevice.mockReturnValue(true);
    pwaInstall.canPromptInstall.mockReturnValue(true);
    pushApi.isStandaloneApp.mockReturnValue(true);

    render(<PushNotificationSection portal="tenant" />);

    await screen.findByText("moto als App geöffnet");
    expect(
      screen.queryByRole("button", { name: "moto installieren" }),
    ).not.toBeInTheDocument();
  });

  it("restarts the guided setup from the card", async () => {
    render(<PushNotificationSection portal="tenant" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Einrichtung erneut starten" }),
    );

    expect(await screen.findByTestId("setup-dialog")).toBeInTheDocument();
  });

  it("hides the restart button without a known account", async () => {
    shellAuth.useShellAuthSafe.mockReturnValue(undefined);

    render(<PushNotificationSection portal="tenant" />);

    await screen.findByText("moto als App geöffnet");
    expect(
      screen.queryByRole("button", { name: "Einrichtung erneut starten" }),
    ).not.toBeInTheDocument();
  });
});
