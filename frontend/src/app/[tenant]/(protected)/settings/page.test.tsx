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
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/settings",
}));

// Die Statuszeile zählt die vom Standard abweichenden Einstellungen.
const mockUseSettingsSchema = vi.fn(() => ({ data: null, isLoading: false }));
vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: () => mockUseSettingsSchema(),
}));

// Mock useSettingsTabs — returns null by default (no access)
const mockUseSettingsTabs = vi.fn();
vi.mock("~/components/settings/settings-page", () => ({
  useSettingsTabs: () => mockUseSettingsTabs(),
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
    mockUseSettingsSchema.mockReturnValue({ data: null, isLoading: false });
  });

  it("should show loading when session is loading", () => {
    mockUseSession.mockReturnValue({ data: null, status: "loading" });

    // Der Ladezustand kommt aus dem Seitengerüst (TenantPage).
    const { container } = render(<SettingsPage />);

    expect(container.querySelector('[aria-busy="true"]')).not.toBeNull();
  });

  it("keeps loading while the settings schema is loading", () => {
    mockUseSettingsSchema.mockReturnValue({ data: null, isLoading: true });

    const { container } = render(<SettingsPage />);

    expect(container.querySelector('[aria-busy="true"]')).not.toBeNull();
    expect(
      screen.queryByText(/Keine Einstellungen verfügbar/),
    ).not.toBeInTheDocument();
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
      // Leerzustand ohne Aktion und Symbol = EIN Satz aus Titel und
      // Beschreibung.
      expect(
        screen.getByText(/Keine Einstellungen verfügbar\./),
      ).toBeInTheDocument();
    });
  });

  it("should render the settings tabs when they are available", async () => {
    mockUseSettingsTabs.mockReturnValue({
      tabs: [
        { id: "settings-operations", label: "Betrieb", icon: "settings" },
        { id: "settings-gdpr", label: "Datenschutz", icon: "settings" },
      ],
      renderTab: () => <div>Tab content</div>,
    });

    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Betrieb" })).toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: "Datenschutz" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Tab content")).toBeInTheDocument();
    });
  });
});
