import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ModalProvider } from "~/components/dashboard/modal-context";
import {
  NotificationSetupDialog,
  setupStorageKey,
} from "./notification-setup-dialog";

const mocks = vi.hoisted(() => ({
  fetchPreferences: vi.fn(),
  setPreference: vi.fn(),
  needsIOSInstall: vi.fn(),
  isPushSupported: vi.fn(),
  syncSubscription: vi.fn(),
  subscribePush: vi.fn(),
  verifyPushConfiguration: vi.fn(),
  isStandaloneApp: vi.fn(),
  isAndroidDevice: vi.fn(),
  isSamsungInternet: vi.fn(),
  canPromptInstall: vi.fn(),
  isInstallationCompleted: vi.fn(),
  triggerInstallPrompt: vi.fn(),
}));

vi.mock("~/lib/notification-preferences-api", () => ({
  fetchNotificationPreferences: mocks.fetchPreferences,
  setNotificationPreference: mocks.setPreference,
}));

vi.mock("~/lib/push-api", () => ({
  isPushConfigurationMissing: () => false,
  needsIOSInstall: mocks.needsIOSInstall,
  isPushSupported: mocks.isPushSupported,
  syncExistingPushSubscription: mocks.syncSubscription,
  subscribePush: mocks.subscribePush,
  verifyPushConfiguration: mocks.verifyPushConfiguration,
  isStandaloneApp: mocks.isStandaloneApp,
}));

vi.mock("~/lib/pwa-install-prompt", () => ({
  isAndroidDevice: mocks.isAndroidDevice,
  isSamsungInternet: mocks.isSamsungInternet,
  canPromptInstall: mocks.canPromptInstall,
  isInstallationCompleted: mocks.isInstallationCompleted,
  subscribeInstallPrompt: () => () => undefined,
  triggerInstallPrompt: mocks.triggerInstallPrompt,
}));

function renderDialog(props: {
  portal: "tenant" | "parent" | "school";
  restartToken?: number;
}) {
  return render(
    <ModalProvider>
      <NotificationSetupDialog accountId="42" {...props} />
    </ModalProvider>,
  );
}

describe("NotificationSetupDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.stubGlobal("Notification", { permission: "default" });
    mocks.needsIOSInstall.mockReturnValue(false);
    mocks.isPushSupported.mockReturnValue(true);
    mocks.syncSubscription.mockResolvedValue(null);
    mocks.subscribePush.mockResolvedValue(undefined);
    mocks.verifyPushConfiguration.mockResolvedValue(undefined);
    mocks.isStandaloneApp.mockReturnValue(false);
    mocks.isAndroidDevice.mockReturnValue(false);
    mocks.isSamsungInternet.mockReturnValue(false);
    mocks.canPromptInstall.mockReturnValue(false);
    mocks.isInstallationCompleted.mockReturnValue(false);
    mocks.triggerInstallPrompt.mockResolvedValue("accepted");
    mocks.setPreference.mockResolvedValue(undefined);
    mocks.fetchPreferences.mockResolvedValue({
      tenant_enabled: true,
      types: [
        {
          key: "pickup_upcoming",
          label: "Abholung",
          description: "",
          group: "abholung",
          enabled: false,
          available: true,
        },
      ],
    });
  });

  it("keeps the parents storage key so an earlier decision still counts", () => {
    expect(setupStorageKey("parent", "42")).toBe(
      "moto.parent.notification-setup.v1.42",
    );
  });

  it("asks staff about their own device, not about a child", async () => {
    renderDialog({ portal: "tenant" });

    expect(
      await screen.findByText(
        "Schalten Sie Benachrichtigungen ein. moto meldet sich dann bei wichtigen Ereignissen, auch wenn die App geschlossen ist.",
      ),
    ).toBeInTheDocument();
  });

  it("reads the school portal's own preferences and subscription", async () => {
    renderDialog({ portal: "school" });

    await waitFor(() =>
      expect(mocks.fetchPreferences).toHaveBeenCalledWith("school"),
    );
    expect(mocks.syncSubscription).toHaveBeenCalledWith("school");
  });

  it("explains the home-screen step first on iPhone and iPad", async () => {
    mocks.needsIOSInstall.mockReturnValue(true);

    renderDialog({ portal: "tenant" });

    expect(
      await screen.findByRole("heading", {
        name: "moto zum Home-Bildschirm hinzufügen",
      }),
    ).toBeInTheDocument();
  });

  it("stays closed once this browser finished the setup", async () => {
    localStorage.setItem(
      setupStorageKey("tenant", "42"),
      JSON.stringify({ done: true }),
    );

    renderDialog({ portal: "tenant" });

    await waitFor(() => expect(mocks.fetchPreferences).not.toHaveBeenCalled());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("runs again when the settings card restarts the setup", async () => {
    localStorage.setItem(
      setupStorageKey("tenant", "42"),
      JSON.stringify({ done: true }),
    );

    renderDialog({ portal: "tenant", restartToken: 1 });

    await waitFor(() =>
      expect(mocks.fetchPreferences).toHaveBeenCalledWith("tenant"),
    );
  });
});
