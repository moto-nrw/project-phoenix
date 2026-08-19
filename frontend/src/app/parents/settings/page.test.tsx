import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import ParentSettingsPage from "./page";

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
  it("carries both account settings", async () => {
    render(<ParentSettingsPage />);

    expect(
      await screen.findByRole("heading", { name: "Einstellungen", level: 1 }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", { name: "Sprache" }),
    ).toBeInTheDocument();
    const languageSwitcher = screen.getByTestId("language-switcher");
    const languageHeader = screen
      .getByRole("heading", { name: "Sprache" })
      .closest("section")?.firstElementChild;

    expect(languageHeader).toContainElement(languageSwitcher);
    expect(languageSwitcher.parentElement).toHaveClass("ms-auto");
    // What the parent chooses to hear about, and on which device — the pair
    // that used to sit unreachable at the bottom of the dashboard (#1671).
    expect(
      await screen.findByRole("heading", { name: "Benachrichtigungen" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", {
        name: "Benachrichtigungen auf diesem Gerät",
      }),
    ).toBeInTheDocument();
  });
});
