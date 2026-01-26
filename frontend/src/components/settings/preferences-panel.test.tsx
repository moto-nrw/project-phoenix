import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PreferencesPanel } from "./preferences-panel";
import type { ResolvedSetting } from "~/lib/settings-helpers";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

// Mock settings-api
const mockFetchUserSettings = vi.fn();
const mockUpdateUserSetting = vi.fn();

vi.mock("~/lib/settings-api", () => ({
  fetchUserSettings: (...args: unknown[]) =>
    mockFetchUserSettings(...args) as Promise<unknown>,
  updateUserSetting: (...args: unknown[]) =>
    mockUpdateUserSetting(...args) as Promise<unknown>,
}));

// Mock ToastContext
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
  })),
}));

// Mock sub-components
vi.mock("./setting-input", () => ({
  SettingInput: ({
    setting,
  }: {
    setting: { key: string; description?: string };
  }) => (
    <div data-testid={`setting-input-${setting.key}`}>
      input-{setting.key}
    </div>
  ),
  SettingLoadingSpinner: ({ variant }: { variant?: string }) => (
    <div data-testid="loading-spinner" data-variant={variant}>
      Loading...
    </div>
  ),
}));

function createMockSetting(
  overrides: Partial<ResolvedSetting> = {},
): ResolvedSetting {
  return {
    key: "test.setting",
    value: "test-value",
    type: "string",
    category: "session",
    description: "Test Setting",
    isDefault: true,
    isActive: true,
    canModify: true,
    ...overrides,
  };
}

describe("PreferencesPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading spinner initially", () => {
    mockFetchUserSettings.mockReturnValue(new Promise(vi.fn()));
    render(<PreferencesPanel />);

    expect(screen.getByTestId("loading-spinner")).toBeInTheDocument();
    expect(screen.getByTestId("loading-spinner")).toHaveAttribute(
      "data-variant",
      "gray",
    );
  });

  it("shows empty state when no settings", async () => {
    mockFetchUserSettings.mockResolvedValue([]);
    render(<PreferencesPanel />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Einstellungen verfügbar."),
      ).toBeInTheDocument();
    });
  });

  it("renders settings after loading", async () => {
    mockFetchUserSettings.mockResolvedValue([
      createMockSetting({
        key: "session.timeout",
        description: "Session Timeout",
        category: "session",
      }),
    ]);
    render(<PreferencesPanel />);

    await waitFor(() => {
      expect(
        screen.getByTestId("setting-input-session.timeout"),
      ).toBeInTheDocument();
    });
  });

  it("shows category label", async () => {
    mockFetchUserSettings.mockResolvedValue([
      createMockSetting({ category: "session" }),
    ]);
    render(<PreferencesPanel />);

    await waitFor(() => {
      expect(screen.getByText("Sitzung")).toBeInTheDocument();
    });
  });

  it("shows 'Angepasst' badge for non-default settings", async () => {
    mockFetchUserSettings.mockResolvedValue([
      createMockSetting({ isDefault: false }),
    ]);
    render(<PreferencesPanel />);

    await waitFor(() => {
      expect(screen.getByText("Angepasst")).toBeInTheDocument();
    });
  });

  it("does not show 'Angepasst' badge for default settings", async () => {
    mockFetchUserSettings.mockResolvedValue([
      createMockSetting({ isDefault: true }),
    ]);
    render(<PreferencesPanel />);

    await waitFor(() => {
      expect(screen.getByText("Test Setting")).toBeInTheDocument();
    });

    expect(screen.queryByText("Angepasst")).not.toBeInTheDocument();
  });

  it("shows source label", async () => {
    mockFetchUserSettings.mockResolvedValue([
      createMockSetting({ isDefault: true }),
    ]);
    render(<PreferencesPanel />);

    await waitFor(() => {
      expect(screen.getByText("Standard")).toBeInTheDocument();
    });
  });

  it("shows toast error when fetch fails", async () => {
    vi.spyOn(console, "error").mockImplementation(vi.fn());
    mockFetchUserSettings.mockRejectedValue(new Error("Failed"));
    render(<PreferencesPanel />);

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Fehler beim Laden der Einstellungen",
      );
    });
  });

  it("shows group name when not _ungrouped", async () => {
    mockFetchUserSettings.mockResolvedValue([
      createMockSetting({ groupName: "Timeouts" }),
    ]);
    render(<PreferencesPanel />);

    await waitFor(() => {
      expect(screen.getByText("Timeouts")).toBeInTheDocument();
    });
  });
});
