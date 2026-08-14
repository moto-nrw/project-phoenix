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
    expect(screen.getByText(/Teilen-Symbol/)).toBeInTheDocument();
    expect(mocks.subscribePush).not.toHaveBeenCalled();
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
