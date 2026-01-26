import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SystemSettingsPanel } from "./system-settings-panel";
import type { ResolvedSetting } from "~/lib/settings-helpers";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

// Mock settings-api
const mockFetchSystemSettings = vi.fn();
const mockUpdateSystemSetting = vi.fn();
const mockResetSystemSetting = vi.fn();

vi.mock("~/lib/settings-api", () => ({
  fetchSystemSettings: (...args: unknown[]) =>
    mockFetchSystemSettings(...args) as Promise<unknown>,
  updateSystemSetting: (...args: unknown[]) =>
    mockUpdateSystemSetting(...args) as Promise<unknown>,
  resetSystemSetting: (...args: unknown[]) =>
    mockResetSystemSetting(...args) as Promise<unknown>,
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

vi.mock("./setting-history", () => ({
  SettingHistory: ({ title }: { title?: string }) => (
    <div data-testid="setting-history">{title}</div>
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

describe("SystemSettingsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading spinner with purple variant", () => {
    mockFetchSystemSettings.mockReturnValue(new Promise(vi.fn()));
    render(<SystemSettingsPanel />);

    expect(screen.getByTestId("loading-spinner")).toBeInTheDocument();
    expect(screen.getByTestId("loading-spinner")).toHaveAttribute(
      "data-variant",
      "purple",
    );
  });

  it("shows empty state when no settings", async () => {
    mockFetchSystemSettings.mockResolvedValue([]);
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Systemeinstellungen verfügbar."),
      ).toBeInTheDocument();
    });
  });

  it("shows administrator notice", async () => {
    mockFetchSystemSettings.mockResolvedValue([createMockSetting()]);
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(screen.getByText("Administratorbereich")).toBeInTheDocument();
    });
  });

  it("renders settings after loading", async () => {
    mockFetchSystemSettings.mockResolvedValue([
      createMockSetting({
        key: "session.timeout",
        description: "Session Timeout",
      }),
    ]);
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(
        screen.getByTestId("setting-input-session.timeout"),
      ).toBeInTheDocument();
    });
  });

  it("shows category label", async () => {
    mockFetchSystemSettings.mockResolvedValue([
      createMockSetting({ category: "session" }),
    ]);
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(screen.getByText("Sitzung")).toBeInTheDocument();
    });
  });

  it("shows 'Angepasst' badge for non-default settings", async () => {
    mockFetchSystemSettings.mockResolvedValue([
      createMockSetting({ isDefault: false }),
    ]);
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(screen.getByText("Angepasst")).toBeInTheDocument();
    });
  });

  it("shows reset button for non-default modifiable settings", async () => {
    mockFetchSystemSettings.mockResolvedValue([
      createMockSetting({ isDefault: false, canModify: true }),
    ]);
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(
        screen.getByText("Auf Standardwert zurücksetzen"),
      ).toBeInTheDocument();
    });
  });

  it("hides reset button for default settings", async () => {
    mockFetchSystemSettings.mockResolvedValue([
      createMockSetting({ isDefault: true }),
    ]);
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(screen.getByText("Test Setting")).toBeInTheDocument();
    });

    expect(
      screen.queryByText("Auf Standardwert zurücksetzen"),
    ).not.toBeInTheDocument();
  });

  it("shows setting history when showHistory=true", async () => {
    mockFetchSystemSettings.mockResolvedValue([createMockSetting()]);
    render(<SystemSettingsPanel showHistory />);

    await waitFor(() => {
      expect(screen.getByTestId("setting-history")).toBeInTheDocument();
      expect(
        screen.getByText("Letzte Systemänderungen"),
      ).toBeInTheDocument();
    });
  });

  it("hides setting history when showHistory=false", async () => {
    mockFetchSystemSettings.mockResolvedValue([createMockSetting()]);
    render(<SystemSettingsPanel showHistory={false} />);

    await waitFor(() => {
      expect(screen.getByText("Test Setting")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("setting-history")).not.toBeInTheDocument();
  });

  it("shows toast error when fetch fails", async () => {
    vi.spyOn(console, "error").mockImplementation(vi.fn());
    mockFetchSystemSettings.mockRejectedValue(new Error("Failed"));
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Fehler beim Laden der Systemeinstellungen",
      );
    });
  });

  it("shows setting key in monospace", async () => {
    mockFetchSystemSettings.mockResolvedValue([
      createMockSetting({ key: "session.timeout" }),
    ]);
    render(<SystemSettingsPanel />);

    await waitFor(() => {
      expect(screen.getByText("session.timeout")).toBeInTheDocument();
    });
  });
});
