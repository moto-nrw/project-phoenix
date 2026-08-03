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
}));

describe("ParentSettingsPage", () => {
  it("carries both account settings", async () => {
    render(<ParentSettingsPage />);

    expect(
      await screen.findByRole("heading", { name: "Einstellungen", level: 1 }),
    ).toBeInTheDocument();
    // What the parent chooses to hear about, and on which device — the pair
    // that used to sit unreachable at the bottom of the dashboard (#1671).
    expect(
      await screen.findByRole("heading", { name: "Benachrichtigungen" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", { name: "Push-Benachrichtigungen" }),
    ).toBeInTheDocument();
  });
});
