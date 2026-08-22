import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ModalProvider } from "~/components/dashboard/modal-context";
import { ParentNotificationOnboarding } from "./parent-notification-onboarding";

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
  canPromptInstall: mocks.canPromptInstall,
  isInstallationCompleted: mocks.isInstallationCompleted,
  subscribeInstallPrompt: () => () => undefined,
  triggerInstallPrompt: mocks.triggerInstallPrompt,
}));

function renderOnboarding() {
  return render(
    <ModalProvider>
      <ParentNotificationOnboarding accountId="42" />
    </ModalProvider>,
  );
}

describe("ParentNotificationOnboarding", () => {
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
    mocks.canPromptInstall.mockReturnValue(false);
    mocks.isInstallationCompleted.mockReturnValue(false);
    mocks.triggerInstallPrompt.mockResolvedValue("accepted");
    mocks.setPreference.mockResolvedValue(undefined);
    mocks.fetchPreferences.mockResolvedValue({
      tenant_enabled: true,
      types: [
        {
          key: "parent_message",
          label: "Neue Nachricht",
          description: "Neue Nachricht der OGS",
          group: "mitteilungen",
          enabled: false,
          available: true,
        },
      ],
    });
  });

  it("enables push and the available notification types together", async () => {
    renderOnboarding();

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Benachrichtigungen aktivieren",
      }),
    );

    await waitFor(() => {
      expect(mocks.subscribePush).toHaveBeenCalledWith("parent");
      expect(mocks.setPreference).toHaveBeenCalledWith(
        "parent_message",
        true,
        "parent",
      );
    });
  });

  it("shows the Home Screen guide on iPhone and iPad", async () => {
    mocks.needsIOSInstall.mockReturnValue(true);
    renderOnboarding();

    expect(
      await screen.findByRole("dialog", {
        name: "moto zum Home-Bildschirm hinzufügen",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText(/auf Teilen/)).toBeInTheDocument();
    expect(mocks.subscribePush).not.toHaveBeenCalled();
  });

  it("installs the app on Android before enabling notifications", async () => {
    mocks.isAndroidDevice.mockReturnValue(true);
    mocks.canPromptInstall.mockReturnValue(true);
    renderOnboarding();

    expect(
      await screen.findByRole("dialog", { name: "moto als App installieren" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "App installieren" }));

    await waitFor(() =>
      expect(mocks.triggerInstallPrompt).toHaveBeenCalledOnce(),
    );
    expect(
      await screen.findByRole("button", {
        name: "Benachrichtigungen aktivieren",
      }),
    ).toBeInTheDocument();
  });

  it("shows Android browser-menu instructions without a one-tap prompt", async () => {
    mocks.isAndroidDevice.mockReturnValue(true);
    renderOnboarding();

    expect(
      await screen.findByText(/Öffnen Sie das Browser-Menü/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "App installieren" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anleitung schließen" }),
    ).toBeInTheDocument();
  });

  it("führt in nummerierten Schritten zum Home-Bildschirm", async () => {
    mocks.needsIOSInstall.mockReturnValue(true);
    renderOnboarding();

    const steps = await screen.findAllByRole("listitem");
    expect(steps).toHaveLength(3);
    expect(steps[0]).toHaveTextContent("1");
    expect(steps[0]).toHaveTextContent(/auf Teilen/);
    expect(steps[1]).toHaveTextContent("2");
    expect(steps[1]).toHaveTextContent(/Zum Home-Bildschirm/);
    expect(steps[2]).toHaveTextContent("3");
    expect(steps[2]).toHaveTextContent(/Hinzufügen/);
  });

  it("bietet im Anleitungsmodus eine eindeutige Schließen-Aktion", async () => {
    mocks.needsIOSInstall.mockReturnValue(true);
    renderOnboarding();

    expect(
      await screen.findByRole("button", { name: "Anleitung schließen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Später erinnern" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Benachrichtigungen aktivieren" }),
    ).not.toBeInTheDocument();
  });

  it("markiert eine geschlossene Installationsanleitung nicht als erledigt", async () => {
    mocks.needsIOSInstall.mockReturnValue(true);
    renderOnboarding();

    fireEvent.click(
      await screen.findByRole("button", { name: "Anleitung schließen" }),
    );

    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    const stored = JSON.parse(
      localStorage.getItem("moto.parent.notification-setup.v1.42") ?? "{}",
    ) as { remindAfter?: number; done?: boolean };
    expect(stored.remindAfter).toBeGreaterThan(Date.now());
    expect(stored.done).toBeUndefined();
  });

  it("erinnert später erneut, wenn die Einrichtung vertagt wird", async () => {
    renderOnboarding();

    fireEvent.click(
      await screen.findByRole("button", { name: "Später erinnern" }),
    );

    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    const stored = JSON.parse(
      localStorage.getItem("moto.parent.notification-setup.v1.42") ?? "{}",
    ) as { remindAfter?: number; done?: boolean };
    expect(stored.remindAfter).toBeGreaterThan(Date.now());
    expect(stored.done).toBeUndefined();
  });

  it("does not promise notifications when the school disabled them", async () => {
    mocks.fetchPreferences.mockResolvedValue({
      tenant_enabled: false,
      types: [
        {
          key: "parent_message",
          label: "Neue Nachricht",
          description: "Neue Nachricht der OGS",
          group: "mitteilungen",
          enabled: false,
          available: true,
        },
      ],
    });

    renderOnboarding();

    await waitFor(() => expect(mocks.fetchPreferences).toHaveBeenCalled());
    expect(
      screen.queryByRole("button", {
        name: "Benachrichtigungen aktivieren",
      }),
    ).not.toBeInTheDocument();
    expect(mocks.subscribePush).not.toHaveBeenCalled();
  });
});
