import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PushNotificationSection } from "./push-notification-section";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

// The shared error path shows failures through the toast provider (#2517).
function renderWithToast(
  ui: Parameters<typeof render>[0],
  options?: Parameters<typeof render>[1],
) {
  return render(ui, { wrapper: ToastProvider, ...options });
}

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

// Testversand und Neustart der Einrichtung liegen seit dem Mobil-Fix im
// Kebab-Menü des Kartenkopfs, damit drei Textknöpfe die Karte auf dem Telefon
// nicht mehr sprengen.
async function openSecondaryMenu() {
  fireEvent.click(
    await screen.findByRole("button", { name: "Weitere Aktionen" }),
  );
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

    renderWithToast(<PushNotificationSection portal="parent" />);

    expect(
      screen.getByTestId("push-notification-skeleton"),
    ).toBeInTheDocument();
  });

  it("shows the enable button when push is supported and not subscribed", async () => {
    renderWithToast(<PushNotificationSection />);
    expect(
      await screen.findByRole("button", {
        name: "Benachrichtigungen einschalten",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/moto informiert Sie über wichtige Neuigkeiten/),
    ).toHaveClass("text-sm", "leading-6");
  });

  it("shows the iOS install hint before checking generic push support", async () => {
    pushApi.needsIOSInstall.mockReturnValue(true);
    pushApi.isPushSupported.mockReturnValue(false);
    renderWithToast(<PushNotificationSection />);
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
    renderWithToast(<PushNotificationSection />);
    expect(
      await screen.findByText(
        "Öffnen Sie moto in Safari, Chrome, Edge oder Firefox und versuchen Sie es dort erneut.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("moto als App geöffnet")).not.toBeInTheDocument();
    expect(
      screen.queryByText("moto darf Sie benachrichtigen"),
    ).not.toBeInTheDocument();
  });

  it("guides parent users through Android installation before push", async () => {
    pwaInstall.isAndroidDevice.mockReturnValue(true);
    pwaInstall.canPromptInstall.mockReturnValue(true);

    renderWithToast(<PushNotificationSection portal="parent" />);

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

    renderWithToast(<PushNotificationSection portal="parent" />);

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

    renderWithToast(<PushNotificationSection portal="parent" />);

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

    renderWithToast(<PushNotificationSection />);

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

  it("marks unavailable parent notifications for the first-steps tour", async () => {
    pushApi.syncExistingPushSubscription.mockRejectedValue(
      new Error("web push is not configured"),
    );
    pushApi.isPushConfigurationMissing.mockReturnValue(true);

    const { container } = renderWithToast(
      <PushNotificationSection portal="parent" />,
    );

    await waitFor(() =>
      expect(pushApi.syncExistingPushSubscription).toHaveBeenCalled(),
    );
    expect(
      container.querySelector(
        '[data-parent-tour="notification-device-unavailable"]',
      ),
    ).toBeInTheDocument();
  });

  it("hides the iOS guide when VAPID is not configured", async () => {
    pushApi.needsIOSInstall.mockReturnValue(true);
    pushApi.verifyPushConfiguration.mockRejectedValue(
      new Error("web push is not configured"),
    );
    pushApi.isPushConfigurationMissing.mockReturnValue(true);

    renderWithToast(<PushNotificationSection />);

    await waitFor(() =>
      expect(pushApi.verifyPushConfiguration).toHaveBeenCalledWith("tenant"),
    );
    expect(screen.queryByText(/Zum Home-Bildschirm/)).not.toBeInTheDocument();
  });

  it("shows the blocked message when permission is denied", async () => {
    stubNotificationPermission("denied");
    renderWithToast(<PushNotificationSection />);
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

    renderWithToast(<PushNotificationSection portal="tenant" />);
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
    renderWithToast(<PushNotificationSection portal="parent" />);
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
    renderWithToast(<PushNotificationSection />);
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

    renderWithToast(<PushNotificationSection />);
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

  it("shows a failed parent unsubscribe on the shared path with retry", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });
    pushApi.unsubscribePush
      .mockRejectedValueOnce(
        new ApiError("diag", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue(undefined);

    renderWithToast(<PushNotificationSection portal="parent" />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Benachrichtigungen ausschalten",
      }),
    );

    expect(
      await screen.findByText(
        catalogText(
          "general.unavailable",
          "das Ausschalten der Benachrichtigungen",
        ),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() =>
      expect(pushApi.unsubscribePush).toHaveBeenCalledTimes(2),
    );
    expect(pushApi.unsubscribePush).toHaveBeenLastCalledWith("parent");
  });

  it("offers a test notification only for an active tenant subscription", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });

    const { rerender } = renderWithToast(
      <PushNotificationSection portal="tenant" />,
    );
    await openSecondaryMenu();
    expect(
      screen.getByRole("menuitem", { name: "Testbenachrichtigung senden" }),
    ).toBeInTheDocument();
    // Der Kartenkopf trägt nur noch eine Schaltfläche plus Menü; die
    // Testaktion ist kein zweiter Textknopf mehr.
    expect(
      screen.queryByRole("button", { name: "Testbenachrichtigung senden" }),
    ).not.toBeInTheDocument();

    rerender(<PushNotificationSection portal="parent" />);
    await openSecondaryMenu();
    await waitFor(() =>
      expect(
        screen.queryByRole("menuitem", {
          name: "Testbenachrichtigung senden",
        }),
      ).not.toBeInTheDocument(),
    );
  });

  it("sends a test notification and confirms success", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });

    renderWithToast(<PushNotificationSection />);
    await openSecondaryMenu();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Testbenachrichtigung senden" }),
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

    renderWithToast(<PushNotificationSection />);
    await openSecondaryMenu();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Testbenachrichtigung senden" }),
    );

    // Der Ladezustand steckte früher im Knopf. Im Menü kann er das nicht,
    // deshalb nennt ihn die Karte selbst.
    expect(await screen.findByText("Wird gesendet …")).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Benachrichtigungen ausschalten",
      }),
    ).toBeDisabled();
    await openSecondaryMenu();
    expect(
      screen.getByRole("menuitem", { name: "Testbenachrichtigung senden" }),
    ).toBeDisabled();

    finishRequest?.();
    await screen.findByText("Testbenachrichtigung wurde gesendet.");
  });

  it("surfaces test notification errors", async () => {
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });
    notificationApi.sendTestNotification.mockRejectedValue(
      new ApiError("disabled", 409, {
        code: "communication.notifications_disabled",
      }),
    );

    renderWithToast(<PushNotificationSection />);
    await openSecondaryMenu();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Testbenachrichtigung senden" }),
    );

    // #2517: the code's own catalog text, as a toast.
    expect(
      await screen.findByText(
        catalogText(
          "communication.notifications_disabled",
          "die Testbenachrichtigung",
        ),
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

    renderWithToast(<PushNotificationSection portal="tenant" />);

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

    renderWithToast(<PushNotificationSection portal="tenant" />);

    expect(await screen.findByText("Blockiert")).toBeInTheDocument();
  });

  it("guides staff through Android installation too, not only parents", async () => {
    pwaInstall.isAndroidDevice.mockReturnValue(true);
    pwaInstall.canPromptInstall.mockReturnValue(true);

    renderWithToast(<PushNotificationSection portal="tenant" />);

    expect(
      await screen.findByRole("button", { name: "App installieren" }),
    ).toBeInTheDocument();
  });

  it("offers installation on a desktop browser that can install", async () => {
    pwaInstall.isDesktopDevice.mockReturnValue(true);
    pwaInstall.canPromptInstall.mockReturnValue(true);

    renderWithToast(<PushNotificationSection portal="tenant" />);

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

    renderWithToast(<PushNotificationSection portal="tenant" />);

    await screen.findByText("moto als App geöffnet");
    expect(
      screen.queryByRole("button", { name: "moto installieren" }),
    ).not.toBeInTheDocument();
  });

  it("restarts the guided setup from the card", async () => {
    renderWithToast(<PushNotificationSection portal="tenant" />);

    await openSecondaryMenu();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Einrichtung erneut starten" }),
    );

    expect(await screen.findByTestId("setup-dialog")).toBeInTheDocument();
  });

  it("hides the restart action without a known account", async () => {
    shellAuth.useShellAuthSafe.mockReturnValue(undefined);

    renderWithToast(<PushNotificationSection portal="tenant" />);

    await screen.findByText("moto als App geöffnet");
    // Ohne Konto bleibt im nicht abonnierten Zustand keine Zweitaktion übrig,
    // also erscheint auch kein Menü.
    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  // Der Fix gegen die aus der Karte ragende Knopfreihe: der Kartenkopf trägt
  // im eingeschalteten Zustand genau eine Schaltfläche plus Menü.
  it("keeps the card head at one action plus menu when push is active", async () => {
    stubNotificationPermission("granted");
    pushApi.syncExistingPushSubscription.mockResolvedValue({
      endpoint: "https://push.example/e",
    });

    renderWithToast(<PushNotificationSection portal="tenant" />);

    const disableButton = await screen.findByRole("button", {
      name: "Benachrichtigungen ausschalten",
    });
    const menuTrigger = screen.getByRole("button", {
      name: "Weitere Aktionen",
    });
    const actionArea = disableButton.parentElement;
    expect(actionArea).not.toBeNull();
    expect(actionArea?.contains(menuTrigger)).toBe(true);
    // Genau diese beiden Bedienelemente stehen im Kartenkopf; alles Weitere
    // liegt hinter dem Menü.
    expect(actionArea?.querySelectorAll("button")).toHaveLength(2);
  });
});
