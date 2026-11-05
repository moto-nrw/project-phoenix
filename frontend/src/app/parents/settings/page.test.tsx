import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ParentSettingsPage from "./page";
import { ToastProvider } from "~/contexts/ToastContext";

// The shared error path shows failures through the toast provider (#2517).
function renderWithToast(
  ui: Parameters<typeof render>[0],
  options?: Parameters<typeof render>[1],
) {
  return render(ui, { wrapper: ToastProvider, ...options });
}

let mockSearch = "";
vi.mock("next/navigation", () => ({
  usePathname: () => "/settings",
  useSearchParams: () => new URLSearchParams(mockSearch),
}));

// The two cards fetch on mount; this page only has to place them, so the
// network is stubbed out rather than exercised (they have their own tests).
vi.mock("~/lib/notification-preferences-api", () => ({
  fetchNotificationPreferences: vi.fn().mockResolvedValue({
    tenant_enabled: true,
    types: [],
  }),
  setNotificationPreference: vi.fn(),
  disableAllNotificationPreferences: vi.fn(),
}));

vi.mock("~/lib/push-api", () => ({
  isPushConfigurationMissing: vi.fn().mockReturnValue(false),
  isPushSupported: vi.fn().mockReturnValue(false),
  isStandaloneApp: vi.fn().mockReturnValue(false),
  needsIOSInstall: vi.fn().mockReturnValue(false),
  syncExistingPushSubscription: vi.fn().mockResolvedValue(undefined),
  subscribePush: vi.fn().mockResolvedValue(undefined),
  unsubscribePush: vi.fn().mockResolvedValue(undefined),
  verifyPushConfiguration: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("~/components/parent/language-switcher", () => ({
  LanguageSwitcher: () => <div data-testid="language-switcher" />,
}));

describe("ParentSettingsPage", () => {
  beforeEach(() => {
    mockSearch = "";
  });

  it("opens the notifications without ?tab= (#5750)", async () => {
    renderWithToast(<ParentSettingsPage />);

    // What the parent chooses to hear about, and on which device — the pair
    // that used to sit unreachable at the bottom of the dashboard (#1671).
    expect(
      await screen.findByRole("heading", {
        name: "Benachrichtigungen",
        level: 1,
      }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", {
        name: "Benachrichtigungen auf diesem Gerät",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("language-switcher")).not.toBeInTheDocument();
  });

  it("carries the language in its own section (#5750)", async () => {
    mockSearch = "tab=language";
    renderWithToast(<ParentSettingsPage />);

    expect(
      await screen.findByRole("heading", { name: "Sprache", level: 1 }),
    ).toBeInTheDocument();
    const languageSwitcher = screen.getByTestId("language-switcher");
    expect(languageSwitcher.parentElement).toHaveClass("ms-auto");
    expect(
      document.querySelector("[data-parent-tour-install-app]"),
    ).not.toBeInTheDocument();
  });

  it("offers app installation separately from notifications", async () => {
    renderWithToast(<ParentSettingsPage />);

    const heading = await screen.findByRole("heading", {
      name: "moto als App hinzufügen",
    });
    expect(
      screen.getByText(
        "Fügen Sie moto zum Startbildschirm Ihres Handys hinzu. Dann öffnen Sie moto wie eine App.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Anleitung öffnen" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("/help/moto-als-app-fuer-eltern"),
    );
    expect(document.querySelector("[data-parent-tour-install-app]")).toBe(
      heading.closest("[data-parent-tour-install-app]"),
    );
  });
});
