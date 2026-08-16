/* eslint-disable @typescript-eslint/no-unsafe-return */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import SettingsPage from "./page";

// Mock next-auth
const mockUseSession = vi.fn();
vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

// Mock next/navigation
const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  redirect: (url: string) => mockRedirect(url),
}));

// Mock useSettingsTabs — returns null by default (no access)
const mockUseSettingsTabs = vi.fn();
vi.mock("~/components/settings/settings-page", () => ({
  useSettingsTabs: () => mockUseSettingsTabs(),
}));

vi.mock("~/components/shared/settings-layout", () => ({
  SettingsLayout: ({ tabs }: { tabs: { id: string; label: string }[] }) => (
    <div data-testid="settings-layout">
      {tabs.map((t) => (
        <span key={t.id}>{t.label}</span>
      ))}
    </div>
  ),
}));

describe("SettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          name: "Admin User",
          email: "admin@example.com",
          token: "test-token",
        },
      },
      status: "authenticated",
    });

    mockUseSettingsTabs.mockReturnValue(null);
  });

  it("should show loading when session is loading", () => {
    mockUseSession.mockReturnValue({ data: null, status: "loading" });

    render(<SettingsPage />);

    expect(
      screen.getByLabelText("Einstellungen werden geladen…"),
    ).toBeInTheDocument();
  });

  it("should redirect when unauthenticated", () => {
    mockUseSession.mockReturnValue({ data: null, status: "unauthenticated" });

    render(<SettingsPage />);

    expect(mockRedirect).toHaveBeenCalledWith("/");
  });

  it("should show empty state when no settings tabs available", async () => {
    mockUseSettingsTabs.mockReturnValue(null);

    render(<SettingsPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Einstellungen verfügbar."),
      ).toBeInTheDocument();
    });
  });

  it("should render settings layout when tabs are available", async () => {
    mockUseSettingsTabs.mockReturnValue({
      tabs: [
        { id: "settings-operations", label: "Betrieb", icon: "settings" },
        { id: "settings-gdpr", label: "Datenschutz", icon: "settings" },
      ],
      renderTab: () => <div>Tab content</div>,
    });

    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("settings-layout")).toBeInTheDocument();
      expect(screen.getByText("Betrieb")).toBeInTheDocument();
      expect(screen.getByText("Datenschutz")).toBeInTheDocument();
    });
  });
});
