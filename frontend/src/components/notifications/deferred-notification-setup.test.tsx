import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DeferredNotificationSetup } from "./deferred-notification-setup";
import { setupStorageKey } from "./notification-setup-decision";

const mocks = vi.hoisted(() => ({
  needsIOSInstall: vi.fn(),
  isPushSupported: vi.fn(),
  isStandaloneApp: vi.fn(),
  isAndroidDevice: vi.fn(),
  isSamsungInternet: vi.fn(),
  useTenantSlugSafe: vi.fn(),
  notificationSetupDialog: vi.fn(),
}));

vi.mock("~/lib/push-api", () => ({
  needsIOSInstall: mocks.needsIOSInstall,
  isPushSupported: mocks.isPushSupported,
  isStandaloneApp: mocks.isStandaloneApp,
}));
vi.mock("~/lib/pwa-install-prompt", () => ({
  isAndroidDevice: mocks.isAndroidDevice,
  isSamsungInternet: mocks.isSamsungInternet,
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: mocks.useTenantSlugSafe,
}));
vi.mock("./notification-setup-dialog", () => ({
  NotificationSetupDialog: (props: Record<string, unknown>) => {
    mocks.notificationSetupDialog(props);
    return <div data-testid="setup-dialog" />;
  },
}));

// Wartet, bis ein angestoßener Import des Dialogs geladen und gerendert wäre:
// der Test lädt dasselbe Modul und lässt React die Folge-Updates ausführen.
// Der sichtbare Fall findet den Dialog nach derselben Wartezeit synchron.
async function settleDialogImport() {
  await act(async () => {
    await import("./notification-setup-dialog");
  });
}

describe("DeferredNotificationSetup", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mocks.needsIOSInstall.mockReturnValue(false);
    mocks.isPushSupported.mockReturnValue(true);
    mocks.isStandaloneApp.mockReturnValue(false);
    mocks.isAndroidDevice.mockReturnValue(false);
    mocks.isSamsungInternet.mockReturnValue(false);
    mocks.useTenantSlugSafe.mockReturnValue("school-a");
  });

  it("loads the guided dialog for a supported device without a decision", async () => {
    render(<DeferredNotificationSetup portal="tenant" accountId="42" />);

    expect(await screen.findByTestId("setup-dialog")).toBeInTheDocument();
    expect(mocks.notificationSetupDialog).toHaveBeenCalledWith(
      expect.objectContaining({ portal: "tenant", accountId: "42" }),
    );
  });

  it("does not load the dialog after this device completed setup", async () => {
    localStorage.setItem(
      setupStorageKey("tenant", "42", "school-a"),
      JSON.stringify({ done: true }),
    );

    const { unmount } = render(
      <DeferredNotificationSetup portal="tenant" accountId="42" />,
    );
    await settleDialogImport();

    expect(screen.queryByTestId("setup-dialog")).not.toBeInTheDocument();
    expect(mocks.notificationSetupDialog).not.toHaveBeenCalled();
    unmount();

    localStorage.clear();
    render(<DeferredNotificationSetup portal="tenant" accountId="42" />);
    await settleDialogImport();
    // Anker: ohne abgeschlossene Einrichtung findet dieselbe Abfrage den Dialog
    // nach derselben Wartezeit.
    expect(screen.getByTestId("setup-dialog")).toBeInTheDocument();
  });

  it("does not load the dialog on an unsupported device", async () => {
    mocks.isPushSupported.mockReturnValue(false);

    const { unmount } = render(
      <DeferredNotificationSetup portal="tenant" accountId="42" />,
    );
    await settleDialogImport();

    expect(screen.queryByTestId("setup-dialog")).not.toBeInTheDocument();
    expect(mocks.notificationSetupDialog).not.toHaveBeenCalled();
    unmount();

    mocks.isPushSupported.mockReturnValue(true);
    render(<DeferredNotificationSetup portal="tenant" accountId="42" />);
    await settleDialogImport();
    // Anker: auf einem unterstützten Gerät findet dieselbe Abfrage den Dialog
    // nach derselben Wartezeit.
    expect(screen.getByTestId("setup-dialog")).toBeInTheDocument();
  });
});
